package attendance

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/omanjaya/nouschool/internal/platform/clock"
	"github.com/omanjaya/nouschool/internal/platform/httpx"
	"github.com/omanjaya/nouschool/internal/platform/reqctx"
)

// -- consumer-side interfaces (lihat CLAUDE.md: antar-modul lewat interface
// kecil dideklarasikan di sisi PEMAKAI, di-inject lewat constructor di
// main.go). Semua signature sengaja hanya memakai tipe primitif supaya
// *identity.Service, *tenant.Service, dan *student.Service memenuhi interface
// ini secara struktural TANPA attendance perlu mengimpor package-package itu.

// IdentityGateway adalah kebutuhan modul attendance dari modul identity:
// gerbang permission untuk endpoint dengan otorisasi campuran (attendance:write
// ATAU attendance:report) dan audit log.
type IdentityGateway interface {
	HasPermission(role, perm string) bool
	Log(ctx context.Context, schoolID, userID *int64, action, entity string, entityID *int64, oldValue, newValue any) error
}

// AcademicYearLookup adalah kebutuhan modul attendance dari modul tenant.
type AcademicYearLookup interface {
	ActiveAcademicYearID(ctx context.Context, schoolID int64) (id int64, ok bool, err error)
}

// StudentAccess adalah kebutuhan modul attendance dari modul student:
// object-level check (siswa boleh lihat dirinya sendiri, orang tua boleh
// lihat anaknya) untuk GET /api/students/{id}/attendance — CanViewStudent;
// dan (Fase 8) resolve profil siswa (id+rombel) dari user login untuk self
// check-in GPS — MyStudentID. Dipenuhi *student.Service secara struktural
// (method ditambahkan khusus untuk kebutuhan ini — lihat
// internal/student/service.go).
type StudentAccess interface {
	CanViewStudent(ctx context.Context, userID int64, role string, schoolID, studentID int64) error
	MyStudentID(ctx context.Context, schoolID, userID int64) (studentID, classID int64, ok bool, err error)
	// GuardianUserIDs (fase 9, docs/08-notification.md) — resolve penerima
	// notifikasi "attendance.student_absent". Dipenuhi *student.Service
	// secara struktural (method GuardianUserIDs, lihat internal/student/service.go).
	GuardianUserIDs(ctx context.Context, schoolID, studentID int64) ([]int64, error)
}

// Notifier adalah kebutuhan modul attendance dari modul notification (fase 9,
// docs/08-notification.md) — consumer-side interface kecil dideklarasikan di
// sisi PEMAKAI (lihat CLAUDE.md). Dipenuhi *notification.Service secara
// struktural lewat method Notify (signature primitif, lihat
// internal/notification/service.go).
type Notifier interface {
	Notify(ctx context.Context, schoolID int64, event string, userIDs []int64, data map[string]any) error
}

// EventStudentAbsent — nilai HARUS sama persis dengan
// notification.EventAttendanceStudentAbsent (didefinisikan ulang di sini
// karena attendance TIDAK boleh mengimpor notification, lihat CLAUDE.md).
const EventStudentAbsent = "attendance.student_absent"

// Realtime adalah kebutuhan modul attendance dari modul realtime (Fase 12,
// bus event WebSocket read-only server->client) — consumer-side interface
// kecil dideklarasikan di sisi PEMAKAI (lihat CLAUDE.md). TIDAK dipenuhi
// *realtime.Hub secara langsung (signature Hub.Publish beda, pakai
// realtime.Event) — dijembatani adapter kecil di cmd/server
// (realtimeadapter.go, pola sama scheduleadapter.go).
type Realtime interface {
	Publish(schoolID int64, eventType string, data map[string]any)
	PublishTo(schoolID int64, eventType string, data map[string]any, roles []string, userIDs []int64)
}

// SlotToday adalah potongan data slot jadwal yang dibutuhkan attendance dari
// modul schedule (fase 6, docs/06-teaching.md "Absensi per-mapel untuk guru")
// — primitif saja (lihat catatan interface teaching.ScheduleGateway,
// dijembatani lewat adapter kecil di cmd/server/main.go).
type SlotToday struct {
	ID          int64
	ClassID     int64
	ClassName   string
	SubjectID   int64
	SubjectCode string
	SubjectName string
	PeriodStart int
	PeriodEnd   int
	StartsAt    string
	EndsAt      string
	IsNow       bool
}

// ScheduleSlotLookup adalah kebutuhan modul attendance dari modul schedule:
// kepemilikan slot (validasi "guru hanya boleh buka sesi utk slotnya
// sendiri") & daftar slot jadwal guru hari ini (GET /api/attendance/slots-today).
type ScheduleSlotLookup interface {
	SlotOwnership(ctx context.Context, schoolID, slotID int64) (classID, teacherID int64, ok bool, err error)
	SlotsTodayForTeacher(ctx context.Context, schoolID, teacherID int64, at time.Time) ([]SlotToday, error)
}

// TeacherLookup adalah kebutuhan modul attendance dari modul student:
// resolve profil guru dari user login (dipenuhi *student.Service secara
// struktural lewat method MyTeacherID, sudah ada sejak fase 5).
type TeacherLookup interface {
	MyTeacherID(ctx context.Context, schoolID, userID int64) (teacherID int64, ok bool, err error)
}

// Permission kanonik dipakai modul attendance (nilai HARUS sama persis
// dengan docs/02-identity.md — didefinisikan ulang di sini karena attendance
// tidak boleh mengimpor identity).
const (
	PermAttendanceWrite     = "attendance:write"
	PermAttendanceReport    = "attendance:report"
	PermAttendanceSelfCheck = "attendance:self_checkin"
	// PermStudentManage — QR kartu siswa (Fase 8) digerbang perm modul
	// student (mengelola kartu = mengelola data siswa), bukan perm baru.
	PermStudentManage = "student:manage"
)

// ErrNoActiveAcademicYear — sekolah belum mengaktifkan tahun ajaran.
var ErrNoActiveAcademicYear = httpx.Validation("Sekolah belum punya tahun ajaran aktif. Aktifkan dulu di menu tahun ajaran.")

// ErrSessionLocked — sesi sudah difinalisasi, hanya admin_sekolah yang boleh mengubah.
var ErrSessionLocked = &httpx.Error{Status: http.StatusForbidden, Code: "session_locked", Message: "Sesi sudah dikunci."}

// Service berisi aturan bisnis modul attendance.
type Service struct {
	repo          attendanceRepository
	identity      IdentityGateway
	years         AcademicYearLookup
	students      StudentAccess
	scheduleSlots ScheduleSlotLookup
	teacherLookup TeacherLookup
	clock         clock.Clock
	notifier      Notifier // opsional — nil = notifikasi dilewati (lihat SetNotifier)
	realtime      Realtime // opsional — nil = event realtime dilewati (lihat SetRealtime)
}

