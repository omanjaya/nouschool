package attendance

import (
	"context"
	"errors"
	"fmt"
	"net/http"
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
// lihat anaknya) untuk GET /api/students/{id}/attendance. Dipenuhi
// *student.Service secara struktural lewat method CanViewStudent (ditambahkan
// khusus untuk kebutuhan ini — lihat internal/student/service.go).
type StudentAccess interface {
	CanViewStudent(ctx context.Context, userID int64, role string, schoolID, studentID int64) error
}

// Permission kanonik dipakai modul attendance (nilai HARUS sama persis
// dengan docs/02-identity.md — didefinisikan ulang di sini karena attendance
// tidak boleh mengimpor identity).
const (
	PermAttendanceWrite  = "attendance:write"
	PermAttendanceReport = "attendance:report"
)

// ErrNoActiveAcademicYear — sekolah belum mengaktifkan tahun ajaran.
var ErrNoActiveAcademicYear = httpx.Validation("Sekolah belum punya tahun ajaran aktif. Aktifkan dulu di menu tahun ajaran.")

// ErrSessionLocked — sesi sudah difinalisasi, hanya admin_sekolah yang boleh mengubah.
var ErrSessionLocked = &httpx.Error{Status: http.StatusForbidden, Code: "session_locked", Message: "Sesi sudah dikunci."}

// Service berisi aturan bisnis modul attendance.
type Service struct {
	repo     attendanceRepository
	identity IdentityGateway
	years    AcademicYearLookup
	students StudentAccess
	clock    clock.Clock
}

func NewService(repo *Repository, identity IdentityGateway, years AcademicYearLookup, students StudentAccess, clk clock.Clock) *Service {
	if clk == nil {
		clk = clock.System{}
	}
	return &Service{repo: repo, identity: identity, years: years, students: students, clock: clk}
}

// newServiceForTest membangun Service dengan repository FAKE (in-memory,
// tanpa DB) — dipakai test di package ini saja (service_test.go).
func newServiceForTest(repo attendanceRepository, identity IdentityGateway, years AcademicYearLookup, students StudentAccess, clk clock.Clock) *Service {
	return &Service{repo: repo, identity: identity, years: years, students: students, clock: clk}
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
type NewSessionInput struct {
	ClassID int64
	Date    string // "" = hari ini (waktu lokal sekolah)
}

// CreateSession membuat-atau-mengambil sesi daily untuk (class_id, date) —
// idempoten lewat partial unique index (class_id,date) WHERE type='daily'
// (docs/05-attendance.md). Percobaan kedua pada rombel+tanggal yang sama
// TIDAK membuat sesi baru, hanya mengembalikan sesi yang sudah ada.
func (s *Service) CreateSession(ctx context.Context, actorUserID, schoolID int64, in NewSessionInput) (SessionDetail, error) {
	date, err := parseDateParam(in.Date, s.clock.Now(), schoolTimezone(ctx))
	if err != nil {
		return SessionDetail{}, err
	}
	yearID, err := s.requireActiveYear(ctx, schoolID)
	if err != nil {
		return SessionDetail{}, err
	}
	if in.ClassID == 0 {
		return SessionDetail{}, httpx.Validation("class_id wajib diisi.")
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

	return s.buildSessionDetail(ctx, schoolID, sess.ID)
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
	for _, st := range roster {
		rosterSet[st.ID] = true
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
	}
	if len(changedOld) > 0 {
		s.audit(ctx, schoolID, actorUserID, "attendance.update", "attendance_session", sessionID, changedOld, changedNew)
	}

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
