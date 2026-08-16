package exitpermit

import (
	"context"
	"crypto/rand"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/omanjaya/nouschool/internal/platform/clock"
	"github.com/omanjaya/nouschool/internal/platform/httpx"
	"github.com/omanjaya/nouschool/internal/platform/reqctx"
)

// -- consumer-side interface (lihat CLAUDE.md). Semua dipenuhi
// *identity.Service / *tenant.Service / *student.Service / *duty.Service /
// *teacherqr.Service secara STRUKTURAL, KECUALI ScheduleGateway (butuh
// adapter kecil cmd/server/b2adapter.go — lihat komentar interface itu).

type IdentityGateway interface {
	HasPermission(role, perm string) bool
	Log(ctx context.Context, schoolID, userID *int64, action, entity string, entityID *int64, oldValue, newValue any) error
}

type AcademicYearLookup interface {
	ActiveAcademicYearID(ctx context.Context, schoolID int64) (id int64, ok bool, err error)
}

// StudentAccess adalah kebutuhan modul exitpermit dari modul student.
type StudentAccess interface {
	CanViewStudent(ctx context.Context, userID int64, role string, schoolID, studentID int64) error
	GuardianUserIDs(ctx context.Context, schoolID, studentID int64) ([]int64, error)
	MyStudentID(ctx context.Context, schoolID, userID int64) (studentID, classID int64, ok bool, err error)
	MyChildStudentIDs(ctx context.Context, schoolID, userID int64) ([]int64, error)
}

// DutyGateway adalah kebutuhan modul exitpermit dari modul duty: otorisasi
// tahap piket/BK/pimpinan/security via CAPABILITY FLAGS.
type DutyGateway interface {
	UserHasFlag(ctx context.Context, schoolID, userID int64, flag string) (bool, error)
}

// ScheduleGateway adalah kebutuhan modul exitpermit dari modul schedule
// (dijembatani adapter cmd/server/b2adapter.go — signature di sini SUDAH
// primitif murni, tapi implementasinya butuh MENGGABUNGKAN
// schedule.Service.ClassSlotNowOrNext + student.Service.MyTeacherID, dua
// modul sekaligus, karenanya tetap lewat adapter, pola sama
// scheduleadapter.go).
type ScheduleGateway interface {
	// TeacherMatchesClassNow — matches=true bila teacherUserID adalah guru
	// yang sedang/berikutnya mengajar classID hari ini (waktu lokal sekolah
	// `at`). hasSlot=false bila TIDAK ADA slot kelas ini sekarang/berikutnya
	// hari itu sama sekali (beda pesan error dari "guru salah").
	TeacherMatchesClassNow(ctx context.Context, schoolID, classID, teacherUserID int64, at time.Time) (matches, hasSlot bool, err error)
	// GateExpiryToday — akhir period TERAKHIR hari ini (waktu lokal sekolah
	// `at`), fallback at+6 jam bila sekolah belum punya period.
	GateExpiryToday(ctx context.Context, schoolID int64, at time.Time) (time.Time, error)
}

// TeacherQRGateway adalah kebutuhan modul exitpermit dari modul teacherqr.
type TeacherQRGateway interface {
	ConsumeToken(ctx context.Context, schoolID int64, rawToken string) (teacherUserID int64, err error)
}

// Notifier adalah kebutuhan modul exitpermit dari modul notification.
type Notifier interface {
	Notify(ctx context.Context, schoolID int64, event string, userIDs []int64, data map[string]any) error
}

// Realtime adalah kebutuhan modul exitpermit dari modul realtime.
type Realtime interface {
	PublishTo(schoolID int64, eventType string, data map[string]any, roles []string, userIDs []int64)
}

var ErrNoActiveAcademicYear = httpx.Validation("Sekolah belum punya tahun ajaran aktif. Aktifkan dulu di menu tahun ajaran.")

// Service berisi aturan bisnis modul exitpermit.
type Service struct {
	repo      exitPermitRepository
	identity  IdentityGateway
	years     AcademicYearLookup
	students  StudentAccess
	duties    DutyGateway
	schedule  ScheduleGateway
	teacherqr TeacherQRGateway
	clock     clock.Clock
	notifier  Notifier
	realtime  Realtime
}