// SetNotifier menyuntikkan Notifier SETELAH konstruksi (opsional, disuntik
// main.go — lihat cmd/server/main.go). Dipakai setter, bukan parameter
// constructor, supaya call site test yang sudah ada (fake repo/students
// tanpa notifikasi) tidak perlu diubah — nil aman (notifyAbsentIfNeeded
// no-op bila notifier belum diset).
func (s *Service) SetNotifier(n Notifier) { s.notifier = n }

// SetRealtime menyuntikkan Realtime SETELAH konstruksi (opsional, disuntik
// main.go — pola yang sama dengan SetNotifier, supaya call site test yang
// sudah ada tidak perlu diubah; nil aman/no-op — lihat emitSession/emitSummary).
func (s *Service) SetRealtime(r Realtime) { s.realtime = r }

// emitSession memancarkan "attendance.session" {session_id, class_id, date}
// broadcast satu sekolah, DAN "attendance.summary" {date} ke roles
// admin_sekolah/kepala_sekolah/display (docs tugas Fase 12) — dipanggil
// SETELAH operasi sukses (create session/PUT records/scan/self-checkin/
// finalize), best-effort (nil-safe, tidak pernah mengembalikan error).
func (s *Service) emitSession(schoolID, sessionID, classID int64, date time.Time) {
	if s.realtime == nil {
		return
	}
	dateStr := date.Format("2006-01-02")
	s.realtime.Publish(schoolID, "attendance.session", map[string]any{
		"session_id": sessionID, "class_id": classID, "date": dateStr,
	})
	s.realtime.PublishTo(schoolID, "attendance.summary", map[string]any{"date": dateStr},
		[]string{RoleAdminSekolah, RoleKepalaSekolah, RoleDisplay}, nil)
}

func NewService(repo *Repository, identity IdentityGateway, years AcademicYearLookup, students StudentAccess, scheduleSlots ScheduleSlotLookup, teacherLookup TeacherLookup, clk clock.Clock) *Service {
	if clk == nil {
		clk = clock.System{}
	}
	return &Service{repo: repo, identity: identity, years: years, students: students, scheduleSlots: scheduleSlots, teacherLookup: teacherLookup, clock: clk}
}

// newServiceForTest membangun Service dengan repository FAKE (in-memory,
// tanpa DB) — dipakai test di package ini saja (service_test.go).
func newServiceForTest(repo attendanceRepository, identity IdentityGateway, years AcademicYearLookup, students StudentAccess, scheduleSlots ScheduleSlotLookup, teacherLookup TeacherLookup, clk clock.Clock) *Service {
	return &Service{repo: repo, identity: identity, years: years, students: students, scheduleSlots: scheduleSlots, teacherLookup: teacherLookup, clock: clk}
}

func (s *Service) audit(ctx context.Context, schoolID, actorUserID int64, action, entity string, entityID int64, oldValue, newValue any) {
	sid, uid, eid := schoolID, actorUserID, entityID
	_ = s.identity.Log(ctx, &sid, &uid, action, entity, &eid, oldValue, newValue)
}

func (s *Service) requireActiveYear(ctx context.Context, schoolID int64) (int64, error) {
	id, ok, err := s.years.ActiveAcademicYearID(ctx, schoolID)
	if err != nil {
		return 0, err
	}
	if !ok {
		return 0, ErrNoActiveAcademicYear
	}
	return id, nil
}

// -- tanggal lokal sekolah (docs/05: "tanggal absensi SELALU dari waktu lokal
// sekolah, bukan UTC") --

// schoolToday mengembalikan tanggal (tanpa jam) HARI INI menurut zona waktu
// sekolah tz, dari waktu "now" (biasanya s.clock.Now()) — dites dengan
// clock.Fixed di service_test.go (sesi dibuat 23:30 WIB tidak boleh jadi
// tanggal UTC besok).
func schoolToday(now time.Time, tz string) time.Time {
	local := clock.InZone(now, tz)
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.UTC)
}

func parseDateParam(raw string, now time.Time, tz string) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return schoolToday(now, tz), nil
	}
	t, err := time.Parse("2006-01-02", raw)
	if err != nil {
		return time.Time{}, httpx.Validation("Format tanggal harus YYYY-MM-DD.")
	}
	return t, nil
}

func schoolTimezone(ctx context.Context) string {
	sch, _ := reqctx.SchoolFromContext(ctx)
	return sch.Timezone
}

// -- GET /api/attendance/classes --

func (s *Service) ListClassesForDate(ctx context.Context, schoolID int64, dateStr string) ([]ClassForDate, error) {
	date, err := parseDateParam(dateStr, s.clock.Now(), schoolTimezone(ctx))
	if err != nil {
		return nil, err
	}
	yearID, err := s.requireActiveYear(ctx, schoolID)
	if err != nil {
		return nil, err
	}
	rows, err := s.repo.ListClassesForDate(ctx, schoolID, yearID, date)
	if err != nil {
		return nil, err
	}
	out := make([]ClassForDate, 0, len(rows))
	for _, row := range rows {
		item := ClassForDate{ClassID: row.ClassID, Name: row.Name, StudentCount: row.StudentCount}
		if row.SessionID != 0 {
			item.Session = &ClassAttendanceStatus{ID: row.SessionID, Status: row.SessionStatus, MarkedCount: row.MarkedCount}
		}
		out = append(out, item)
	}
	return out, nil
}

// buildSessionDetail memuat ulang shape lengkap sesi (dipakai CreateSession,
// GetSession, UpdateRecords — response selalu sama, docs/05-attendance.md).
func (s *Service) buildSessionDetail(ctx context.Context, schoolID, sessionID int64) (SessionDetail, error) {
	detail, err := s.repo.GetSessionDetail(ctx, schoolID, sessionID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return SessionDetail{}, httpx.ErrNotFound
		}
		return SessionDetail{}, err
	}
	roster, err := s.repo.ListClassStudents(ctx, schoolID, detail.ClassID)
	if err != nil {
		return SessionDetail{}, err
	}
	records, err := s.repo.ListSessionRecords(ctx, sessionID)
	if err != nil {
		return SessionDetail{}, err
	}
	recByStudent := make(map[int64]RecordRow, len(records))
	for _, r := range records {
		recByStudent[r.StudentID] = r
	}

	students := make([]StudentInSession, 0, len(roster))
	for _, st := range roster {
		item := StudentInSession{StudentID: st.ID, Name: st.Name, NIS: st.NIS}
		if rec, ok := recByStudent[st.ID]; ok {
			item.Record = &RecordView{Status: rec.Status, Note: rec.Note, Method: rec.Method, MarkedAt: rec.MarkedAt}
		}
		students = append(students, item)
	}

	return SessionDetail{
		Session: SessionView{
			ID: detail.ID, ClassID: detail.ClassID, ClassName: detail.ClassName,
			Date: NewDate(detail.Date), Type: detail.Type, Status: detail.Status, OpenedByName: detail.OpenedByName,
		},
		Students: students,
	}, nil
}

// -- POST /api/attendance/sessions --

