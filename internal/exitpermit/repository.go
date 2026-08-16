package exitpermit

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/omanjaya/nouschool/internal/exitpermit/exitpermitdb"
)

var ErrNotFound = errors.New("exitpermit: data tidak ditemukan")

// exitPermitRepository adalah kontrak yang dibutuhkan Service dari
// repository — dipenuhi *Repository secara struktural, supaya Service bisa
// dites dengan fake repository in-memory, tanpa DB (pola sama modul lain).
type exitPermitRepository interface {
	CreateRequest(ctx context.Context, schoolID, academicYearID, studentID int64, reason string) (int64, error)
	CountActive(ctx context.Context, schoolID, studentID int64) (int64, error)
	GetDetail(ctx context.Context, schoolID, id int64) (Detail, error)
	GetByGateToken(ctx context.Context, schoolID int64, token string) (Detail, bool, error)
	ListForStudents(ctx context.Context, schoolID int64, studentIDs []int64) ([]Detail, error)
	ListActiveToday(ctx context.Context, schoolID int64, from, to time.Time) ([]Detail, error)
	ListAll(ctx context.Context, schoolID int64, from, to *time.Time) ([]Detail, error)
	ListExitedOnDate(ctx context.Context, schoolID int64, from, to time.Time) ([]Detail, error)

	UpdateDutyStage(ctx context.Context, schoolID, id, dutyBy int64) (int64, error)
	UpdateClassStage(ctx context.Context, schoolID, id, classBy int64) (int64, error)
	UpdateBKStage(ctx context.Context, schoolID, id, bkBy int64) (int64, error)
	UpdateLeadershipStage(ctx context.Context, schoolID, id, leadershipBy int64, gateToken string, gateExpiresAt time.Time) (int64, error)
	Reject(ctx context.Context, schoolID, id, rejectedBy int64, comment string) (int64, error)
	Cancel(ctx context.Context, schoolID, id int64) (int64, error)
	GateScan(ctx context.Context, schoolID int64, gateToken string, exitedBy int64) (int64, error)

	// -- Fase 15 GAP 3 (reject per tahap) --
	GetStudentClassID(ctx context.Context, schoolID, academicYearID, studentID int64) (classID int64, ok bool, err error)
	GetStudentUserID(ctx context.Context, schoolID, studentID int64) (userID int64, err error)
}

var _ exitPermitRepository = (*Repository)(nil)

