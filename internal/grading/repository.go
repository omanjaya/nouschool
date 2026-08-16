package grading

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/omanjaya/nouschool/internal/grading/gradingdb"
)

// ErrNotFound menandai baris tidak ditemukan di repository modul grading.
var ErrNotFound = errors.New("grading: data tidak ditemukan")

// ErrConflict menandai pelanggaran unique constraint.
var ErrConflict = errors.New("grading: data sudah ada/bentrok")

// gradingRepository adalah kontrak yang dibutuhkan Service dari repository —
// dideklarasikan sebagai interface (dipenuhi *Repository secara struktural)
// supaya Service bisa dites dengan fake repository in-memory, tanpa DB —
// pola yang sama dengan internal/discipline.
//
// Catatan desain: kolom student_grades.score (numeric(5,2)) SENGAJA
// di-cast ::float8 di SETIAP query (queries.sql) supaya sqlc menghasilkan
// Go float64 murni, bukan pgtype.Numeric — tidak ada modul lain di repo ini
// yang perlu menangani pgtype.Numeric, dan float64 cukup presisi untuk skala
// nilai 0..100 dua desimal.
type gradingRepository interface {
	CreateComponent(ctx context.Context, in CreateComponentInput) (ComponentRecord, error)
	UpdateComponent(ctx context.Context, schoolID, id int64, name, typ *string, weight, kktp *int) (ComponentRecord, error)
	GetComponentByID(ctx context.Context, schoolID, id int64) (ComponentRecord, error)
	ListComponentsByClassSubject(ctx context.Context, schoolID, classID, subjectID int64) ([]ComponentWithCounts, error)
	ListComponentsRaw(ctx context.Context, schoolID, classID, subjectID int64) ([]ComponentRecord, error)
	DeleteComponent(ctx context.Context, schoolID, id int64) (int64, error)

	UpsertGrade(ctx context.Context, schoolID, componentID, studentID int64, score float64, gradedBy int64) error
	DeleteGrade(ctx context.Context, componentID, studentID int64) error
	ListGradesForComponent(ctx context.Context, componentID int64) (map[int64]float64, error)
	ListGradesForComponents(ctx context.Context, componentIDs []int64) (map[int64]map[int64]float64, error) // componentID -> studentID -> score

	UpsertPublication(ctx context.Context, schoolID, academicYearID, classID, subjectID, publishedBy int64) error
	DeletePublication(ctx context.Context, schoolID, academicYearID, classID, subjectID int64) error
	IsPublished(ctx context.Context, schoolID, academicYearID, classID, subjectID int64) (bool, error)
	ListPublishedSubjectsForClass(ctx context.Context, schoolID, academicYearID, classID int64) ([]int64, error)

	CreateStar(ctx context.Context, in CreateStarInput) (StarRecord, error)
	GetStarByID(ctx context.Context, schoolID, id int64) (StarRecord, error)
	DeleteStar(ctx context.Context, schoolID, id int64) (int64, error)
	ListStarsForStudent(ctx context.Context, schoolID, studentID int64, onlyVisible bool) ([]StarRecord, error)
	ListStarTotalsForClass(ctx context.Context, schoolID, classID int64) ([]StarTotalRecord, error)

	GetClassByID(ctx context.Context, schoolID, id int64) (ClassRef, bool, error)
	GetSubjectByID(ctx context.Context, schoolID, id int64) (SubjectRef, bool, error)
	ListClassRoster(ctx context.Context, schoolID, classID int64) ([]RosterStudent, error)
	GetStudentBasic(ctx context.Context, schoolID, studentID int64) (name, nis string, ok bool, err error)
	StudentClassID(ctx context.Context, schoolID, academicYearID, studentID int64) (classID int64, ok bool, err error)
	ListSubjectsWithComponentsForClass(ctx context.Context, schoolID, classID int64) ([]SubjectRef, error)

	// -- report (Fase 15 GAP 1) --
	ReplaceTPMappings(ctx context.Context, schoolID, academicYearID, classID, subjectID int64, mappings []TPMappingInput) error
	ListTPMappingsForClassSubject(ctx context.Context, schoolID, classID, subjectID int64) ([]TPMappingRecord, error)
	ListTPMappingsForClass(ctx context.Context, schoolID, classID int64) ([]TPMappingWithSubject, error)
	UpsertManualScore(ctx context.Context, schoolID, academicYearID, classID, subjectID, studentID int64, kind string, score float64, note string, setBy int64) error
	DeleteManualScore(ctx context.Context, schoolID, academicYearID, classID, subjectID, studentID int64, kind string) error
	ListManualScoresForClassSubject(ctx context.Context, schoolID, academicYearID, classID, subjectID int64) ([]ManualScoreRecord, error)
}