// NewSessionInput adalah parameter POST /api/attendance/sessions.
// ScheduleSlotID != 0 -> buat sesi type='subject' dari slot jadwal (fase 6,
// docs/06-teaching.md) alih-alih sesi daily dari class_id.
type NewSessionInput struct {
	ClassID        int64
	ScheduleSlotID int64
	Date           string // "" = hari ini (waktu lokal sekolah)
}

// CreateSession membuat-atau-mengambil sesi untuk (class_id, date) mode
// daily, ATAU (schedule_slot_id, date) mode subject — idempoten lewat
// partial unique index masing-masing (docs/05-attendance.md). Percobaan
// kedua pada kunci yang sama TIDAK membuat sesi baru, hanya mengembalikan
// sesi yang sudah ada.
func (s *Service) CreateSession(ctx context.Context, actorUserID, schoolID int64, in NewSessionInput) (SessionDetail, error) {
	date, err := parseDateParam(in.Date, s.clock.Now(), schoolTimezone(ctx))
	if err != nil {
		return SessionDetail{}, err
	}
	yearID, err := s.requireActiveYear(ctx, schoolID)
	if err != nil {
		return SessionDetail{}, err
	}

	if in.ScheduleSlotID != 0 {
		return s.createSubjectSessionFromSlot(ctx, actorUserID, schoolID, yearID, in.ScheduleSlotID, date)
	}
	if in.ClassID == 0 {
		return SessionDetail{}, httpx.Validation("class_id atau schedule_slot_id wajib diisi.")
	}
	cls, err := s.repo.GetClassMeta(ctx, schoolID, in.ClassID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return SessionDetail{}, httpx.Validation("Rombel tidak ditemukan.")
		}
		return SessionDetail{}, err
	}

	sess, err := s.repo.CreateSession(ctx, CreateSessionInput{
		SchoolID: schoolID, AcademicYearID: yearID, ClassID: cls.ID, Date: date, OpenedBy: actorUserID,
	})
	switch {
	case err == nil:
		s.audit(ctx, schoolID, actorUserID, "attendance.session_open", "attendance_session", sess.ID, nil,
			map[string]any{"class_id": cls.ID, "date": date.Format("2006-01-02")})
	case errors.Is(err, ErrConflict):
		sess, err = s.repo.GetSessionByClassDate(ctx, schoolID, cls.ID, date)
		if err != nil {
			return SessionDetail{}, err
		}
	default:
		return SessionDetail{}, err
	}

	s.emitSession(schoolID, sess.ID, cls.ID, date)
	return s.buildSessionDetail(ctx, schoolID, sess.ID)
}

// createSubjectSessionFromSlot — object-level: guru hanya boleh membuka sesi
// utk slot MILIKNYA sendiri kecuali admin_sekolah (docs/06-teaching.md
// "validasi slot milik guru itu kecuali admin").
func (s *Service) createSubjectSessionFromSlot(ctx context.Context, actorUserID, schoolID, yearID, slotID int64, date time.Time) (SessionDetail, error) {
	classID, teacherID, ok, err := s.scheduleSlots.SlotOwnership(ctx, schoolID, slotID)
	if err != nil {
		return SessionDetail{}, err
	}
	if !ok {
		return SessionDetail{}, httpx.Validation("Slot jadwal tidak ditemukan.")
	}

	if reqctx.Role(ctx) != RoleAdminSekolah {
		myTeacherID, ok, err := s.teacherLookup.MyTeacherID(ctx, schoolID, actorUserID)
		if err != nil {
			return SessionDetail{}, err
		}
		if !ok || myTeacherID != teacherID {
			return SessionDetail{}, httpx.ErrForbidden
		}
	}

	sess, err := s.repo.CreateSubjectSession(ctx, CreateSubjectSessionInput{
		SchoolID: schoolID, AcademicYearID: yearID, ClassID: classID, ScheduleSlotID: slotID, Date: date, OpenedBy: actorUserID,
	})
	switch {
	case err == nil:
		s.audit(ctx, schoolID, actorUserID, "attendance.session_open", "attendance_session", sess.ID, nil,
			map[string]any{"schedule_slot_id": slotID, "date": date.Format("2006-01-02")})
	case errors.Is(err, ErrConflict):
		sess, err = s.repo.GetSessionBySlotDate(ctx, schoolID, slotID, date)
		if err != nil {
			return SessionDetail{}, err
		}
	default:
		return SessionDetail{}, err
	}

	s.emitSession(schoolID, sess.ID, classID, date)
	return s.buildSessionDetail(ctx, schoolID, sess.ID)
}

// OpenSubjectSession buka-atau-ambil sesi absen subject utk satu slot —
// interface publik fase 6 dipakai modul teaching lewat consumer-side
// interface AttendanceGateway (docs/06-teaching.md "satu scan: jurnal terisi
// + sesi absen siap"). TIDAK mengecek kepemilikan slot (pemanggil/teaching
// SUDAH memvalidasinya lewat SlotNow(teacherID) sebelum memanggil ini).
func (s *Service) OpenSubjectSession(ctx context.Context, schoolID, actorUserID, scheduleSlotID int64, date string) (int64, error) {
	d, err := time.Parse("2006-01-02", strings.TrimSpace(date))
	if err != nil {
		return 0, httpx.Validation("Format tanggal harus YYYY-MM-DD.")
	}
	yearID, err := s.requireActiveYear(ctx, schoolID)
	if err != nil {
		return 0, err
	}
	classID, _, ok, err := s.scheduleSlots.SlotOwnership(ctx, schoolID, scheduleSlotID)
	if err != nil {
		return 0, err
	}
	if !ok {
		return 0, httpx.Validation("Slot jadwal tidak ditemukan.")
	}

	sess, err := s.repo.CreateSubjectSession(ctx, CreateSubjectSessionInput{
		SchoolID: schoolID, AcademicYearID: yearID, ClassID: classID, ScheduleSlotID: scheduleSlotID, Date: d, OpenedBy: actorUserID,
	})
	switch {
	case err == nil:
		s.audit(ctx, schoolID, actorUserID, "attendance.session_open", "attendance_session", sess.ID, nil,
			map[string]any{"schedule_slot_id": scheduleSlotID, "date": date})
		return sess.ID, nil
	case errors.Is(err, ErrConflict):
		sess, err = s.repo.GetSessionBySlotDate(ctx, schoolID, scheduleSlotID, d)
		if err != nil {
			return 0, err
		}
		return sess.ID, nil
	default:
		return 0, err
	}
}

// -- GET /api/attendance/slots-today --