func NewService(repo *Repository, identity IdentityGateway, years AcademicYearLookup, students StudentAccess, duties DutyGateway, schedule ScheduleGateway, teacherqr TeacherQRGateway, clk clock.Clock) *Service {
	if clk == nil {
		clk = clock.System{}
	}
	return &Service{repo: repo, identity: identity, years: years, students: students, duties: duties, schedule: schedule, teacherqr: teacherqr, clock: clk}
}

func (s *Service) SetNotifier(n Notifier) { s.notifier = n }
func (s *Service) SetRealtime(r Realtime) { s.realtime = r }

// newServiceForTest membangun Service dengan repository FAKE (in-memory,
// tanpa DB) — dipakai test di package ini saja.
func newServiceForTest(repo exitPermitRepository, identity IdentityGateway, years AcademicYearLookup, students StudentAccess, duties DutyGateway, schedule ScheduleGateway, teacherqr TeacherQRGateway, clk clock.Clock) *Service {
	return &Service{repo: repo, identity: identity, years: years, students: students, duties: duties, schedule: schedule, teacherqr: teacherqr, clock: clk}
}

func (s *Service) audit(ctx context.Context, schoolID, actorUserID int64, action, entity string, entityID int64, oldValue, newValue any) {
	sid, uid, eid := schoolID, actorUserID, entityID
	_ = s.identity.Log(ctx, &sid, &uid, action, entity, &eid, oldValue, newValue)
}

func (s *Service) notify(ctx context.Context, schoolID int64, event string, userIDs []int64, data map[string]any) {
	if s.notifier == nil || len(userIDs) == 0 {
		return
	}
	_ = s.notifier.Notify(ctx, schoolID, event, userIDs, data)
}

