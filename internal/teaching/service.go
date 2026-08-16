package teaching

import (
	"context"
	"errors"
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
// main.go). Signature primitif kecuali SlotInfo — struct itu didefinisikan
// DI MODUL INI (bukan diimpor dari schedule) supaya teaching TIDAK
// mengimpor package schedule; kepuasan interface oleh *schedule.Service
// dijembatani lewat adapter kecil di cmd/server/main.go (dan cmd/bootstrap
// bila perlu) — satu-satunya tempat yang boleh mengimpor kedua modul
// sekaligus, konsisten dengan "wiring eksplisit di main.go" (CLAUDE.md).

// IdentityGateway adalah kebutuhan modul teaching dari modul identity:
// gerbang permission (scope=all butuh teaching:monitor) dan audit log.
type IdentityGateway interface {
	HasPermission(role, perm string) bool
	Log(ctx context.Context, schoolID, userID *int64, action, entity string, entityID *int64, oldValue, newValue any) error
}

// SlotInfo adalah potongan data slot jadwal yang dibutuhkan teaching dari
// modul schedule — primitif saja (lihat catatan interface di atas).
// PeriodStartsAt/PeriodEndsAt ("HH:MM") dipakai derivasi status (docs/06).
type SlotInfo struct {
	ID             int64
	ClassID        int64
	ClassName      string
	SubjectID      int64
	SubjectCode    string
	SubjectName    string
	TeacherID      int64
	TeacherName    string
	RoomID         int64 // 0 = slot tanpa ruang ditetapkan
	RoomName       string
	DayOfWeek      int
	PeriodStart    int
	PeriodEnd      int
	PeriodStartsAt string
	PeriodEndsAt   string
}

// ScheduleGateway adalah kebutuhan modul teaching dari modul schedule:
// SlotNow (scan QR), SlotsForDayOfWeek (monitoring lintas guru), SlotByID
// (derivasi status jurnal individual), CurrentPeriod (info jam berjalan).
type ScheduleGateway interface {
	SlotNow(ctx context.Context, schoolID, teacherID int64, at time.Time) (SlotInfo, bool, error)
	SlotByID(ctx context.Context, schoolID, slotID int64) (SlotInfo, bool, error)
	SlotsForDayOfWeek(ctx context.Context, schoolID int64, dayOfWeek int) ([]SlotInfo, error)
	CurrentPeriod(ctx context.Context, schoolID int64, at time.Time) (number int, startsAt, endsAt string, ok bool, err error)
}

// LeaveGateway adalah kebutuhan modul teaching dari modul leave: status izin
// guru pada tanggal tertentu (docs/06 "Izin (kuning)"). Dipenuhi *leave.Service
// secara struktural lewat method ApprovedOn (sudah ada sejak fase 4).
type LeaveGateway interface {
	ApprovedOn(ctx context.Context, schoolID, teacherUserID int64, date string) (bool, error)
}

// AttendanceGateway adalah kebutuhan modul teaching dari modul attendance:
// buka-atau-ambil sesi absen subject untuk satu slot (satu scan = jurnal +
// sesi absen siap, docs/06). Dipenuhi *attendance.Service secara struktural
// lewat method OpenSubjectSession (ditambahkan fase 6).
type AttendanceGateway interface {
	OpenSubjectSession(ctx context.Context, schoolID, actorUserID, scheduleSlotID int64, date string) (sessionID int64, err error)
}

// StudentGateway adalah kebutuhan modul teaching dari modul student:
// resolve profil guru dari user login. Dipenuhi *student.Service secara
// struktural lewat method MyTeacherID (sudah ada sejak fase 5).
type StudentGateway interface {
	MyTeacherID(ctx context.Context, schoolID, userID int64) (teacherID int64, ok bool, err error)
}

// SubstitutionLookup adalah kebutuhan modul teaching dari modul substitution
// (Fase 14 Gelombang D, docs/12-sion-parity.md): status pengganti ACCEPTED
// utk suatu slot+tanggal. Dipenuhi *substitution.Service secara STRUKTURAL
// (signature primitif) — teaching TIDAK mengimpor package substitution.
// Opsional (lihat SetSubstitutions): nil = jalur pengganti dilewati sama
// sekali, perilaku scan/status IDENTIK sebelum Gelombang D ini ada.
type SubstitutionLookup interface {
	// SubstituteName — nama pengganti ACCEPTED utk (slotID,date), ok=false
	// bila tidak ada (dipakai Status: "teacher_name tampil {pengganti}
	// (pengganti)").
	SubstituteName(ctx context.Context, schoolID, slotID int64, date string) (name string, ok bool, err error)
	// IsSubstituteToday — daftar schedule_slot_id yang teacherUserID adalah
	// pengganti ACCEPTED pada tanggal date (dipakai Scan: scanner BUKAN guru
	// slot TAPI pengganti accepted -> diizinkan buka jurnal+sesi).
	IsSubstituteToday(ctx context.Context, schoolID, teacherUserID int64, date string) (slotIDs []int64, err error)
}

// Realtime adalah kebutuhan modul teaching dari modul realtime (Fase 12, bus
// event WebSocket read-only server->client) — consumer-side interface kecil
// dideklarasikan di sisi PEMAKAI (lihat CLAUDE.md). TIDAK dipenuhi
// *realtime.Hub secara langsung — dijembatani adapter kecil di cmd/server
// (realtimeadapter.go).
type Realtime interface {
	PublishTo(schoolID int64, eventType string, data map[string]any, roles []string, userIDs []int64)
}

// Service berisi aturan bisnis modul teaching.
type Service struct {
	repo          teachingRepository
	identity      IdentityGateway
	schedule      ScheduleGateway
	leave         LeaveGateway
	attendance    AttendanceGateway
	students      StudentGateway
	clock         clock.Clock
	realtime      Realtime           // opsional — nil = event realtime dilewati (lihat SetRealtime)
	substitutions SubstitutionLookup // opsional — nil = jalur pengganti dilewati (lihat SetSubstitutions)
}

func NewService(repo *Repository, identity IdentityGateway, schedule ScheduleGateway, leave LeaveGateway, attendance AttendanceGateway, students StudentGateway, clk clock.Clock) *Service {
	if clk == nil {
		clk = clock.System{}
	}
	return &Service{repo: repo, identity: identity, schedule: schedule, leave: leave, attendance: attendance, students: students, clock: clk}
}

// SetRealtime menyuntikkan Realtime SETELAH konstruksi (opsional, disuntik
// main.go — pola yang sama dengan internal/attendance.SetNotifier; nil
// aman/no-op — lihat emitStatus).
func (s *Service) SetRealtime(r Realtime) { s.realtime = r }

// SetSubstitutions menyuntikkan SubstitutionLookup SETELAH konstruksi
// (opsional, disuntik main.go — pola yang sama dengan SetRealtime; nil
// aman/no-op — lihat Scan & Status).
func (s *Service) SetSubstitutions(sl SubstitutionLookup) { s.substitutions = sl }

// emitStatus memancarkan "teaching.status" {date} ke roles
// admin_sekolah/kepala_sekolah/display (docs tugas Fase 12: "scan / create
// journal / end / patch") — dipanggil SETELAH operasi sukses, best-effort
// (nil-safe, tidak pernah mengembalikan error).
func (s *Service) emitStatus(schoolID int64, date time.Time) {
	if s.realtime == nil {
		return
	}
	s.realtime.PublishTo(schoolID, "teaching.status", map[string]any{"date": date.Format("2006-01-02")},
		[]string{RoleAdminSekolah, RoleKepalaSekolah, RoleDisplay}, nil)
}

// newServiceForTest membangun Service dengan repository FAKE (in-memory,
// tanpa DB) — dipakai test di package ini saja (service_test.go).
func newServiceForTest(repo teachingRepository, identity IdentityGateway, schedule ScheduleGateway, leave LeaveGateway, attendance AttendanceGateway, students StudentGateway, clk clock.Clock) *Service {
	return &Service{repo: repo, identity: identity, schedule: schedule, leave: leave, attendance: attendance, students: students, clock: clk}
}

func (s *Service) audit(ctx context.Context, schoolID, actorUserID int64, action, entity string, entityID int64, oldValue, newValue any) {
	sid, uid, eid := schoolID, actorUserID, entityID
	_ = s.identity.Log(ctx, &sid, &uid, action, entity, &eid, oldValue, newValue)
}

func schoolTimezone(ctx context.Context) string {
	sch, _ := reqctx.SchoolFromContext(ctx)
	return sch.Timezone
}

// -- tanggal & jam lokal sekolah (pola sama internal/attendance/service.go) --

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

// clockMinutes mengonversi "HH:MM" -> menit sejak tengah malam (redefinisi
// lokal — sama seperti internal/schedule, teaching TIDAK mengimpor schedule).
func clockMinutes(s string) int {
	t, err := time.Parse("15:04", strings.TrimSpace(s))
	if err != nil {
		return 0
	}
	return t.Hour()*60 + t.Minute()
}

// combineLocal menggabungkan tanggal (date, jam 00:00) + jam "HH:MM" (clockStr)
// pada zona waktu sekolah tz menjadi satu instant absolut — dipakai
// membandingkan "sekarang" terhadap awal/akhir slot (docs/06 derivasi status).
func combineLocal(date time.Time, clockStr, tz string) time.Time {
	loc, err := time.LoadLocation(tz)
	if err != nil {
		loc = time.FixedZone("WIB", 7*3600)
	}
	mins := clockMinutes(clockStr)
	return time.Date(date.Year(), date.Month(), date.Day(), mins/60, mins%60, 0, 0, loc)
}

func slotRoomRef(sl SlotInfo) *RoomRef {
	if sl.RoomID == 0 {
		return nil
	}
	return &RoomRef{ID: sl.RoomID, Name: sl.RoomName}
}

// -- derivasi shape jurnal (docs/06-teaching.md "Status mengajar (derivasi,
// bukan tabel)"): ended_at terisi -> done; else slot punya period_end & waktu
// lokal sudah lewat akhir period -> done; else ongoing. --

func (s *Service) journalView(ctx context.Context, now time.Time, d JournalDetail) (JournalView, error) {
	status := JournalOngoing
	switch {
	case !d.EndedAt.IsZero():
		status = JournalDone
	case d.ScheduleSlotID != 0:
		slot, ok, err := s.schedule.SlotByID(ctx, d.SchoolID, d.ScheduleSlotID)
		if err != nil {
			return JournalView{}, err
		}
		if ok {
			slotEnd := combineLocal(d.Date, slot.PeriodEndsAt, schoolTimezone(ctx))
			if now.After(slotEnd) {
				status = JournalDone
			}
		}
	}

	view := JournalView{
		ID:        d.ID,
		Teacher:   TeacherRef{ID: d.TeacherID, Name: d.TeacherName},
		Class:     ClassRef{ID: d.ClassID, Name: d.ClassName},
		Date:      NewDate(d.Date),
		StartedAt: d.StartedAt,
		Material:  d.Material,
		Note:      d.Note,
		Flags:     d.Flags,
		Status:    status,
	}
	if d.SubjectID != 0 {
		view.Subject = &SubjectRef{ID: d.SubjectID, Code: d.SubjectCode, Name: d.SubjectName}
	}
	if d.RoomID != 0 {
		view.Room = &RoomRef{ID: d.RoomID, Name: d.RoomName}
	}
	if d.ScheduleSlotID != 0 {
		id := d.ScheduleSlotID
		view.ScheduleSlotID = &id
	}
	if !d.EndedAt.IsZero() {
		t := d.EndedAt
		view.EndedAt = &t
	}
	return view, nil
}

// -- POST /api/teaching/scan --

// ScanInput adalah parameter POST /api/teaching/scan. ManualPick menandai
// klien memakai daftar ruangan manual (kamera tidak tersedia) alih-alih scan
// QR — tidak mengubah alur server (room_id tetap divalidasi sama), hanya
// sinyal niat dari klien (docs/06 "fallback manual").
type ScanInput struct {
	RoomToken  string
	RoomID     int64
	ManualPick bool
}

// Scan — konsep inti fase 6 (docs/06-teaching.md "scan QR ruangan = satu
// aksi, tiga hasil"): resolve room -> cari SlotNow guru -> slot ketemu bikin
// journal + buka sesi absen subject sekaligus (idempoten per slot+tanggal);
// slot tidak ketemu -> needs_manual TANPA membuat journal.
func (s *Service) Scan(ctx context.Context, actorUserID, schoolID int64, in ScanInput) (ScanResult, error) {
	teacherID, ok, err := s.students.MyTeacherID(ctx, schoolID, actorUserID)
	if err != nil {
		return ScanResult{}, err
	}
	if !ok {
		return ScanResult{}, httpx.Validation("Akun ini tidak terdaftar sebagai guru.")
	}

	var room RoomRef
	switch {
	case strings.TrimSpace(in.RoomToken) != "":
		room, err = s.repo.LookupRoomByToken(ctx, schoolID, strings.TrimSpace(in.RoomToken))
	case in.RoomID != 0:
		room, err = s.repo.LookupRoomByID(ctx, schoolID, in.RoomID)
	default:
		return ScanResult{}, httpx.Validation("room_token atau room_id wajib diisi.")
	}
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return ScanResult{}, httpx.Validation("Ruangan tidak ditemukan atau QR tidak valid.")
		}
		return ScanResult{}, err
	}

	now := s.clock.Now()
	date := schoolToday(now, schoolTimezone(ctx))

	slot, found, err := s.schedule.SlotNow(ctx, schoolID, teacherID, now)
	if err != nil {
		return ScanResult{}, err
	}
	// substituteFlag — TRUE bila scanner BUKAN guru pemilik slot tapi
	// pengganti ACCEPTED utk slot+tanggal itu (Fase 14 Gelombang D,
	// docs/12-sion-parity.md "scan pengganti diizinkan"). Dicoba HANYA bila
	// jalur normal (guru pemilik) tidak menemukan slot berjalan.
	substituteFlag := false
	if !found && s.substitutions != nil {
		var serr error
		slot, found, serr = s.findSubstituteSlotNow(ctx, schoolID, actorUserID, room.ID, now)
		if serr != nil {
			return ScanResult{}, serr
		}
		substituteFlag = found
	}
	if !found {
		r := room
		return ScanResult{NeedsManual: true, Room: &r}, nil
	}

	// Idempoten: scan ulang slot+tanggal sama -> kembalikan journal existing
	// (tetap pastikan sesi absen subject terbuka, aman dipanggil berkali-kali
	// karena OpenSubjectSession sendiri create-or-get).
	if existing, err := s.repo.GetJournalDetailBySlotDate(ctx, schoolID, slot.ID, date); err == nil {
		sessionID, err := s.attendance.OpenSubjectSession(ctx, schoolID, actorUserID, slot.ID, date.Format("2006-01-02"))
		if err != nil {
			return ScanResult{}, err
		}
		view, err := s.journalView(ctx, now, existing)
		if err != nil {
			return ScanResult{}, err
		}
		s.emitStatus(schoolID, date)
		return ScanResult{Journal: &view, AttendanceSessionID: &sessionID, NeedsManual: false}, nil
	} else if !errors.Is(err, ErrNotFound) {
		return ScanResult{}, err
	}

	flags := []string{}
	if slot.RoomID != 0 && slot.RoomID != room.ID {
		flags = append(flags, FlagRoomMismatch)
	}
	if substituteFlag {
		flags = append(flags, FlagSubstitute)
	}

	rec, err := s.repo.InsertJournal(ctx, JournalInput{
		SchoolID: schoolID, TeacherID: teacherID, ScheduleSlotID: slot.ID, ClassID: slot.ClassID,
		SubjectID: slot.SubjectID, RoomID: room.ID, Date: date, StartedAt: now, Flags: flags,
	})
	if err != nil {
		return ScanResult{}, err
	}

	sessionID, err := s.attendance.OpenSubjectSession(ctx, schoolID, actorUserID, slot.ID, date.Format("2006-01-02"))
	if err != nil {
		return ScanResult{}, err
	}

	s.audit(ctx, schoolID, actorUserID, "teaching.scan", "teaching_journal", rec.ID, nil, map[string]any{
		"schedule_slot_id": slot.ID, "room_id": room.ID, "flags": flags,
	})

	detail, err := s.repo.GetJournalDetail(ctx, schoolID, rec.ID)
	if err != nil {
		return ScanResult{}, err
	}
	view, err := s.journalView(ctx, now, detail)
	if err != nil {
		return ScanResult{}, err
	}
	s.emitStatus(schoolID, date)
	return ScanResult{Journal: &view, AttendanceSessionID: &sessionID, NeedsManual: false}, nil
}