// SlotsToday — slot jadwal GURU INI hari ini + status sesi subject
// masing-masing (docs/06-teaching.md). Bukan guru (mis. admin memanggil
// endpoint ini) -> daftar kosong, bukan error (endpoint ini murni "layar
// saya", tidak berlaku utk role tanpa profil guru).
func (s *Service) SlotsToday(ctx context.Context, schoolID, actorUserID int64) ([]SlotTodayView, error) {
	teacherID, ok, err := s.teacherLookup.MyTeacherID(ctx, schoolID, actorUserID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return []SlotTodayView{}, nil
	}

	now := s.clock.Now()
	slots, err := s.scheduleSlots.SlotsTodayForTeacher(ctx, schoolID, teacherID, now)
	if err != nil {
		return nil, err
	}
	date := schoolToday(now, schoolTimezone(ctx))

	out := make([]SlotTodayView, 0, len(slots))
	for _, sl := range slots {
		view := SlotTodayView{Slot: SlotTodaySlot{
			ID: sl.ID, Class: SlotClassRef{ID: sl.ClassID, Name: sl.ClassName},
			Subject:     SlotSubjectRef{ID: sl.SubjectID, Code: sl.SubjectCode, Name: sl.SubjectName},
			PeriodStart: sl.PeriodStart, PeriodEnd: sl.PeriodEnd, StartsAt: sl.StartsAt, EndsAt: sl.EndsAt, IsNow: sl.IsNow,
		}}

		sess, serr := s.repo.GetSessionBySlotDate(ctx, schoolID, sl.ID, date)
		switch {
		case serr == nil:
			roster, rerr := s.repo.ListClassStudents(ctx, schoolID, sl.ClassID)
			if rerr != nil {
				return nil, rerr
			}
			marked, merr := s.repo.CountRecordsForSession(ctx, sess.ID)
			if merr != nil {
				return nil, merr
			}
			view.Session = &SlotTodaySession{ID: sess.ID, Status: sess.Status, MarkedCount: marked, Total: int64(len(roster))}
		case errors.Is(serr, ErrNotFound):
			// belum ada sesi -> Session tetap nil.
		default:
			return nil, serr
		}

		out = append(out, view)
	}
	return out, nil
}

// -- GET /api/attendance/sessions/{id} --

// checkSessionAccess menegakkan otorisasi GET sesi: attendance:write ATAU
// attendance:report (docs/05-attendance.md).
func (s *Service) checkSessionAccess(ctx context.Context) error {
	role := reqctx.Role(ctx)
	if s.identity.HasPermission(role, PermAttendanceWrite) || s.identity.HasPermission(role, PermAttendanceReport) {
		return nil
	}
	return httpx.ErrForbidden
}

func (s *Service) GetSession(ctx context.Context, schoolID, sessionID int64) (SessionDetail, error) {
	if err := s.checkSessionAccess(ctx); err != nil {
		return SessionDetail{}, err
	}
	return s.buildSessionDetail(ctx, schoolID, sessionID)
}

// notifyAbsentIfNeeded mengirim notifikasi "attendance.student_absent" ke
// wali siswa (fase 9, docs/08-notification.md) — dipanggil dari UpdateRecords
// (bulk manual), SelfCheckin, dan ScanQRCard (docs tugas fase 9: "setelah
// bulk PUT records / scan / self-checkin"). Aturan dedup (docs tugas):
// kirim HANYA bila status baru != status lama (record baru dihitung sebagai
// "status lama" kosong) DAN status baru BUKAN hadir — supaya guru yang
// mengoreksi bolak-balik tidak memicu notifikasi berulang, dan status hadir
// (termasuk downgrade balik ke hadir) tidak pernah memicu notifikasi sama
// sekali.
func (s *Service) notifyAbsentIfNeeded(ctx context.Context, schoolID, studentID int64, studentName string, date time.Time, oldStatus string, oldExisted bool, newStatus string) {
	if s.notifier == nil || newStatus == StatusHadir {
		return
	}
	if oldExisted && oldStatus == newStatus {
		return
	}
	guardianIDs, err := s.students.GuardianUserIDs(ctx, schoolID, studentID)
	if err != nil || len(guardianIDs) == 0 {
		return
	}
	_ = s.notifier.Notify(ctx, schoolID, EventStudentAbsent, guardianIDs, map[string]any{
		"student": studentName, "date": date.Format("2006-01-02"), "status": newStatus,
	})
}

// -- PUT /api/attendance/sessions/{id}/records --

// RecordInputRequest adalah satu baris body PUT .../records.
type RecordInputRequest struct {
	StudentID int64
	Status    string
	Note      string
}

// UpdateRecords melakukan bulk upsert record absensi dalam satu transaksi.
// Aturan (docs/05-attendance.md):
//   - sesi finalized -> ditolak (403) kecuali admin_sekolah.
//   - melewati EditWindowHours sejak sesi dibuat -> ditolak (403) kecuali admin_sekolah.
//   - perubahan pada record yang SUDAH punya nilai sebelumnya -> audit_log
//     ringkasan "attendance.update" (old/new per siswa yang berubah).
func (s *Service) UpdateRecords(ctx context.Context, actorUserID, schoolID, sessionID int64, records []RecordInputRequest) (SessionDetail, error) {
	role := reqctx.Role(ctx)
	isAdmin := role == RoleAdminSekolah

	sess, err := s.repo.GetSessionByID(ctx, schoolID, sessionID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return SessionDetail{}, httpx.ErrNotFound
		}
		return SessionDetail{}, err
	}

	if sess.Status == SessionFinalized && !isAdmin {
		return SessionDetail{}, ErrSessionLocked
	}

	if !isAdmin {
		settings, err := s.repo.GetSettings(ctx, schoolID)
		if err != nil {
			return SessionDetail{}, err
		}
		if err := checkEditWindow(sess.CreatedAt, s.clock.Now(), settings.EditWindowHours); err != nil {
			return SessionDetail{}, err
		}
	}

	if len(records) == 0 {
		return SessionDetail{}, httpx.Validation("records wajib diisi.")
	}

	roster, err := s.repo.ListClassStudents(ctx, schoolID, sess.ClassID)
	if err != nil {
		return SessionDetail{}, err
	}
	rosterSet := make(map[int64]bool, len(roster))
	rosterName := make(map[int64]string, len(roster))
	for _, st := range roster {
		rosterSet[st.ID] = true
		rosterName[st.ID] = st.Name
	}

	seen := make(map[int64]bool, len(records))
	items := make([]RecordInput, 0, len(records))
	for _, rec := range records {
		if !rosterSet[rec.StudentID] {
			return SessionDetail{}, httpx.Validation("Siswa tidak terdaftar di rombel sesi ini.")
		}
		if seen[rec.StudentID] {
			return SessionDetail{}, httpx.Validation("student_id duplikat dalam records.")
		}
		seen[rec.StudentID] = true
		if !validStatus(rec.Status) {
			return SessionDetail{}, httpx.Validation("Status absensi tidak valid: " + rec.Status)
		}
		items = append(items, RecordInput{StudentID: rec.StudentID, Status: rec.Status, Note: strings.TrimSpace(rec.Note)})
	}

	oldRecords, err := s.repo.ListSessionRecords(ctx, sessionID)
	if err != nil {
		return SessionDetail{}, err
	}
	oldByStudent := make(map[int64]RecordRow, len(oldRecords))
	for _, r := range oldRecords {
		oldByStudent[r.StudentID] = r
	}

	if err := s.repo.BulkUpsertRecords(ctx, schoolID, sessionID, actorUserID, string(MethodManual), items); err != nil {
		return SessionDetail{}, err
	}

	var changedOld, changedNew []map[string]any
	for _, item := range items {
		old, existed := oldByStudent[item.StudentID]
		if existed && (old.Status != item.Status || old.Note != item.Note) {
			changedOld = append(changedOld, map[string]any{"student_id": item.StudentID, "status": old.Status, "note": old.Note})
			changedNew = append(changedNew, map[string]any{"student_id": item.StudentID, "status": item.Status, "note": item.Note})
		}
		s.notifyAbsentIfNeeded(ctx, schoolID, item.StudentID, rosterName[item.StudentID], sess.Date, old.Status, existed, item.Status)
	}
	if len(changedOld) > 0 {
		s.audit(ctx, schoolID, actorUserID, "attendance.update", "attendance_session", sessionID, changedOld, changedNew)
	}

	s.emitSession(schoolID, sessionID, sess.ClassID, sess.Date)
	return s.buildSessionDetail(ctx, schoolID, sessionID)
}