var _ gradingRepository = (*Repository)(nil)

// Repository membungkus akses data modul grading (sqlc hand-written + pgx).
type Repository struct {
	pool *pgxpool.Pool
	q    *gradingdb.Queries
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool, q: gradingdb.New(pool)}
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

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	return false
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

func textOrNilFromPtr(v *string) pgtype.Text {
	if v == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: *v, Valid: true}
}

func int4OrNilFromIntPtr(v *int) pgtype.Int4 {
	if v == nil {
		return pgtype.Int4{}
	}
	return pgtype.Int4{Int32: int32(*v), Valid: true}
}

// -- domain records --

// ComponentRecord adalah representasi domain satu baris assessment_components.
type ComponentRecord struct {
	ID             int64
	SchoolID       int64
	AcademicYearID int64
	ClassID        int64
	SubjectID      int64
	Name           string
	Type           string
	Weight         int
	Kktp           int
	CreatedAt      time.Time
}

func componentFromDB(c gradingdb.AssessmentComponent) ComponentRecord {
	return ComponentRecord{
		ID: c.ID, SchoolID: c.SchoolID, AcademicYearID: c.AcademicYearID, ClassID: c.ClassID, SubjectID: c.SubjectID,
		Name: c.Name, Type: c.Type, Weight: int(c.Weight), Kktp: int(c.Kktp), CreatedAt: c.CreatedAt.Time,
	}
}

// ComponentWithCounts adalah ComponentRecord + jumlah siswa sudah dinilai &
// jumlah siswa di rombel — dipakai GET /api/grading/components.
type ComponentWithCounts struct {
	ComponentRecord
	GradedCount  int64
	StudentCount int64
}

// CreateComponentInput adalah parameter Repository.CreateComponent.
type CreateComponentInput struct {
	SchoolID       int64
	AcademicYearID int64
	ClassID        int64
	SubjectID      int64
	Name           string
	Type           string
	Weight         int
	Kktp           int
	CreatedBy      int64
}

// StarRecord adalah representasi domain satu baris classroom_star_events
// (+ nama pemberi, join users — dipakai langsung sebagai StarItem).
type StarRecord struct {
	ID            int64
	StudentID     int64
	GivenByUserID int64
	Delta         int
	Note          string
	Visibility    string
	GivenByName   string
	CreatedAt     time.Time
}

// CreateStarInput adalah parameter Repository.CreateStar.
type CreateStarInput struct {
	SchoolID       int64
	AcademicYearID int64
	StudentID      int64
	GivenBy        int64
	Delta          int
	Note           string
	Visibility     string
}

// StarTotalRecord adalah satu baris hasil Repository.ListStarTotalsForClass.
type StarTotalRecord struct {
	StudentID int64
	Name      string
	NIS       string
	Total     int
}

// ClassRef adalah representasi domain ringkas satu baris classes — dipakai
// INTERNAL repository saja untuk validasi lintas modul (mis. cek academic_year_id
// saat membuat komponen). SubjectRef memakai tipe response model.go langsung
// (bentuknya identik: ID/Code/Name) supaya tidak duplikasi definisi.
type ClassRef struct {
	ID             int64
	Name           string
	AcademicYearID int64
}

// RosterStudent adalah satu baris roster kelas (join enrollments+students).
type RosterStudent struct {
	ID     int64
	Name   string
	NIS    string
	UserID int64
}

// -- assessment_components --

