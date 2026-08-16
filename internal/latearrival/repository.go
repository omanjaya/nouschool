package latearrival

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/omanjaya/nouschool/internal/latearrival/latearrivaldb"
)

var ErrNotFound = errors.New("latearrival: data tidak ditemukan")

// lateArrivalRepository adalah kontrak yang dibutuhkan Service dari
// repository — dipenuhi *Repository secara struktural (pola sama modul
// lain), supaya Service bisa dites dengan fake repository in-memory.
type lateArrivalRepository interface {
	CreateRecord(ctx context.Context, schoolID, academicYearID, studentID int64, arrivedAt time.Time, lateCount int, action string, dutyBy int64) (int64, error)
	CountForStudentYear(ctx context.Context, schoolID, academicYearID, studentID int64) (int64, error)
	GetDetail(ctx context.Context, schoolID, id int64) (Detail, error)
	GetTodayForStudent(ctx context.Context, schoolID, studentID int64, from, to time.Time) (Detail, bool, error)
	ListForStudents(ctx context.Context, schoolID int64, studentIDs []int64, from, to *time.Time) ([]Detail, error)
	ListToday(ctx context.Context, schoolID int64, from, to time.Time) ([]Detail, error)
	ListAll(ctx context.Context, schoolID int64, from, to *time.Time) ([]Detail, error)
	SummaryByMonth(ctx context.Context, schoolID int64, from, to time.Time) ([]SummaryDetail, error)

	UpdateLeadershipStage(ctx context.Context, schoolID, id, leadershipBy int64) (int64, error)
	UpdateClassStage(ctx context.Context, schoolID, id, classBy int64) (int64, error)
}

var _ lateArrivalRepository = (*Repository)(nil)

