package counseling

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/omanjaya/nouschool/internal/counseling/counselingdb"
)

// ErrNotFound menandai baris tidak ditemukan di repository modul counseling.
var ErrNotFound = errors.New("counseling: data tidak ditemukan")

// counselingRepository adalah kontrak yang dibutuhkan Service dari repository
// — dideklarasikan sebagai interface (dipenuhi *Repository secara struktural)
// supaya Service bisa dites dengan fake repository in-memory, tanpa DB (lihat
// service_test.go), sama seperti pola modul lain.
type counselingRepository interface {
	CreateCounseling(ctx context.Context, in CreateRecordInput) (Record, error)
	GetRow(ctx context.Context, schoolID, id int64) (Row, error)
	GetByID(ctx context.Context, schoolID, id int64) (Record, error)
	ListRows(ctx context.Context, schoolID, studentID int64, limit, offset int) ([]Row, error)
	CountRows(ctx context.Context, schoolID, studentID int64) (int64, error)
	UpdateCounseling(ctx context.Context, schoolID, id int64, in UpdateRecordInput) (Record, error)
	UpdateEvidence(ctx context.Context, schoolID, id int64, path, name, mime string) error
	DeleteCounseling(ctx context.Context, schoolID, id int64) (int64, error)
	GetStudentBasic(ctx context.Context, schoolID, studentID int64) (StudentBasic, error)
}

var _ counselingRepository = (*Repository)(nil)

