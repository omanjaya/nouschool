package exitpermit

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/omanjaya/nouschool/internal/platform/clock"
	"github.com/omanjaya/nouschool/internal/platform/httpx"
	"github.com/omanjaya/nouschool/internal/platform/reqctx"
)

// -- fakes (in-memory, tanpa DB) --

type fakeRow struct {
	id             int64
	schoolID       int64
	academicYearID int64
	studentID      int64
	reason         string
	status         string
	dutyBy         int64
	dutyAt         time.Time
	classBy        int64
	classAt        time.Time
	bkBy           int64
	bkAt           time.Time
	leadershipBy   int64
	leadershipAt   time.Time
	rejectedBy     int64
	rejectedAt     time.Time
	rejectComment  string
	gateToken      string
	gateExpiresAt  time.Time
	exitedBy       int64
	exitedAt       time.Time
	createdAt      time.Time
}

func (r *fakeRow) toDetail(names map[int64]string) Detail {
	return Detail{
		ID: r.id, SchoolID: r.schoolID, AcademicYearID: r.academicYearID, StudentID: r.studentID,
		Reason: r.reason, Status: r.status,
		DutyBy: r.dutyBy, DutyAt: r.dutyAt, DutyByName: names[r.dutyBy],
		ClassBy: r.classBy, ClassAt: r.classAt, ClassByName: names[r.classBy],
		BKBy: r.bkBy, BKAt: r.bkAt, BKByName: names[r.bkBy],
		LeadershipBy: r.leadershipBy, LeadershipAt: r.leadershipAt, LeadershipByName: names[r.leadershipBy],
		RejectedBy: r.rejectedBy, RejectedAt: r.rejectedAt, RejectedByName: names[r.rejectedBy], RejectComment: r.rejectComment,
		GateToken: r.gateToken, GateExpiresAt: r.gateExpiresAt,
		ExitedBy: r.exitedBy, ExitedAt: r.exitedAt, ExitedByName: names[r.exitedBy],
		CreatedAt: r.createdAt, StudentName: "Siswa " + itoa(r.studentID), StudentNIS: "NIS" + itoa(r.studentID), ClassName: "XII RPL 1",
	}
}

func itoa(v int64) string {
	if v == 0 {
		return "0"
	}
	digits := []byte{}
	for v > 0 {
		digits = append([]byte{byte('0' + v%10)}, digits...)
		v /= 10
	}
	return string(digits)
}

type fakeExitRepo struct {
	nextID int64
	rows   map[int64]*fakeRow
	names  map[int64]string
	now    time.Time
}

func newFakeExitRepo(now time.Time) *fakeExitRepo {
	return &fakeExitRepo{rows: map[int64]*fakeRow{}, names: map[int64]string{}, now: now}
}

func (f *fakeExitRepo) CreateRequest(ctx context.Context, schoolID, academicYearID, studentID int64, reason string) (int64, error) {
	f.nextID++
	f.rows[f.nextID] = &fakeRow{id: f.nextID, schoolID: schoolID, academicYearID: academicYearID, studentID: studentID, reason: reason, status: StatusPendingDutyTeacher, createdAt: f.now}
	return f.nextID, nil
}

func (f *fakeExitRepo) CountActive(ctx context.Context, schoolID, studentID int64) (int64, error) {
	var n int64
	for _, r := range f.rows {
		if r.schoolID == schoolID && r.studentID == studentID && isActivePermit(r.status) {
			n++
		}
	}
	return n, nil
}

func (f *fakeExitRepo) GetDetail(ctx context.Context, schoolID, id int64) (Detail, error) {
	r, ok := f.rows[id]
	if !ok || r.schoolID != schoolID {
		return Detail{}, ErrNotFound
	}
	return r.toDetail(f.names), nil
}

func (f *fakeExitRepo) GetByGateToken(ctx context.Context, schoolID int64, token string) (Detail, bool, error) {
	for _, r := range f.rows {
		if r.schoolID == schoolID && r.gateToken == token {
			return r.toDetail(f.names), true, nil
		}
	}
	return Detail{}, false, nil
}

func (f *fakeExitRepo) ListForStudents(ctx context.Context, schoolID int64, studentIDs []int64) ([]Detail, error) {
	set := map[int64]bool{}
	for _, id := range studentIDs {
		set[id] = true
	}
	var out []Detail
	for _, r := range f.rows {
		if r.schoolID == schoolID && set[r.studentID] {
			out = append(out, r.toDetail(f.names))
		}
	}
	return out, nil
}

