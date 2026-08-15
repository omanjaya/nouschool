package student

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/omanjaya/nouschool/internal/student/studentdb"
)

// ErrNotFound menandai baris tidak ditemukan di repository modul student.
var ErrNotFound = errors.New("student: data tidak ditemukan")

// studentRepository adalah kontrak yang dibutuhkan Service dari repository —
// dideklarasikan sebagai interface (dipenuhi *Repository secara struktural)
// supaya Service bisa dites dengan fake repository in-memory, tanpa DB (lihat
// service_test.go, import_test.go).
type studentRepository interface {
	ListStudents(ctx context.Context, f ListStudentsFilter) ([]StudentRecord, int64, error)
	GetStudent(ctx context.Context, schoolID, academicYearID, id int64) (StudentRecord, error)
	GetStudentByNIS(ctx context.Context, schoolID int64, nis string) (StudentRecord, error)
	ListStudentsByIDs(ctx context.Context, schoolID int64, ids []int64) ([]StudentRecord, error)
	CreateStudent(ctx context.Context, in CreateStudentInput) (StudentRecord, error)
	UpdateStudent(ctx context.Context, in UpdateStudentInput) (StudentRecord, error)
	SetStudentUserID(ctx context.Context, id, userID int64) error

	CreateClass(ctx context.Context, in ClassRecord) (ClassRecord, error)
	UpdateClass(ctx context.Context, in UpdateClassInput) (ClassRecord, error)
	GetClassByID(ctx context.Context, schoolID, id int64) (ClassRecord, error)
	GetClassByNameYear(ctx context.Context, schoolID, academicYearID int64, name string) (ClassRecord, error)
	ListClasses(ctx context.Context, schoolID, academicYearID int64) ([]Class, error)

	EnrollStudentsBatch(ctx context.Context, schoolID, classID, academicYearID int64, studentIDs []int64) ([]EnrollResult, error)
	UnenrollStudent(ctx context.Context, studentID, classID int64) error

	CreateGuardian(ctx context.Context, schoolID, userID, studentID int64, relation string) error
	IsGuardianOf(ctx context.Context, userID, studentID int64) (bool, error)
	HasGuardian(ctx context.Context, studentID int64) (bool, error)
	ListChildrenForGuardian(ctx context.Context, schoolID, userID, academicYearID int64) ([]ChildRef, error)

	CreateTeacher(ctx context.Context, schoolID, userID int64, nip string) (Teacher, error)
	UpdateTeacher(ctx context.Context, schoolID, id int64, nip string) (Teacher, error)
	GetTeacherByID(ctx context.Context, schoolID, id int64) (Teacher, error)
	GetTeacherByUserID(ctx context.Context, schoolID, userID int64) (Teacher, error)
	GetTeacherByEmail(ctx context.Context, schoolID int64, email string) (Teacher, error)
	GetTeacherByNIP(ctx context.Context, schoolID int64, nip string) (Teacher, error)
	ListTeachers(ctx context.Context, schoolID int64) ([]Teacher, error)

	CreateSubject(ctx context.Context, schoolID int64, code, name string) (Subject, error)
	UpdateSubject(ctx context.Context, schoolID, id int64, code, name string) (Subject, error)
	GetSubjectByCode(ctx context.Context, schoolID int64, code string) (Subject, error)
	ListSubjects(ctx context.Context, schoolID int64) ([]Subject, error)

	CommitStudentImport(ctx context.Context, schoolID, academicYearID int64, rows []ImportStudentRow) (created, updated int, err error)
}

var _ studentRepository = (*Repository)(nil)

// Repository membungkus akses data modul student (sqlc + pgx).
type Repository struct {
	pool *pgxpool.Pool
	q    *studentdb.Queries
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool, q: studentdb.New(pool)}
}

func mapNoRows(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

func textOrNil(s string) pgtype.Text {
	if s == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: s, Valid: true}
}

func int8OrNil(v int64) pgtype.Int8 {
	if v == 0 {
		return pgtype.Int8{}
	}
	return pgtype.Int8{Int64: v, Valid: true}
}

func int8ToInt64(v pgtype.Int8) int64 {
	if !v.Valid {
		return 0
	}
	return v.Int64
}

func dateOrNil(d *Date) pgtype.Date {
	if d == nil || d.Time.IsZero() {
		return pgtype.Date{}
	}
	return pgtype.Date{Time: d.Time, Valid: true}
}

