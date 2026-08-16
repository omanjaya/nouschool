package counseling

import (
	"context"
	"errors"
	"testing"

	"github.com/omanjaya/nouschool/internal/platform/clock"
	"github.com/omanjaya/nouschool/internal/platform/httpx"
	"github.com/omanjaya/nouschool/internal/platform/reqctx"
)

// -- fakes (in-memory, tanpa DB) --

type fakeRepo struct {
	rows   map[int64]Row
	nextID int64
}

func newFakeRepo() *fakeRepo { return &fakeRepo{rows: map[int64]Row{}} }

var _ counselingRepository = (*fakeRepo)(nil)

func (f *fakeRepo) CreateCounseling(ctx context.Context, in CreateRecordInput) (Record, error) {
	f.nextID++
	rec := Record{
		ID: f.nextID, SchoolID: in.SchoolID, AcademicYearID: in.AcademicYearID, StudentID: in.StudentID,
		CounselorID: in.CounselorID, CareerGoals: in.CareerGoals, ProblemDescription: in.ProblemDescription,
		FollowUpPlan: in.FollowUpPlan, Evidence: in.Evidence, EvidenceName: in.EvidenceName, EvidenceMime: in.EvidenceMime,
	}
	f.rows[rec.ID] = Row{Record: rec, StudentName: "Siswa", StudentNIS: "0001", ClassName: "XII RPL 1", CounselorName: "Konselor"}
	return rec, nil
}

func (f *fakeRepo) GetRow(ctx context.Context, schoolID, id int64) (Row, error) {
	r, ok := f.rows[id]
	if !ok || r.SchoolID != schoolID {
		return Row{}, ErrNotFound
	}
	return r, nil
}

func (f *fakeRepo) GetByID(ctx context.Context, schoolID, id int64) (Record, error) {
	r, ok := f.rows[id]
	if !ok || r.SchoolID != schoolID {
		return Record{}, ErrNotFound
	}
	return r.Record, nil
}

func (f *fakeRepo) ListRows(ctx context.Context, schoolID, studentID int64, limit, offset int) ([]Row, error) {
	var out []Row
	for _, r := range f.rows {
		if r.SchoolID != schoolID {
			continue
		}
		if studentID != 0 && r.StudentID != studentID {
			continue
		}
		out = append(out, r)
	}
	return out, nil
}

func (f *fakeRepo) CountRows(ctx context.Context, schoolID, studentID int64) (int64, error) {
	rows, _ := f.ListRows(ctx, schoolID, studentID, 0, 0)
	return int64(len(rows)), nil
}

func (f *fakeRepo) UpdateCounseling(ctx context.Context, schoolID, id int64, in UpdateRecordInput) (Record, error) {
	r, ok := f.rows[id]
	if !ok || r.SchoolID != schoolID {
		return Record{}, ErrNotFound
	}
	r.CareerGoals, r.ProblemDescription, r.FollowUpPlan = in.CareerGoals, in.ProblemDescription, in.FollowUpPlan
	f.rows[id] = r
	return r.Record, nil
}

func (f *fakeRepo) UpdateEvidence(ctx context.Context, schoolID, id int64, path, name, mime string) error {
	return nil
}

func (f *fakeRepo) DeleteCounseling(ctx context.Context, schoolID, id int64) (int64, error) {
	r, ok := f.rows[id]
	if !ok || r.SchoolID != schoolID {
		return 0, nil
	}
	delete(f.rows, id)
	return 1, nil
}

func (f *fakeRepo) GetStudentBasic(ctx context.Context, schoolID, studentID int64) (StudentBasic, error) {
	if studentID == 0 {
		return StudentBasic{}, ErrNotFound
	}
	return StudentBasic{ID: studentID, Name: "Siswa", NIS: "0001"}, nil
}

type fakeIdentity struct{}

func (fakeIdentity) Log(ctx context.Context, schoolID, userID *int64, action, entity string, entityID *int64, oldValue, newValue any) error {
	return nil
}

type fakeYears struct{ active int64 }

func (f fakeYears) ActiveAcademicYearID(ctx context.Context, schoolID int64) (int64, bool, error) {
	if f.active == 0 {
		return 0, false, nil
	}
	return f.active, true, nil
}

type fakeDuty struct{ flagHolders map[int64]bool }

func (f fakeDuty) UserHasFlag(ctx context.Context, schoolID, userID int64, flag string) (bool, error) {
	return f.flagHolders[userID], nil
}

func newTestService(duty fakeDuty) *Service {
	return newServiceForTest(newFakeRepo(), fakeIdentity{}, fakeYears{active: 1}, duty, clock.System{})
}

func ctxAs(userID int64, role string) context.Context {
	ctx := reqctx.WithUser(context.Background(), userID, role, false)
	ctx = reqctx.WithSchool(ctx, reqctx.School{ID: 1, Name: "Demo", Slug: "demo", Timezone: "Asia/Jakarta", Status: "active"})
	return ctx
}

func TestCreateCounseling_BKFlagHolderAllowed(t *testing.T) {
	svc := newTestService(fakeDuty{flagHolders: map[int64]bool{10: true}})
	ctx := ctxAs(10, "guru")
	_, err := svc.CreateCounseling(ctx, 10, 1, CreateInput{StudentID: 5, CareerGoals: "jadi dokter"})
	if err != nil {
		t.Fatalf("guru pemegang flag leave_issuance harus boleh membuat sesi konseling: %v", err)
	}
}