func (s *Service) emit(schoolID, permitID int64, extraUserIDs []int64) {
	if s.realtime == nil {
		return
	}
	s.realtime.PublishTo(schoolID, "exitpermit", map[string]any{"permit_id": permitID},
		[]string{RoleAdminSekolah, RoleKepalaSekolah}, extraUserIDs)
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

func dedupInt64(ids ...int64) []int64 {
	out := make([]int64, 0, len(ids))
	seen := make(map[int64]bool, len(ids))
	for _, id := range ids {
		if id == 0 || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}

func generateGateToken() (string, error) {
	const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	const length = 24
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	out := make([]byte, length)
	for i, v := range b {
		out[i] = chars[int(v)%len(chars)]
	}
	return string(out), nil
}

// -- POST /api/exit-permits (siswa) --

var errMaxActivePermit = httpx.Conflict("Anda sudah punya pengajuan dispensasi keluar yang masih aktif.")

// SubmitRequest — HANYA siswa mengajukan untuk dirinya sendiri (docs tugas:
// beda dari studentleave yang juga menerima orang tua). Maks 1 permit AKTIF
// (pending_*/issued) per siswa.
func (s *Service) SubmitRequest(ctx context.Context, actorUserID, schoolID int64, reason string) (PermitView, error) {
	if reqctx.Role(ctx) != RoleSiswa {
		return PermitView{}, httpx.ErrForbidden
	}
	studentID, _, ok, err := s.students.MyStudentID(ctx, schoolID, actorUserID)
	if err != nil {
		return PermitView{}, err
	}
	if !ok {
		return PermitView{}, httpx.Validation("Akun Anda belum tertaut ke data siswa.")
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return PermitView{}, httpx.Validation("Alasan wajib diisi.")
	}

	yearID, err := s.requireActiveYear(ctx, schoolID)
	if err != nil {
		return PermitView{}, err
	}

	count, err := s.repo.CountActive(ctx, schoolID, studentID)
	if err != nil {
		return PermitView{}, err
	}
	if count > 0 {
		return PermitView{}, errMaxActivePermit
	}

	id, err := s.repo.CreateRequest(ctx, schoolID, yearID, studentID, reason)
	if err != nil {
		return PermitView{}, err
	}
	detail, err := s.repo.GetDetail(ctx, schoolID, id)
	if err != nil {
		return PermitView{}, err
	}
	s.audit(ctx, schoolID, actorUserID, "exitpermit.submit", "student_exit_permit", id, nil, map[string]any{"student_id": studentID, "reason": reason})
	s.emit(schoolID, id, []int64{actorUserID})
	return detail.View(""), nil
}

// -- POST /api/exit-permits/{id}/scan (siswa pemilik) --

var errNotPending = httpx.Validation("Pengajuan ini sudah tidak menunggu scan (mungkin ditolak/dibatalkan/sudah selesai).")
var errNotOwner = httpx.ErrForbidden
var errWrongDuty = httpx.Validation("Guru ini bukan pemegang tugas Guru Piket.")
var errSamePerson = httpx.Validation("Guru pengajar tahap ini harus berbeda dari guru piket tahap sebelumnya.")
var errNoClassSlot = httpx.Validation("Tidak ada guru yang mengajar kelas Anda sekarang atau berikutnya hari ini.")
var errWrongClassTeacher = httpx.Validation("Guru ini tidak sesuai jadwal mengajar kelas Anda saat ini/berikutnya.")
var errWrongBK = httpx.Validation("Guru ini bukan pemegang tugas BK untuk dispensasi keluar.")
var errWrongLeadership = httpx.Validation("Guru ini bukan pemegang tugas Pimpinan.")
var errStageRace = httpx.Validation("Gagal menyimpan (kemungkinan sudah diproses pihak lain). Muat ulang.")

// Scan — POST /api/exit-permits/{id}/scan {token}: maju SATU tahap sesuai
// status pengajuan saat ini (state machine SION, lihat model.go). SELALU
// dipanggil siswa PEMILIK permit (bukan orang tua) — siswa yang berjalan
// membawa HP-nya sendiri ke guru untuk discan.
func (s *Service) Scan(ctx context.Context, actorUserID, schoolID, permitID int64, rawToken string) (PermitView, error) {
	if reqctx.Role(ctx) != RoleSiswa {
		return PermitView{}, errNotOwner
	}
	studentID, classID, ok, err := s.students.MyStudentID(ctx, schoolID, actorUserID)
	if err != nil {
		return PermitView{}, err
	}
	if !ok {
		return PermitView{}, httpx.Validation("Akun Anda belum tertaut ke data siswa.")
	}
	detail, err := s.repo.GetDetail(ctx, schoolID, permitID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return PermitView{}, httpx.ErrNotFound
		}
		return PermitView{}, err
	}
	if detail.StudentID != studentID {
		return PermitView{}, errNotOwner
	}
	if !isPending(detail.Status) {
		return PermitView{}, errNotPending
	}

	teacherUserID, err := s.teacherqr.ConsumeToken(ctx, schoolID, rawToken)
	if err != nil {
		return PermitView{}, err
	}

	now := s.clock.Now()
	var rows int64
	var nextStatus string

	switch detail.Status {
	case StatusPendingDutyTeacher:
		hasFlag, ferr := s.duties.UserHasFlag(ctx, schoolID, teacherUserID, FlagLateArrivalDuty)
		if ferr != nil {
			return PermitView{}, ferr
		}
		if !hasFlag {
			return PermitView{}, errWrongDuty
		}
		nextStatus = StatusPendingClassTeacher
		rows, err = s.repo.UpdateDutyStage(ctx, schoolID, permitID, teacherUserID)

	case StatusPendingClassTeacher:
		if teacherUserID == detail.DutyBy {
			return PermitView{}, errSamePerson
		}
		matches, hasSlot, serr := s.schedule.TeacherMatchesClassNow(ctx, schoolID, classID, teacherUserID, now)
		if serr != nil {
			return PermitView{}, serr
		}
		if !hasSlot {
			return PermitView{}, errNoClassSlot
		}
		if !matches {
			return PermitView{}, errWrongClassTeacher
		}
		nextStatus = StatusPendingBK
		rows, err = s.repo.UpdateClassStage(ctx, schoolID, permitID, teacherUserID)

	case StatusPendingBK:
		hasFlag, ferr := s.duties.UserHasFlag(ctx, schoolID, teacherUserID, FlagExitBKApproval)
		if ferr != nil {
			return PermitView{}, ferr
		}
		if !hasFlag {
			return PermitView{}, errWrongBK
		}
		nextStatus = StatusPendingLeadership
		rows, err = s.repo.UpdateBKStage(ctx, schoolID, permitID, teacherUserID)

	case StatusPendingLeadership:
		hasFlag, ferr := s.duties.UserHasFlag(ctx, schoolID, teacherUserID, FlagExitLeadershipApproval)
		if ferr != nil {
			return PermitView{}, ferr
		}
		if !hasFlag {
			return PermitView{}, errWrongLeadership
		}
		gateExpiresAt, gerr := s.schedule.GateExpiryToday(ctx, schoolID, now)
		if gerr != nil {
			return PermitView{}, gerr
		}
		gateToken, terr := generateGateToken()
		if terr != nil {
			return PermitView{}, terr
		}
		nextStatus = StatusIssued
		rows, err = s.repo.UpdateLeadershipStage(ctx, schoolID, permitID, teacherUserID, gateToken, gateExpiresAt)
	}
	if err != nil {
		return PermitView{}, err
	}
	if rows == 0 {
		return PermitView{}, errStageRace
	}

	s.audit(ctx, schoolID, actorUserID, "exitpermit.scan", "student_exit_permit", permitID,
		map[string]any{"status": detail.Status}, map[string]any{"status": nextStatus, "by": teacherUserID})

	updated, err := s.repo.GetDetail(ctx, schoolID, permitID)
	if err != nil {
		return PermitView{}, err
	}

	if nextStatus == StatusIssued {
		// detail.StudentID adalah id PROFIL siswa, bukan user_id akun login
		// — target notif siswa dipakai actorUserID (akun yang scan).
		guardians, _ := s.students.GuardianUserIDs(ctx, schoolID, detail.StudentID)
		recipients := dedupInt64(append([]int64{actorUserID}, guardians...)...)
		s.notify(ctx, schoolID, EventExitPermitIssued, recipients, map[string]any{"student": detail.StudentName})
		s.emit(schoolID, permitID, recipients)
	} else {
		s.emit(schoolID, permitID, []int64{actorUserID, teacherUserID})
	}

	return updated.View(nextStatus), nil
}

// -- POST /api/exit-permits/{id}/cancel (pemilik, pending saja) --

func (s *Service) CancelRequest(ctx context.Context, actorUserID, schoolID, permitID int64) error {
	studentID, _, ok, err := s.students.MyStudentID(ctx, schoolID, actorUserID)
	if err != nil {
		return err
	}
	if !ok {
		return httpx.ErrForbidden
	}
	detail, err := s.repo.GetDetail(ctx, schoolID, permitID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return httpx.ErrNotFound
		}
		return err
	}
	if detail.StudentID != studentID {
		return httpx.ErrForbidden
	}
	if !isPending(detail.Status) {
		return httpx.Validation("Hanya pengajuan yang masih menunggu yang bisa dibatalkan.")
	}
	rows, err := s.repo.Cancel(ctx, schoolID, permitID)
	if err != nil {
		return err
	}
	if rows == 0 {
		return httpx.Validation("Pengajuan sudah tidak bisa dibatalkan (mungkin baru saja diproses).")
	}
	s.audit(ctx, schoolID, actorUserID, "exitpermit.cancel", "student_exit_permit", permitID,
		map[string]any{"status": detail.Status}, map[string]any{"status": StatusCanceled})
	s.emit(schoolID, permitID, []int64{actorUserID})
	return nil
}