func dateFromDB(d pgtype.Date) *Date {
	if !d.Valid {
		return nil
	}
	v := NewDate(d.Time)
	return &v
}

func studentFromDB(s studentdb.Student) StudentRecord {
	return StudentRecord{
		ID: s.ID, SchoolID: s.SchoolID, NIS: s.Nis, NISN: s.Nisn.String, Name: s.Name,
		Gender: s.Gender.String, BirthDate: dateFromDB(s.BirthDate), UserID: int8ToInt64(s.UserID),
		Status: s.Status,
	}
}

func classFromDB(c studentdb.Class) ClassRecord {
	return ClassRecord{
		ID: c.ID, SchoolID: c.SchoolID, AcademicYearID: c.AcademicYearID, Name: c.Name,
		Grade: c.Grade, Major: c.Major.String, HomeroomTeacherID: int8ToInt64(c.HomeroomTeacherID),
	}
}

// -- students --

type ListStudentsFilter struct {
	SchoolID       int64
	AcademicYearID int64
	Query          string
	ClassID        int64
	Limit          int64
	Offset         int64
}

func (r *Repository) ListStudents(ctx context.Context, f ListStudentsFilter) ([]StudentRecord, int64, error) {
	rows, err := r.q.ListStudents(ctx, studentdb.ListStudentsParams{
		AcademicYearID: f.AcademicYearID,
		SchoolID:       f.SchoolID,
		Q:              textOrNil(f.Query),
		ClassID:        int8OrNil(f.ClassID),
		Limit:          f.Limit,
		Offset:         f.Offset,
	})
	if err != nil {
		return nil, 0, err
	}
	total, err := r.q.CountStudents(ctx, studentdb.CountStudentsParams{
		SchoolID: f.SchoolID,
		Q:        textOrNil(f.Query),
		ClassID:  int8OrNil(f.ClassID),
	})
	if err != nil {
		return nil, 0, err
	}
	out := make([]StudentRecord, 0, len(rows))
	for _, row := range rows {
		out = append(out, StudentRecord{
			ID: row.ID, SchoolID: f.SchoolID, NIS: row.Nis, NISN: row.Nisn.String, Name: row.Name,
			Gender: row.Gender.String, BirthDate: dateFromDB(row.BirthDate), Status: row.Status,
			ClassID: int8ToInt64(row.ClassID), ClassName: row.ClassName.String,
		})
	}
	return out, total, nil
}

func (r *Repository) GetStudent(ctx context.Context, schoolID, academicYearID, id int64) (StudentRecord, error) {
	row, err := r.q.GetStudentByID(ctx, studentdb.GetStudentByIDParams{
		AcademicYearID: academicYearID, ID: id, SchoolID: schoolID,
	})
	if err != nil {
		return StudentRecord{}, mapNoRows(err)
	}
	return StudentRecord{
		ID: row.ID, SchoolID: row.SchoolID, NIS: row.Nis, NISN: row.Nisn.String, Name: row.Name,
		Gender: row.Gender.String, BirthDate: dateFromDB(row.BirthDate), UserID: int8ToInt64(row.UserID),
		Status: row.Status, ClassID: int8ToInt64(row.ClassID), ClassName: row.ClassName.String,
	}, nil
}

func (r *Repository) GetStudentByNIS(ctx context.Context, schoolID int64, nis string) (StudentRecord, error) {
	row, err := r.q.GetStudentByNIS(ctx, studentdb.GetStudentByNISParams{SchoolID: schoolID, Nis: nis})
	if err != nil {
		return StudentRecord{}, mapNoRows(err)
	}
	return studentFromDB(row), nil
}

func (r *Repository) ListStudentsByIDs(ctx context.Context, schoolID int64, ids []int64) ([]StudentRecord, error) {
	rows, err := r.q.ListStudentsByIDs(ctx, studentdb.ListStudentsByIDsParams{SchoolID: schoolID, Ids: ids})
	if err != nil {
		return nil, err
	}
	out := make([]StudentRecord, 0, len(rows))
	for _, row := range rows {
		out = append(out, studentFromDB(row))
	}
	return out, nil
}

type CreateStudentInput struct {
	SchoolID  int64
	NIS       string
	NISN      string
	Name      string
	Gender    string
	BirthDate *Date
}

