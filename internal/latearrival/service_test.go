package latearrival

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/omanjaya/nouschool/internal/platform/clock"
	"github.com/omanjaya/nouschool/internal/platform/httpx"
	"github.com/omanjaya/nouschool/internal/platform/reqctx"
)

// -- fungsi murni --

func TestLateArrivalAction(t *testing.T) {
	cases := map[int]string{
		1: ActionNone, 2: ActionCallParent, 3: ActionSendHome, 4: ActionNone,
		5: ActionCallParent, 6: ActionSendHome, 7: ActionNone,
	}
	for count, want := range cases {
		if got := lateArrivalAction(count); got != want {
			t.Errorf("lateArrivalAction(%d) = %q, want %q", count, got, want)
		}
	}
}

// -- fakes --

type fakeRow struct {
	id             int64
	schoolID       int64
	academicYearID int64
	studentID      int64
	arrivedAt      time.Time
	lateCount      int
	action         string
	status         string
	dutyBy         int64
	dutyAt         time.Time
	leadershipBy   int64
	leadershipAt   time.Time
	classBy        int64
	classAt        time.Time
	createdAt      time.Time
}

func (r *fakeRow) toDetail() Detail {
	return Detail{
		ID: r.id, SchoolID: r.schoolID, AcademicYearID: r.academicYearID, StudentID: r.studentID,
		ArrivedAt: r.arrivedAt, LateCount: r.lateCount, Action: r.action, Status: r.status,
		DutyBy: r.dutyBy, DutyAt: r.dutyAt, LeadershipBy: r.leadershipBy, LeadershipAt: r.leadershipAt,
		ClassBy: r.classBy, ClassAt: r.classAt, CreatedAt: r.createdAt,
		StudentName: "Siswa", StudentNIS: "22101", ClassName: "XII RPL 1",
	}
}

type fakeRepo struct {
	nextID int64
	rows   map[int64]*fakeRow
}

func newFakeRepo() *fakeRepo { return &fakeRepo{rows: map[int64]*fakeRow{}} }

func (f *fakeRepo) CreateRecord(ctx context.Context, schoolID, academicYearID, studentID int64, arrivedAt time.Time, lateCount int, action string, dutyBy int64) (int64, error) {
	f.nextID++
	f.rows[f.nextID] = &fakeRow{
		id: f.nextID, schoolID: schoolID, academicYearID: academicYearID, studentID: studentID,
		arrivedAt: arrivedAt, lateCount: lateCount, action: action, status: StatusPendingLeadership,
		dutyBy: dutyBy, dutyAt: arrivedAt, createdAt: arrivedAt,
	}
	return f.nextID, nil
}

func (f *fakeRepo) CountForStudentYear(ctx context.Context, schoolID, academicYearID, studentID int64) (int64, error) {
	var n int64
	for _, r := range f.rows {
		if r.schoolID == schoolID && r.academicYearID == academicYearID && r.studentID == studentID {
			n++
		}
	}
	return n, nil
}

func (f *fakeRepo) GetDetail(ctx context.Context, schoolID, id int64) (Detail, error) {
	r, ok := f.rows[id]
	if !ok || r.schoolID != schoolID {
		return Detail{}, ErrNotFound
	}
	return r.toDetail(), nil
}

func (f *fakeRepo) GetTodayForStudent(ctx context.Context, schoolID, studentID int64, from, to time.Time) (Detail, bool, error) {
	for _, r := range f.rows {
		if r.schoolID == schoolID && r.studentID == studentID && !r.arrivedAt.Before(from) && r.arrivedAt.Before(to) {
			return r.toDetail(), true, nil
		}
	}
	return Detail{}, false, nil
}

func (f *fakeRepo) ListForStudents(ctx context.Context, schoolID int64, studentIDs []int64, from, to *time.Time) ([]Detail, error) {
	set := map[int64]bool{}
	for _, id := range studentIDs {
		set[id] = true
	}
	var out []Detail
	for _, r := range f.rows {
		if r.schoolID == schoolID && set[r.studentID] {
			out = append(out, r.toDetail())
		}
	}
	return out, nil
}

