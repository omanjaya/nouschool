package latearrival

import (
	"context"
	"strings"
	"time"

	"github.com/omanjaya/nouschool/internal/platform/clock"
	"github.com/omanjaya/nouschool/internal/platform/httpx"
	"github.com/omanjaya/nouschool/internal/platform/reqctx"
)

// -- consumer-side interface (lihat CLAUDE.md) --

type IdentityGateway interface {
	HasPermission(role, perm string) bool
	Log(ctx context.Context, schoolID, userID *int64, action, entity string, entityID *int64, oldValue, newValue any) error
}

type AcademicYearLookup interface {
	ActiveAcademicYearID(ctx context.Context, schoolID int64) (id int64, ok bool, err error)
}

type StudentAccess interface {
	GuardianUserIDs(ctx context.Context, schoolID, studentID int64) ([]int64, error)
	MyStudentID(ctx context.Context, schoolID, userID int64) (studentID, classID int64, ok bool, err error)
	MyChildStudentIDs(ctx context.Context, schoolID, userID int64) ([]int64, error)
}

type DutyGateway interface {
	UserHasFlag(ctx context.Context, schoolID, userID int64, flag string) (bool, error)
}

// ScheduleGateway adalah kebutuhan modul latearrival dari modul schedule
// (dijembatani adapter cmd/server/b2adapter.go, pola sama exitpermit).
type ScheduleGateway interface {
	// TeacherTeachesClassToday — true bila teacherUserID punya SETIDAKNYA
	// SATU slot mengajar classID hari ini (waktu lokal sekolah `at`), TANPA
	// syarat jam berjalan/berikutnya (beda dari exitpermit.ScheduleGateway).
	TeacherTeachesClassToday(ctx context.Context, schoolID, classID, teacherUserID int64, at time.Time) (bool, error)
}

type TeacherQRGateway interface {
	ConsumeToken(ctx context.Context, schoolID int64, rawToken string) (teacherUserID int64, err error)
}

type Notifier interface {
	Notify(ctx context.Context, schoolID int64, event string, userIDs []int64, data map[string]any) error
}

type Realtime interface {
	PublishTo(schoolID int64, eventType string, data map[string]any, roles []string, userIDs []int64)
}

var ErrNoActiveAcademicYear = httpx.Validation("Sekolah belum punya tahun ajaran aktif. Aktifkan dulu di menu tahun ajaran.")