func (f *fakeExitRepo) ListActiveToday(ctx context.Context, schoolID int64, from, to time.Time) ([]Detail, error) {
	var out []Detail
	for _, r := range f.rows {
		if r.schoolID == schoolID && !isFinal(r.status) && !r.createdAt.Before(from) && r.createdAt.Before(to) {
			out = append(out, r.toDetail(f.names))
		}
	}
	return out, nil
}

func (f *fakeExitRepo) ListAll(ctx context.Context, schoolID int64, from, to *time.Time) ([]Detail, error) {
	var out []Detail
	for _, r := range f.rows {
		if r.schoolID != schoolID {
			continue
		}
		if from != nil && r.createdAt.Before(*from) {
			continue
		}
		if to != nil && !r.createdAt.Before(*to) {
			continue
		}
		out = append(out, r.toDetail(f.names))
	}
	return out, nil
}

func (f *fakeExitRepo) ListExitedOnDate(ctx context.Context, schoolID int64, from, to time.Time) ([]Detail, error) {
	var out []Detail
	for _, r := range f.rows {
		if r.schoolID == schoolID && r.status == StatusExited && !r.exitedAt.Before(from) && r.exitedAt.Before(to) {
			out = append(out, r.toDetail(f.names))
		}
	}
	return out, nil
}

func (f *fakeExitRepo) UpdateDutyStage(ctx context.Context, schoolID, id, dutyBy int64) (int64, error) {
	r, ok := f.rows[id]
	if !ok || r.schoolID != schoolID || r.status != StatusPendingDutyTeacher {
		return 0, nil
	}
	r.status, r.dutyBy, r.dutyAt = StatusPendingClassTeacher, dutyBy, f.now
	return 1, nil
}

func (f *fakeExitRepo) UpdateClassStage(ctx context.Context, schoolID, id, classBy int64) (int64, error) {
	r, ok := f.rows[id]
	if !ok || r.schoolID != schoolID || r.status != StatusPendingClassTeacher {
		return 0, nil
	}
	r.status, r.classBy, r.classAt = StatusPendingBK, classBy, f.now
	return 1, nil
}

func (f *fakeExitRepo) UpdateBKStage(ctx context.Context, schoolID, id, bkBy int64) (int64, error) {
	r, ok := f.rows[id]
	if !ok || r.schoolID != schoolID || r.status != StatusPendingBK {
		return 0, nil
	}
	r.status, r.bkBy, r.bkAt = StatusPendingLeadership, bkBy, f.now
	return 1, nil
}

func (f *fakeExitRepo) UpdateLeadershipStage(ctx context.Context, schoolID, id, leadershipBy int64, gateToken string, gateExpiresAt time.Time) (int64, error) {
	r, ok := f.rows[id]
	if !ok || r.schoolID != schoolID || r.status != StatusPendingLeadership {
		return 0, nil
	}
	r.status, r.leadershipBy, r.leadershipAt = StatusIssued, leadershipBy, f.now
	r.gateToken, r.gateExpiresAt = gateToken, gateExpiresAt
	return 1, nil
}

func (f *fakeExitRepo) Reject(ctx context.Context, schoolID, id, rejectedBy int64, comment string) (int64, error) {
	r, ok := f.rows[id]
	if !ok || r.schoolID != schoolID || isFinal(r.status) {
		return 0, nil
	}
	r.status, r.rejectedBy, r.rejectedAt, r.rejectComment = StatusRejected, rejectedBy, f.now, comment
	return 1, nil
}

func (f *fakeExitRepo) Cancel(ctx context.Context, schoolID, id int64) (int64, error) {
	r, ok := f.rows[id]
	if !ok || r.schoolID != schoolID || !isPending(r.status) {
		return 0, nil
	}
	r.status = StatusCanceled
	return 1, nil
}

func (f *fakeExitRepo) GateScan(ctx context.Context, schoolID int64, gateToken string, exitedBy int64) (int64, error) {
	for _, r := range f.rows {
		if r.schoolID == schoolID && r.gateToken == gateToken && r.status == StatusIssued {
			r.status, r.exitedBy, r.exitedAt = StatusExited, exitedBy, f.now
			return 1, nil
		}
	}
	return 0, nil
}

// -- gateways --

