package duty

import (
	"context"
	"errors"
	"testing"

	"github.com/omanjaya/nouschool/internal/platform/httpx"
	"github.com/omanjaya/nouschool/internal/platform/reqctx"
)

// -- fake repository (in-memory, tanpa DB) --

type fakeAssignment struct {
	DutyID, UserID, AcademicYearID int64
}

type fakeRepo struct {
	duties      map[int64]DutyRecord
	nextID      int64
	assignments []fakeAssignment
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{duties: map[int64]DutyRecord{}}
}

var _ dutyRepository = (*fakeRepo)(nil)

func (f *fakeRepo) CreateDuty(ctx context.Context, schoolID int64, name, forRole string, flags []string) (DutyRecord, error) {
	for _, d := range f.duties {
		if d.SchoolID == schoolID && d.Name == name {
			return DutyRecord{}, ErrConflict
		}
	}
	f.nextID++
	rec := DutyRecord{ID: f.nextID, SchoolID: schoolID, Name: name, ForRole: forRole, Flags: flags, Active: true}
	f.duties[rec.ID] = rec
	return rec, nil
}

func (f *fakeRepo) UpdateDuty(ctx context.Context, schoolID, id int64, name *string, forRole *string, flags []string, active *bool) (DutyRecord, error) {
	rec, ok := f.duties[id]
	if !ok || rec.SchoolID != schoolID {
		return DutyRecord{}, ErrNotFound
	}
	if name != nil {
		rec.Name = *name
	}
	if forRole != nil {
		rec.ForRole = *forRole
	}
	if flags != nil {
		rec.Flags = flags
	}
	if active != nil {
		rec.Active = *active
	}
	f.duties[id] = rec
	return rec, nil
}

func (f *fakeRepo) GetDutyByID(ctx context.Context, schoolID, id int64) (DutyRecord, error) {
	rec, ok := f.duties[id]
	if !ok || rec.SchoolID != schoolID {
		return DutyRecord{}, ErrNotFound
	}
	return rec, nil
}

func (f *fakeRepo) ListDutiesWithAssigneeCount(ctx context.Context, schoolID, activeYearID int64) ([]DutyWithCount, error) {
	var out []DutyWithCount
	for _, d := range f.duties {
		if d.SchoolID != schoolID {
			continue
		}
		var n int64
		for _, a := range f.assignments {
			if a.DutyID == d.ID && a.AcademicYearID == activeYearID {
				n++
			}
		}
		out = append(out, DutyWithCount{DutyRecord: d, AssigneeCount: n})
	}
	return out, nil
}

func (f *fakeRepo) DeleteDuty(ctx context.Context, schoolID, id int64) (int64, error) {
	rec, ok := f.duties[id]
	if !ok || rec.SchoolID != schoolID {
		return 0, nil
	}
	delete(f.duties, id)
	return 1, nil
}

func (f *fakeRepo) CountAssignmentsForDuty(ctx context.Context, schoolID, dutyID int64) (int64, error) {
	var n int64
	for _, a := range f.assignments {
		if a.DutyID == dutyID {
			n++
		}
	}
	return n, nil
}

func (f *fakeRepo) ReplaceAssignments(ctx context.Context, schoolID, dutyID, academicYearID int64, userIDs []int64) error {
	kept := f.assignments[:0]
	for _, a := range f.assignments {
		if a.DutyID == dutyID && a.AcademicYearID == academicYearID {
			continue
		}
		kept = append(kept, a)
	}
	f.assignments = kept
	for _, uid := range userIDs {
		f.assignments = append(f.assignments, fakeAssignment{DutyID: dutyID, UserID: uid, AcademicYearID: academicYearID})
	}
	return nil
}

func (f *fakeRepo) ListAssignmentsForDutyYear(ctx context.Context, schoolID, dutyID, academicYearID int64) ([]AssignmentUser, error) {
	var out []AssignmentUser
	for _, a := range f.assignments {
		if a.DutyID == dutyID && a.AcademicYearID == academicYearID {
			out = append(out, AssignmentUser{UserID: a.UserID, Name: "User"})
		}
	}
	return out, nil
}