func (r *Repository) CreateComponent(ctx context.Context, in CreateComponentInput) (ComponentRecord, error) {
	row, err := r.q.CreateComponent(ctx, gradingdb.CreateComponentParams{
		SchoolID: in.SchoolID, AcademicYearID: in.AcademicYearID, ClassID: in.ClassID, SubjectID: in.SubjectID,
		Name: in.Name, Type: in.Type, Weight: int32(in.Weight), Kktp: int32(in.Kktp), CreatedBy: int8OrNil(in.CreatedBy),
	})
	if err != nil {
		return ComponentRecord{}, err
	}
	return componentFromDB(row), nil
}

func (r *Repository) UpdateComponent(ctx context.Context, schoolID, id int64, name, typ *string, weight, kktp *int) (ComponentRecord, error) {
	row, err := r.q.UpdateComponent(ctx, gradingdb.UpdateComponentParams{
		Name: textOrNilFromPtr(name), Type: textOrNilFromPtr(typ), Weight: int4OrNilFromIntPtr(weight), Kktp: int4OrNilFromIntPtr(kktp),
		ID: id, SchoolID: schoolID,
	})
	if err != nil {
		return ComponentRecord{}, mapNoRows(err)
	}
	return componentFromDB(row), nil
}

func (r *Repository) GetComponentByID(ctx context.Context, schoolID, id int64) (ComponentRecord, error) {
	row, err := r.q.GetComponentByID(ctx, gradingdb.GetComponentByIDParams{ID: id, SchoolID: schoolID})
	if err != nil {
		return ComponentRecord{}, mapNoRows(err)
	}
	return componentFromDB(row), nil
}

func (r *Repository) ListComponentsByClassSubject(ctx context.Context, schoolID, classID, subjectID int64) ([]ComponentWithCounts, error) {
	rows, err := r.q.ListComponentsByClassSubject(ctx, gradingdb.ListComponentsByClassSubjectParams{SchoolID: schoolID, ClassID: classID, SubjectID: subjectID})
	if err != nil {
		return nil, err
	}
	out := make([]ComponentWithCounts, 0, len(rows))
	for _, row := range rows {
		out = append(out, ComponentWithCounts{
			ComponentRecord: ComponentRecord{
				ID: row.ID, SchoolID: row.SchoolID, AcademicYearID: row.AcademicYearID, ClassID: row.ClassID, SubjectID: row.SubjectID,
				Name: row.Name, Type: row.Type, Weight: int(row.Weight), Kktp: int(row.Kktp), CreatedAt: row.CreatedAt.Time,
			},
			GradedCount: row.GradedCount, StudentCount: row.StudentCount,
		})
	}
	return out, nil
}

func (r *Repository) ListComponentsRaw(ctx context.Context, schoolID, classID, subjectID int64) ([]ComponentRecord, error) {
	rows, err := r.q.ListComponentsRawByClassSubject(ctx, gradingdb.ListComponentsRawByClassSubjectParams{SchoolID: schoolID, ClassID: classID, SubjectID: subjectID})
	if err != nil {
		return nil, err
	}
	out := make([]ComponentRecord, 0, len(rows))
	for _, row := range rows {
		out = append(out, componentFromDB(row))
	}
	return out, nil
}

func (r *Repository) DeleteComponent(ctx context.Context, schoolID, id int64) (int64, error) {
	return r.q.DeleteComponent(ctx, gradingdb.DeleteComponentParams{ID: id, SchoolID: schoolID})
}

// -- student_grades --

func (r *Repository) UpsertGrade(ctx context.Context, schoolID, componentID, studentID int64, score float64, gradedBy int64) error {
	_, err := r.q.UpsertGrade(ctx, gradingdb.UpsertGradeParams{
		SchoolID: schoolID, ComponentID: componentID, StudentID: studentID, Score: score, GradedBy: int8OrNil(gradedBy),
	})
	return err
}

func (r *Repository) DeleteGrade(ctx context.Context, componentID, studentID int64) error {
	return r.q.DeleteGrade(ctx, gradingdb.DeleteGradeParams{ComponentID: componentID, StudentID: studentID})
}

func (r *Repository) ListGradesForComponent(ctx context.Context, componentID int64) (map[int64]float64, error) {
	rows, err := r.q.ListGradesForComponent(ctx, componentID)
	if err != nil {
		return nil, err
	}
	out := make(map[int64]float64, len(rows))
	for _, row := range rows {
		out[row.StudentID] = row.Score
	}
	return out, nil
}