func (f *fakeRepo) ListToday(ctx context.Context, schoolID int64, from, to time.Time) ([]Detail, error) {
	var out []Detail
	for _, r := range f.rows {
		if r.schoolID == schoolID && !r.arrivedAt.Before(from) && r.arrivedAt.Before(to) {
			out = append(out, r.toDetail())
		}
	}
	return out, nil
}

func (f *fakeRepo) ListAll(ctx context.Context, schoolID int64, from, to *time.Time) ([]Detail, error) {
	var out []Detail
	for _, r := range f.rows {
		if r.schoolID == schoolID {
			out = append(out, r.toDetail())
		}
	}
	return out, nil
}

func (f *fakeRepo) SummaryByMonth(ctx context.Context, schoolID int64, from, to time.Time) ([]SummaryDetail, error) {
	counts := map[int64]int64{}
	for _, r := range f.rows {
		if r.schoolID == schoolID && !r.arrivedAt.Before(from) && r.arrivedAt.Before(to) {
			counts[r.studentID]++
		}
	}
	var out []SummaryDetail
	for sid, c := range counts {
		out = append(out, SummaryDetail{StudentID: sid, StudentName: "Siswa", StudentNIS: "22101", ClassName: "XII RPL 1", Count: c})
	}
	return out, nil
}

func (f *fakeRepo) UpdateLeadershipStage(ctx context.Context, schoolID, id, leadershipBy int64) (int64, error) {
	r, ok := f.rows[id]
	if !ok || r.schoolID != schoolID || r.status != StatusPendingLeadership {
		return 0, nil
	}
	r.status, r.leadershipBy, r.leadershipAt = StatusPendingClassTeacher, leadershipBy, r.arrivedAt
	return 1, nil
}

func (f *fakeRepo) UpdateClassStage(ctx context.Context, schoolID, id, classBy int64) (int64, error) {
	r, ok := f.rows[id]
	if !ok || r.schoolID != schoolID || r.status != StatusPendingClassTeacher {
		return 0, nil
	}
	r.status, r.classBy, r.classAt = StatusCompleted, classBy, r.arrivedAt
	return 1, nil
}

// -- gateways --

type fakeIdentity struct{ perms map[string]map[string]bool }

func (f fakeIdentity) HasPermission(role, perm string) bool { return f.perms[role][perm] }
func (f fakeIdentity) Log(ctx context.Context, schoolID, userID *int64, action, entity string, entityID *int64, oldValue, newValue any) error {
	return nil
}

func defaultIdentity() fakeIdentity {
	return fakeIdentity{perms: map[string]map[string]bool{
		RoleAdminSekolah: {PermStudentManage: true, PermStudentRead: true},
	}}
}

type fakeYears struct{ id int64 }

func (f fakeYears) ActiveAcademicYearID(ctx context.Context, schoolID int64) (int64, bool, error) {
	if f.id == 0 {
		return 0, false, nil
	}
	return f.id, true, nil
}

type fakeStudents struct {
	myStudent map[int64][2]int64
	children  map[int64][]int64
	guardians map[int64][]int64
}

func (f fakeStudents) GuardianUserIDs(ctx context.Context, schoolID, studentID int64) ([]int64, error) {
	return f.guardians[studentID], nil
}

func (f fakeStudents) MyStudentID(ctx context.Context, schoolID, userID int64) (int64, int64, bool, error) {
	v, ok := f.myStudent[userID]
	if !ok {
		return 0, 0, false, nil
	}
	return v[0], v[1], true, nil
}

func (f fakeStudents) MyChildStudentIDs(ctx context.Context, schoolID, userID int64) ([]int64, error) {
	return f.children[userID], nil
}

type fakeDuties struct{ flags map[int64]map[string]bool }

func (f fakeDuties) UserHasFlag(ctx context.Context, schoolID, userID int64, flag string) (bool, error) {
	return f.flags[userID][flag], nil
}