// -- POST /api/exit-permits/{id}/reject (admin ATAU approver sah tahap
// berjalan — Fase 15 GAP 3) --
//
// Riwayat (Fase 14 Gelombang B2): alur rantai QR SION awalnya TIDAK punya
// endpoint "reject" formal per tahap approver — hanya admin_sekolah (perm
// student:manage) yang punya jalur override administratif. Fase 15 GAP 3
// (docs tugas) MEMPERLUAS ini: approver SAH TAHAP BERJALAN sekarang JUGA
// boleh menolak — validasi otorisasi IDENTIK dengan Scan (lihat
// requireRejectAuthority) TAPI TANPA token (aktor sudah teridentifikasi
// lewat sesi login, bukan scan QR guru). Perilaku admin (bebas tahap mana
// pun sebelum exited) TETAP seperti semula.
var errCommentRequired = httpx.Validation("Komentar wajib diisi.")

// errWrongClassTeacherReject — pesan KHUSUS reject tahap 2 (beda dari Scan
// karena di sini tidak ada errNoClassSlot terpisah, satu pesan cukup).
var errWrongClassTeacherReject = httpx.Validation("Anda bukan guru pengajar kelas siswa ini saat ini/berikutnya, atau sama dengan guru piket tahap 1.")

// requireRejectAuthority menegakkan otorisasi Fase 15 GAP 3: admin
// (student:manage) SELALU boleh, tahap status apa pun (perilaku lama TETAP).
// SELAIN admin, HANYA approver SAH TAHAP BERJALAN yang boleh reject —
// validasi PERSIS sama dengan Scan per tahap (lihat switch di Scan), TANPA
// mengonsumsi token QR (aktor sudah pasti guru login, bukan hasil scan).
func (s *Service) requireRejectAuthority(ctx context.Context, actorUserID, schoolID int64, detail Detail, now time.Time) error {
	if s.identity.HasPermission(reqctx.Role(ctx), PermStudentManage) {
		return nil
	}
	switch detail.Status {
	case StatusPendingDutyTeacher:
		hasFlag, err := s.duties.UserHasFlag(ctx, schoolID, actorUserID, FlagLateArrivalDuty)
		if err != nil {
			return err
		}
		if !hasFlag {
			return httpx.ErrForbidden
		}
		return nil

	case StatusPendingClassTeacher:
		if actorUserID == detail.DutyBy {
			return errWrongClassTeacherReject
		}
		classID, ok, err := s.repo.GetStudentClassID(ctx, schoolID, detail.AcademicYearID, detail.StudentID)
		if err != nil {
			return err
		}
		if !ok {
			return errWrongClassTeacherReject
		}
		matches, hasSlot, err := s.schedule.TeacherMatchesClassNow(ctx, schoolID, classID, actorUserID, now)
		if err != nil {
			return err
		}
		if !hasSlot || !matches {
			return errWrongClassTeacherReject
		}
		return nil

	case StatusPendingBK:
		hasFlag, err := s.duties.UserHasFlag(ctx, schoolID, actorUserID, FlagExitBKApproval)
		if err != nil {
			return err
		}
		if !hasFlag {
			return httpx.ErrForbidden
		}
		return nil

	case StatusPendingLeadership:
		hasFlag, err := s.duties.UserHasFlag(ctx, schoolID, actorUserID, FlagExitLeadershipApproval)
		if err != nil {
			return err
		}
		if !hasFlag {
			return httpx.ErrForbidden
		}
		return nil

	default:
		return httpx.ErrForbidden
	}
}