func (r *Repository) ListGradesForComponents(ctx context.Context, componentIDs []int64) (map[int64]map[int64]float64, error) {
	if len(componentIDs) == 0 {
		return map[int64]map[int64]float64{}, nil
	}
	rows, err := r.q.ListGradesForComponents(ctx, componentIDs)
	if err != nil {
		return nil, err
	}
	out := make(map[int64]map[int64]float64, len(componentIDs))
	for _, row := range rows {
		if out[row.ComponentID] == nil {
			out[row.ComponentID] = map[int64]float64{}
		}
		out[row.ComponentID][row.StudentID] = row.Score
	}
	return out, nil
}

// -- grade_publications --

func (r *Repository) UpsertPublication(ctx context.Context, schoolID, academicYearID, classID, subjectID, publishedBy int64) error {
	return r.q.UpsertPublication(ctx, gradingdb.UpsertPublicationParams{
		SchoolID: schoolID, AcademicYearID: academicYearID, ClassID: classID, SubjectID: subjectID, PublishedBy: int8OrNil(publishedBy),
	})
}

func (r *Repository) DeletePublication(ctx context.Context, schoolID, academicYearID, classID, subjectID int64) error {
	return r.q.DeletePublication(ctx, gradingdb.DeletePublicationParams{
		SchoolID: schoolID, AcademicYearID: academicYearID, ClassID: classID, SubjectID: subjectID,
	})
}