func (r *Repository) CreateStudent(ctx context.Context, in CreateStudentInput) (StudentRecord, error) {
	row, err := r.q.CreateStudent(ctx, studentdb.CreateStudentParams{
		SchoolID: in.SchoolID, Nis: in.NIS, Nisn: textOrNil(in.NISN), Name: in.Name,
		Gender: textOrNil(in.Gender), BirthDate: dateOrNil(in.BirthDate),
	})
	if err != nil {
		return StudentRecord{}, err
	}
	return studentFromDB(row), nil
}

// UpdateStudentInput — field kosong ("") / nil berarti tidak diubah (PATCH semantics).
type UpdateStudentInput struct {
	SchoolID  int64
	ID        int64
	NIS       string
	NISN      string
	Name      string
	Gender    string
	BirthDate *Date
	Status    string
}

func (r *Repository) UpdateStudent(ctx context.Context, in UpdateStudentInput) (StudentRecord, error) {
	row, err := r.q.UpdateStudent(ctx, studentdb.UpdateStudentParams{
		Nis: textOrNil(in.NIS), Nisn: textOrNil(in.NISN), Name: textOrNil(in.Name),
		Gender: textOrNil(in.Gender), BirthDate: dateOrNil(in.BirthDate), Status: textOrNil(in.Status),
		ID: in.ID, SchoolID: in.SchoolID,
	})
	if err != nil {
		return StudentRecord{}, mapNoRows(err)
	}
	return studentFromDB(row), nil
}

func (r *Repository) SetStudentUserID(ctx context.Context, id, userID int64) error {
	return r.q.SetStudentUserID(ctx, studentdb.SetStudentUserIDParams{ID: id, UserID: int8OrNil(userID)})
}

// -- classes --

func (r *Repository) CreateClass(ctx context.Context, in ClassRecord) (ClassRecord, error) {
	row, err := r.q.CreateClass(ctx, studentdb.CreateClassParams{
		SchoolID: in.SchoolID, AcademicYearID: in.AcademicYearID, Name: in.Name, Grade: in.Grade,
		Major: textOrNil(in.Major), HomeroomTeacherID: int8OrNil(in.HomeroomTeacherID),
	})
	if err != nil {
		return ClassRecord{}, err
	}
	return classFromDB(row), nil
}

type UpdateClassInput struct {
	SchoolID          int64
	ID                int64
	Name              string
	Grade             string
	Major             string
	HomeroomTeacherID int64 // 0 berarti tidak diubah (PATCH semantics)
}

func (r *Repository) UpdateClass(ctx context.Context, in UpdateClassInput) (ClassRecord, error) {
	row, err := r.q.UpdateClass(ctx, studentdb.UpdateClassParams{
		Name: textOrNil(in.Name), Grade: textOrNil(in.Grade), Major: textOrNil(in.Major),
		HomeroomTeacherID: int8OrNil(in.HomeroomTeacherID), ID: in.ID, SchoolID: in.SchoolID,
	})
	if err != nil {
		return ClassRecord{}, mapNoRows(err)
	}
	return classFromDB(row), nil
}

func (r *Repository) GetClassByID(ctx context.Context, schoolID, id int64) (ClassRecord, error) {
	row, err := r.q.GetClassByID(ctx, studentdb.GetClassByIDParams{ID: id, SchoolID: schoolID})
	if err != nil {
		return ClassRecord{}, mapNoRows(err)
	}
	return classFromDB(row), nil
}

func (r *Repository) GetClassByNameYear(ctx context.Context, schoolID, academicYearID int64, name string) (ClassRecord, error) {
	row, err := r.q.GetClassByNameYear(ctx, studentdb.GetClassByNameYearParams{
		SchoolID: schoolID, AcademicYearID: academicYearID, Name: name,
	})
	if err != nil {
		return ClassRecord{}, mapNoRows(err)
	}
	return classFromDB(row), nil
}

func (r *Repository) ListClasses(ctx context.Context, schoolID, academicYearID int64) ([]Class, error) {
	rows, err := r.q.ListClasses(ctx, studentdb.ListClassesParams{SchoolID: schoolID, AcademicYearID: academicYearID})
	if err != nil {
		return nil, err
	}
	out := make([]Class, 0, len(rows))
	for _, row := range rows {
		c := Class{
			ID: row.ID, Name: row.Name, Grade: row.Grade, Major: row.Major.String,
			StudentCount: row.StudentCount,
		}
		if row.TeacherRowID.Valid {
			c.HomeroomTeacher = &TeacherRef{ID: row.TeacherRowID.Int64, Name: row.TeacherName.String}
		}
		out = append(out, c)
	}
	return out, nil
}