func (s *Service) RejectRequest(ctx context.Context, actorUserID, schoolID, permitID int64, comment string) (PermitView, error) {
	detail, err := s.repo.GetDetail(ctx, schoolID, permitID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return PermitView{}, httpx.ErrNotFound
		}
		return PermitView{}, err
	}
	if isFinal(detail.Status) {
		return PermitView{}, httpx.Validation("Pengajuan ini sudah final (exited/rejected/canceled).")
	}
	comment = strings.TrimSpace(comment)
	if comment == "" {
		return PermitView{}, errCommentRequired
	}
	if err := s.requireRejectAuthority(ctx, actorUserID, schoolID, detail, s.clock.Now()); err != nil {
		return PermitView{}, err
	}
	rows, err := s.repo.Reject(ctx, schoolID, permitID, actorUserID, comment)
	if err != nil {
		return PermitView{}, err
	}
	if rows == 0 {
		return PermitView{}, errStageRace
	}
	s.audit(ctx, schoolID, actorUserID, "exitpermit.reject", "student_exit_permit", permitID,
		map[string]any{"status": detail.Status}, map[string]any{"status": StatusRejected, "comment": comment})
	updated, err := s.repo.GetDetail(ctx, schoolID, permitID)
	if err != nil {
		return PermitView{}, err
	}

	// Notifikasi BARU exitpermit.rejected -> siswa + ortu (docs tugas GAP 3).
	guardians, _ := s.students.GuardianUserIDs(ctx, schoolID, detail.StudentID)
	studentUserID, uerr := s.repo.GetStudentUserID(ctx, schoolID, detail.StudentID)
	recipients := guardians
	if uerr == nil && studentUserID != 0 {
		recipients = append(recipients, studentUserID)
	}
	recipients = dedupInt64(recipients...)
	s.notify(ctx, schoolID, EventExitPermitRejected, recipients, map[string]any{"student": detail.StudentName, "comment": comment})
	s.emit(schoolID, permitID, recipients)
	return updated.View(""), nil
}