// Service berisi aturan bisnis modul latearrival.
type Service struct {
	repo      lateArrivalRepository
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
func newServiceForTest(repo lateArrivalRepository, identity IdentityGateway, years AcademicYearLookup, students StudentAccess, duties DutyGateway, schedule ScheduleGateway, teacherqr TeacherQRGateway, clk clock.Clock) *Service {
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

func (s *Service) emit(schoolID, recordID int64, extraUserIDs []int64) {
	if s.realtime == nil {
		return
	}
	s.realtime.PublishTo(schoolID, "latearrival", map[string]any{}, []string{RoleAdminSekolah, RoleKepalaSekolah}, extraUserIDs)
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

func localMonthRangeUTC(monthStr string, now time.Time, tz string) (time.Time, time.Time, error) {
	loc, lerr := time.LoadLocation(tz)
	if lerr != nil {
		loc = time.FixedZone("WIB", 7*3600)
	}
	if monthStr == "" {
		local := clock.InZone(now, tz)
		start := time.Date(local.Year(), local.Month(), 1, 0, 0, 0, 0, loc)
		return start.UTC(), start.AddDate(0, 1, 0).UTC(), nil
	}
	t, err := time.Parse("2006-01", strings.TrimSpace(monthStr))
	if err != nil {
		return time.Time{}, time.Time{}, httpx.Validation("Format bulan harus YYYY-MM.")
	}
	start := time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, loc)
	return start.UTC(), start.AddDate(0, 1, 0).UTC(), nil
}

// -- POST /api/late-arrivals/scan (siswa) --

var errNotStudent = httpx.ErrForbidden
var errAlreadyCompletedToday = httpx.Conflict("Siswa ini sudah tercatat terlambat & selesai untuk hari ini.")
var errWrongDuty = httpx.Validation("Guru ini bukan pemegang tugas Guru Piket.")
var errWrongLeadership = httpx.Validation("Guru ini bukan pemegang tugas Pimpinan.")
var errWrongClassTeacher = httpx.Validation("Guru ini tidak mengajar kelas Anda hari ini.")
var errStageRace = httpx.Validation("Gagal menyimpan (kemungkinan sudah diproses pihak lain). Muat ulang.")

// Scan — POST /api/late-arrivals/scan {token}: SATU endpoint untuk seluruh
// state machine (docs tugas): belum ada record aktif hari ini -> CREATE
// (butuh flag late_arrival_duty) -> pending_leadership -> (butuh flag
// late_arrival_leadership) -> pending_class_teacher -> (guru ber-slot kelas
// siswa hari itu) -> completed. Maks SATU record per hari (record
// 'completed' hari ini memblokir scan baru, docs tugas: "tambahkan maks 1
// record per hari per siswa").
func (s *Service) Scan(ctx context.Context, actorUserID, schoolID int64, rawToken string) (ScanResult, error) {
	if reqctx.Role(ctx) != RoleSiswa {
		return ScanResult{}, errNotStudent
	}
	studentID, classID, ok, err := s.students.MyStudentID(ctx, schoolID, actorUserID)
	if err != nil {
		return ScanResult{}, err
	}
	if !ok {
		return ScanResult{}, httpx.Validation("Akun Anda belum tertaut ke data siswa.")
	}

	yearID, err := s.requireActiveYear(ctx, schoolID)
	if err != nil {
		return ScanResult{}, err
	}

	tz := s.schoolTimezone(ctx)
	now := s.clock.Now()
	dayFrom, dayTo := localDayRangeUTC(now, tz)

	existing, hasExisting, err := s.repo.GetTodayForStudent(ctx, schoolID, studentID, dayFrom, dayTo)
	if err != nil {
		return ScanResult{}, err
	}

	teacherUserID, err := s.teacherqr.ConsumeToken(ctx, schoolID, rawToken)
	if err != nil {
		return ScanResult{}, err
	}

	if !hasExisting {
		hasFlag, ferr := s.duties.UserHasFlag(ctx, schoolID, teacherUserID, FlagLateArrivalDuty)
		if ferr != nil {
			return ScanResult{}, ferr
		}
		if !hasFlag {
			return ScanResult{}, errWrongDuty
		}
		countSoFar, cerr := s.repo.CountForStudentYear(ctx, schoolID, yearID, studentID)
		if cerr != nil {
			return ScanResult{}, cerr
		}
		lateCount := int(countSoFar) + 1
		action := lateArrivalAction(lateCount)
		id, cerr := s.repo.CreateRecord(ctx, schoolID, yearID, studentID, now, lateCount, action, teacherUserID)
		if cerr != nil {
			return ScanResult{}, cerr
		}
		detail, derr := s.repo.GetDetail(ctx, schoolID, id)
		if derr != nil {
			return ScanResult{}, derr
		}
		s.audit(ctx, schoolID, actorUserID, "latearrival.record", "student_late_arrival", id, nil,
			map[string]any{"student_id": studentID, "late_count": lateCount, "action": action})
		guardians, _ := s.students.GuardianUserIDs(ctx, schoolID, studentID)
		s.notify(ctx, schoolID, EventLateArrivalRecorded, dedupInt64(guardians...), map[string]any{
			"student": detail.StudentName, "late_count": lateCount, "action": actionLabel(action),
		})
		s.emit(schoolID, id, dedupInt64(append([]int64{actorUserID, teacherUserID}, guardians...)...))
		return ScanResult{Record: detail.View(), Action: action, LateCount: lateCount}, nil
	}

	switch existing.Status {
	case StatusCompleted:
		return ScanResult{}, errAlreadyCompletedToday

	case StatusPendingLeadership:
		hasFlag, ferr := s.duties.UserHasFlag(ctx, schoolID, teacherUserID, FlagLateArrivalLeadership)
		if ferr != nil {
			return ScanResult{}, ferr
		}
		if !hasFlag {
			return ScanResult{}, errWrongLeadership
		}
		rows, uerr := s.repo.UpdateLeadershipStage(ctx, schoolID, existing.ID, teacherUserID)
		if uerr != nil {
			return ScanResult{}, uerr
		}
		if rows == 0 {
			return ScanResult{}, errStageRace
		}

	case StatusPendingClassTeacher:
		teaches, terr := s.schedule.TeacherTeachesClassToday(ctx, schoolID, classID, teacherUserID, now)
		if terr != nil {
			return ScanResult{}, terr
		}
		if !teaches {
			return ScanResult{}, errWrongClassTeacher
		}
		rows, uerr := s.repo.UpdateClassStage(ctx, schoolID, existing.ID, teacherUserID)
		if uerr != nil {
			return ScanResult{}, uerr
		}
		if rows == 0 {
			return ScanResult{}, errStageRace
		}

	default:
		return ScanResult{}, errStageRace
	}

	s.audit(ctx, schoolID, actorUserID, "latearrival.scan", "student_late_arrival", existing.ID,
		map[string]any{"status": existing.Status}, map[string]any{"by": teacherUserID})

	updated, err := s.repo.GetDetail(ctx, schoolID, existing.ID)
	if err != nil {
		return ScanResult{}, err
	}
	s.emit(schoolID, existing.ID, []int64{actorUserID, teacherUserID})
	return ScanResult{Record: updated.View(), Action: updated.Action, LateCount: updated.LateCount}, nil
}

// -- GET /api/late-arrivals?scope=mine|today|all&month= --

func (s *Service) ListRecords(ctx context.Context, actorUserID, schoolID int64, scope, month string) (RecordPage, error) {
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
				return RecordPage{}, serr
			}
			if ok {
				studentIDs = []int64{id}
			}
		case RoleOrangTua:
			ids, serr := s.students.MyChildStudentIDs(ctx, schoolID, actorUserID)
			if serr != nil {
				return RecordPage{}, serr
			}
			studentIDs = ids
		}
		if len(studentIDs) == 0 {
			return RecordPage{Items: []RecordView{}}, nil
		}
		var fromPtr, toPtr *time.Time
		if month != "" {
			from, to, merr := localMonthRangeUTC(month, s.clock.Now(), s.schoolTimezone(ctx))
			if merr != nil {
				return RecordPage{}, merr
			}
			fromPtr, toPtr = &from, &to
		}
		details, err = s.repo.ListForStudents(ctx, schoolID, studentIDs, fromPtr, toPtr)

	case "today":
		if !s.identity.HasPermission(role, PermStudentRead) {
			return RecordPage{}, httpx.ErrForbidden
		}
		from, to := localDayRangeUTC(s.clock.Now(), s.schoolTimezone(ctx))
		details, err = s.repo.ListToday(ctx, schoolID, from, to)

	case "all":
		if !s.identity.HasPermission(role, PermStudentManage) && role != RoleKepalaSekolah {
			return RecordPage{}, httpx.ErrForbidden
		}
		var fromPtr, toPtr *time.Time
		if month != "" {
			from, to, merr := localMonthRangeUTC(month, s.clock.Now(), s.schoolTimezone(ctx))
			if merr != nil {
				return RecordPage{}, merr
			}
			fromPtr, toPtr = &from, &to
		}
		details, err = s.repo.ListAll(ctx, schoolID, fromPtr, toPtr)

	default:
		return RecordPage{}, httpx.Validation("scope harus 'mine', 'today', atau 'all'.")
	}
	if err != nil {
		return RecordPage{}, err
	}

	items := make([]RecordView, 0, len(details))
	for _, d := range details {
		items = append(items, d.View())
	}
	return RecordPage{Items: items, Total: int64(len(items))}, nil
}

// -- GET /api/late-arrivals/summary?month= (admin/kepsek) --

func (s *Service) Summary(ctx context.Context, schoolID int64, month string) ([]SummaryRow, error) {
	role := reqctx.Role(ctx)
	if !s.identity.HasPermission(role, PermStudentManage) && role != RoleKepalaSekolah {
		return nil, httpx.ErrForbidden
	}
	from, to, err := localMonthRangeUTC(month, s.clock.Now(), s.schoolTimezone(ctx))
	if err != nil {
		return nil, err
	}
	rows, err := s.repo.SummaryByMonth(ctx, schoolID, from, to)
	if err != nil {
		return nil, err
	}
	out := make([]SummaryRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, SummaryRow{
			Student: StudentRef{ID: r.StudentID, Name: r.StudentName, NIS: r.StudentNIS, ClassName: r.ClassName},
			Count:   r.Count,
		})
	}
	return out, nil
}