type fakeSchedule struct{ classTeachers map[int64]map[int64]bool } // classID -> set of teacherUserID

func (f fakeSchedule) TeacherTeachesClassToday(ctx context.Context, schoolID, classID, teacherUserID int64, at time.Time) (bool, error) {
	return f.classTeachers[classID][teacherUserID], nil
}

type fakeTeacherQR struct {
	tokens  map[string]int64
	consume map[string]bool
}

func newFakeTeacherQR() *fakeTeacherQR {
	return &fakeTeacherQR{tokens: map[string]int64{}, consume: map[string]bool{}}
}
func (f *fakeTeacherQR) issue(token string, userID int64) { f.tokens[token] = userID }
func (f *fakeTeacherQR) ConsumeToken(ctx context.Context, schoolID int64, rawToken string) (int64, error) {
	if f.consume[rawToken] {
		return 0, errors.New("token sudah dipakai")
	}
	uid, ok := f.tokens[rawToken]
	if !ok {
		return 0, errors.New("token tidak dikenal")
	}
	f.consume[rawToken] = true
	return uid, nil
}

func ctxAs(role string, userID int64) context.Context {
	ctx := reqctx.WithUser(context.Background(), userID, role, false)
	return reqctx.WithSchool(ctx, reqctx.School{ID: 1, Name: "Sekolah Uji", Slug: "uji", Timezone: "Asia/Jakarta"})
}

func domainStatus(t *testing.T, err error) int {
	t.Helper()
	var de *httpx.Error
	if !errors.As(err, &de) {
		t.Fatalf("expected domain error, got: %v", err)
	}
	return de.Status
}

const (
	studentUserID = 100
	studentID     = 10
	classID       = 1
	dutyUserID    = 21
	leaderUserID  = 22
	classTeachID  = 23
)

func newSetup(now time.Time) (*Service, *fakeTeacherQR) {
	repo := newFakeRepo()
	tq := newFakeTeacherQR()
	duties := fakeDuties{flags: map[int64]map[string]bool{
		dutyUserID:   {FlagLateArrivalDuty: true},
		leaderUserID: {FlagLateArrivalLeadership: true},
	}}
	sched := fakeSchedule{classTeachers: map[int64]map[int64]bool{classID: {classTeachID: true}}}
	students := fakeStudents{
		myStudent: map[int64][2]int64{studentUserID: {studentID, classID}},
		guardians: map[int64][]int64{studentID: {999}},
	}
	svc := newServiceForTest(repo, defaultIdentity(), fakeYears{id: 1}, students, duties, sched, tq, clock.Fixed{T: now})
	return svc, tq
}

func TestLateArrival_FullChain(t *testing.T) {
	now := time.Date(2026, 8, 16, 7, 0, 0, 0, time.UTC)
	svc, tq := newSetup(now)

	tq.issue("t-duty", dutyUserID)
	r1, err := svc.Scan(ctxAs(RoleSiswa, studentUserID), studentUserID, 1, "t-duty")
	if err != nil {
		t.Fatalf("scan duty: %v", err)
	}
	if r1.Record.Status != StatusPendingLeadership || r1.LateCount != 1 || r1.Action != ActionNone {
		t.Fatalf("unexpected first scan result: %+v", r1)
	}

	tq.issue("t-lead", leaderUserID)
	r2, err := svc.Scan(ctxAs(RoleSiswa, studentUserID), studentUserID, 1, "t-lead")
	if err != nil {
		t.Fatalf("scan leadership: %v", err)
	}
	if r2.Record.Status != StatusPendingClassTeacher {
		t.Fatalf("expected pending_class_teacher, got %s", r2.Record.Status)
	}

	tq.issue("t-class", classTeachID)
	r3, err := svc.Scan(ctxAs(RoleSiswa, studentUserID), studentUserID, 1, "t-class")
	if err != nil {
		t.Fatalf("scan class teacher: %v", err)
	}
	if r3.Record.Status != StatusCompleted {
		t.Fatalf("expected completed, got %s", r3.Record.Status)
	}

	// Scan lagi hari yang sama (sudah completed) -> 409.
	tq.issue("t-again", dutyUserID)
	if _, err := svc.Scan(ctxAs(RoleSiswa, studentUserID), studentUserID, 1, "t-again"); err == nil {
		t.Fatal("expected error, got nil")
	} else if domainStatus(t, err) != 409 {
		t.Fatalf("expected 409, got %v", err)
	}
}