// -- GET /api/exit-permits?scope=mine|active|all&date= --

func (s *Service) schoolTimezone(ctx context.Context) string {
	sch, _ := reqctx.SchoolFromContext(ctx)
	return sch.Timezone
}

func localDayRangeUTC(t time.Time, tz string) (time.Time, time.Time) {
	local := clock.InZone(t, tz)
	loc := local.Location()
	start := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, loc)
	return start.UTC(), start.AddDate(0, 0, 1).UTC()
}

func parseLocalDate(raw, tz string) (time.Time, time.Time, error) {
	t, err := time.Parse("2006-01-02", strings.TrimSpace(raw))
	if err != nil {
		return time.Time{}, time.Time{}, httpx.Validation("Format tanggal harus YYYY-MM-DD.")
	}
	loc, lerr := time.LoadLocation(tz)
	if lerr != nil {
		loc = time.FixedZone("WIB", 7*3600)
	}
	start := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, loc)
	return start.UTC(), start.AddDate(0, 0, 1).UTC(), nil
}

func (s *Service) ListRequests(ctx context.Context, actorUserID, schoolID int64, scope, date string) (PermitPage, error) {
	role := reqctx.Role(ctx)
	var details []Detail
	var err error

	switch scope {
	case "", "mine":
		var studentIDs []int64
		switch role {
		case RoleSiswa:
			id, _, ok, serr := s.students.MyStudentID(ctx, schoolID, actorUserID)
			if serr != nil {
				return PermitPage{}, serr
			}
			if ok {
				studentIDs = []int64{id}
			}
		case RoleOrangTua:
			ids, serr := s.students.MyChildStudentIDs(ctx, schoolID, actorUserID)
			if serr != nil {
				return PermitPage{}, serr
			}
			studentIDs = ids
		}
		if len(studentIDs) == 0 {
			return PermitPage{Items: []PermitView{}}, nil
		}
		details, err = s.repo.ListForStudents(ctx, schoolID, studentIDs)

	case "active":
		if !s.identity.HasPermission(role, PermStudentRead) {
			return PermitPage{}, httpx.ErrForbidden
		}
		tz := s.schoolTimezone(ctx)
		var from, to time.Time
		if date != "" {
			from, to, err = parseLocalDate(date, tz)
			if err != nil {
				return PermitPage{}, err
			}
		} else {
			from, to = localDayRangeUTC(s.clock.Now(), tz)
		}
		details, err = s.repo.ListActiveToday(ctx, schoolID, from, to)

	case "all":
		if !s.identity.HasPermission(role, PermStudentManage) {
			return PermitPage{}, httpx.ErrForbidden
		}
		var fromPtr, toPtr *time.Time
		if date != "" {
			from, to, derr := parseLocalDate(date, s.schoolTimezone(ctx))
			if derr != nil {
				return PermitPage{}, derr
			}
			fromPtr, toPtr = &from, &to
		}
		details, err = s.repo.ListAll(ctx, schoolID, fromPtr, toPtr)

	default:
		return PermitPage{}, httpx.Validation("scope harus 'mine', 'active', atau 'all'.")
	}
	if err != nil {
		return PermitPage{}, err
	}

	items := make([]PermitView, 0, len(details))
	for _, d := range details {
		items = append(items, d.View(""))
	}
	return PermitPage{Items: items, Total: int64(len(items))}, nil
}

// -- POST /api/exit-permits/gate-scan (flag exit_security) --

var errGateNotFound = httpx.ErrNotFound
var errGateAlreadyExited = httpx.Conflict("Izin ini sudah pernah keluar gerbang.")
var errGateNotIssued = httpx.Conflict("Izin ini belum diterbitkan (belum lolos seluruh tahap persetujuan).")
var errGateExpired = &httpx.Error{Status: http.StatusGone, Code: "gate_expired", Message: "Izin sudah kedaluwarsa."}