// -- enrollments --

// EnrollResult melaporkan hasil satu enroll (dipakai batch enroll & import).
type EnrollResult struct {
	StudentID   int64
	Enrolled    bool // baru terdaftar / sudah ada di kelas ini (idempoten)
	Moved       bool // dipindah dari kelas lain di tahun ajaran yang sama
	FromClassID int64
}

// EnrollStudentsBatch mendaftarkan sekumpulan siswa ke satu rombel dalam satu
// transaksi. Siswa yang sudah terdaftar di rombel ITU diabaikan (idempoten).
// Siswa yang sudah terdaftar di rombel LAIN pada tahun ajaran yang sama
// dipindahkan (enrollment lama dihapus).
func (r *Repository) EnrollStudentsBatch(ctx context.Context, schoolID, classID, academicYearID int64, studentIDs []int64) ([]EnrollResult, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := r.q.WithTx(tx)

	results := make([]EnrollResult, 0, len(studentIDs))
	for _, studentID := range studentIDs {
		res := EnrollResult{StudentID: studentID}

		existing, err := qtx.GetEnrollmentInYear(ctx, studentdb.GetEnrollmentInYearParams{
			StudentID: studentID, AcademicYearID: academicYearID,
		})
		switch {
		case err == nil && existing.ClassID == classID:
			// sudah terdaftar di rombel ini — idempoten, tidak ada perubahan.
			res.Enrolled = true
		case err == nil:
			// terdaftar di rombel lain tahun ajaran yang sama — pindahkan.
			if err := qtx.DeleteEnrollment(ctx, existing.ID); err != nil {
				return nil, err
			}
			if _, err := qtx.UpsertEnrollment(ctx, studentdb.UpsertEnrollmentParams{
				SchoolID: schoolID, StudentID: studentID, ClassID: classID,
			}); err != nil {
				return nil, err
			}
			res.Enrolled = true
			res.Moved = true
			res.FromClassID = existing.ClassID
		case errors.Is(err, pgx.ErrNoRows):
			if _, err := qtx.UpsertEnrollment(ctx, studentdb.UpsertEnrollmentParams{
				SchoolID: schoolID, StudentID: studentID, ClassID: classID,
			}); err != nil {
				return nil, err
			}
			res.Enrolled = true
		default:
			return nil, err
		}
		results = append(results, res)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return results, nil
}

func (r *Repository) UnenrollStudent(ctx context.Context, studentID, classID int64) error {
	return r.q.DeleteEnrollmentByStudentClass(ctx, studentdb.DeleteEnrollmentByStudentClassParams{
		StudentID: studentID, ClassID: classID,
	})
}

// -- guardians --

func (r *Repository) CreateGuardian(ctx context.Context, schoolID, userID, studentID int64, relation string) error {
	_, err := r.q.CreateGuardian(ctx, studentdb.CreateGuardianParams{
		SchoolID: schoolID, UserID: userID, StudentID: studentID, Relation: relation,
	})
	return err
}

// IsGuardianOf melaporkan apakah user (userID) adalah wali dari siswa (studentID).
func (r *Repository) IsGuardianOf(ctx context.Context, userID, studentID int64) (bool, error) {
	_, err := r.q.GetGuardianByUserStudent(ctx, studentdb.GetGuardianByUserStudentParams{
		UserID: userID, StudentID: studentID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// HasGuardian melaporkan apakah siswa (studentID) sudah punya wali terdaftar
// (dipakai generator undangan: kalau sudah ada, tidak perlu kode wali baru).
func (r *Repository) HasGuardian(ctx context.Context, studentID int64) (bool, error) {
	n, err := r.q.CountGuardiansForStudent(ctx, studentID)
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// ListChildrenForGuardian — dipakai GET /api/me/children (role orang_tua).
func (r *Repository) ListChildrenForGuardian(ctx context.Context, schoolID, userID, academicYearID int64) ([]ChildRef, error) {
	rows, err := r.q.ListChildrenForGuardian(ctx, studentdb.ListChildrenForGuardianParams{
		AcademicYearID: academicYearID, UserID: userID, SchoolID: schoolID,
	})
	if err != nil {
		return nil, err
	}
	out := make([]ChildRef, 0, len(rows))
	for _, row := range rows {
		out = append(out, ChildRef{StudentID: row.ID, Name: row.Name, ClassName: row.ClassName.String})
	}
	return out, nil
}

// -- teachers --

func teacherView(id, userID int64, nip pgtype.Text, name string, email pgtype.Text) Teacher {
	return Teacher{ID: id, UserID: userID, Name: name, Email: email.String, NIP: nip.String}
}

func (r *Repository) CreateTeacher(ctx context.Context, schoolID, userID int64, nip string) (Teacher, error) {
	row, err := r.q.CreateTeacher(ctx, studentdb.CreateTeacherParams{SchoolID: schoolID, UserID: userID, Nip: textOrNil(nip)})
	if err != nil {
		return Teacher{}, err
	}
	// CreateTeacher tidak join users — ambil profil lengkap sekali lagi.
	return r.GetTeacherByID(ctx, schoolID, row.ID)
}

func (r *Repository) UpdateTeacher(ctx context.Context, schoolID, id int64, nip string) (Teacher, error) {
	if _, err := r.q.UpdateTeacher(ctx, studentdb.UpdateTeacherParams{Nip: textOrNil(nip), ID: id, SchoolID: schoolID}); err != nil {
		return Teacher{}, mapNoRows(err)
	}
	return r.GetTeacherByID(ctx, schoolID, id)
}

func (r *Repository) GetTeacherByID(ctx context.Context, schoolID, id int64) (Teacher, error) {
	row, err := r.q.GetTeacherByID(ctx, studentdb.GetTeacherByIDParams{ID: id, SchoolID: schoolID})
	if err != nil {
		return Teacher{}, mapNoRows(err)
	}
	return teacherView(row.ID, row.UserID, row.Nip, row.UserName, row.UserEmail), nil
}

func (r *Repository) GetTeacherByUserID(ctx context.Context, schoolID, userID int64) (Teacher, error) {
	row, err := r.q.GetTeacherByUserID(ctx, studentdb.GetTeacherByUserIDParams{SchoolID: schoolID, UserID: userID})
	if err != nil {
		return Teacher{}, mapNoRows(err)
	}
	return teacherView(row.ID, row.UserID, row.Nip, row.UserName, row.UserEmail), nil
}

func (r *Repository) GetTeacherByEmail(ctx context.Context, schoolID int64, email string) (Teacher, error) {
	row, err := r.q.GetTeacherByEmail(ctx, studentdb.GetTeacherByEmailParams{SchoolID: schoolID, Email: textOrNil(email)})
	if err != nil {
		return Teacher{}, mapNoRows(err)
	}
	return teacherView(row.ID, row.UserID, row.Nip, row.UserName, row.UserEmail), nil
}

func (r *Repository) GetTeacherByNIP(ctx context.Context, schoolID int64, nip string) (Teacher, error) {
	row, err := r.q.GetTeacherByNIP(ctx, studentdb.GetTeacherByNIPParams{SchoolID: schoolID, Nip: textOrNil(nip)})
	if err != nil {
		return Teacher{}, mapNoRows(err)
	}
	return teacherView(row.ID, row.UserID, row.Nip, row.UserName, row.UserEmail), nil
}

func (r *Repository) ListTeachers(ctx context.Context, schoolID int64) ([]Teacher, error) {
	rows, err := r.q.ListTeachers(ctx, schoolID)
	if err != nil {
		return nil, err
	}
	out := make([]Teacher, 0, len(rows))
	for _, row := range rows {
		out = append(out, teacherView(row.ID, row.UserID, row.Nip, row.UserName, row.UserEmail))
	}
	return out, nil
}

// -- subjects --

func subjectFromDB(s studentdb.Subject) Subject {
	return Subject{ID: s.ID, Code: s.Code, Name: s.Name}
}

func (r *Repository) CreateSubject(ctx context.Context, schoolID int64, code, name string) (Subject, error) {
	row, err := r.q.CreateSubject(ctx, studentdb.CreateSubjectParams{SchoolID: schoolID, Code: code, Name: name})
	if err != nil {
		return Subject{}, err
	}
	return subjectFromDB(row), nil
}

func (r *Repository) UpdateSubject(ctx context.Context, schoolID, id int64, code, name string) (Subject, error) {
	row, err := r.q.UpdateSubject(ctx, studentdb.UpdateSubjectParams{
		Code: textOrNil(code), Name: textOrNil(name), ID: id, SchoolID: schoolID,
	})
	if err != nil {
		return Subject{}, mapNoRows(err)
	}
	return subjectFromDB(row), nil
}

func (r *Repository) GetSubjectByCode(ctx context.Context, schoolID int64, code string) (Subject, error) {
	row, err := r.q.GetSubjectByCode(ctx, studentdb.GetSubjectByCodeParams{SchoolID: schoolID, Code: code})
	if err != nil {
		return Subject{}, mapNoRows(err)
	}
	return subjectFromDB(row), nil
}

func (r *Repository) ListSubjects(ctx context.Context, schoolID int64) ([]Subject, error) {
	rows, err := r.q.ListSubjects(ctx, schoolID)
	if err != nil {
		return nil, err
	}
	out := make([]Subject, 0, len(rows))
	for _, row := range rows {
		out = append(out, subjectFromDB(row))
	}
	return out, nil
}

// -- import commit (transaksional) --

// ImportStudentRow adalah satu baris import siswa yang sudah divalidasi,
// dipakai CommitStudentImport.
type ImportStudentRow struct {
	NIS       string
	NISN      string
	Name      string
	Gender    string
	BirthDate *Date
	ClassID   int64 // 0 = tidak diisi rombel
	Existing  bool  // true bila NIS sudah ada (update), false bila baru (create)
}

// CommitStudentImport melakukan upsert siswa by (school_id, nis) + enroll ke
// rombel (dengan logika pindah kelas yang sama seperti EnrollStudentsBatch),
// dalam satu transaksi.
func (r *Repository) CommitStudentImport(ctx context.Context, schoolID, academicYearID int64, rows []ImportStudentRow) (created, updated int, err error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return 0, 0, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := r.q.WithTx(tx)

	for _, row := range rows {
		var studentID int64
		if row.Existing {
			existing, gerr := qtx.GetStudentByNIS(ctx, studentdb.GetStudentByNISParams{SchoolID: schoolID, Nis: row.NIS})
			if gerr != nil {
				return 0, 0, gerr
			}
			updatedRow, uerr := qtx.UpdateStudent(ctx, studentdb.UpdateStudentParams{
				Nis: textOrNil(row.NIS), Nisn: textOrNil(row.NISN), Name: textOrNil(row.Name),
				Gender: textOrNil(row.Gender), BirthDate: dateOrNil(row.BirthDate),
				ID: existing.ID, SchoolID: schoolID,
			})
			if uerr != nil {
				return 0, 0, uerr
			}
			studentID = updatedRow.ID
			updated++
		} else {
			createdRow, cerr := qtx.CreateStudent(ctx, studentdb.CreateStudentParams{
				SchoolID: schoolID, Nis: row.NIS, Nisn: textOrNil(row.NISN), Name: row.Name,
				Gender: textOrNil(row.Gender), BirthDate: dateOrNil(row.BirthDate),
			})
			if cerr != nil {
				return 0, 0, cerr
			}
			studentID = createdRow.ID
			created++
		}

		if row.ClassID == 0 {
			continue
		}
		existingEnr, eerr := qtx.GetEnrollmentInYear(ctx, studentdb.GetEnrollmentInYearParams{
			StudentID: studentID, AcademicYearID: academicYearID,
		})
		switch {
		case eerr == nil && existingEnr.ClassID == row.ClassID:
			// sudah di rombel yang benar.
		case eerr == nil:
			if err := qtx.DeleteEnrollment(ctx, existingEnr.ID); err != nil {
				return 0, 0, err
			}
			if _, err := qtx.UpsertEnrollment(ctx, studentdb.UpsertEnrollmentParams{
				SchoolID: schoolID, StudentID: studentID, ClassID: row.ClassID,
			}); err != nil {
				return 0, 0, err
			}
		case errors.Is(eerr, pgx.ErrNoRows):
			if _, err := qtx.UpsertEnrollment(ctx, studentdb.UpsertEnrollmentParams{
				SchoolID: schoolID, StudentID: studentID, ClassID: row.ClassID,
			}); err != nil {
				return 0, 0, err
			}
		default:
			return 0, 0, eerr
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, 0, err
	}
	return created, updated, nil
}