// checkEditWindow adalah fungsi murni (mudah dites tanpa repo/DB — lihat
// service_test.go) yang menegakkan aturan jendela edit (docs/05-attendance.md).
func checkEditWindow(sessionCreatedAt, now time.Time, editWindowHours int) error {
	deadline := sessionCreatedAt.Add(time.Duration(editWindowHours) * time.Hour)
	if now.After(deadline) {
		return &httpx.Error{
			Status: http.StatusForbidden, Code: "edit_window_closed",
			Message: fmt.Sprintf("Jendela edit absensi (%d jam sejak sesi dibuat) sudah lewat. Hanya admin sekolah yang bisa mengubah absensi ini.", editWindowHours),
		}
	}
	return nil
}

// -- POST /api/attendance/sessions/{id}/finalize --

func (s *Service) Finalize(ctx context.Context, actorUserID, schoolID, sessionID int64) (FinalizeResult, error) {
	sess, err := s.repo.GetSessionByID(ctx, schoolID, sessionID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return FinalizeResult{}, httpx.ErrNotFound
		}
		return FinalizeResult{}, err
	}
	if sess.Status == SessionFinalized {
		return FinalizeResult{}, httpx.Validation("Sesi sudah difinalisasi sebelumnya.")
	}

	total, err := s.repo.CountEnrolledForClass(ctx, sess.ClassID)
	if err != nil {
		return FinalizeResult{}, err
	}
	marked, err := s.repo.CountRecordsForSession(ctx, sessionID)
	if err != nil {
		return FinalizeResult{}, err
	}
	unmarked := total - marked
	if unmarked > 0 {
		return FinalizeResult{}, httpx.Validation(fmt.Sprintf("Masih ada %d siswa belum diabsen.", unmarked))
	}

	if _, err := s.repo.FinalizeSession(ctx, schoolID, sessionID); err != nil {
		if errors.Is(err, ErrNotFound) {
			return FinalizeResult{}, httpx.ErrNotFound
		}
		return FinalizeResult{}, err
	}
	s.audit(ctx, schoolID, actorUserID, "attendance.finalize", "attendance_session", sessionID, nil,
		map[string]any{"status": SessionFinalized})

	detail, err := s.repo.GetSessionDetail(ctx, schoolID, sessionID)
	if err != nil {
		return FinalizeResult{}, err
	}
	s.emitSession(schoolID, sessionID, sess.ClassID, sess.Date)
	return FinalizeResult{
		Session: SessionView{
			ID: detail.ID, ClassID: detail.ClassID, ClassName: detail.ClassName,
			Date: NewDate(detail.Date), Type: detail.Type, Status: detail.Status, OpenedByName: detail.OpenedByName,
		},
		UnmarkedCount: unmarked,
	}, nil
}

// -- GET /api/attendance/summary --

func (s *Service) Summary(ctx context.Context, schoolID int64, dateStr string, classID int64) ([]ClassSummary, error) {
	date, err := parseDateParam(dateStr, s.clock.Now(), schoolTimezone(ctx))
	if err != nil {
		return nil, err
	}
	yearID, err := s.requireActiveYear(ctx, schoolID)
	if err != nil {
		return nil, err
	}
	rows, err := s.repo.SummaryByDate(ctx, schoolID, yearID, date, classID)
	if err != nil {
		return nil, err
	}
	out := make([]ClassSummary, 0, len(rows))
	for _, row := range rows {
		marked := row.Hadir + row.Terlambat + row.Izin + row.Sakit + row.Alpa
		out = append(out, ClassSummary{
			ClassID: row.ClassID, ClassName: row.ClassName, Total: row.Total,
			Hadir: row.Hadir, Terlambat: row.Terlambat, Izin: row.Izin, Sakit: row.Sakit, Alpa: row.Alpa,
			Unmarked: row.Total - marked, SessionStatus: row.SessionStatus,
		})
	}
	return out, nil
}

// -- GET /api/students/{id}/attendance --

// StudentHistory — perm attendance:report ATAU object-level (siswa sendiri /
// orang tua anaknya, lewat StudentAccess.CanViewStudent — docs/05-attendance.md).
func (s *Service) StudentHistory(ctx context.Context, schoolID, studentID int64, fromStr, toStr string) (HistoryResult, error) {
	role := reqctx.Role(ctx)
	if !s.identity.HasPermission(role, PermAttendanceReport) {
		userID := reqctx.UserID(ctx)
		if err := s.students.CanViewStudent(ctx, userID, role, schoolID, studentID); err != nil {
			return HistoryResult{}, err
		}
	}

	today := schoolToday(s.clock.Now(), schoolTimezone(ctx))
	to := today
	if strings.TrimSpace(toStr) != "" {
		t, err := time.Parse("2006-01-02", strings.TrimSpace(toStr))
		if err != nil {
			return HistoryResult{}, httpx.Validation("Format 'to' harus YYYY-MM-DD.")
		}
		to = t
	}
	from := to.AddDate(0, 0, -30)
	if strings.TrimSpace(fromStr) != "" {
		t, err := time.Parse("2006-01-02", strings.TrimSpace(fromStr))
		if err != nil {
			return HistoryResult{}, httpx.Validation("Format 'from' harus YYYY-MM-DD.")
		}
		from = t
	}
	if from.After(to) {
		return HistoryResult{}, httpx.Validation("'from' tidak boleh setelah 'to'.")
	}

	rows, err := s.repo.StudentHistory(ctx, schoolID, studentID, from, to)
	if err != nil {
		return HistoryResult{}, err
	}
	counts, err := s.repo.StudentCounts(ctx, schoolID, studentID, from, to)
	if err != nil {
		return HistoryResult{}, err
	}

	items := make([]HistoryItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, HistoryItem{Date: NewDate(r.Date), Status: r.Status, Note: r.Note})
	}
	return HistoryResult{
		Counts: HistoryCounts{Hadir: counts.Hadir, Terlambat: counts.Terlambat, Izin: counts.Izin, Sakit: counts.Sakit, Alpa: counts.Alpa},
		Items:  items,
	}, nil
}

// -- QR kartu siswa (Fase 8, docs/05-attendance.md "QR kartu siswa") --