// findSubstituteSlotNow mencari slot yang SEDANG berjalan (period berjalan)
// di ruangan roomID yang actorUserID adalah pengganti ACCEPTED untuknya hari
// ini (Fase 14 Gelombang D). ok=false bila tidak ada kecocokan (bukan
// pengganti siapa pun hari ini, atau tidak ada slot yang cocok jam+ruangan
// saat ini) — pemanggil (Scan) memperlakukan itu sama seperti "slot tidak
// ketemu" (needs_manual).
func (s *Service) findSubstituteSlotNow(ctx context.Context, schoolID, actorUserID, roomID int64, now time.Time) (SlotInfo, bool, error) {
	dateStr := schoolToday(now, schoolTimezone(ctx)).Format("2006-01-02")
	slotIDs, err := s.substitutions.IsSubstituteToday(ctx, schoolID, actorUserID, dateStr)
	if err != nil {
		return SlotInfo{}, false, err
	}
	if len(slotIDs) == 0 {
		return SlotInfo{}, false, nil
	}
	num, _, _, ok, err := s.schedule.CurrentPeriod(ctx, schoolID, now)
	if err != nil {
		return SlotInfo{}, false, err
	}
	if !ok {
		return SlotInfo{}, false, nil
	}
	for _, sid := range slotIDs {
		si, found, serr := s.schedule.SlotByID(ctx, schoolID, sid)
		if serr != nil {
			return SlotInfo{}, false, serr
		}
		if !found || si.RoomID != roomID {
			continue
		}
		if si.PeriodStart <= num && num <= si.PeriodEnd {
			return si, true, nil
		}
	}
	return SlotInfo{}, false, nil
}

