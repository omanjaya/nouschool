package substitution

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/omanjaya/nouschool/internal/substitution/substitutiondb"
)

// ErrNotFound menandai baris tidak ditemukan di repository modul substitution.
var ErrNotFound = errors.New("substitution: data tidak ditemukan")

// ErrConflict menandai pelanggaran unique constraint (sudah ada permintaan
// aktif pending/accepted utk slot+tanggal yang sama).
var ErrConflict = errors.New("substitution: sudah ada permintaan aktif untuk slot & tanggal ini")

// ErrStateChanged menandai TransitionSubstitutionStatus tidak menemukan baris
// dengan status from_status yang diharapkan (sudah diputuskan/dibatalkan
// pihak lain — race guard, lihat queries.sql).
var ErrStateChanged = errors.New("substitution: status permintaan sudah berubah")

type substitutionRepository interface {
	Create(ctx context.Context, in CreateInput) (Record, error)
	GetByID(ctx context.Context, schoolID, id int64) (Record, error)
	GetRow(ctx context.Context, schoolID, id int64) (Row, error)
	ListRows(ctx context.Context, schoolID int64, requestedBy, substituteUserID int64, date string) ([]Row, error)
	Transition(ctx context.Context, schoolID, id int64, from, to string, decidedAt time.Time) (Record, error)
	GetSlotBasic(ctx context.Context, schoolID, slotID int64) (SlotBasic, error)
	GetTeacherUserID(ctx context.Context, schoolID, teacherID int64) (int64, error)
	IsActiveGuru(ctx context.Context, schoolID, userID int64) (bool, error)
	ActiveSubstituteForSlotDate(ctx context.Context, schoolID, slotID int64, date time.Time) (userID int64, name string, ok bool, err error)
	SlotIDsAcceptedSubstituteForDate(ctx context.Context, schoolID, substituteUserID int64, date time.Time) ([]int64, error)
}

var _ substitutionRepository = (*Repository)(nil)

