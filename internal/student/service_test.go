package student

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/omanjaya/nouschool/internal/platform/httpx"
	"github.com/omanjaya/nouschool/internal/platform/reqctx"
)

// fakeRepo implementasi studentRepository in-memory (tanpa DB) — dipakai
// menguji object-level access GetStudent tanpa Postgres.
type fakeRepo struct {
	students  map[int64]StudentRecord
	guardians map[int64]map[int64]bool // studentID -> set(userID)
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{students: make(map[int64]StudentRecord), guardians: make(map[int64]map[int64]bool)}
}

func (f *fakeRepo) ListStudents(ctx context.Context, filter ListStudentsFilter) ([]StudentRecord, int64, error) {
	return nil, 0, nil
}

func (f *fakeRepo) GetStudent(ctx context.Context, schoolID, academicYearID, id int64) (StudentRecord, error) {
	rec, ok := f.students[id]
	if !ok || rec.SchoolID != schoolID {
		return StudentRecord{}, ErrNotFound
	}
	return rec, nil
}

func (f *fakeRepo) GetStudentByNIS(ctx context.Context, schoolID int64, nis string) (StudentRecord, error) {
	for _, r := range f.students {
		if r.SchoolID == schoolID && r.NIS == nis {
			return r, nil
		}
	}
	return StudentRecord{}, ErrNotFound
}

func (f *fakeRepo) ListStudentsByIDs(ctx context.Context, schoolID int64, ids []int64) ([]StudentRecord, error) {
	return nil, nil
}
func (f *fakeRepo) CreateStudent(ctx context.Context, in CreateStudentInput) (StudentRecord, error) {
	return StudentRecord{}, nil
}
func (f *fakeRepo) UpdateStudent(ctx context.Context, in UpdateStudentInput) (StudentRecord, error) {
	return StudentRecord{}, nil
}
func (f *fakeRepo) SetStudentUserID(ctx context.Context, id, userID int64) error { return nil }

func (f *fakeRepo) CreateClass(ctx context.Context, in ClassRecord) (ClassRecord, error) {
	return ClassRecord{}, nil
}
func (f *fakeRepo) UpdateClass(ctx context.Context, in UpdateClassInput) (ClassRecord, error) {
	return ClassRecord{}, nil
}
func (f *fakeRepo) GetClassByID(ctx context.Context, schoolID, id int64) (ClassRecord, error) {
	return ClassRecord{}, ErrNotFound
}
func (f *fakeRepo) GetClassByNameYear(ctx context.Context, schoolID, academicYearID int64, name string) (ClassRecord, error) {
	return ClassRecord{}, ErrNotFound
}
func (f *fakeRepo) ListClasses(ctx context.Context, schoolID, academicYearID int64) ([]Class, error) {
	return nil, nil
}

func (f *fakeRepo) EnrollStudentsBatch(ctx context.Context, schoolID, classID, academicYearID int64, studentIDs []int64) ([]EnrollResult, error) {
	return nil, nil
}
func (f *fakeRepo) UnenrollStudent(ctx context.Context, studentID, classID int64) error { return nil }

func (f *fakeRepo) CreateGuardian(ctx context.Context, schoolID, userID, studentID int64, relation string) error {
	if f.guardians[studentID] == nil {
		f.guardians[studentID] = map[int64]bool{}
	}
	f.guardians[studentID][userID] = true
	return nil
}

func (f *fakeRepo) IsGuardianOf(ctx context.Context, userID, studentID int64) (bool, error) {
	return f.guardians[studentID][userID], nil
}

func (f *fakeRepo) HasGuardian(ctx context.Context, studentID int64) (bool, error) {
	return len(f.guardians[studentID]) > 0, nil
}

func (f *fakeRepo) ListChildrenForGuardian(ctx context.Context, schoolID, userID, academicYearID int64) ([]ChildRef, error) {
	return nil, nil
}

func (f *fakeRepo) CreateTeacher(ctx context.Context, schoolID, userID int64, nip string) (Teacher, error) {
	return Teacher{}, nil
}
func (f *fakeRepo) UpdateTeacher(ctx context.Context, schoolID, id int64, nip string) (Teacher, error) {
	return Teacher{}, nil
}
func (f *fakeRepo) GetTeacherByID(ctx context.Context, schoolID, id int64) (Teacher, error) {
	return Teacher{}, ErrNotFound
}
func (f *fakeRepo) GetTeacherByUserID(ctx context.Context, schoolID, userID int64) (Teacher, error) {
	return Teacher{}, ErrNotFound
}
func (f *fakeRepo) GetTeacherByEmail(ctx context.Context, schoolID int64, email string) (Teacher, error) {
	return Teacher{}, ErrNotFound
}
func (f *fakeRepo) GetTeacherByNIP(ctx context.Context, schoolID int64, nip string) (Teacher, error) {
	return Teacher{}, ErrNotFound
}
func (f *fakeRepo) ListTeachers(ctx context.Context, schoolID int64) ([]Teacher, error) {
	return nil, nil
}

func (f *fakeRepo) CreateSubject(ctx context.Context, schoolID int64, code, name string) (Subject, error) {
	return Subject{}, nil
}
func (f *fakeRepo) UpdateSubject(ctx context.Context, schoolID, id int64, code, name string) (Subject, error) {
	return Subject{}, nil
}
func (f *fakeRepo) GetSubjectByCode(ctx context.Context, schoolID int64, code string) (Subject, error) {
	return Subject{}, ErrNotFound
}
func (f *fakeRepo) ListSubjects(ctx context.Context, schoolID int64) ([]Subject, error) {
	return nil, nil
}

func (f *fakeRepo) CommitStudentImport(ctx context.Context, schoolID, academicYearID int64, rows []ImportStudentRow) (int, int, error) {
	return 0, 0, nil
}