// -- POST /api/teaching/journals (unscheduled) --

// UnscheduledInput adalah parameter POST /api/teaching/journals.
type UnscheduledInput struct {
	RoomID    int64
	ClassID   int64
	SubjectID int64
}

// CreateUnscheduled — entry jurnal guru pengganti/insidental (TANPA slot
// jadwal, TANPA sesi absen otomatis — docs/06-teaching.md).
func (s *Service) CreateUnscheduled(ctx context.Context, actorUserID, schoolID int64, in UnscheduledInput) (JournalView, error) {
	teacherID, ok, err := s.students.MyTeacherID(ctx, schoolID, actorUserID)
	if err != nil {
		return JournalView{}, err
	}
	if !ok {
		return JournalView{}, httpx.Validation("Akun ini tidak terdaftar sebagai guru.")
	}
	if in.ClassID == 0 || in.SubjectID == 0 || in.RoomID == 0 {
		return JournalView{}, httpx.Validation("room_id, class_id, dan subject_id wajib diisi.")
	}
	if _, err := s.repo.LookupClassRef(ctx, schoolID, in.ClassID); err != nil {
		if errors.Is(err, ErrNotFound) {
			return JournalView{}, httpx.Validation("Rombel tidak ditemukan.")
		}
		return JournalView{}, err
	}
	if _, err := s.repo.LookupSubjectRef(ctx, schoolID, in.SubjectID); err != nil {
		if errors.Is(err, ErrNotFound) {
			return JournalView{}, httpx.Validation("Mapel tidak ditemukan.")
		}
		return JournalView{}, err
	}
	if _, err := s.repo.LookupRoomByID(ctx, schoolID, in.RoomID); err != nil {
		if errors.Is(err, ErrNotFound) {
			return JournalView{}, httpx.Validation("Ruangan tidak ditemukan.")
		}
		return JournalView{}, err
	}

	now := s.clock.Now()
	date := schoolToday(now, schoolTimezone(ctx))
	rec, err := s.repo.InsertJournal(ctx, JournalInput{
		SchoolID: schoolID, TeacherID: teacherID, ClassID: in.ClassID, SubjectID: in.SubjectID, RoomID: in.RoomID,
		Date: date, StartedAt: now, Flags: []string{FlagUnscheduled},
	})
	if err != nil {
		return JournalView{}, err
	}
	s.audit(ctx, schoolID, actorUserID, "teaching.journal_create_unscheduled", "teaching_journal", rec.ID, nil, map[string]any{
		"class_id": in.ClassID, "subject_id": in.SubjectID, "room_id": in.RoomID,
	})
	detail, err := s.repo.GetJournalDetail(ctx, schoolID, rec.ID)
	if err != nil {
		return JournalView{}, err
	}
	s.emitStatus(schoolID, date)
	return s.journalView(ctx, now, detail)
}