// qrCardTokenBytes — 18 byte acak -> base64url TANPA padding (RawURLEncoding)
// = tepat 24 karakter (18*4/3), sesuai spesifikasi kartu siswa.
const qrCardTokenBytes = 18

func randomQRCardToken() (string, error) {
	b := make([]byte, qrCardTokenBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// normalizeQRToken melucuti awalan "nouschool:student:" bila ada — endpoint
// scan menerima isi QR MENTAH (dengan awalan) maupun token polos (docs/05).
func normalizeQRToken(raw string) string {
	return strings.TrimPrefix(strings.TrimSpace(raw), qrCardPrefix)
}

// ErrQRUnknown — token tidak dikenal ATAU sudah dicabut (docs/05-attendance.md).
var ErrQRUnknown = &httpx.Error{Status: http.StatusNotFound, Code: "qr_unknown", Message: "Kartu tidak dikenal atau sudah dicabut."}

// ErrQRNotIssued — siswa belum pernah digenerate kartu QR-nya.
var ErrQRNotIssued = &httpx.Error{Status: http.StatusNotFound, Code: "qr_not_issued", Message: "Token QR untuk siswa ini belum dibuat. Generate kartu dulu."}

func classForQRCards(ctx context.Context, s *Service, schoolID, classID int64) error {
	if classID == 0 {
		return httpx.Validation("class_id wajib diisi.")
	}
	if _, err := s.repo.GetClassMeta(ctx, schoolID, classID); err != nil {
		if errors.Is(err, ErrNotFound) {
			return httpx.Validation("Rombel tidak ditemukan.")
		}
		return err
	}
	return nil
}

// ListQRCards — GET /api/attendance/qr-cards?class_id= (baca saja, tanpa
// menerbitkan token baru).
func (s *Service) ListQRCards(ctx context.Context, schoolID, classID int64) ([]QRCardView, error) {
	if err := classForQRCards(ctx, s, schoolID, classID); err != nil {
		return nil, err
	}
	rows, err := s.repo.ListClassStudentsWithActiveToken(ctx, schoolID, classID)
	if err != nil {
		return nil, err
	}
	out := make([]QRCardView, 0, len(rows))
	for _, row := range rows {
		out = append(out, QRCardView{StudentID: row.StudentID, Name: row.Name, NIS: row.NIS, Token: row.Token})
	}
	return out, nil
}

// GenerateQRCards — POST /api/attendance/qr-cards/generate {class_id}.
// Idempoten: siswa yang SUDAH punya token aktif tidak diganti (docs/05: kartu
// dicetak sekali, hanya diganti lewat revoke eksplisit kalau hilang).
// Response selalu berisi SELURUH siswa rombel + token aktifnya masing-masing.
func (s *Service) GenerateQRCards(ctx context.Context, actorUserID, schoolID, classID int64) ([]QRCardView, error) {
	if err := classForQRCards(ctx, s, schoolID, classID); err != nil {
		return nil, err
	}
	rows, err := s.repo.ListClassStudentsWithActiveToken(ctx, schoolID, classID)
	if err != nil {
		return nil, err
	}

	out := make([]QRCardView, 0, len(rows))
	generated := 0
	for _, row := range rows {
		token := row.Token
		if token == "" {
			token, err = randomQRCardToken()
			if err != nil {
				return nil, err
			}
			if err := s.repo.InsertQRToken(ctx, schoolID, row.StudentID, token); err != nil {
				if !errors.Is(err, ErrConflict) {
					return nil, err
				}
				// Race jarang: token aktif sudah dibuat panggilan lain di
				// antara ListClassStudentsWithActiveToken & InsertQRToken —
				// ambil yang itu supaya tetap idempoten.
				token, err = s.repo.GetActiveQRTokenByStudent(ctx, schoolID, row.StudentID)
				if err != nil {
					return nil, err
				}
			} else {
				generated++
			}
		}
		out = append(out, QRCardView{StudentID: row.StudentID, Name: row.Name, NIS: row.NIS, Token: token})
	}
	if generated > 0 {
		s.audit(ctx, schoolID, actorUserID, "attendance.qr_cards_generate", "class", classID, nil,
			map[string]any{"class_id": classID, "generated": generated})
	}
	return out, nil
}

// RevokeQRCard — POST /api/attendance/qr-cards/{studentId}/revoke: cabut
// token aktif (mis. kartu hilang) + langsung terbitkan token baru, supaya
// siswa tetap bisa dicetakkan kartu baru tanpa panggilan generate terpisah.
func (s *Service) RevokeQRCard(ctx context.Context, actorUserID, schoolID, studentID int64) (QRCardView, error) {
	st, err := s.repo.GetStudentBasic(ctx, schoolID, studentID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return QRCardView{}, httpx.ErrNotFound
		}
		return QRCardView{}, err
	}
	if err := s.repo.RevokeActiveQRToken(ctx, schoolID, studentID); err != nil {
		return QRCardView{}, err
	}
	token, err := randomQRCardToken()
	if err != nil {
		return QRCardView{}, err
	}
	if err := s.repo.InsertQRToken(ctx, schoolID, studentID, token); err != nil {
		return QRCardView{}, err
	}
	s.audit(ctx, schoolID, actorUserID, "attendance.qr_card_revoke", "student", studentID, nil,
		map[string]any{"student_id": studentID})
	return QRCardView{StudentID: studentID, Name: st.Name, NIS: st.NIS, Token: token}, nil
}

// QRCardPNG — GET /api/attendance/qr-cards/{studentId}/qr.png.
func (s *Service) QRCardPNG(ctx context.Context, schoolID, studentID int64) ([]byte, error) {
	if _, err := s.repo.GetStudentBasic(ctx, schoolID, studentID); err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, httpx.ErrNotFound
		}
		return nil, err
	}
	token, err := s.repo.GetActiveQRTokenByStudent(ctx, schoolID, studentID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrQRNotIssued
		}
		return nil, err
	}
	return generateStudentQRPNG(token)
}