func (r *Repository) IsPublished(ctx context.Context, schoolID, academicYearID, classID, subjectID int64) (bool, error) {
	_, err := r.q.GetPublication(ctx, gradingdb.GetPublicationParams{
		SchoolID: schoolID, AcademicYearID: academicYearID, ClassID: classID, SubjectID: subjectID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (r *Repository) ListPublishedSubjectsForClass(ctx context.Context, schoolID, academicYearID, classID int64) ([]int64, error) {
	rows, err := r.q.ListPublishedSubjectsForClass(ctx, gradingdb.ListPublishedSubjectsForClassParams{
		SchoolID: schoolID, AcademicYearID: academicYearID, ClassID: classID,
	})
	if err != nil {
		return nil, err
	}
	return rows, nil
}

// -- classroom_star_events --

func (r *Repository) CreateStar(ctx context.Context, in CreateStarInput) (StarRecord, error) {
	row, err := r.q.CreateStar(ctx, gradingdb.CreateStarParams{
		SchoolID: in.SchoolID, AcademicYearID: in.AcademicYearID, StudentID: in.StudentID, GivenBy: in.GivenBy,
		Delta: int32(in.Delta), Note: in.Note, Visibility: in.Visibility,
	})
	if err != nil {
		if isUniqueViolation(err) {
			return StarRecord{}, ErrConflict
		}
		return StarRecord{}, err
	}
	return StarRecord{ID: row.ID, StudentID: row.StudentID, GivenByUserID: row.GivenBy, Delta: int(row.Delta), Note: row.Note, Visibility: row.Visibility, CreatedAt: row.CreatedAt.Time}, nil
}

func (r *Repository) GetStarByID(ctx context.Context, schoolID, id int64) (StarRecord, error) {
	row, err := r.q.GetStarByID(ctx, gradingdb.GetStarByIDParams{ID: id, SchoolID: schoolID})
	if err != nil {
		return StarRecord{}, mapNoRows(err)
	}
	return StarRecord{ID: row.ID, StudentID: row.StudentID, GivenByUserID: row.GivenBy, Delta: int(row.Delta), Note: row.Note, Visibility: row.Visibility, CreatedAt: row.CreatedAt.Time}, nil
}

func (r *Repository) DeleteStar(ctx context.Context, schoolID, id int64) (int64, error) {
	return r.q.DeleteStar(ctx, gradingdb.DeleteStarParams{ID: id, SchoolID: schoolID})
}

func (r *Repository) ListStarsForStudent(ctx context.Context, schoolID, studentID int64, onlyVisible bool) ([]StarRecord, error) {
	if onlyVisible {
		rows, err := r.q.ListStarsForStudentVisible(ctx, gradingdb.ListStarsForStudentVisibleParams{SchoolID: schoolID, StudentID: studentID})
		if err != nil {
			return nil, err
		}
		out := make([]StarRecord, 0, len(rows))
		for _, row := range rows {
			out = append(out, StarRecord{ID: row.ID, StudentID: studentID, Delta: int(row.Delta), Note: row.Note, Visibility: row.Visibility, GivenByName: row.GivenByName, CreatedAt: row.CreatedAt.Time})
		}
		return out, nil
	}
	rows, err := r.q.ListStarsForStudent(ctx, gradingdb.ListStarsForStudentParams{SchoolID: schoolID, StudentID: studentID})
	if err != nil {
		return nil, err
	}
	out := make([]StarRecord, 0, len(rows))
	for _, row := range rows {
		out = append(out, StarRecord{ID: row.ID, StudentID: studentID, Delta: int(row.Delta), Note: row.Note, Visibility: row.Visibility, GivenByName: row.GivenByName, CreatedAt: row.CreatedAt.Time})
	}
	return out, nil
}

func (r *Repository) ListStarTotalsForClass(ctx context.Context, schoolID, classID int64) ([]StarTotalRecord, error) {
	rows, err := r.q.ListStarTotalsForClass(ctx, gradingdb.ListStarTotalsForClassParams{SchoolID: schoolID, ClassID: classID})
	if err != nil {
		return nil, err
	}
	out := make([]StarTotalRecord, 0, len(rows))
	for _, row := range rows {
		out = append(out, StarTotalRecord{StudentID: row.StudentID, Name: row.StudentName, NIS: row.StudentNis, Total: int(row.Total)})
	}
	return out, nil
}

// -- lookup & validasi lintas modul --

func (r *Repository) GetClassByID(ctx context.Context, schoolID, id int64) (ClassRef, bool, error) {
	row, err := r.q.GetClassByID(ctx, gradingdb.GetClassByIDParams{ID: id, SchoolID: schoolID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ClassRef{}, false, nil
		}
		return ClassRef{}, false, err
	}
	return ClassRef{ID: row.ID, Name: row.Name, AcademicYearID: row.AcademicYearID}, true, nil
}

func (r *Repository) GetSubjectByID(ctx context.Context, schoolID, id int64) (SubjectRef, bool, error) {
	row, err := r.q.GetSubjectByID(ctx, gradingdb.GetSubjectByIDParams{ID: id, SchoolID: schoolID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return SubjectRef{}, false, nil
		}
		return SubjectRef{}, false, err
	}
	return SubjectRef{ID: row.ID, Code: row.Code, Name: row.Name}, true, nil
}

func (r *Repository) ListClassRoster(ctx context.Context, schoolID, classID int64) ([]RosterStudent, error) {
	rows, err := r.q.ListClassRoster(ctx, gradingdb.ListClassRosterParams{ClassID: classID, SchoolID: schoolID})
	if err != nil {
		return nil, err
	}
	out := make([]RosterStudent, 0, len(rows))
	for _, row := range rows {
		out = append(out, RosterStudent{ID: row.ID, Name: row.Name, NIS: row.Nis, UserID: row.UserID})
	}
	return out, nil
}

func (r *Repository) GetStudentBasic(ctx context.Context, schoolID, studentID int64) (string, string, bool, error) {
	row, err := r.q.GetStudentBasic(ctx, gradingdb.GetStudentBasicParams{ID: studentID, SchoolID: schoolID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", "", false, nil
		}
		return "", "", false, err
	}
	return row.Name, row.Nis, true, nil
}

func (r *Repository) StudentClassID(ctx context.Context, schoolID, academicYearID, studentID int64) (int64, bool, error) {
	id, err := r.q.StudentClassID(ctx, gradingdb.StudentClassIDParams{StudentID: studentID, SchoolID: schoolID, AcademicYearID: academicYearID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, false, nil
		}
		return 0, false, err
	}
	return id, true, nil
}

var _ = int8ToInt64 // dipakai bila kolom nullable lain ditambah nanti (dijaga dari unused func selama refactor)