// -- PATCH /api/teaching/journals/{id} --

// PatchInput adalah parameter PATCH /api/teaching/journals/{id}. Kedua field
// SELALU diterapkan (bukan partial-patch): mengirim "" mengosongkan field
// itu — pola paling sederhana yang cukup untuk jurnal (bukan absensi kritikal
// seperti attendance_records), didokumentasikan sebagai keputusan sendiri.
type PatchInput struct {
	Material string
	Note     string
}

// UpdateJournal — pemilik ATAU admin_sekolah, boleh diisi sampai H+2
// (docs/06-teaching.md "guru melengkapi jurnal ... boleh diisi belakangan").
func (s *Service) UpdateJournal(ctx context.Context, actorUserID, schoolID, id int64, in PatchInput) (JournalView, error) {
	rec, err := s.repo.GetJournalByID(ctx, schoolID, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return JournalView{}, httpx.ErrNotFound
		}
		return JournalView{}, err
	}

	role := reqctx.Role(ctx)
	isAdmin := role == RoleAdminSekolah
	if !isAdmin {
		teacherID, ok, err := s.students.MyTeacherID(ctx, schoolID, actorUserID)
		if err != nil {
			return JournalView{}, err
		}
		if !ok || teacherID != rec.TeacherID {
			return JournalView{}, httpx.ErrForbidden
		}
	}

	now := s.clock.Now()
	today := schoolToday(now, schoolTimezone(ctx))
	deadline := rec.Date.AddDate(0, 0, 2)
	if !isAdmin && today.After(deadline) {
		return JournalView{}, &httpx.Error{
			Status: http.StatusForbidden, Code: "edit_window_closed",
			Message: "Jendela edit jurnal (2 hari sejak tanggal mengajar) sudah lewat. Hanya admin sekolah yang bisa mengubah jurnal ini.",
		}
	}

	if _, err := s.repo.UpdateJournalNote(ctx, schoolID, id, in.Material, in.Note); err != nil {
		if errors.Is(err, ErrNotFound) {
			return JournalView{}, httpx.ErrNotFound
		}
		return JournalView{}, err
	}
	s.audit(ctx, schoolID, actorUserID, "teaching.journal_update", "teaching_journal", id, nil, map[string]any{
		"material": in.Material, "note": in.Note,
	})

	detail, err := s.repo.GetJournalDetail(ctx, schoolID, id)
	if err != nil {
		return JournalView{}, err
	}
	s.emitStatus(schoolID, rec.Date)
	return s.journalView(ctx, now, detail)
}