// Repository membungkus akses data modul exitpermit (sqlc + pgx).
type Repository struct {
	q *exitpermitdb.Queries
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{q: exitpermitdb.New(pool)}
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

// Detail adalah representasi domain LENGKAP satu baris student_exit_permits
// (join siswa+kelas+SEMUA nama approver tahap) — sumber tunggal shape
// PermitView (pola sama internal/studentleave RequestDetail).
type Detail struct {
	ID               int64
	SchoolID         int64
	AcademicYearID   int64
	StudentID        int64
	Reason           string
	Status           string
	DutyBy           int64
	DutyAt           time.Time
	DutyByName       string
	ClassBy          int64
	ClassAt          time.Time
	ClassByName      string
	BKBy             int64
	BKAt             time.Time
	BKByName         string
	LeadershipBy     int64
	LeadershipAt     time.Time
	LeadershipByName string
	RejectedBy       int64
	RejectedAt       time.Time
	RejectedByName   string
	RejectComment    string
	GateToken        string
	GateExpiresAt    time.Time
	ExitedBy         int64
	ExitedAt         time.Time
	ExitedByName     string
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

// View converte Detail menjadi PermitView (shape response publik). stageTo
// (opsional, "" bila tidak dipakai) mengisi PermitView.StageAdvancedTo —
// dipakai HANYA respons POST .../scan.
func (d Detail) View(stageTo string) PermitView {
	v := PermitView{
		ID:              d.ID,
		Student:         StudentRef{ID: d.StudentID, Name: d.StudentName, NIS: d.StudentNIS, ClassName: d.ClassName},
		Reason:          d.Reason,
		Status:          d.Status,
		Duty:            d.stage(d.DutyBy, d.DutyAt, d.DutyByName),
		Class:           d.stage(d.ClassBy, d.ClassAt, d.ClassByName),
		BK:              d.stage(d.BKBy, d.BKAt, d.BKByName),
		Leadership:      d.stage(d.LeadershipBy, d.LeadershipAt, d.LeadershipByName),
		CreatedAt:       d.CreatedAt,
		StageAdvancedTo: stageTo,
	}
	if d.RejectedBy != 0 {
		at := d.RejectedAt
		v.Rejected = &RejectInfo{ByName: d.RejectedByName, At: &at, Comment: d.RejectComment}
	}
	if d.GateToken != "" {
		gt := d.GateToken
		v.GateToken = &gt
	}
	if !d.GateExpiresAt.IsZero() {
		ge := d.GateExpiresAt
		v.GateExpiresAt = &ge
	}
	if !d.ExitedAt.IsZero() {
		ea := d.ExitedAt
		v.ExitedAt = &ea
	}
	return v
}

func detailFromRow(row exitpermitdb.GetExitPermitDetailRow) Detail {
	return Detail{
		ID: row.ID, SchoolID: row.SchoolID, AcademicYearID: row.AcademicYearID, StudentID: row.StudentID,
		Reason: row.Reason, Status: row.Status,
		DutyBy: int8ToInt64(row.DutyBy), DutyAt: tsToTime(row.DutyAt), DutyByName: row.DutyByName.String,
		ClassBy: int8ToInt64(row.ClassBy), ClassAt: tsToTime(row.ClassAt), ClassByName: row.ClassByName.String,
		BKBy: int8ToInt64(row.BkBy), BKAt: tsToTime(row.BkAt), BKByName: row.BkByName.String,
		LeadershipBy: int8ToInt64(row.LeadershipBy), LeadershipAt: tsToTime(row.LeadershipAt), LeadershipByName: row.LeadershipByName.String,
		RejectedBy: int8ToInt64(row.RejectedBy), RejectedAt: tsToTime(row.RejectedAt), RejectedByName: row.RejectedByName.String,
		RejectComment: row.RejectComment,
		GateToken:     row.GateToken.String, GateExpiresAt: tsToTime(row.GateExpiresAt),
		ExitedBy: int8ToInt64(row.ExitedBy), ExitedAt: tsToTime(row.ExitedAt), ExitedByName: row.ExitedByName.String,
		CreatedAt:   tsToTime(row.CreatedAt),
		StudentName: row.StudentName, StudentNIS: row.StudentNis, ClassName: row.ClassName,
	}
}

func (r *Repository) CreateRequest(ctx context.Context, schoolID, academicYearID, studentID int64, reason string) (int64, error) {
	return r.q.CreateExitPermit(ctx, exitpermitdb.CreateExitPermitParams{
		SchoolID: schoolID, AcademicYearID: academicYearID, StudentID: studentID, Reason: reason,
	})
}

func (r *Repository) CountActive(ctx context.Context, schoolID, studentID int64) (int64, error) {
	return r.q.CountActiveExitPermits(ctx, exitpermitdb.CountActiveExitPermitsParams{SchoolID: schoolID, StudentID: studentID})
}

func (r *Repository) GetDetail(ctx context.Context, schoolID, id int64) (Detail, error) {
	row, err := r.q.GetExitPermitDetail(ctx, exitpermitdb.GetExitPermitDetailParams{ID: id, SchoolID: schoolID})
	if err != nil {
		return Detail{}, mapNoRows(err)
	}
	return detailFromRow(row), nil
}

func (r *Repository) GetByGateToken(ctx context.Context, schoolID int64, token string) (Detail, bool, error) {
	row, err := r.q.GetExitPermitByGateToken(ctx, exitpermitdb.GetExitPermitByGateTokenParams{GateToken: textOrNilFrom(token), SchoolID: schoolID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Detail{}, false, nil
		}
		return Detail{}, false, err
	}
	return detailFromRow(exitpermitdb.GetExitPermitDetailRow(row)), true, nil
}

func (r *Repository) ListForStudents(ctx context.Context, schoolID int64, studentIDs []int64) ([]Detail, error) {
	rows, err := r.q.ListExitPermitsForStudents(ctx, exitpermitdb.ListExitPermitsForStudentsParams{SchoolID: schoolID, StudentIds: studentIDs})
	if err != nil {
		return nil, err
	}
	out := make([]Detail, 0, len(rows))
	for _, row := range rows {
		out = append(out, detailFromRow(exitpermitdb.GetExitPermitDetailRow(row)))
	}
	return out, nil
}

func (r *Repository) ListActiveToday(ctx context.Context, schoolID int64, from, to time.Time) ([]Detail, error) {
	rows, err := r.q.ListExitPermitsActiveToday(ctx, exitpermitdb.ListExitPermitsActiveTodayParams{SchoolID: schoolID, FromAt: tsOrNil(from), ToAt: tsOrNil(to)})
	if err != nil {
		return nil, err
	}
	out := make([]Detail, 0, len(rows))
	for _, row := range rows {
		out = append(out, detailFromRow(exitpermitdb.GetExitPermitDetailRow(row)))
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
	rows, err := r.q.ListExitPermitsAll(ctx, exitpermitdb.ListExitPermitsAllParams{SchoolID: schoolID, FromAt: tsOrNil(fromT), ToAt: tsOrNil(toT)})
	if err != nil {
		return nil, err
	}
	out := make([]Detail, 0, len(rows))
	for _, row := range rows {
		out = append(out, detailFromRow(exitpermitdb.GetExitPermitDetailRow(row)))
	}
	return out, nil
}

func (r *Repository) ListExitedOnDate(ctx context.Context, schoolID int64, from, to time.Time) ([]Detail, error) {
	rows, err := r.q.ListExitPermitsExitedOnDate(ctx, exitpermitdb.ListExitPermitsExitedOnDateParams{SchoolID: schoolID, FromAt: tsOrNil(from), ToAt: tsOrNil(to)})
	if err != nil {
		return nil, err
	}
	out := make([]Detail, 0, len(rows))
	for _, row := range rows {
		out = append(out, detailFromRow(exitpermitdb.GetExitPermitDetailRow(row)))
	}
	return out, nil
}

func (r *Repository) UpdateDutyStage(ctx context.Context, schoolID, id, dutyBy int64) (int64, error) {
	return r.q.UpdateExitPermitDutyStage(ctx, exitpermitdb.UpdateExitPermitDutyStageParams{ID: id, SchoolID: schoolID, DutyBy: int8OrNil(dutyBy)})
}

func (r *Repository) UpdateClassStage(ctx context.Context, schoolID, id, classBy int64) (int64, error) {
	return r.q.UpdateExitPermitClassStage(ctx, exitpermitdb.UpdateExitPermitClassStageParams{ID: id, SchoolID: schoolID, ClassBy: int8OrNil(classBy)})
}

func (r *Repository) UpdateBKStage(ctx context.Context, schoolID, id, bkBy int64) (int64, error) {
	return r.q.UpdateExitPermitBKStage(ctx, exitpermitdb.UpdateExitPermitBKStageParams{ID: id, SchoolID: schoolID, BkBy: int8OrNil(bkBy)})
}

func (r *Repository) UpdateLeadershipStage(ctx context.Context, schoolID, id, leadershipBy int64, gateToken string, gateExpiresAt time.Time) (int64, error) {
	return r.q.UpdateExitPermitLeadershipStage(ctx, exitpermitdb.UpdateExitPermitLeadershipStageParams{
		ID: id, SchoolID: schoolID, LeadershipBy: int8OrNil(leadershipBy),
		GateToken: textOrNilFrom(gateToken), GateExpiresAt: tsOrNil(gateExpiresAt),
	})
}

func (r *Repository) Reject(ctx context.Context, schoolID, id, rejectedBy int64, comment string) (int64, error) {
	return r.q.RejectExitPermit(ctx, exitpermitdb.RejectExitPermitParams{ID: id, SchoolID: schoolID, RejectedBy: int8OrNil(rejectedBy), RejectComment: comment})
}

func (r *Repository) Cancel(ctx context.Context, schoolID, id int64) (int64, error) {
	return r.q.CancelExitPermit(ctx, exitpermitdb.CancelExitPermitParams{ID: id, SchoolID: schoolID})
}

func (r *Repository) GateScan(ctx context.Context, schoolID int64, gateToken string, exitedBy int64) (int64, error) {
	return r.q.GateScanExitPermit(ctx, exitpermitdb.GateScanExitPermitParams{GateToken: textOrNilFrom(gateToken), SchoolID: schoolID, ExitedBy: int8OrNil(exitedBy)})
}

func (r *Repository) GetStudentClassID(ctx context.Context, schoolID, academicYearID, studentID int64) (int64, bool, error) {
	id, err := r.q.GetStudentClassID(ctx, exitpermitdb.GetStudentClassIDParams{StudentID: studentID, SchoolID: schoolID, AcademicYearID: academicYearID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, false, nil
		}
		return 0, false, err
	}
	return id, true, nil
}

func (r *Repository) GetStudentUserID(ctx context.Context, schoolID, studentID int64) (int64, error) {
	return r.q.GetStudentUserID(ctx, exitpermitdb.GetStudentUserIDParams{ID: studentID, SchoolID: schoolID})
}