func (f *fakeRepo) UserHasFlag(ctx context.Context, schoolID, userID, academicYearID int64, flag string) (bool, error) {
	for _, a := range f.assignments {
		if a.UserID != userID || a.AcademicYearID != academicYearID {
			continue
		}
		d, ok := f.duties[a.DutyID]
		if !ok || d.SchoolID != schoolID || !d.Active {
			continue
		}
		for _, fl := range d.Flags {
			if fl == flag {
				return true, nil
			}
		}
	}
	return false, nil
}

func (f *fakeRepo) UserIDsWithFlag(ctx context.Context, schoolID, academicYearID int64, flag string) ([]int64, error) {
	seen := map[int64]bool{}
	var out []int64
	for _, a := range f.assignments {
		if a.AcademicYearID != academicYearID {
			continue
		}
		d, ok := f.duties[a.DutyID]
		if !ok || d.SchoolID != schoolID || !d.Active {
			continue
		}
		hasFlag := false
		for _, fl := range d.Flags {
			if fl == flag {
				hasFlag = true
				break
			}
		}
		if hasFlag && !seen[a.UserID] {
			seen[a.UserID] = true
			out = append(out, a.UserID)
		}
	}
	return out, nil
}

// -- fake consumer-side dependencies --

type fakeIdentity struct {
	eligibleByRole map[string][]int64
}

func (fakeIdentity) HasPermission(role, perm string) bool {
	return role == "admin_sekolah" && perm == PermDutyManage
}
func (fakeIdentity) Log(ctx context.Context, schoolID, userID *int64, action, entity string, entityID *int64, oldValue, newValue any) error {
	return nil
}
func (f fakeIdentity) UsersWithRole(ctx context.Context, schoolID int64, role string) ([]int64, error) {
	return f.eligibleByRole[role], nil
}

type fakeYears struct{ id int64 }

func (f fakeYears) ActiveAcademicYearID(ctx context.Context, schoolID int64) (int64, bool, error) {
	return f.id, true, nil
}

const testSchoolID = 1
const testYearID = 1

func ctxAs(role string, userID int64) context.Context {
	ctx := reqctx.WithUser(context.Background(), userID, role, false)
	ctx = reqctx.WithSchool(ctx, reqctx.School{ID: testSchoolID, Name: "Sekolah Uji", Slug: "uji", Timezone: "Asia/Jakarta"})
	return ctx
}

func newTestService(repo *fakeRepo, identity fakeIdentity) *Service {
	return newServiceForTest(repo, identity, fakeYears{id: testYearID})
}

// -- tests --

func TestUserHasFlag_ActiveYearAndActiveDuty(t *testing.T) {
	repo := newFakeRepo()
	svc := newTestService(repo, fakeIdentity{eligibleByRole: map[string][]int64{RoleGuru: {5}}})
	dutyView, err := svc.CreateDuty(ctxAs("admin_sekolah", 1), 1, testSchoolID, CreateDutyInput{
		Name: "Wali Kelas", ForRole: RoleGuru, Flags: []string{FlagLeaveHomeroomReview},
	})
	if err != nil {
		t.Fatalf("create duty: %v", err)
	}
	if _, err := svc.PutAssignments(ctxAs("admin_sekolah", 1), 1, testSchoolID, dutyView.ID, []int64{5}); err != nil {
		t.Fatalf("put assignments: %v", err)
	}

	ok, err := svc.UserHasFlag(context.Background(), testSchoolID, 5, FlagLeaveHomeroomReview)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("user dengan assignment TA aktif & duty aktif harus punya flag")
	}

	// duty dinonaktifkan -> flag hilang.
	if _, err := svc.UpdateDuty(ctxAs("admin_sekolah", 1), 1, testSchoolID, dutyView.ID, PatchDutyInput{Active: boolPtr(false)}); err != nil {
		t.Fatalf("update duty: %v", err)
	}
	ok, err = svc.UserHasFlag(context.Background(), testSchoolID, 5, FlagLeaveHomeroomReview)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("duty nonaktif harus TIDAK memberi flag")
	}
}

