package announcement

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/omanjaya/nouschool/internal/announcement/announcementdb"
)

// ErrNotFound menandai baris tidak ditemukan di repository modul announcement.
var ErrNotFound = errors.New("announcement: data tidak ditemukan")

// announcementRepository adalah kontrak yang dibutuhkan Service dari
// repository — dideklarasikan sebagai interface (dipenuhi *Repository secara
// struktural) supaya Service bisa dites dengan fake repository in-memory,
// tanpa DB — pola yang sama dengan internal/teaching & internal/attendance.
type announcementRepository interface {
	Insert(ctx context.Context, in InsertInput) (Record, error)
	GetByID(ctx context.Context, schoolID, id int64) (Record, error)
	ListAll(ctx context.Context, schoolID int64) ([]Record, error)
	ListActive(ctx context.Context, schoolID int64, date time.Time) ([]Record, error)
	Update(ctx context.Context, schoolID, id int64, title, body string, startsAt, endsAt time.Time) (Record, error)
	Delete(ctx context.Context, schoolID, id int64) error
}

var _ announcementRepository = (*Repository)(nil)

// Repository membungkus akses data modul announcement (sqlc + pgx).
type Repository struct {
	pool *pgxpool.Pool
	q    *announcementdb.Queries
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool, q: announcementdb.New(pool)}
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

func dateOf(t time.Time) pgtype.Date {
	if t.IsZero() {
		return pgtype.Date{}
	}
	return pgtype.Date{Time: t, Valid: true}
}

// Record adalah representasi mentah satu baris announcements.
type Record struct {
	ID        int64
	SchoolID  int64
	Title     string
	Body      string
	StartsAt  time.Time
	EndsAt    time.Time
	CreatedBy int64
	CreatedAt time.Time
}

func recordFromDB(a announcementdb.Announcement) Record {
	return Record{
		ID: a.ID, SchoolID: a.SchoolID, Title: a.Title, Body: a.Body,
		StartsAt: a.StartsAt.Time, EndsAt: a.EndsAt.Time, CreatedBy: a.CreatedBy,
		CreatedAt: a.CreatedAt.Time,
	}
}

// InsertInput adalah parameter INSERT satu pengumuman.
type InsertInput struct {
	SchoolID  int64
	Title     string
	Body      string
	StartsAt  time.Time
	EndsAt    time.Time
	CreatedBy int64
}

func (r *Repository) Insert(ctx context.Context, in InsertInput) (Record, error) {
	row, err := r.q.InsertAnnouncement(ctx, announcementdb.InsertAnnouncementParams{
		SchoolID: in.SchoolID, Title: in.Title, Body: in.Body,
		StartsAt: dateOf(in.StartsAt), EndsAt: dateOf(in.EndsAt), CreatedBy: in.CreatedBy,
	})
	if err != nil {
		return Record{}, err
	}
	return recordFromDB(row), nil
}

func (r *Repository) GetByID(ctx context.Context, schoolID, id int64) (Record, error) {
	row, err := r.q.GetAnnouncementByID(ctx, announcementdb.GetAnnouncementByIDParams{ID: id, SchoolID: schoolID})
	if err != nil {
		return Record{}, mapNoRows(err)
	}
	return recordFromDB(row), nil
}

func (r *Repository) ListAll(ctx context.Context, schoolID int64) ([]Record, error) {
	rows, err := r.q.ListAnnouncements(ctx, schoolID)
	if err != nil {
		return nil, err
	}
	out := make([]Record, 0, len(rows))
	for _, row := range rows {
		out = append(out, recordFromDB(row))
	}
	return out, nil
}

func (r *Repository) ListActive(ctx context.Context, schoolID int64, date time.Time) ([]Record, error) {
	rows, err := r.q.ListActiveAnnouncements(ctx, announcementdb.ListActiveAnnouncementsParams{
		SchoolID: schoolID, StartsAt: dateOf(date),
	})
	if err != nil {
		return nil, err
	}
	out := make([]Record, 0, len(rows))
	for _, row := range rows {
		out = append(out, recordFromDB(row))
	}
	return out, nil
}

func (r *Repository) Update(ctx context.Context, schoolID, id int64, title, body string, startsAt, endsAt time.Time) (Record, error) {
	row, err := r.q.UpdateAnnouncement(ctx, announcementdb.UpdateAnnouncementParams{
		ID: id, SchoolID: schoolID, Title: title, Body: body,
		StartsAt: dateOf(startsAt), EndsAt: dateOf(endsAt),
	})
	if err != nil {
		return Record{}, mapNoRows(err)
	}
	return recordFromDB(row), nil
}

func (r *Repository) Delete(ctx context.Context, schoolID, id int64) error {
	n, err := r.q.DeleteAnnouncement(ctx, announcementdb.DeleteAnnouncementParams{ID: id, SchoolID: schoolID})
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
