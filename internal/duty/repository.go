package duty

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/omanjaya/nouschool/internal/duty/dutydb"
)

// ErrNotFound / ErrConflict — sama pola dengan internal/discipline.
var ErrNotFound = errors.New("duty: data tidak ditemukan")
var ErrConflict = errors.New("duty: data sudah ada/bentrok")

// dutyRepository adalah kontrak yang dibutuhkan Service dari repository —
// dideklarasikan sebagai interface (dipenuhi *Repository secara struktural)
// supaya Service bisa dites dengan fake repository in-memory, tanpa DB.
type dutyRepository interface {
	CreateDuty(ctx context.Context, schoolID int64, name, forRole string, flags []string) (DutyRecord, error)
	UpdateDuty(ctx context.Context, schoolID, id int64, name *string, forRole *string, flags []string, active *bool) (DutyRecord, error)
	GetDutyByID(ctx context.Context, schoolID, id int64) (DutyRecord, error)
	ListDutiesWithAssigneeCount(ctx context.Context, schoolID, activeYearID int64) ([]DutyWithCount, error)
	DeleteDuty(ctx context.Context, schoolID, id int64) (int64, error)
	CountAssignmentsForDuty(ctx context.Context, schoolID, dutyID int64) (int64, error)

	ReplaceAssignments(ctx context.Context, schoolID, dutyID, academicYearID int64, userIDs []int64) error
	ListAssignmentsForDutyYear(ctx context.Context, schoolID, dutyID, academicYearID int64) ([]AssignmentUser, error)

	UserHasFlag(ctx context.Context, schoolID, userID, academicYearID int64, flag string) (bool, error)
	UserIDsWithFlag(ctx context.Context, schoolID, academicYearID int64, flag string) ([]int64, error)
}

var _ dutyRepository = (*Repository)(nil)

// Repository membungkus akses data modul duty (sqlc + pgx).
type Repository struct {
	pool *pgxpool.Pool
	q    *dutydb.Queries
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool, q: dutydb.New(pool)}
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

func textOrNilFromPtr(v *string) pgtype.Text {
	if v == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: *v, Valid: true}
}

func boolOrNilFromPtr(v *bool) pgtype.Bool {
	if v == nil {
		return pgtype.Bool{}
	}
	return pgtype.Bool{Bool: *v, Valid: true}
}

// -- domain records --

// DutyRecord adalah representasi domain satu baris duties.
type DutyRecord struct {
	ID       int64
	SchoolID int64
	Name     string
	ForRole  string
	Flags    []string
	Active   bool
}

func (d DutyRecord) view(assigneeCount int64) DutyView {
	return DutyView{ID: d.ID, Name: d.Name, ForRole: d.ForRole, Flags: d.Flags, Active: d.Active, AssigneeCount: assigneeCount}
}

// DutyWithCount adalah DutyRecord + assignee_count (TA aktif).
type DutyWithCount struct {
	DutyRecord
	AssigneeCount int64
}

// AssignmentUser adalah satu baris pemegang tugas (dipakai GET .../assignments).
type AssignmentUser struct {
	UserID int64
	Name   string
}

func dutyFromDB(d dutydb.Duty) DutyRecord {
	return DutyRecord{ID: d.ID, SchoolID: d.SchoolID, Name: d.Name, ForRole: d.ForRole, Flags: d.Flags, Active: d.Active}
}

// -- duties --

func (r *Repository) CreateDuty(ctx context.Context, schoolID int64, name, forRole string, flags []string) (DutyRecord, error) {
	if flags == nil {
		flags = []string{}
	}
	row, err := r.q.CreateDuty(ctx, dutydb.CreateDutyParams{SchoolID: schoolID, Name: name, ForRole: forRole, Flags: flags})
	if err != nil {
		if isUniqueViolation(err) {
			return DutyRecord{}, ErrConflict
		}
		return DutyRecord{}, err
	}
	return dutyFromDB(row), nil
}

func (r *Repository) UpdateDuty(ctx context.Context, schoolID, id int64, name *string, forRole *string, flags []string, active *bool) (DutyRecord, error) {
	var flagsArg []string
	if flags != nil {
		flagsArg = flags
	}
	row, err := r.q.UpdateDuty(ctx, dutydb.UpdateDutyParams{
		ID: id, SchoolID: schoolID, Name: textOrNilFromPtr(name), ForRole: textOrNilFromPtr(forRole),
		Flags: flagsArg, Active: boolOrNilFromPtr(active),
	})
	if err != nil {
		if isUniqueViolation(err) {
			return DutyRecord{}, ErrConflict
		}
		return DutyRecord{}, mapNoRows(err)
	}
	return dutyFromDB(row), nil
}