// Repository membungkus akses data modul counseling (sqlc + pgx).
type Repository struct {
	q *counselingdb.Queries
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{q: counselingdb.New(pool)}
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

// -- domain records --

// Record adalah baris counselings mentah (tanpa join).
type Record struct {
	ID                 int64
	SchoolID           int64
	AcademicYearID     int64
	StudentID          int64
	CounselorID        int64
	CareerGoals        string
	ProblemDescription string
	FollowUpPlan       string
	Evidence           string
	EvidenceName       string
	EvidenceMime       string
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

func recordFromDB(c counselingdb.Counseling) Record {
	return Record{
		ID: c.ID, SchoolID: c.SchoolID, AcademicYearID: c.AcademicYearID, StudentID: c.StudentID,
		CounselorID: c.CounselorID, CareerGoals: c.CareerGoals, ProblemDescription: c.ProblemDescription,
		FollowUpPlan: c.FollowUpPlan, Evidence: c.Evidence.String, EvidenceName: c.EvidenceName.String,
		EvidenceMime: c.EvidenceMime.String, CreatedAt: c.CreatedAt.Time, UpdatedAt: c.UpdatedAt.Time,
	}
}

// Row adalah Record + nama siswa/kelas/konselor (hasil join, dipakai list & detail).
type Row struct {
	Record
	StudentName   string
	StudentNIS    string
	ClassName     string
	CounselorName string
}

// StudentBasic adalah potongan info siswa dipakai validasi student_id.
type StudentBasic struct {
	ID   int64
	Name string
	NIS  string
}

// CreateRecordInput adalah parameter Repository.CreateCounseling.
type CreateRecordInput struct {
	SchoolID           int64
	AcademicYearID     int64
	StudentID          int64
	CounselorID        int64
	CareerGoals        string
	ProblemDescription string
	FollowUpPlan       string
	Evidence           string
	EvidenceName       string
	EvidenceMime       string
}

func (r *Repository) CreateCounseling(ctx context.Context, in CreateRecordInput) (Record, error) {
	rec, err := r.q.CreateCounseling(ctx, counselingdb.CreateCounselingParams{
		SchoolID: in.SchoolID, AcademicYearID: in.AcademicYearID, StudentID: in.StudentID, CounselorID: in.CounselorID,
		CareerGoals: in.CareerGoals, ProblemDescription: in.ProblemDescription, FollowUpPlan: in.FollowUpPlan,
		Evidence: textOrNil(in.Evidence), EvidenceName: textOrNil(in.EvidenceName), EvidenceMime: textOrNil(in.EvidenceMime),
	})
	if err != nil {
		return Record{}, err
	}
	return recordFromDB(rec), nil
}

func rowFromDBGet(row counselingdb.GetCounselingRowRow) Row {
	return Row{
		Record: Record{
			ID: row.ID, SchoolID: row.SchoolID, AcademicYearID: row.AcademicYearID, StudentID: row.StudentID,
			CounselorID: row.CounselorID, CareerGoals: row.CareerGoals, ProblemDescription: row.ProblemDescription,
			FollowUpPlan: row.FollowUpPlan, Evidence: row.Evidence.String, EvidenceName: row.EvidenceName.String,
			EvidenceMime: row.EvidenceMime.String, CreatedAt: row.CreatedAt.Time, UpdatedAt: row.UpdatedAt.Time,
		},
		StudentName: row.StudentName, StudentNIS: row.StudentNis, ClassName: row.ClassName, CounselorName: row.CounselorName,
	}
}

func (r *Repository) GetRow(ctx context.Context, schoolID, id int64) (Row, error) {
	row, err := r.q.GetCounselingRow(ctx, counselingdb.GetCounselingRowParams{ID: id, SchoolID: schoolID})
	if err != nil {
		return Row{}, mapNoRows(err)
	}
	return rowFromDBGet(row), nil
}

func (r *Repository) GetByID(ctx context.Context, schoolID, id int64) (Record, error) {
	rec, err := r.q.GetCounselingByID(ctx, counselingdb.GetCounselingByIDParams{ID: id, SchoolID: schoolID})
	if err != nil {
		return Record{}, mapNoRows(err)
	}
	return recordFromDB(rec), nil
}

func (r *Repository) ListRows(ctx context.Context, schoolID, studentID int64, limit, offset int) ([]Row, error) {
	rows, err := r.q.ListCounselings(ctx, counselingdb.ListCounselingsParams{
		SchoolID: schoolID, StudentID: int8OrNil(studentID), LimitRows: int32(limit), OffsetRows: int32(offset),
	})
	if err != nil {
		return nil, err
	}
	out := make([]Row, 0, len(rows))
	for _, row := range rows {
		out = append(out, Row{
			Record: Record{
				ID: row.ID, SchoolID: row.SchoolID, AcademicYearID: row.AcademicYearID, StudentID: row.StudentID,
				CounselorID: row.CounselorID, CareerGoals: row.CareerGoals, ProblemDescription: row.ProblemDescription,
				FollowUpPlan: row.FollowUpPlan, Evidence: row.Evidence.String, EvidenceName: row.EvidenceName.String,
				EvidenceMime: row.EvidenceMime.String, CreatedAt: row.CreatedAt.Time, UpdatedAt: row.UpdatedAt.Time,
			},
			StudentName: row.StudentName, StudentNIS: row.StudentNis, ClassName: row.ClassName, CounselorName: row.CounselorName,
		})
	}
	return out, nil
}

func (r *Repository) CountRows(ctx context.Context, schoolID, studentID int64) (int64, error) {
	return r.q.CountCounselings(ctx, counselingdb.CountCounselingsParams{SchoolID: schoolID, StudentID: int8OrNil(studentID)})
}

// UpdateRecordInput adalah parameter Repository.UpdateCounseling.
type UpdateRecordInput struct {
	CareerGoals        string
	ProblemDescription string
	FollowUpPlan       string
}

func (r *Repository) UpdateCounseling(ctx context.Context, schoolID, id int64, in UpdateRecordInput) (Record, error) {
	rec, err := r.q.UpdateCounseling(ctx, counselingdb.UpdateCounselingParams{
		ID: id, SchoolID: schoolID, CareerGoals: in.CareerGoals,
		ProblemDescription: in.ProblemDescription, FollowUpPlan: in.FollowUpPlan,
	})
	if err != nil {
		return Record{}, mapNoRows(err)
	}
	return recordFromDB(rec), nil
}

func (r *Repository) UpdateEvidence(ctx context.Context, schoolID, id int64, path, name, mime string) error {
	return r.q.UpdateCounselingEvidence(ctx, counselingdb.UpdateCounselingEvidenceParams{
		ID: id, SchoolID: schoolID, Evidence: textOrNil(path), EvidenceName: textOrNil(name), EvidenceMime: textOrNil(mime),
	})
}

func (r *Repository) DeleteCounseling(ctx context.Context, schoolID, id int64) (int64, error) {
	return r.q.DeleteCounseling(ctx, counselingdb.DeleteCounselingParams{ID: id, SchoolID: schoolID})
}

func (r *Repository) GetStudentBasic(ctx context.Context, schoolID, studentID int64) (StudentBasic, error) {
	row, err := r.q.GetStudentBasic(ctx, counselingdb.GetStudentBasicParams{ID: studentID, SchoolID: schoolID})
	if err != nil {
		return StudentBasic{}, mapNoRows(err)
	}
	return StudentBasic{ID: row.ID, Name: row.Name, NIS: row.Nis}, nil
}