// ScanQRCard — POST /api/attendance/sessions/{id}/scan {token}: absen cepat
// via kartu QR (docs/05 "QR kartu siswa"). Record yang SUDAH ada TIDAK PERNAH
// didowngrade — scan kedua (atau scan setelah guru mengubah manual) hanya
// mengembalikan status yang tersimpan dengan already_marked=true.
func (s *Service) ScanQRCard(ctx context.Context, actorUserID, schoolID, sessionID int64, rawToken string) (ScanResult, error) {
	sess, err := s.repo.GetSessionByID(ctx, schoolID, sessionID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return ScanResult{}, httpx.ErrNotFound
		}
		return ScanResult{}, err
	}
	// Sesi finalized -> ditolak, KECUALI admin_sekolah (aturan sama seperti
	// UpdateRecords, lihat ErrSessionLocked).
	if sess.Status == SessionFinalized && reqctx.Role(ctx) != RoleAdminSekolah {
		return ScanResult{}, ErrSessionLocked
	}

	token := normalizeQRToken(rawToken)
	if token == "" {
		return ScanResult{}, httpx.Validation("token wajib diisi.")
	}
	studentID, err := s.repo.ResolveActiveQRToken(ctx, schoolID, token)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return ScanResult{}, ErrQRUnknown
		}
		return ScanResult{}, err
	}

	roster, err := s.repo.ListClassStudents(ctx, schoolID, sess.ClassID)
	if err != nil {
		return ScanResult{}, err
	}
	var st *RosterStudent
	for i := range roster {
		if roster[i].ID == studentID {
			st = &roster[i]
			break
		}
	}
	if st == nil {
		return ScanResult{}, httpx.Validation("Siswa ini bukan anggota rombel sesi ini.")
	}

	rec, err := s.repo.InsertRecordWithMeta(ctx, InsertRecordInput{
		SchoolID: schoolID, SessionID: sessionID, StudentID: studentID,
		Status: StatusHadir, Method: string(MethodQRCard), MarkedBy: actorUserID,
	})
	alreadyMarked := false
	switch {
	case errors.Is(err, ErrConflict):
		rec, err = s.repo.GetRecordForStudent(ctx, sessionID, studentID)
		if err != nil {
			return ScanResult{}, err
		}
		alreadyMarked = true
	case err != nil:
		return ScanResult{}, err
	}

	// Scan QR selalu menghasilkan status hadir (baik baris baru maupun yang
	// sudah ada) -> notifyAbsentIfNeeded TIDAK PERNAH mengirim dari jalur ini
	// (status hadir dikecualikan) — dipanggil di sini untuk konsistensi
	// dengan UpdateRecords/SelfCheckin (docs tugas fase 9 menyebut ketiganya
	// sebagai titik hook), bukan karena efeknya berbeda.
	s.notifyAbsentIfNeeded(ctx, schoolID, studentID, st.Name, sess.Date, rec.Status, alreadyMarked, rec.Status)
	s.emitSession(schoolID, sessionID, sess.ClassID, sess.Date)

	return ScanResult{StudentID: studentID, Name: st.Name, NIS: st.NIS, Status: rec.Status, AlreadyMarked: alreadyMarked}, nil
}

// -- Self check-in siswa (Fase 8, docs/05-attendance.md "Self check-in siswa") --

// haversineMeters menghitung jarak great-circle (meter) antara dua titik
// lat/lng — fungsi murni, dites tanpa DB (service_test.go).
func haversineMeters(lat1, lng1, lat2, lng2 float64) float64 {
	const earthRadiusM = 6371000.0
	rad := func(deg float64) float64 { return deg * math.Pi / 180 }
	dLat := rad(lat2 - lat1)
	dLng := rad(lng2 - lng1)
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(rad(lat1))*math.Cos(rad(lat2))*math.Sin(dLng/2)*math.Sin(dLng/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return earthRadiusM * c
}

// clockOnDate mengembalikan waktu jam:menit "HH:MM" pada tanggal & lokasi base.
func clockOnDate(base time.Time, hhmm string) (time.Time, error) {
	var h, m int
	if _, err := fmt.Sscanf(strings.TrimSpace(hhmm), "%d:%d", &h, &m); err != nil || h < 0 || h > 23 || m < 0 || m > 59 {
		return time.Time{}, httpx.Validation("Format jam pengaturan self check-in tidak valid.")
	}
	return time.Date(base.Year(), base.Month(), base.Day(), h, m, 0, 0, base.Location()), nil
}

// addMinutesToClock menambah menit ke string jam "HH:MM" -> "HH:MM" baru,
// tanpa perlu tanggal/zona (dipakai status endpoint menghitung late_after).
func addMinutesToClock(hhmm string, minutes int) (string, error) {
	t, err := clockOnDate(time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC), hhmm)
	if err != nil {
		return "", err
	}
	return t.Add(time.Duration(minutes) * time.Minute).Format("15:04"), nil
}

// round5 membulatkan koordinat ke 5 desimal (~1.1 m) — dipakai Anomalies
// mengelompokkan self check-in dari koordinat yang (hampir) identik.
func round5(v float64) float64 { return math.Round(v*100000) / 100000 }

func selfCheckinEnabled(settings Settings) bool {
	if settings.SelfCheckin == nil {
		return false
	}
	for _, m := range settings.Methods {
		if m == MethodSelfCheckin {
			return true
		}
	}
	return false
}

// SelfCheckinStatus — GET /api/attendance/self-checkin/status.
func (s *Service) SelfCheckinStatus(ctx context.Context, schoolID int64) (SelfCheckinStatusView, error) {
	settings, err := s.repo.GetSettings(ctx, schoolID)
	if err != nil {
		return SelfCheckinStatusView{}, err
	}

	view := SelfCheckinStatusView{Enabled: selfCheckinEnabled(settings)}
	if view.Enabled {
		rule := settings.SelfCheckin
		view.Window = &SelfCheckinWindow{OpenFrom: rule.OpenFrom, CloseAt: rule.CloseAt}
		if late, err := addMinutesToClock(rule.OpenFrom, settings.LateAfterMin); err == nil {
			view.LateAfter = &late
		}
	}

	studentID, classID, ok, err := s.students.MyStudentID(ctx, schoolID, reqctx.UserID(ctx))
	if err != nil {
		return SelfCheckinStatusView{}, err
	}
	if !ok {
		return view, nil
	}

	today := schoolToday(s.clock.Now(), schoolTimezone(ctx))
	sess, err := s.repo.GetSessionByClassDate(ctx, schoolID, classID, today)
	if errors.Is(err, ErrNotFound) {
		return view, nil
	}
	if err != nil {
		return SelfCheckinStatusView{}, err
	}

	rec, err := s.repo.GetRecordForStudent(ctx, sess.ID, studentID)
	if errors.Is(err, ErrNotFound) {
		return view, nil
	}
	if err != nil {
		return SelfCheckinStatusView{}, err
	}
	view.Today = &SelfCheckinToday{Status: rec.Status, CheckedAt: rec.MarkedAt}
	return view, nil
}

// SelfCheckinInput adalah parameter POST /api/attendance/self-checkin.
type SelfCheckinInput struct {
	Lat, Lng, Accuracy float64
	UserAgent          string
}