// -- POST /api/teaching/journals/{id}/end --

// EndJournal — HANYA pemilik (docs/06-teaching.md "guru boleh menutup manual
// lebih awal").
func (s *Service) EndJournal(ctx context.Context, actorUserID, schoolID, id int64) (JournalView, error) {
	rec, err := s.repo.GetJournalByID(ctx, schoolID, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return JournalView{}, httpx.ErrNotFound
		}
		return JournalView{}, err
	}
	teacherID, ok, err := s.students.MyTeacherID(ctx, schoolID, actorUserID)
	if err != nil {
		return JournalView{}, err
	}
	if !ok || teacherID != rec.TeacherID {
		return JournalView{}, httpx.ErrForbidden
	}
	if !rec.EndedAt.IsZero() {
		return JournalView{}, httpx.Validation("Jurnal sudah diakhiri.")
	}

	now := s.clock.Now()
	if _, err := s.repo.EndJournal(ctx, schoolID, id, now); err != nil {
		if errors.Is(err, ErrNotFound) {
			return JournalView{}, httpx.ErrNotFound
		}
		return JournalView{}, err
	}
	s.audit(ctx, schoolID, actorUserID, "teaching.journal_end", "teaching_journal", id, nil, map[string]any{"ended_at": now})

	detail, err := s.repo.GetJournalDetail(ctx, schoolID, id)
	if err != nil {
		return JournalView{}, err
	}
	s.emitStatus(schoolID, rec.Date)
	return s.journalView(ctx, now, detail)
}