type fakeIdentity struct{ perms map[string]map[string]bool }

func (f fakeIdentity) HasPermission(role, perm string) bool { return f.perms[role][perm] }
func (f fakeIdentity) Log(ctx context.Context, schoolID, userID *int64, action, entity string, entityID *int64, oldValue, newValue any) error {
	return nil
}

func defaultIdentity() fakeIdentity {
	return fakeIdentity{perms: map[string]map[string]bool{
		RoleAdminSekolah:  {PermStudentManage: true, PermStudentRead: true},
		RoleKepalaSekolah: {PermStudentRead: true},
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
	myStudent map[int64][2]int64 // userID -> [studentID, classID]
	children  map[int64][]int64  // parentUserID -> studentIDs
	guardians map[int64][]int64  // studentID -> guardian userIDs
}

func (f fakeStudents) CanViewStudent(ctx context.Context, userID int64, role string, schoolID, studentID int64) error {
	if role == RoleOrangTua {
		for _, id := range f.children[userID] {
			if id == studentID {
				return nil
			}
		}
		return httpx.ErrForbidden
	}
	return nil
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

type fakeSchedule struct {
	classTeacherNow map[int64]int64 // classID -> teacherUserID (0 = no slot)
	gateExpiry      time.Time
	gateExpiryErr   error
}

func (f fakeSchedule) TeacherMatchesClassNow(ctx context.Context, schoolID, classID, teacherUserID int64, at time.Time) (bool, bool, error) {
	want, hasSlot := f.classTeacherNow[classID]
	if !hasSlot || want == 0 {
		return false, false, nil
	}
	return want == teacherUserID, true, nil
}

func (f fakeSchedule) GateExpiryToday(ctx context.Context, schoolID int64, at time.Time) (time.Time, error) {
	if f.gateExpiryErr != nil {
		return time.Time{}, f.gateExpiryErr
	}
	return f.gateExpiry, nil
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

// -- test setup helper --

const (
	studentUserID = 100
	studentID     = 10
	classID       = 1
	dutyUserID    = 21 // piket
	classTeachID  = 22 // guru pengajar (BEDA dari piket)
	bkUserID      = 23
	leaderUserID  = 24
	securityID    = 25
)

func newFullSetup(now time.Time) (*Service, *fakeExitRepo, *fakeTeacherQR, *fakeSchedule) {
	repo := newFakeExitRepo(now)
	tq := newFakeTeacherQR()
	sched := &fakeSchedule{classTeacherNow: map[int64]int64{classID: classTeachID}, gateExpiry: now.Add(3 * time.Hour)}
	duties := fakeDuties{flags: map[int64]map[string]bool{
		dutyUserID:   {FlagLateArrivalDuty: true},
		bkUserID:     {FlagExitBKApproval: true},
		leaderUserID: {FlagExitLeadershipApproval: true},
		securityID:   {FlagExitSecurity: true},
	}}
	students := fakeStudents{
		myStudent: map[int64][2]int64{studentUserID: {studentID, classID}},
		guardians: map[int64][]int64{studentID: {999}},
	}
	svc := newServiceForTest(repo, defaultIdentity(), fakeYears{id: 1}, students, duties, sched, tq, clock.Fixed{T: now})
	return svc, repo, tq, sched
}

// -- tests --

func TestExitPermit_FullChain(t *testing.T) {
	now := time.Date(2026, 8, 16, 8, 0, 0, 0, time.UTC)
	svc, _, tq, _ := newFullSetup(now)

	created, err := svc.SubmitRequest(ctxAs(RoleSiswa, studentUserID), studentUserID, 1, "Sakit perut")
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if created.Status != StatusPendingDutyTeacher {
		t.Fatalf("expected pending_duty_teacher, got %s", created.Status)
	}

	tq.issue("t-duty", dutyUserID)
	v1, err := svc.Scan(ctxAs(RoleSiswa, studentUserID), studentUserID, 1, created.ID, "t-duty")
	if err != nil {
		t.Fatalf("scan duty: %v", err)
	}
	if v1.Status != StatusPendingClassTeacher {
		t.Fatalf("expected pending_class_teacher, got %s", v1.Status)
	}

	// Tahap 2 dengan guru SAMA (piket) HARUS ditolak (beda orang).
	tq.issue("t-same", dutyUserID)
	if _, err := svc.Scan(ctxAs(RoleSiswa, studentUserID), studentUserID, 1, created.ID, "t-same"); err == nil {
		t.Fatal("expected error same person, got nil")
	} else if domainStatus(t, err) != 422 {
		t.Fatalf("expected 422, got %v", err)
	}

	tq.issue("t-class", classTeachID)
	v2, err := svc.Scan(ctxAs(RoleSiswa, studentUserID), studentUserID, 1, created.ID, "t-class")
	if err != nil {
		t.Fatalf("scan class teacher: %v", err)
	}
	if v2.Status != StatusPendingBK {
		t.Fatalf("expected pending_bk, got %s", v2.Status)
	}

	tq.issue("t-bk", bkUserID)
	v3, err := svc.Scan(ctxAs(RoleSiswa, studentUserID), studentUserID, 1, created.ID, "t-bk")
	if err != nil {
		t.Fatalf("scan bk: %v", err)
	}
	if v3.Status != StatusPendingLeadership {
		t.Fatalf("expected pending_leadership, got %s", v3.Status)
	}

	tq.issue("t-lead", leaderUserID)
	v4, err := svc.Scan(ctxAs(RoleSiswa, studentUserID), studentUserID, 1, created.ID, "t-lead")
	if err != nil {
		t.Fatalf("scan leadership: %v", err)
	}
	if v4.Status != StatusIssued || v4.GateToken == nil || v4.GateExpiresAt == nil {
		t.Fatalf("expected issued+gate_token+gate_expires_at, got %+v", v4)
	}
}

func TestExitPermit_WrongStageDuty(t *testing.T) {
	now := time.Date(2026, 8, 16, 8, 0, 0, 0, time.UTC)
	svc, _, tq, _ := newFullSetup(now)
	created, _ := svc.SubmitRequest(ctxAs(RoleSiswa, studentUserID), studentUserID, 1, "Sakit")

	tq.issue("t-wrong", classTeachID) // bukan pemegang flag late_arrival_duty
	if _, err := svc.Scan(ctxAs(RoleSiswa, studentUserID), studentUserID, 1, created.ID, "t-wrong"); err == nil {
		t.Fatal("expected error, got nil")
	} else if domainStatus(t, err) != 422 {
		t.Fatalf("expected 422, got %v", err)
	}
}

func TestExitPermit_MaxOneActivePermit(t *testing.T) {
	now := time.Date(2026, 8, 16, 8, 0, 0, 0, time.UTC)
	svc, _, _, _ := newFullSetup(now)
	if _, err := svc.SubmitRequest(ctxAs(RoleSiswa, studentUserID), studentUserID, 1, "Sakit"); err != nil {
		t.Fatalf("submit 1: %v", err)
	}
	if _, err := svc.SubmitRequest(ctxAs(RoleSiswa, studentUserID), studentUserID, 1, "Sakit lagi"); err == nil {
		t.Fatal("expected conflict, got nil")
	} else if domainStatus(t, err) != 409 {
		t.Fatalf("expected 409, got %v", err)
	}
}

func TestExitPermit_CancelOnlyPendingAndOwner(t *testing.T) {
	now := time.Date(2026, 8, 16, 8, 0, 0, 0, time.UTC)
	svc, _, _, _ := newFullSetup(now)
	created, _ := svc.SubmitRequest(ctxAs(RoleSiswa, studentUserID), studentUserID, 1, "Sakit")

	if err := svc.CancelRequest(ctxAs(RoleSiswa, 999), 999, 1, created.ID); err == nil {
		t.Fatal("expected forbidden for non-owner, got nil")
	}
	if err := svc.CancelRequest(ctxAs(RoleSiswa, studentUserID), studentUserID, 1, created.ID); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if err := svc.CancelRequest(ctxAs(RoleSiswa, studentUserID), studentUserID, 1, created.ID); err == nil {
		t.Fatal("expected error cancel twice, got nil")
	}
}

func TestExitPermit_RejectByAdmin(t *testing.T) {
	now := time.Date(2026, 8, 16, 8, 0, 0, 0, time.UTC)
	svc, _, _, _ := newFullSetup(now)
	created, _ := svc.SubmitRequest(ctxAs(RoleSiswa, studentUserID), studentUserID, 1, "Sakit")

	if _, err := svc.RejectRequest(ctxAs(RoleSiswa, studentUserID), studentUserID, 1, created.ID, "tidak jelas"); err == nil {
		t.Fatal("expected forbidden for siswa, got nil")
	}
	rejected, err := svc.RejectRequest(ctxAs(RoleAdminSekolah, 1), 1, 1, created.ID, "tidak jelas")
	if err != nil {
		t.Fatalf("reject: %v", err)
	}
	if rejected.Status != StatusRejected {
		t.Fatalf("expected rejected, got %s", rejected.Status)
	}
}

func TestExitPermit_GateScan_ExpiryAndDoubleScan(t *testing.T) {
	now := time.Date(2026, 8, 16, 8, 0, 0, 0, time.UTC)
	svc, _, tq, _ := newFullSetup(now)
	created, _ := svc.SubmitRequest(ctxAs(RoleSiswa, studentUserID), studentUserID, 1, "Sakit")
	tq.issue("t-duty", dutyUserID)
	svc.Scan(ctxAs(RoleSiswa, studentUserID), studentUserID, 1, created.ID, "t-duty")
	tq.issue("t-class", classTeachID)
	svc.Scan(ctxAs(RoleSiswa, studentUserID), studentUserID, 1, created.ID, "t-class")
	tq.issue("t-bk", bkUserID)
	svc.Scan(ctxAs(RoleSiswa, studentUserID), studentUserID, 1, created.ID, "t-bk")
	tq.issue("t-lead", leaderUserID)
	issued, err := svc.Scan(ctxAs(RoleSiswa, studentUserID), studentUserID, 1, created.ID, "t-lead")
	if err != nil {
		t.Fatalf("scan leadership: %v", err)
	}

	// Sebelum expiry -> sukses.
	view, err := svc.GateScan(ctxAs("pegawai", securityID), securityID, 1, *issued.GateToken)
	if err != nil {
		t.Fatalf("gate scan: %v", err)
	}
	if view.Student.ID != studentID {
		t.Fatalf("expected student id %d, got %d", studentID, view.Student.ID)
	}

	// Scan ulang setelah exited -> gagal (409).
	if _, err := svc.GateScan(ctxAs("pegawai", securityID), securityID, 1, *issued.GateToken); err == nil {
		t.Fatal("expected error double scan, got nil")
	} else if domainStatus(t, err) != 409 {
		t.Fatalf("expected 409, got %v", err)
	}

	// Kasus TERPISAH: gate token kedaluwarsa -> 410 (clock.Fixed sebelum/sesudah).
	svc2, _, tq2, sched2 := newFullSetup(now)
	sched2.gateExpiry = now.Add(-time.Minute) // sudah lewat SAAT issued (edge case uji gerbang)
	created2, _ := svc2.SubmitRequest(ctxAs(RoleSiswa, studentUserID), studentUserID, 1, "Sakit")
	tq2.issue("t-duty2", dutyUserID)
	svc2.Scan(ctxAs(RoleSiswa, studentUserID), studentUserID, 1, created2.ID, "t-duty2")
	tq2.issue("t-class2", classTeachID)
	svc2.Scan(ctxAs(RoleSiswa, studentUserID), studentUserID, 1, created2.ID, "t-class2")
	tq2.issue("t-bk2", bkUserID)
	svc2.Scan(ctxAs(RoleSiswa, studentUserID), studentUserID, 1, created2.ID, "t-bk2")
	tq2.issue("t-lead2", leaderUserID)
	issued2, err := svc2.Scan(ctxAs(RoleSiswa, studentUserID), studentUserID, 1, created2.ID, "t-lead2")
	if err != nil {
		t.Fatalf("scan leadership 2: %v", err)
	}
	if _, err := svc2.GateScan(ctxAs("pegawai", securityID), securityID, 1, *issued2.GateToken); err == nil {
		t.Fatal("expected expired error, got nil")
	} else if domainStatus(t, err) != 410 {
		t.Fatalf("expected 410, got %v", err)
	}
}

func TestExitPermit_GateScan_RequiresSecurityFlag(t *testing.T) {
	now := time.Date(2026, 8, 16, 8, 0, 0, 0, time.UTC)
	svc, _, _, _ := newFullSetup(now)
	if _, err := svc.GateScan(ctxAs("pegawai", 999), 999, 1, "token-apa-saja"); err == nil {
		t.Fatal("expected forbidden, got nil")
	} else if domainStatus(t, err) != 403 {
		t.Fatalf("expected 403, got %v", err)
	}
}