func (s *Service) GateScan(ctx context.Context, actorUserID, schoolID int64, gateToken string) (GateExitView, error) {
	hasFlag, err := s.duties.UserHasFlag(ctx, schoolID, actorUserID, FlagExitSecurity)
	if err != nil {
		return GateExitView{}, err
	}
	if !hasFlag {
		return GateExitView{}, httpx.ErrForbidden
	}
	gateToken = strings.TrimSpace(gateToken)
	if gateToken == "" {
		return GateExitView{}, httpx.Validation("gate_token wajib diisi.")
	}
	detail, ok, err := s.repo.GetByGateToken(ctx, schoolID, gateToken)
	if err != nil {
		return GateExitView{}, err
	}
	if !ok {
		return GateExitView{}, errGateNotFound
	}
	if detail.Status == StatusExited {
		return GateExitView{}, errGateAlreadyExited
	}
	if detail.Status != StatusIssued {
		return GateExitView{}, errGateNotIssued
	}
	if !detail.GateExpiresAt.IsZero() && detail.GateExpiresAt.Before(s.clock.Now()) {
		return GateExitView{}, errGateExpired
	}

	rows, err := s.repo.GateScan(ctx, schoolID, gateToken, actorUserID)
	if err != nil {
		return GateExitView{}, err
	}
	if rows == 0 {
		return GateExitView{}, errGateAlreadyExited
	}
	updated, foundAfter, err := s.repo.GetByGateToken(ctx, schoolID, gateToken)
	if err != nil {
		return GateExitView{}, err
	}
	if !foundAfter {
		return GateExitView{}, errGateNotFound
	}

	s.audit(ctx, schoolID, actorUserID, "exitpermit.gate_scan", "student_exit_permit", updated.ID,
		map[string]any{"status": StatusIssued}, map[string]any{"status": StatusExited})

	guardians, _ := s.students.GuardianUserIDs(ctx, schoolID, updated.StudentID)
	tz := s.schoolTimezone(ctx)
	jam := clock.InZone(updated.ExitedAt, tz).Format("15:04")
	s.notify(ctx, schoolID, EventExitPermitExited, dedupInt64(guardians...), map[string]any{"student": updated.StudentName, "time": jam})
	s.emit(schoolID, updated.ID, dedupInt64(guardians...))

	return GateExitView{
		Student:       StudentRef{ID: updated.StudentID, Name: updated.StudentName, NIS: updated.StudentNIS, ClassName: updated.ClassName},
		Reason:        updated.Reason,
		IssuedAt:      updated.LeadershipAt,
		GateExpiresAt: updated.GateExpiresAt,
		ExitedAt:      updated.ExitedAt,
	}, nil
}

// -- GET /api/exit-permits/gate-history?date= (flag exit_security ATAU student:read) --

func (s *Service) GateHistory(ctx context.Context, actorUserID, schoolID int64, date string) ([]GateHistoryRow, error) {
	role := reqctx.Role(ctx)
	hasFlag, err := s.duties.UserHasFlag(ctx, schoolID, actorUserID, FlagExitSecurity)
	if err != nil {
		return nil, err
	}
	if !hasFlag && !s.identity.HasPermission(role, PermStudentRead) {
		return nil, httpx.ErrForbidden
	}
	tz := s.schoolTimezone(ctx)
	var from, to time.Time
	if date != "" {
		from, to, err = parseLocalDate(date, tz)
		if err != nil {
			return nil, err
		}
	} else {
		from, to = localDayRangeUTC(s.clock.Now(), tz)
	}
	details, err := s.repo.ListExitedOnDate(ctx, schoolID, from, to)
	if err != nil {
		return nil, err
	}
	out := make([]GateHistoryRow, 0, len(details))
	for _, d := range details {
		out = append(out, GateHistoryRow{
			ID:       d.ID,
			Student:  StudentRef{ID: d.StudentID, Name: d.StudentName, NIS: d.StudentNIS, ClassName: d.ClassName},
			Reason:   d.Reason,
			ExitedAt: d.ExitedAt,
		})
	}
	return out, nil
}