// -- GET /api/teaching/journals --

// ListQuery adalah parameter GET /api/teaching/journals.
type ListQuery struct {
	Scope string // "mine" (default) | "all"
	Date  string // YYYY-MM-DD
	Month string // YYYY-MM
}

// ListJournals — mine: milik sendiri (default); all: perm teaching:monitor
// (docs/06-teaching.md). Urut started_at desc, max 100 (ditegakkan query SQL).
func (s *Service) ListJournals(ctx context.Context, schoolID int64, in ListQuery) ([]JournalView, error) {
	scope := strings.TrimSpace(in.Scope)
	if scope == "" {
		scope = "mine"
	}

	var teacherFilter int64
	switch scope {
	case "mine":
		tid, ok, err := s.students.MyTeacherID(ctx, schoolID, reqctx.UserID(ctx))
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, httpx.ErrForbidden
		}
		teacherFilter = tid
	case "all":
		if !s.identity.HasPermission(reqctx.Role(ctx), PermTeachingMonitor) {
			return nil, httpx.ErrForbidden
		}
	default:
		return nil, httpx.Validation("scope harus mine atau all.")
	}

	var dateFilter time.Time
	if raw := strings.TrimSpace(in.Date); raw != "" {
		t, err := time.Parse("2006-01-02", raw)
		if err != nil {
			return nil, httpx.Validation("Format date harus YYYY-MM-DD.")
		}
		dateFilter = t
	}
	var monthStart, monthEnd time.Time
	if raw := strings.TrimSpace(in.Month); raw != "" {
		t, err := time.Parse("2006-01", raw)
		if err != nil {
			return nil, httpx.Validation("Format month harus YYYY-MM.")
		}
		monthStart = t
		monthEnd = t.AddDate(0, 1, 0)
	}

	details, err := s.repo.ListJournals(ctx, schoolID, teacherFilter, dateFilter, monthStart, monthEnd)
	if err != nil {
		return nil, err
	}
	now := s.clock.Now()
	out := make([]JournalView, 0, len(details))
	for _, d := range details {
		v, err := s.journalView(ctx, now, d)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

// -- GET /api/teaching/status --

// Status — INTI monitoring (docs/06-teaching.md "Status mengajar (derivasi,
// bukan tabel)"). Untuk SETIAP slot jadwal pada tanggal itu (lintas guru):
//
//   - journal ada (schedule_slot_id+date match) -> "mengajar" bila now <=
//     akhir period slot, else "selesai".
//   - tanpa journal & guru punya izin approved tanggal itu -> "izin" (menang
//     atas belum_masuk, dicek SEBELUM aturan waktu — guru cuti logisnya tidak
//     akan datang sama sekali, terlepas dari kapan slot berlangsung).
//   - tanpa journal & slot SUDAH LEWAT (now > akhir period) -> "belum_masuk"
//     ("tetap merah di rekap" — diprioritaskan di atas grace period supaya
//     slot pendek + threshold besar tidak salah kebaca "belum_mulai").
//   - tanpa journal & now < mulai_slot + not_started_after_min -> "belum_mulai"
//     (mencakup baik "belum mencapai jam mulai" MAUPUN grace period toleransi
//     setelah mulai — docs tidak memberi nama terpisah utk grace period,
//     keputusan sendiri: tetap abu-abu sampai threshold lewat, baru merah).
//   - selainnya (now sudah lewat threshold, slot masih berjalan) -> "belum_masuk".
func (s *Service) Status(ctx context.Context, schoolID int64, dateStr string) (StatusResult, error) {
	// Otorisasi teaching:monitor ditegakkan di route level (requirePerm) —
	// lihat routes.go, pola yang sama dengan GET /api/attendance/summary.
	now := s.clock.Now()
	tz := schoolTimezone(ctx)
	date, err := parseDateParam(dateStr, now, tz)
	if err != nil {
		return StatusResult{}, err
	}
	dow := int(date.Weekday())

	slots, err := s.schedule.SlotsForDayOfWeek(ctx, schoolID, dow)
	if err != nil {
		return StatusResult{}, err
	}
	sort.Slice(slots, func(i, j int) bool {
		if slots[i].PeriodStart != slots[j].PeriodStart {
			return slots[i].PeriodStart < slots[j].PeriodStart
		}
		return slots[i].ClassName < slots[j].ClassName
	})

	settings, err := s.repo.GetSettings(ctx, schoolID)
	if err != nil {
		return StatusResult{}, err
	}
	threshold := time.Duration(settings.NotStartedAfterMin) * time.Minute
	dateForLeave := date.Format("2006-01-02")

	rows := make([]StatusRow, 0, len(slots))
	summary := StatusSummary{}
	for _, sl := range slots {
		slotStart := combineLocal(date, sl.PeriodStartsAt, tz)
		slotEnd := combineLocal(date, sl.PeriodEndsAt, tz)

		var journalID *int64
		var roomActual *RoomRef
		var status string

		jd, jerr := s.repo.GetJournalDetailBySlotDate(ctx, schoolID, sl.ID, date)
		switch {
		case jerr == nil:
			id := jd.ID
			journalID = &id
			if jd.RoomID != 0 {
				roomActual = &RoomRef{ID: jd.RoomID, Name: jd.RoomName}
			}
			if now.After(slotEnd) {
				status = StatusSelesai
			} else {
				status = StatusMengajar
			}
		case errors.Is(jerr, ErrNotFound):
			userID, uerr := s.repo.LookupTeacherUserID(ctx, schoolID, sl.TeacherID)
			if uerr != nil {
				return StatusResult{}, uerr
			}
			approved, lerr := s.leave.ApprovedOn(ctx, schoolID, userID, dateForLeave)
			if lerr != nil {
				return StatusResult{}, lerr
			}
			switch {
			case approved:
				status = StatusIzin
			case now.After(slotEnd):
				status = StatusBelumMasuk
			case now.Before(slotStart.Add(threshold)):
				status = StatusBelumMulai
			default:
				status = StatusBelumMasuk
			}
		default:
			return StatusResult{}, jerr
		}

		switch status {
		case StatusMengajar:
			summary.Mengajar++
		case StatusBelumMasuk:
			summary.BelumMasuk++
		case StatusIzin:
			summary.Izin++
		case StatusBelumMulai:
			summary.BelumMulai++
		case StatusSelesai:
			summary.Selesai++
		}

		// Fase 14 Gelombang D: bila ada guru pengganti ACCEPTED utk slot+tanggal
		// ini, nama guru yang tampil di monitoring/TV adalah pengganti (docs
		// tugas: "teacher_name tampil {pengganti} (pengganti)") — ID slot
		// pemilik TETAP sl.TeacherID (kepemilikan jadwal tidak berubah, murni
		// perubahan tampilan siapa yang benar-benar mengajar).
		teacherRef := TeacherRef{ID: sl.TeacherID, Name: sl.TeacherName}
		if s.substitutions != nil {
			if name, ok, serr := s.substitutions.SubstituteName(ctx, schoolID, sl.ID, dateForLeave); serr != nil {
				return StatusResult{}, serr
			} else if ok {
				teacherRef.Name = name + " (pengganti)"
			}
		}

		rows = append(rows, StatusRow{
			Slot: StatusSlot{
				ID: sl.ID, Class: ClassRef{ID: sl.ClassID, Name: sl.ClassName},
				Subject:   SubjectRef{ID: sl.SubjectID, Code: sl.SubjectCode, Name: sl.SubjectName},
				Room:      slotRoomRef(sl),
				DayOfWeek: sl.DayOfWeek, PeriodStart: sl.PeriodStart, PeriodEnd: sl.PeriodEnd,
				PeriodStartsAt: sl.PeriodStartsAt, PeriodEndsAt: sl.PeriodEndsAt,
			},
			Teacher:    teacherRef,
			Status:     status,
			JournalID:  journalID,
			RoomActual: roomActual,
		})
	}

	var cp *CurrentPeriodInfo
	if num, startsAt, endsAt, ok, err := s.schedule.CurrentPeriod(ctx, schoolID, now); err != nil {
		return StatusResult{}, err
	} else if ok {
		cp = &CurrentPeriodInfo{Number: num, StartsAt: startsAt, EndsAt: endsAt}
	}

	return StatusResult{CurrentPeriod: cp, Summary: summary, Rows: rows}, nil
}

// -- GET /api/teaching/compliance --

// round1 membulatkan ke 1 desimal (dipakai persentase pct/material_pct).
func round1(v float64) float64 { return math.Round(v*10) / 10 }

// Compliance — rekap ketertiban mengajar per guru pada rentang tanggal
// (fase 7, docs/06-teaching.md "Dashboard kepala sekolah": "% slot
// terlaksana (journal vs jadwal) per guru"). Default rentang: 30 hari
// terakhir (s.d. hari ini, tanggal lokal sekolah).
//
//   - scheduled: jumlah SLOT TERJADWAL pada hari-hari dalam rentang — dihitung
//     dengan mengiterasi setiap tanggal, mengambil day_of_week-nya, lalu
//     menjumlah semua slot ScheduleGateway.SlotsForDayOfWeek pada day_of_week
//     itu per guru (slot Senin dihitung SEKALI per Senin dalam rentang — jadi
//     4 Senin dalam sebulan = scheduled bertambah 4x utk slot yang sama).
//     TIDAK memfilter hari libur/kalender akademik (keputusan sendiri:
//     "sederhana: hitung semua hari kerja yang match day_of_week slot dalam
//     rentang", sesuai instruksi tugas).
//   - taught/unscheduled/material_filled: dari teaching_journals sendiri
//     (repo.JournalComplianceCounts), TIDAK ada logika derivasi jadwal
//     diduplikasi di sini — scheduled murni dari ScheduleGateway yang sudah
//     ada (dipakai juga oleh Status).
//   - pct = taught/scheduled*100 (0 bila scheduled=0); material_pct =
//     material_filled/taught*100 (0 bila taught=0) — keputusan sendiri:
//     material_pct dihitung terhadap slot yang SUDAH diajar (taught), bukan
//     terhadap scheduled, karena guru hanya bisa mengisi materi pada jurnal
//     yang memang ada.
//   - Guru TANPA scheduled DAN TANPA journal apa pun pada rentang ini
//     di-SKIP (tidak relevan ditampilkan di rekap — pct=0 tanpa data akan
//     salah kebaca "paling bolong").
//   - Urut pct ASC (paling bolong di atas), lalu nama guru sbg tie-breaker.
func (s *Service) Compliance(ctx context.Context, schoolID int64, fromStr, toStr string) ([]ComplianceView, error) {
	now := s.clock.Now()
	tz := schoolTimezone(ctx)

	to := schoolToday(now, tz)
	if v := strings.TrimSpace(toStr); v != "" {
		t, err := time.Parse("2006-01-02", v)
		if err != nil {
			return nil, httpx.Validation("Format 'to' harus YYYY-MM-DD.")
		}
		to = t
	}
	from := to.AddDate(0, 0, -30)
	if v := strings.TrimSpace(fromStr); v != "" {
		t, err := time.Parse("2006-01-02", v)
		if err != nil {
			return nil, httpx.Validation("Format 'from' harus YYYY-MM-DD.")
		}
		from = t
	}
	if from.After(to) {
		return nil, httpx.Validation("'from' tidak boleh setelah 'to'.")
	}

	scheduledByTeacher := map[int64]int{}
	teacherNames := map[int64]string{}
	slotsByDow := map[int][]SlotInfo{}
	for d := from; !d.After(to); d = d.AddDate(0, 0, 1) {
		dow := int(d.Weekday())
		slots, ok := slotsByDow[dow]
		if !ok {
			var err error
			slots, err = s.schedule.SlotsForDayOfWeek(ctx, schoolID, dow)
			if err != nil {
				return nil, err
			}
			slotsByDow[dow] = slots
		}
		for _, sl := range slots {
			scheduledByTeacher[sl.TeacherID]++
			teacherNames[sl.TeacherID] = sl.TeacherName
		}
	}

	counts, err := s.repo.JournalComplianceCounts(ctx, schoolID, from, to)
	if err != nil {
		return nil, err
	}
	type agg struct{ taught, unscheduled, materialFilled int64 }
	countsByTeacher := make(map[int64]agg, len(counts))
	for _, c := range counts {
		countsByTeacher[c.TeacherID] = agg{c.Taught, c.Unscheduled, c.MaterialFilled}
		if _, ok := teacherNames[c.TeacherID]; !ok {
			if ref, rerr := s.repo.LookupTeacherRef(ctx, schoolID, c.TeacherID); rerr == nil {
				teacherNames[c.TeacherID] = ref.Name
			}
		}
	}

	teacherIDs := make(map[int64]bool, len(scheduledByTeacher)+len(countsByTeacher))
	for id := range scheduledByTeacher {
		teacherIDs[id] = true
	}
	for id := range countsByTeacher {
		teacherIDs[id] = true
	}

	out := make([]ComplianceView, 0, len(teacherIDs))
	for id := range teacherIDs {
		scheduled := scheduledByTeacher[id]
		a := countsByTeacher[id]

		pct := 0.0
		if scheduled > 0 {
			pct = round1(float64(a.taught) / float64(scheduled) * 100)
		}
		materialPct := 0.0
		if a.taught > 0 {
			materialPct = round1(float64(a.materialFilled) / float64(a.taught) * 100)
		}

		out = append(out, ComplianceView{
			Teacher:        TeacherRef{ID: id, Name: teacherNames[id]},
			Scheduled:      scheduled,
			Taught:         int(a.taught),
			Pct:            pct,
			Unscheduled:    int(a.unscheduled),
			MaterialFilled: int(a.materialFilled),
			MaterialPct:    materialPct,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Pct != out[j].Pct {
			return out[i].Pct < out[j].Pct
		}
		return out[i].Teacher.Name < out[j].Teacher.Name
	})
	return out, nil
}