// SelfCheckin — POST /api/attendance/self-checkin. Validasi BERURUTAN (pesan
// jelas per langkah, docs/05-attendance.md): metode aktif -> jendela waktu ->
// jarak (haversine) -> belum check-in hari ini. Lolos semua -> create-or-get
// sesi daily rombel siswa -> insert record hadir/terlambat sesuai
// open_from+late_after_min. BUKAN anti-curang (fake GPS mudah) — hasilnya
// hanya bukti (meta) yang bisa ditinjau lewat GET /api/attendance/anomalies;
// guru tetap bisa override record manual kapan pun.
func (s *Service) SelfCheckin(ctx context.Context, schoolID int64, in SelfCheckinInput) (SelfCheckinResult, error) {
	userID := reqctx.UserID(ctx)
	studentID, classID, ok, err := s.students.MyStudentID(ctx, schoolID, userID)
	if err != nil {
		return SelfCheckinResult{}, err
	}
	if !ok {
		return SelfCheckinResult{}, httpx.Validation("Akun ini tidak terhubung ke data siswa aktif di tahun ajaran berjalan.")
	}

	settings, err := s.repo.GetSettings(ctx, schoolID)
	if err != nil {
		return SelfCheckinResult{}, err
	}
	if !selfCheckinEnabled(settings) {
		return SelfCheckinResult{}, httpx.Validation("Self check-in tidak aktif di sekolah ini.")
	}
	rule := settings.SelfCheckin

	now := s.clock.Now()
	local := clock.InZone(now, schoolTimezone(ctx))
	openAt, err := clockOnDate(local, rule.OpenFrom)
	if err != nil {
		return SelfCheckinResult{}, err
	}
	closeAt, err := clockOnDate(local, rule.CloseAt)
	if err != nil {
		return SelfCheckinResult{}, err
	}
	if local.Before(openAt) {
		return SelfCheckinResult{}, httpx.Validation(fmt.Sprintf("Self check-in belum dibuka. Jendela: %s–%s.", rule.OpenFrom, rule.CloseAt))
	}
	if local.After(closeAt) {
		return SelfCheckinResult{}, httpx.Validation(fmt.Sprintf("Self check-in sudah ditutup pukul %s.", rule.CloseAt))
	}

	dist := haversineMeters(rule.Lat, rule.Lng, in.Lat, in.Lng)
	if dist > float64(rule.RadiusM) {
		return SelfCheckinResult{}, httpx.Validation(fmt.Sprintf("Kamu berada ~%.0f m dari sekolah (maks %d m).", dist, rule.RadiusM))
	}

	yearID, err := s.requireActiveYear(ctx, schoolID)
	if err != nil {
		return SelfCheckinResult{}, err
	}
	today := schoolToday(now, schoolTimezone(ctx))

	sess, err := s.repo.GetSessionByClassDate(ctx, schoolID, classID, today)
	switch {
	case errors.Is(err, ErrNotFound):
		sess, err = s.repo.CreateSession(ctx, CreateSessionInput{
			SchoolID: schoolID, AcademicYearID: yearID, ClassID: classID, Date: today, OpenedBy: userID,
		})
		switch {
		case err == nil:
			s.audit(ctx, schoolID, userID, "attendance.session_open", "attendance_session", sess.ID, nil,
				map[string]any{"class_id": classID, "date": today.Format("2006-01-02"), "opened_via": "self_checkin"})
		case errors.Is(err, ErrConflict):
			sess, err = s.repo.GetSessionByClassDate(ctx, schoolID, classID, today)
			if err != nil {
				return SelfCheckinResult{}, err
			}
		default:
			return SelfCheckinResult{}, err
		}
	case err != nil:
		return SelfCheckinResult{}, err
	}

	if sess.Status == SessionFinalized {
		return SelfCheckinResult{}, ErrSessionLocked
	}

	if _, err := s.repo.GetRecordForStudent(ctx, sess.ID, studentID); err == nil {
		return SelfCheckinResult{}, httpx.Validation("Kamu sudah check-in hari ini.")
	} else if !errors.Is(err, ErrNotFound) {
		return SelfCheckinResult{}, err
	}

	lateAt := openAt.Add(time.Duration(settings.LateAfterMin) * time.Minute)
	status := StatusHadir
	if local.After(lateAt) {
		status = StatusTerlambat
	}

	meta, err := json.Marshal(map[string]any{"lat": in.Lat, "lng": in.Lng, "accuracy": in.Accuracy, "user_agent": in.UserAgent})
	if err != nil {
		return SelfCheckinResult{}, err
	}

	rec, err := s.repo.InsertRecordWithMeta(ctx, InsertRecordInput{
		SchoolID: schoolID, SessionID: sess.ID, StudentID: studentID,
		Status: status, Method: string(MethodSelfCheckin), MarkedBy: userID, Meta: meta,
	})
	if err != nil {
		if errors.Is(err, ErrConflict) {
			return SelfCheckinResult{}, httpx.Validation("Kamu sudah check-in hari ini.")
		}
		return SelfCheckinResult{}, err
	}

	// Record baru (sudah dipastikan belum ada sebelumnya, lihat GetRecordForStudent
	// di atas) -> oldExisted=false. Status hanya hadir/terlambat (lihat
	// perhitungan status di atas) — notifyAbsentIfNeeded hanya mengirim bila
	// terlambat.
	studentName := ""
	if basic, errBasic := s.repo.GetStudentBasic(ctx, schoolID, studentID); errBasic == nil {
		studentName = basic.Name
	}
	s.notifyAbsentIfNeeded(ctx, schoolID, studentID, studentName, sess.Date, "", false, rec.Status)
	s.emitSession(schoolID, sess.ID, classID, sess.Date)

	return SelfCheckinResult{Status: rec.Status, CheckedAt: rec.MarkedAt}, nil
}

// Anomalies — GET /api/attendance/anomalies?date=&class_id= (docs/05: "alat
// bantu" deteksi self check-in mencurigakan, BUKAN anti-curang — keputusan
// akhir tetap guru/wali kelas). Dua sinyal: (a) >=3 siswa check-in dari
// koordinat yang (dibulatkan 5 desimal) identik, (b) accuracy GPS > 200 m.
func (s *Service) Anomalies(ctx context.Context, schoolID int64, dateStr string, classID int64) ([]AnomalyItem, error) {
	date, err := parseDateParam(dateStr, s.clock.Now(), schoolTimezone(ctx))
	if err != nil {
		return nil, err
	}
	rows, err := s.repo.SelfCheckinRecordsForDate(ctx, schoolID, date, classID)
	if err != nil {
		return nil, err
	}

	type coordKey struct{ lat, lng float64 }
	groups := make(map[coordKey][]int)
	var out []AnomalyItem

	for i, row := range rows {
		if row.Accuracy > 200 {
			out = append(out, AnomalyItem{
				StudentID: row.StudentID, Name: row.StudentName, ClassName: row.ClassName,
				Issue:  "low_accuracy",
				Detail: fmt.Sprintf("Akurasi GPS ~%.0f m saat check-in (di atas 200 m).", row.Accuracy),
			})
		}
		key := coordKey{round5(row.Lat), round5(row.Lng)}
		groups[key] = append(groups[key], i)
	}
	for key, idxs := range groups {
		if len(idxs) < 3 {
			continue
		}
		for _, i := range idxs {
			row := rows[i]
			out = append(out, AnomalyItem{
				StudentID: row.StudentID, Name: row.StudentName, ClassName: row.ClassName,
				Issue:  "same_location",
				Detail: fmt.Sprintf("%d siswa check-in dari koordinat yang sama (%.5f, %.5f).", len(idxs), key.lat, key.lng),
			})
		}
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Name != out[j].Name {
			return out[i].Name < out[j].Name
		}
		return out[i].Issue < out[j].Issue
	})
	return out, nil
}