func TestUserHasFlag_AssignmentOtherYearFalse(t *testing.T) {
	repo := newFakeRepo()
	svc := newTestService(repo, fakeIdentity{eligibleByRole: map[string][]int64{RoleGuru: {5}}})
	dutyView, err := svc.CreateDuty(ctxAs("admin_sekolah", 1), 1, testSchoolID, CreateDutyInput{
		Name: "Wali Kelas", ForRole: RoleGuru, Flags: []string{FlagLeaveHomeroomReview},
	})
	if err != nil {
		t.Fatalf("create duty: %v", err)
	}
	// Assignment langsung di repo utk TA LAIN (99) — bukan TA aktif (1).
	if err := repo.ReplaceAssignments(context.Background(), testSchoolID, dutyView.ID, 99, []int64{5}); err != nil {
		t.Fatal(err)
	}

	ok, err := svc.UserHasFlag(context.Background(), testSchoolID, 5, FlagLeaveHomeroomReview)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("assignment di TA LAIN (bukan TA aktif) harus TIDAK memberi flag")
	}
}

func TestPutAssignments_ValidatesRole(t *testing.T) {
	repo := newFakeRepo()
	// user 7 TIDAK ada di daftar guru (eligibleByRole kosong utk guru).
	svc := newTestService(repo, fakeIdentity{eligibleByRole: map[string][]int64{}})
	dutyView, err := svc.CreateDuty(ctxAs("admin_sekolah", 1), 1, testSchoolID, CreateDutyInput{
		Name: "Wali Kelas", ForRole: RoleGuru, Flags: []string{FlagLeaveHomeroomReview},
	})
	if err != nil {
		t.Fatalf("create duty: %v", err)
	}
	_, err = svc.PutAssignments(ctxAs("admin_sekolah", 1), 1, testSchoolID, dutyView.ID, []int64{7})
	var herr *httpx.Error
	if !errors.As(err, &herr) || herr.Status != 422 {
		t.Fatalf("assign user bukan role for_role harus 422 validation, dapat %v", err)
	}
}

func TestPutAssignments_EligibleRoleOK(t *testing.T) {
	repo := newFakeRepo()
	svc := newTestService(repo, fakeIdentity{eligibleByRole: map[string][]int64{RolePegawai: {8}}})
	dutyView, err := svc.CreateDuty(ctxAs("admin_sekolah", 1), 1, testSchoolID, CreateDutyInput{
		Name: "Security", ForRole: RolePegawai, Flags: []string{FlagExitSecurity},
	})
	if err != nil {
		t.Fatalf("create duty: %v", err)
	}
	items, err := svc.PutAssignments(ctxAs("admin_sekolah", 1), 1, testSchoolID, dutyView.ID, []int64{8})
	if err != nil {
		t.Fatalf("assign user eligible role harus sukses: %v", err)
	}
	if len(items) != 1 || items[0].UserID != 8 {
		t.Fatalf("mau 1 assignment user_id=8, dapat %+v", items)
	}
}

func TestDeleteDuty_InUse409(t *testing.T) {
	repo := newFakeRepo()
	svc := newTestService(repo, fakeIdentity{eligibleByRole: map[string][]int64{RoleGuru: {5}}})
	dutyView, err := svc.CreateDuty(ctxAs("admin_sekolah", 1), 1, testSchoolID, CreateDutyInput{
		Name: "Wali Kelas", ForRole: RoleGuru, Flags: []string{FlagLeaveHomeroomReview},
	})
	if err != nil {
		t.Fatalf("create duty: %v", err)
	}
	if _, err := svc.PutAssignments(ctxAs("admin_sekolah", 1), 1, testSchoolID, dutyView.ID, []int64{5}); err != nil {
		t.Fatalf("put assignments: %v", err)
	}
	err = svc.DeleteDuty(ctxAs("admin_sekolah", 1), 1, testSchoolID, dutyView.ID)
	var herr *httpx.Error
	if !errors.As(err, &herr) || herr.Status != 409 {
		t.Fatalf("hapus duty yang punya assignment harus 409, dapat %v", err)
	}
}

func boolPtr(v bool) *bool { return &v }