func TestCreateCounseling_GuruBiasaForbidden(t *testing.T) {
	svc := newTestService(fakeDuty{flagHolders: map[int64]bool{}})
	ctx := ctxAs(11, "guru")
	_, err := svc.CreateCounseling(ctx, 11, 1, CreateInput{StudentID: 5})
	var de *httpx.Error
	if !errors.As(err, &de) || de.Status != 403 {
		t.Fatalf("guru TANPA flag leave_issuance harus 403, dapat: %v", err)
	}
}

func TestCreateCounseling_AdminAllowed(t *testing.T) {
	svc := newTestService(fakeDuty{flagHolders: map[int64]bool{}})
	ctx := ctxAs(1, "admin_sekolah")
	_, err := svc.CreateCounseling(ctx, 1, 1, CreateInput{StudentID: 5})
	if err != nil {
		t.Fatalf("admin_sekolah harus boleh membuat sesi konseling: %v", err)
	}
}

func TestCreateCounseling_KepsekForbidden(t *testing.T) {
	svc := newTestService(fakeDuty{flagHolders: map[int64]bool{}})
	ctx := ctxAs(2, "kepala_sekolah")
	_, err := svc.CreateCounseling(ctx, 2, 1, CreateInput{StudentID: 5})
	var de *httpx.Error
	if !errors.As(err, &de) || de.Status != 403 {
		t.Fatalf("kepala_sekolah read-only, TIDAK boleh membuat: %v", err)
	}
}

func TestListCounselings_KepsekReadOnlyAllowed(t *testing.T) {
	svc := newTestService(fakeDuty{flagHolders: map[int64]bool{}})
	ctx := ctxAs(2, "kepala_sekolah")
	if _, err := svc.ListCounselings(ctx, 1, 0, 1); err != nil {
		t.Fatalf("kepala_sekolah harus boleh membaca daftar konseling: %v", err)
	}
}

func TestListCounselings_SiswaForbidden(t *testing.T) {
	svc := newTestService(fakeDuty{flagHolders: map[int64]bool{}})
	ctx := ctxAs(20, "siswa")
	_, err := svc.ListCounselings(ctx, 1, 0, 1)
	var de *httpx.Error
	if !errors.As(err, &de) || de.Status != 403 {
		t.Fatalf("siswa TIDAK boleh mengakses konseling (privat BK): %v", err)
	}
}

func TestListCounselings_OrangTuaForbidden(t *testing.T) {
	svc := newTestService(fakeDuty{flagHolders: map[int64]bool{}})
	ctx := ctxAs(21, "orang_tua")
	_, err := svc.ListCounselings(ctx, 1, 0, 1)
	var de *httpx.Error
	if !errors.As(err, &de) || de.Status != 403 {
		t.Fatalf("orang tua TIDAK boleh mengakses konseling (privat BK): %v", err)
	}
}

func TestUpdateCounseling_OwnerAllowed_OtherBKForbidden(t *testing.T) {
	svc := newTestService(fakeDuty{flagHolders: map[int64]bool{10: true, 12: true}})
	owner := ctxAs(10, "guru")
	rec, err := svc.CreateCounseling(owner, 10, 1, CreateInput{StudentID: 5, CareerGoals: "awal"})
	if err != nil {
		t.Fatalf("gagal membuat sesi: %v", err)
	}

	// Pembuat (owner) boleh mengubah.
	if _, err := svc.UpdateCounseling(owner, 10, 1, rec.ID, UpdateInput{CareerGoals: "ubah"}); err != nil {
		t.Fatalf("pembuat harus boleh mengubah: %v", err)
	}

	// Guru BK LAIN (bukan pembuat) TIDAK boleh mengubah (docs tugas: "pembuat/admin").
	other := ctxAs(12, "guru")
	_, err = svc.UpdateCounseling(other, 12, 1, rec.ID, UpdateInput{CareerGoals: "coba ubah"})
	var de *httpx.Error
	if !errors.As(err, &de) || de.Status != 403 {
		t.Fatalf("BK lain (bukan pembuat) harus 403 saat update, dapat: %v", err)
	}

	// Admin boleh mengubah sesi siapa pun.
	admin := ctxAs(1, "admin_sekolah")
	if _, err := svc.UpdateCounseling(admin, 1, 1, rec.ID, UpdateInput{CareerGoals: "admin ubah"}); err != nil {
		t.Fatalf("admin harus boleh mengubah sesi siapa pun: %v", err)
	}
}

func TestDeleteCounseling_OwnerOrAdminOnly(t *testing.T) {
	svc := newTestService(fakeDuty{flagHolders: map[int64]bool{10: true, 12: true}})
	owner := ctxAs(10, "guru")
	rec, err := svc.CreateCounseling(owner, 10, 1, CreateInput{StudentID: 5})
	if err != nil {
		t.Fatalf("gagal membuat sesi: %v", err)
	}

	other := ctxAs(12, "guru")
	if err := svc.DeleteCounseling(other, 12, 1, rec.ID); err == nil {
		t.Fatal("BK lain (bukan pembuat) harus ditolak menghapus")
	}

	if err := svc.DeleteCounseling(owner, 10, 1, rec.ID); err != nil {
		t.Fatalf("pembuat harus boleh menghapus: %v", err)
	}
}