// fakeIdentityGateway implementasi IdentityGateway minimal untuk test.
type fakeIdentityGateway struct {
	perms map[string]map[string]bool // role -> perm -> bool
}

func newFakeIdentityGateway() *fakeIdentityGateway {
	return &fakeIdentityGateway{perms: map[string]map[string]bool{
		"admin_sekolah":  {PermStudentManage: true, PermStudentRead: true},
		"kepala_sekolah": {PermStudentRead: true},
		"guru":           {PermStudentRead: true},
	}}
}

func (f *fakeIdentityGateway) HashPassword(password string) (string, error) {
	return "hash:" + password, nil
}
func (f *fakeIdentityGateway) HasPermission(role, perm string) bool { return f.perms[role][perm] }
func (f *fakeIdentityGateway) CreateAccount(ctx context.Context, email, username, passwordHash, name string) (int64, error) {
	return 1, nil
}
func (f *fakeIdentityGateway) UserIDByEmail(ctx context.Context, email string) (int64, bool, error) {
	return 0, false, nil
}
func (f *fakeIdentityGateway) UserIDByUsername(ctx context.Context, username string) (int64, bool, error) {
	return 0, false, nil
}
func (f *fakeIdentityGateway) CreateMembership(ctx context.Context, userID, schoolID int64, role string) error {
	return nil
}
func (f *fakeIdentityGateway) IssueSession(ctx context.Context, userID, schoolID int64, role, ip, userAgent string) (string, time.Time, error) {
	return "tok", time.Now(), nil
}
func (f *fakeIdentityGateway) SetSessionCookie(w http.ResponseWriter, token string, expiresAt time.Time) {
}
func (f *fakeIdentityGateway) GenerateInvitationCode(ctx context.Context, schoolID int64, role string, targetID int64, ttl time.Duration) (string, time.Time, error) {
	return "CODE1234", time.Now().Add(ttl), nil
}
func (f *fakeIdentityGateway) ValidateInvitationCode(ctx context.Context, code string) (int64, string, int64, bool, error) {
	return 0, "", 0, false, nil
}
func (f *fakeIdentityGateway) MarkInvitationUsed(ctx context.Context, code string) error { return nil }
func (f *fakeIdentityGateway) Log(ctx context.Context, schoolID, userID *int64, action, entity string, entityID *int64, oldValue, newValue any) error {
	return nil
}

// fakeYears implementasi AcademicYearLookup untuk test.
type fakeYears struct{ id int64 }

func (f fakeYears) ActiveAcademicYearID(ctx context.Context, schoolID int64) (int64, bool, error) {
	if f.id == 0 {
		return 0, false, nil
	}
	return f.id, true, nil
}

func ctxAs(role string, userID int64) context.Context {
	ctx := reqctx.WithUser(context.Background(), userID, role, false)
	ctx = reqctx.WithSchool(ctx, reqctx.School{ID: 1, Name: "Sekolah Demo", Slug: "demo"})
	return ctx
}

func TestGetStudentObjectLevelAccess(t *testing.T) {
	repo := newFakeRepo()
	// siswa A (id 10, user_id 100) dan siswa B (id 20, user_id 200), keduanya di sekolah 1.
	repo.students[10] = StudentRecord{ID: 10, SchoolID: 1, Name: "Siswa A", NIS: "1001", UserID: 100, Status: "active"}
	repo.students[20] = StudentRecord{ID: 20, SchoolID: 1, Name: "Siswa B", NIS: "1002", UserID: 200, Status: "active"}
	// orang tua dengan userID 500 adalah wali siswa A (id 10) saja.
	repo.guardians[10] = map[int64]bool{500: true}

	svc := newServiceForTest(repo, newFakeIdentityGateway(), fakeYears{})

	t.Run("orang tua anak sendiri OK", func(t *testing.T) {
		ctx := ctxAs(RoleOrangTua, 500)
		st, err := svc.GetStudent(ctx, 1, 10)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if st.ID != 10 {
			t.Fatalf("dapat siswa salah: %+v", st)
		}
	})

	t.Run("orang tua anak lain FORBIDDEN", func(t *testing.T) {
		ctx := ctxAs(RoleOrangTua, 500)
		_, err := svc.GetStudent(ctx, 1, 20)
		if err != httpx.ErrForbidden {
			t.Fatalf("expected ErrForbidden, dapat: %v", err)
		}
	})

	t.Run("siswa dirinya sendiri OK", func(t *testing.T) {
		ctx := ctxAs(RoleSiswa, 100)
		st, err := svc.GetStudent(ctx, 1, 10)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if st.ID != 10 {
			t.Fatalf("dapat siswa salah: %+v", st)
		}
	})

	t.Run("siswa akses siswa lain FORBIDDEN", func(t *testing.T) {
		ctx := ctxAs(RoleSiswa, 100)
		_, err := svc.GetStudent(ctx, 1, 20)
		if err != httpx.ErrForbidden {
			t.Fatalf("expected ErrForbidden, dapat: %v", err)
		}
	})

	t.Run("guru dengan student:read OK", func(t *testing.T) {
		ctx := ctxAs(RoleGuru, 999)
		st, err := svc.GetStudent(ctx, 1, 20)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if st.ID != 20 {
			t.Fatalf("dapat siswa salah: %+v", st)
		}
	})

	t.Run("siswa tidak ditemukan -> not found", func(t *testing.T) {
		ctx := ctxAs(RoleGuru, 999)
		_, err := svc.GetStudent(ctx, 1, 999)
		if err != httpx.ErrNotFound {
			t.Fatalf("expected ErrNotFound, dapat: %v", err)
		}
	})
}