func TestLateArrival_WrongDutyFlag(t *testing.T) {
	now := time.Date(2026, 8, 16, 7, 0, 0, 0, time.UTC)
	svc, tq := newSetup(now)
	tq.issue("t-wrong", classTeachID)
	if _, err := svc.Scan(ctxAs(RoleSiswa, studentUserID), studentUserID, 1, "t-wrong"); err == nil {
		t.Fatal("expected error, got nil")
	} else if domainStatus(t, err) != 422 {
		t.Fatalf("expected 422, got %v", err)
	}
}

func TestLateArrival_WrongLeadershipFlag(t *testing.T) {
	now := time.Date(2026, 8, 16, 7, 0, 0, 0, time.UTC)
	svc, tq := newSetup(now)
	tq.issue("t-duty", dutyUserID)
	if _, err := svc.Scan(ctxAs(RoleSiswa, studentUserID), studentUserID, 1, "t-duty"); err != nil {
		t.Fatalf("scan duty: %v", err)
	}
	tq.issue("t-wrong-lead", dutyUserID) // bukan pemegang flag leadership
	if _, err := svc.Scan(ctxAs(RoleSiswa, studentUserID), studentUserID, 1, "t-wrong-lead"); err == nil {
		t.Fatal("expected error, got nil")
	} else if domainStatus(t, err) != 422 {
		t.Fatalf("expected 422, got %v", err)
	}
}

func TestLateArrival_WrongClassTeacher(t *testing.T) {
	now := time.Date(2026, 8, 16, 7, 0, 0, 0, time.UTC)
	svc, tq := newSetup(now)
	tq.issue("t-duty", dutyUserID)
	svc.Scan(ctxAs(RoleSiswa, studentUserID), studentUserID, 1, "t-duty")
	tq.issue("t-lead", leaderUserID)
	svc.Scan(ctxAs(RoleSiswa, studentUserID), studentUserID, 1, "t-lead")

	tq.issue("t-wrong-class", dutyUserID) // guru tidak mengajar kelas ini
	if _, err := svc.Scan(ctxAs(RoleSiswa, studentUserID), studentUserID, 1, "t-wrong-class"); err == nil {
		t.Fatal("expected error, got nil")
	} else if domainStatus(t, err) != 422 {
		t.Fatalf("expected 422, got %v", err)
	}
}

func TestLateArrival_CountAcrossDays(t *testing.T) {
	now := time.Date(2026, 8, 16, 7, 0, 0, 0, time.UTC)
	svc, tq := newSetup(now)

	// Selesaikan record hari 1.
	tq.issue("d1-duty", dutyUserID)
	svc.Scan(ctxAs(RoleSiswa, studentUserID), studentUserID, 1, "d1-duty")
	tq.issue("d1-lead", leaderUserID)
	svc.Scan(ctxAs(RoleSiswa, studentUserID), studentUserID, 1, "d1-lead")
	tq.issue("d1-class", classTeachID)
	svc.Scan(ctxAs(RoleSiswa, studentUserID), studentUserID, 1, "d1-class")

	// Hari ke-2 (clock berbeda) -> late_count harus 2 -> action call_parent.
	now2 := now.AddDate(0, 0, 1)
	svc.clock = clock.Fixed{T: now2}
	tq.issue("d2-duty", dutyUserID)
	r, err := svc.Scan(ctxAs(RoleSiswa, studentUserID), studentUserID, 1, "d2-duty")
	if err != nil {
		t.Fatalf("scan day 2: %v", err)
	}
	if r.LateCount != 2 || r.Action != ActionCallParent {
		t.Fatalf("expected late_count=2 action=call_parent, got %+v", r)
	}
}