func (r *Repository) GetDutyByID(ctx context.Context, schoolID, id int64) (DutyRecord, error) {
	row, err := r.q.GetDutyByID(ctx, dutydb.GetDutyByIDParams{ID: id, SchoolID: schoolID})
	if err != nil {
		return DutyRecord{}, mapNoRows(err)
	}
	return dutyFromDB(row), nil
}

func (r *Repository) ListDutiesWithAssigneeCount(ctx context.Context, schoolID, activeYearID int64) ([]DutyWithCount, error) {
	rows, err := r.q.ListDutiesWithAssigneeCount(ctx, dutydb.ListDutiesWithAssigneeCountParams{SchoolID: schoolID, AcademicYearID: activeYearID})
	if err != nil {
		return nil, err
	}
	out := make([]DutyWithCount, 0, len(rows))
	for _, row := range rows {
		out = append(out, DutyWithCount{
			DutyRecord:    DutyRecord{ID: row.ID, SchoolID: row.SchoolID, Name: row.Name, ForRole: row.ForRole, Flags: row.Flags, Active: row.Active},
			AssigneeCount: row.AssigneeCount,
		})
	}
	return out, nil
}

func (r *Repository) DeleteDuty(ctx context.Context, schoolID, id int64) (int64, error) {
	return r.q.DeleteDuty(ctx, dutydb.DeleteDutyParams{ID: id, SchoolID: schoolID})
}

func (r *Repository) CountAssignmentsForDuty(ctx context.Context, schoolID, dutyID int64) (int64, error) {
	return r.q.CountAssignmentsForDuty(ctx, dutydb.CountAssignmentsForDutyParams{DutyID: dutyID, SchoolID: schoolID})
}

// -- duty_assignments --

// ReplaceAssignments mengganti SELURUH pemegang tugas untuk (dutyID,
// academicYearID) dalam SATU transaksi (delete-all lalu insert-all) — pola
// yang sama dengan schedule.ReplacePeriods (replace-all, bukan diff).
func (r *Repository) ReplaceAssignments(ctx context.Context, schoolID, dutyID, academicYearID int64, userIDs []int64) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	qtx := r.q.WithTx(tx)
	if err := qtx.DeleteAssignmentsForDutyYear(ctx, dutydb.DeleteAssignmentsForDutyYearParams{
		DutyID: dutyID, SchoolID: schoolID, AcademicYearID: academicYearID,
	}); err != nil {
		return err
	}
	for _, uid := range userIDs {
		if _, err := qtx.CreateAssignment(ctx, dutydb.CreateAssignmentParams{
			SchoolID: schoolID, DutyID: dutyID, UserID: uid, AcademicYearID: academicYearID,
		}); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (r *Repository) ListAssignmentsForDutyYear(ctx context.Context, schoolID, dutyID, academicYearID int64) ([]AssignmentUser, error) {
	rows, err := r.q.ListAssignmentsForDutyYear(ctx, dutydb.ListAssignmentsForDutyYearParams{
		DutyID: dutyID, SchoolID: schoolID, AcademicYearID: academicYearID,
	})
	if err != nil {
		return nil, err
	}
	out := make([]AssignmentUser, 0, len(rows))
	for _, row := range rows {
		out = append(out, AssignmentUser{UserID: row.UserID, Name: row.Name})
	}
	return out, nil
}

// -- interface publik (dipakai modul lain, mis. studentleave) --

func (r *Repository) UserHasFlag(ctx context.Context, schoolID, userID, academicYearID int64, flag string) (bool, error) {
	return r.q.UserHasFlag(ctx, dutydb.UserHasFlagParams{SchoolID: schoolID, UserID: userID, AcademicYearID: academicYearID, Column4: flag})
}

func (r *Repository) UserIDsWithFlag(ctx context.Context, schoolID, academicYearID int64, flag string) ([]int64, error) {
	return r.q.ListUserIDsWithFlag(ctx, dutydb.ListUserIDsWithFlagParams{SchoolID: schoolID, AcademicYearID: academicYearID, Column3: flag})
}