// Repository membungkus akses data modul substitution (sqlc + pgx).
type Repository struct {
	q *substitutiondb.Queries
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{q: substitutiondb.New(pool)}
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

func dateOf(t time.Time) pgtype.Date { return pgtype.Date{Time: t, Valid: true} }

func int8OrNil(v int64) pgtype.Int8 {
	if v == 0 {
		return pgtype.Int8{}
	}
	return pgtype.Int8{Int64: v, Valid: true}
}

func dateOrNil(s string) pgtype.Date {
	if s == "" {
		return pgtype.Date{}
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return pgtype.Date{}
	}
	return pgtype.Date{Time: t, Valid: true}
}

// -- domain records --

type Record struct {
	ID               int64
	SchoolID         int64
	ScheduleSlotID   int64
	Date             time.Time
	RequestedBy      int64
	SubstituteUserID int64
	Reason           string
	Status           string
	DecidedAt        *time.Time
	CreatedAt        time.Time
}

func recordFromDB(r substitutiondb.TeacherSubstitutionRequest) Record {
	rec := Record{
		ID: r.ID, SchoolID: r.SchoolID, ScheduleSlotID: r.ScheduleSlotID, Date: r.Date.Time,
		RequestedBy: r.RequestedBy, SubstituteUserID: r.SubstituteUserID, Reason: r.Reason,
		Status: r.Status, CreatedAt: r.CreatedAt.Time,
	}
	if r.DecidedAt.Valid {
		t := r.DecidedAt.Time
		rec.DecidedAt = &t
	}
	return rec
}

// Row adalah Record + info slot/nama (hasil join, dipakai list & detail).
type Row struct {
	Record
	ClassName       string
	SubjectName     string
	DayOfWeek       int
	PeriodStart     int
	PeriodEnd       int
	RequestedByName string
	SubstituteName  string
}

// SlotBasic adalah potongan info slot dipakai validasi request.
type SlotBasic struct {
	ID        int64
	TeacherID int64
	DayOfWeek int
}

// CreateInput adalah parameter Repository.Create.
type CreateInput struct {
	SchoolID         int64
	ScheduleSlotID   int64
	Date             time.Time
	RequestedBy      int64
	SubstituteUserID int64
	Reason           string
}

func (r *Repository) Create(ctx context.Context, in CreateInput) (Record, error) {
	rec, err := r.q.CreateSubstitutionRequest(ctx, substitutiondb.CreateSubstitutionRequestParams{
		SchoolID: in.SchoolID, ScheduleSlotID: in.ScheduleSlotID, Date: dateOf(in.Date),
		RequestedBy: in.RequestedBy, SubstituteUserID: in.SubstituteUserID, Reason: in.Reason,
	})
	if err != nil {
		if isUniqueViolation(err) {
			return Record{}, ErrConflict
		}
		return Record{}, err
	}
	return recordFromDB(rec), nil
}

func (r *Repository) GetByID(ctx context.Context, schoolID, id int64) (Record, error) {
	rec, err := r.q.GetSubstitutionRequestByID(ctx, substitutiondb.GetSubstitutionRequestByIDParams{ID: id, SchoolID: schoolID})
	if err != nil {
		return Record{}, mapNoRows(err)
	}
	return recordFromDB(rec), nil
}

func rowFromGet(row substitutiondb.GetSubstitutionRowRow) Row {
	rec := Record{
		ID: row.ID, SchoolID: row.SchoolID, ScheduleSlotID: row.ScheduleSlotID, Date: row.Date.Time,
		RequestedBy: row.RequestedBy, SubstituteUserID: row.SubstituteUserID, Reason: row.Reason,
		Status: row.Status, CreatedAt: row.CreatedAt.Time,
	}
	if row.DecidedAt.Valid {
		t := row.DecidedAt.Time
		rec.DecidedAt = &t
	}
	return Row{
		Record: rec, ClassName: row.ClassName, SubjectName: row.SubjectName,
		DayOfWeek: int(row.DayOfWeek), PeriodStart: int(row.PeriodStart), PeriodEnd: int(row.PeriodEnd),
		RequestedByName: row.RequestedByName, SubstituteName: row.SubstituteName,
	}
}

func (r *Repository) GetRow(ctx context.Context, schoolID, id int64) (Row, error) {
	row, err := r.q.GetSubstitutionRow(ctx, substitutiondb.GetSubstitutionRowParams{ID: id, SchoolID: schoolID})
	if err != nil {
		return Row{}, mapNoRows(err)
	}
	return rowFromGet(row), nil
}

func (r *Repository) ListRows(ctx context.Context, schoolID int64, requestedBy, substituteUserID int64, date string) ([]Row, error) {
	rows, err := r.q.ListSubstitutionRequests(ctx, substitutiondb.ListSubstitutionRequestsParams{
		SchoolID: schoolID, RequestedBy: int8OrNil(requestedBy), SubstituteUserID: int8OrNil(substituteUserID), Date: dateOrNil(date),
	})
	if err != nil {
		return nil, err
	}
	out := make([]Row, 0, len(rows))
	for _, row := range rows {
		rec := Record{
			ID: row.ID, SchoolID: row.SchoolID, ScheduleSlotID: row.ScheduleSlotID, Date: row.Date.Time,
			RequestedBy: row.RequestedBy, SubstituteUserID: row.SubstituteUserID, Reason: row.Reason,
			Status: row.Status, CreatedAt: row.CreatedAt.Time,
		}
		if row.DecidedAt.Valid {
			t := row.DecidedAt.Time
			rec.DecidedAt = &t
		}
		out = append(out, Row{
			Record: rec, ClassName: row.ClassName, SubjectName: row.SubjectName,
			DayOfWeek: int(row.DayOfWeek), PeriodStart: int(row.PeriodStart), PeriodEnd: int(row.PeriodEnd),
			RequestedByName: row.RequestedByName, SubstituteName: row.SubstituteName,
		})
	}
	return out, nil
}

func (r *Repository) Transition(ctx context.Context, schoolID, id int64, from, to string, decidedAt time.Time) (Record, error) {
	rec, err := r.q.TransitionSubstitutionStatus(ctx, substitutiondb.TransitionSubstitutionStatusParams{
		ToStatus: to, DecidedAt: pgtype.Timestamptz{Time: decidedAt, Valid: true}, ID: id, SchoolID: schoolID, FromStatus: from,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Record{}, ErrStateChanged
		}
		return Record{}, err
	}
	return recordFromDB(rec), nil
}

func (r *Repository) GetSlotBasic(ctx context.Context, schoolID, slotID int64) (SlotBasic, error) {
	row, err := r.q.GetSlotBasic(ctx, substitutiondb.GetSlotBasicParams{ID: slotID, SchoolID: schoolID})
	if err != nil {
		return SlotBasic{}, mapNoRows(err)
	}
	return SlotBasic{ID: row.ID, TeacherID: row.TeacherID, DayOfWeek: int(row.DayOfWeek)}, nil
}

func (r *Repository) ActiveSubstituteForSlotDate(ctx context.Context, schoolID, slotID int64, date time.Time) (int64, string, bool, error) {
	row, err := r.q.ActiveSubstituteForSlotDate(ctx, substitutiondb.ActiveSubstituteForSlotDateParams{
		ScheduleSlotID: slotID, SchoolID: schoolID, Date: dateOf(date),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, "", false, nil
		}
		return 0, "", false, err
	}
	return row.UserID, row.Name, true, nil
}

func (r *Repository) GetTeacherUserID(ctx context.Context, schoolID, teacherID int64) (int64, error) {
	id, err := r.q.GetTeacherUserID(ctx, substitutiondb.GetTeacherUserIDParams{ID: teacherID, SchoolID: schoolID})
	if err != nil {
		return 0, mapNoRows(err)
	}
	return id, nil
}

func (r *Repository) IsActiveGuru(ctx context.Context, schoolID, userID int64) (bool, error) {
	return r.q.IsActiveGuru(ctx, substitutiondb.IsActiveGuruParams{UserID: userID, SchoolID: schoolID})
}

func (r *Repository) SlotIDsAcceptedSubstituteForDate(ctx context.Context, schoolID, substituteUserID int64, date time.Time) ([]int64, error) {
	ids, err := r.q.SlotIDsAcceptedSubstituteForDate(ctx, substitutiondb.SlotIDsAcceptedSubstituteForDateParams{
		SchoolID: schoolID, SubstituteUserID: substituteUserID, Date: dateOf(date),
	})
	if err != nil {
		return nil, err
	}
	if ids == nil {
		ids = []int64{}
	}
	return ids, nil
}