// Repository membungkus akses data modul latearrival (sqlc + pgx).
type Repository struct {
	q *latearrivaldb.Queries
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{q: latearrivaldb.New(pool)}
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

// -- domain records --

// Detail adalah representasi domain LENGKAP satu baris student_late_arrivals
// (join siswa+kelas+nama SEMUA approver tahap) — sumber tunggal shape
// RecordView.
type Detail struct {
	ID               int64
	SchoolID         int64
	AcademicYearID   int64
	StudentID        int64
	ArrivedAt        time.Time
	LateCount        int
	Action           string
	Status           string
	DutyBy           int64
	DutyAt           time.Time
	DutyByName       string
	LeadershipBy     int64
	LeadershipAt     time.Time
	LeadershipByName string
	ClassBy          int64
	ClassAt          time.Time
	ClassByName      string
	CreatedAt        time.Time
	StudentName      string
	StudentNIS       string
	ClassName        string
}

func (d Detail) stage(by int64, at time.Time, name string) *StageInfo {
	if by == 0 {
		return nil
	}
	t := at
	return &StageInfo{ByName: name, At: &t}
}

func (d Detail) View() RecordView {
	return RecordView{
		ID:         d.ID,
		Student:    StudentRef{ID: d.StudentID, Name: d.StudentName, NIS: d.StudentNIS, ClassName: d.ClassName},
		ArrivedAt:  d.ArrivedAt,
		LateCount:  d.LateCount,
		Action:     d.Action,
		Status:     d.Status,
		Duty:       d.stage(d.DutyBy, d.DutyAt, d.DutyByName),
		Leadership: d.stage(d.LeadershipBy, d.LeadershipAt, d.LeadershipByName),
		Class:      d.stage(d.ClassBy, d.ClassAt, d.ClassByName),
		CreatedAt:  d.CreatedAt,
	}
}

// SummaryDetail adalah satu baris hasil SummaryByMonth.
type SummaryDetail struct {
	StudentID   int64
	StudentName string
	StudentNIS  string
	ClassName   string
	Count       int64
}

func detailFromRow(row latearrivaldb.GetLateArrivalDetailRow) Detail {
	return Detail{
		ID: row.ID, SchoolID: row.SchoolID, AcademicYearID: row.AcademicYearID, StudentID: row.StudentID,
		ArrivedAt: tsToTime(row.ArrivedAt), LateCount: int(row.LateCount), Action: row.Action, Status: row.Status,
		DutyBy: int8ToInt64(row.DutyBy), DutyAt: tsToTime(row.DutyAt), DutyByName: row.DutyByName.String,
		LeadershipBy: int8ToInt64(row.LeadershipBy), LeadershipAt: tsToTime(row.LeadershipAt), LeadershipByName: row.LeadershipByName.String,
		ClassBy: int8ToInt64(row.ClassBy), ClassAt: tsToTime(row.ClassAt), ClassByName: row.ClassByName.String,
		CreatedAt:   tsToTime(row.CreatedAt),
		StudentName: row.StudentName, StudentNIS: row.StudentNis, ClassName: row.ClassName,
	}
}

func (r *Repository) CreateRecord(ctx context.Context, schoolID, academicYearID, studentID int64, arrivedAt time.Time, lateCount int, action string, dutyBy int64) (int64, error) {
	return r.q.CreateLateArrival(ctx, latearrivaldb.CreateLateArrivalParams{
		SchoolID: schoolID, AcademicYearID: academicYearID, StudentID: studentID,
		ArrivedAt: tsOrNil(arrivedAt), LateCount: int32(lateCount), Action: action, DutyBy: int8OrNil(dutyBy),
	})
}

func (r *Repository) CountForStudentYear(ctx context.Context, schoolID, academicYearID, studentID int64) (int64, error) {
	return r.q.CountLateArrivalsForStudentYear(ctx, latearrivaldb.CountLateArrivalsForStudentYearParams{
		SchoolID: schoolID, AcademicYearID: academicYearID, StudentID: studentID,
	})
}

func (r *Repository) GetDetail(ctx context.Context, schoolID, id int64) (Detail, error) {
	row, err := r.q.GetLateArrivalDetail(ctx, latearrivaldb.GetLateArrivalDetailParams{ID: id, SchoolID: schoolID})
	if err != nil {
		return Detail{}, mapNoRows(err)
	}
	return detailFromRow(row), nil
}

func (r *Repository) GetTodayForStudent(ctx context.Context, schoolID, studentID int64, from, to time.Time) (Detail, bool, error) {
	row, err := r.q.GetTodayLateArrivalForStudent(ctx, latearrivaldb.GetTodayLateArrivalForStudentParams{
		SchoolID: schoolID, StudentID: studentID, FromAt: tsOrNil(from), ToAt: tsOrNil(to),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Detail{}, false, nil
		}
		return Detail{}, false, err
	}
	return detailFromRow(latearrivaldb.GetLateArrivalDetailRow(row)), true, nil
}

func (r *Repository) ListForStudents(ctx context.Context, schoolID int64, studentIDs []int64, from, to *time.Time) ([]Detail, error) {
	var fromT, toT time.Time
	if from != nil {
		fromT = *from
	}
	if to != nil {
		toT = *to
	}
	rows, err := r.q.ListLateArrivalsForStudents(ctx, latearrivaldb.ListLateArrivalsForStudentsParams{
		SchoolID: schoolID, StudentIds: studentIDs, FromAt: tsOrNil(fromT), ToAt: tsOrNil(toT),
	})
	if err != nil {
		return nil, err
	}
	out := make([]Detail, 0, len(rows))
	for _, row := range rows {
		out = append(out, detailFromRow(latearrivaldb.GetLateArrivalDetailRow(row)))
	}
	return out, nil
}

func (r *Repository) ListToday(ctx context.Context, schoolID int64, from, to time.Time) ([]Detail, error) {
	rows, err := r.q.ListLateArrivalsToday(ctx, latearrivaldb.ListLateArrivalsTodayParams{SchoolID: schoolID, FromAt: tsOrNil(from), ToAt: tsOrNil(to)})
	if err != nil {
		return nil, err
	}
	out := make([]Detail, 0, len(rows))
	for _, row := range rows {
		out = append(out, detailFromRow(latearrivaldb.GetLateArrivalDetailRow(row)))
	}
	return out, nil
}

func (r *Repository) ListAll(ctx context.Context, schoolID int64, from, to *time.Time) ([]Detail, error) {
	var fromT, toT time.Time
	if from != nil {
		fromT = *from
	}
	if to != nil {
		toT = *to
	}
	rows, err := r.q.ListLateArrivalsAll(ctx, latearrivaldb.ListLateArrivalsAllParams{SchoolID: schoolID, FromAt: tsOrNil(fromT), ToAt: tsOrNil(toT)})
	if err != nil {
		return nil, err
	}
	out := make([]Detail, 0, len(rows))
	for _, row := range rows {
		out = append(out, detailFromRow(latearrivaldb.GetLateArrivalDetailRow(row)))
	}
	return out, nil
}

func (r *Repository) SummaryByMonth(ctx context.Context, schoolID int64, from, to time.Time) ([]SummaryDetail, error) {
	rows, err := r.q.SummaryLateArrivalsByMonth(ctx, latearrivaldb.SummaryLateArrivalsByMonthParams{SchoolID: schoolID, FromAt: tsOrNil(from), ToAt: tsOrNil(to)})
	if err != nil {
		return nil, err
	}
	out := make([]SummaryDetail, 0, len(rows))
	for _, row := range rows {
		out = append(out, SummaryDetail{StudentID: row.StudentID, StudentName: row.StudentName, StudentNIS: row.StudentNis, ClassName: row.ClassName, Count: row.Cnt})
	}
	return out, nil
}

func (r *Repository) UpdateLeadershipStage(ctx context.Context, schoolID, id, leadershipBy int64) (int64, error) {
	return r.q.UpdateLateArrivalLeadershipStage(ctx, latearrivaldb.UpdateLateArrivalLeadershipStageParams{ID: id, SchoolID: schoolID, LeadershipBy: int8OrNil(leadershipBy)})
}

func (r *Repository) UpdateClassStage(ctx context.Context, schoolID, id, classBy int64) (int64, error) {
	return r.q.UpdateLateArrivalClassStage(ctx, latearrivaldb.UpdateLateArrivalClassStageParams{ID: id, SchoolID: schoolID, ClassBy: int8OrNil(classBy)})
}
