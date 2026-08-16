package platformadmin

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/omanjaya/nouschool/internal/platformadmin/platformadmindb"
)

// ErrNotFound menandai baris tidak ditemukan di repository modul platformadmin.
var ErrNotFound = errors.New("platformadmin: data tidak ditemukan")

// platformAdminRepository adalah kontrak yang dibutuhkan Service dari
// repository — dideklarasikan sebagai interface (dipenuhi *Repository secara
// struktural) supaya Service bisa dites dengan fake repository in-memory,
// tanpa DB (pola yang sama dengan internal/billing.billingRepository).
type platformAdminRepository interface {
	// -- P1 --
	ListSchoolsOverview(ctx context.Context) ([]SchoolOverviewRow, error)
	ListInvoicesAwaitingVerification(ctx context.Context) ([]InvoiceAwaiting, error)
	ListSchoolsWithoutActiveYear(ctx context.Context) ([]SchoolRef, error)
	ListSchoolsWithDeadOutbox(ctx context.Context) ([]OutboxDeadBySchool, error)
	ListSchoolLastActivity(ctx context.Context, limit int) ([]LastActivity, error)
	CountTotalActiveStudents(ctx context.Context) (int, error)
	SumRevenueInRange(ctx context.Context, from, to time.Time) (int64, error)
	CountLeadsSince(ctx context.Context, since time.Time) (int, error)

	// -- P3 --
	CountTeachers(ctx context.Context, schoolID int64) (int, error)
	CountActiveStudentsForSchool(ctx context.Context, schoolID int64) (int, error)
	CountClasses(ctx context.Context, schoolID int64) (int, error)
	CountAttendanceSessionsSince(ctx context.Context, schoolID int64, sinceDate time.Time) (int, error)
	CountJournalsSince(ctx context.Context, schoolID int64, sinceDate time.Time) (int, error)
	CountOutboxByStatusSince(ctx context.Context, schoolID int64, since time.Time) (OutboxCounts, error)
	ListLastLoginsByRole(ctx context.Context, schoolID int64) ([]LastLoginByRole, error)

	// -- P4.4 --
	ListOutboxAdmin(ctx context.Context, status string, schoolID int64, limit, offset int) ([]OutboxItem, error)
	CountOutboxAdmin(ctx context.Context, status string, schoolID int64) (int64, error)
	GetOutboxStatus(ctx context.Context, id int64) (string, error)
	RetryOutboxRow(ctx context.Context, id int64, now time.Time) error
	RetryAllOutbox(ctx context.Context, status string, schoolID int64, now time.Time) (int64, error)

	// -- P2.2 (fase 13 Gelombang 2) --
	SchoolExists(ctx context.Context, schoolID int64) (bool, error)
	SchoolOnboardingStatus(ctx context.Context, schoolID int64) (OnboardingRow, error)

	// -- P5 (fase 13 Gelombang 2) --
	InsertPlatformAnnouncement(ctx context.Context, title, body string, startsAt, endsAt time.Time, createdBy int64) (PlatformAnnouncementRecord, error)
	ListPlatformAnnouncements(ctx context.Context) ([]PlatformAnnouncementRecord, error)
	ListActivePlatformAnnouncements(ctx context.Context, date time.Time) ([]PlatformAnnouncementRecord, error)
	UpdatePlatformAnnouncement(ctx context.Context, id int64, title, body string, startsAt, endsAt time.Time) (PlatformAnnouncementRecord, error)
	DeletePlatformAnnouncement(ctx context.Context, id int64) error
}

var _ platformAdminRepository = (*Repository)(nil)

// Repository membungkus akses data modul platformadmin (sqlc + pgx). SEMUA
// query SELECT di sini adalah agregasi READ-ONLY lintas modul — lihat
// catatan desain di model.go (package doc).
type Repository struct {
	pool *pgxpool.Pool
	q    *platformadmindb.Queries
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool, q: platformadmindb.New(pool)}
}

func tsToPtr(v pgtype.Timestamptz) *time.Time {
	if !v.Valid {
		return nil
	}
	t := v.Time
	return &t
}

func dateToPtr(v pgtype.Date) *time.Time {
	if !v.Valid {
		return nil
	}
	t := v.Time
	return &t
}

func dateOf(t time.Time) pgtype.Date {
	if t.IsZero() {
		return pgtype.Date{}
	}
	return pgtype.Date{Time: t, Valid: true}
}

func tsOf(t time.Time) pgtype.Timestamptz {
	if t.IsZero() {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: t, Valid: true}
}

// -- P1 --

func (r *Repository) ListSchoolsOverview(ctx context.Context) ([]SchoolOverviewRow, error) {
	rows, err := r.q.ListSchoolsOverview(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]SchoolOverviewRow, 0, len(rows))
	for _, row := range rows {
		var subStatus *string
		if row.SubStatus.Valid {
			v := row.SubStatus.String
			subStatus = &v
		}
		out = append(out, SchoolOverviewRow{
			SchoolID: row.SchoolID, Name: row.Name, SchoolStatus: row.SchoolStatus,
			SubStatus: subStatus, SubEndsOn: dateToPtr(row.SubEndsOn),
		})
	}
	return out, nil
}

func (r *Repository) ListInvoicesAwaitingVerification(ctx context.Context) ([]InvoiceAwaiting, error) {
	rows, err := r.q.ListInvoicesAwaitingVerification(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]InvoiceAwaiting, 0, len(rows))
	for _, row := range rows {
		out = append(out, InvoiceAwaiting{
			InvoiceID: row.InvoiceID, Number: row.Number, SchoolID: row.SchoolID,
			SchoolName: row.SchoolName, Amount: row.Amount,
		})
	}
	return out, nil
}

func (r *Repository) ListSchoolsWithoutActiveYear(ctx context.Context) ([]SchoolRef, error) {
	rows, err := r.q.ListSchoolsWithoutActiveYear(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]SchoolRef, 0, len(rows))
	for _, row := range rows {
		out = append(out, SchoolRef{SchoolID: row.SchoolID, Name: row.Name})
	}
	return out, nil
}

func (r *Repository) ListSchoolsWithDeadOutbox(ctx context.Context) ([]OutboxDeadBySchool, error) {
	rows, err := r.q.ListSchoolsWithDeadOutbox(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]OutboxDeadBySchool, 0, len(rows))
	for _, row := range rows {
		out = append(out, OutboxDeadBySchool{SchoolID: row.SchoolID, SchoolName: row.SchoolName, DeadCount: row.DeadCount})
	}
	return out, nil
}

func (r *Repository) ListSchoolLastActivity(ctx context.Context, limit int) ([]LastActivity, error) {
	rows, err := r.q.ListSchoolLastActivity(ctx, int32(limit))
	if err != nil {
		return nil, err
	}
	out := make([]LastActivity, 0, len(rows))
	for _, row := range rows {
		out = append(out, LastActivity{
			SchoolID: row.SchoolID, Name: row.Name,
			LastLogin: tsToPtr(row.LastLogin), LastAttendanceSession: tsToPtr(row.LastAttendanceSession),
		})
	}
	return out, nil
}

func (r *Repository) CountTotalActiveStudents(ctx context.Context) (int, error) {
	n, err := r.q.CountTotalActiveStudents(ctx)
	return int(n), err
}

func (r *Repository) SumRevenueInRange(ctx context.Context, from, to time.Time) (int64, error) {
	return r.q.SumRevenueInRange(ctx, platformadmindb.SumRevenueInRangeParams{FromAt: tsOf(from), ToAt: tsOf(to)})
}

func (r *Repository) CountLeadsSince(ctx context.Context, since time.Time) (int, error) {
	n, err := r.q.CountLeadsSince(ctx, tsOf(since))
	return int(n), err
}

// -- P3 --

func (r *Repository) CountTeachers(ctx context.Context, schoolID int64) (int, error) {
	n, err := r.q.CountTeachersForSchool(ctx, schoolID)
	return int(n), err
}

func (r *Repository) CountActiveStudentsForSchool(ctx context.Context, schoolID int64) (int, error) {
	n, err := r.q.CountActiveStudentsForSchool(ctx, schoolID)
	return int(n), err
}

func (r *Repository) CountClasses(ctx context.Context, schoolID int64) (int, error) {
	n, err := r.q.CountClassesForSchool(ctx, schoolID)
	return int(n), err
}

func (r *Repository) CountAttendanceSessionsSince(ctx context.Context, schoolID int64, sinceDate time.Time) (int, error) {
	n, err := r.q.CountAttendanceSessionsSince(ctx, platformadmindb.CountAttendanceSessionsSinceParams{SchoolID: schoolID, SinceDate: dateOf(sinceDate)})
	return int(n), err
}

func (r *Repository) CountJournalsSince(ctx context.Context, schoolID int64, sinceDate time.Time) (int, error) {
	n, err := r.q.CountJournalsSince(ctx, platformadmindb.CountJournalsSinceParams{SchoolID: schoolID, SinceDate: dateOf(sinceDate)})
	return int(n), err
}

func (r *Repository) CountOutboxByStatusSince(ctx context.Context, schoolID int64, since time.Time) (OutboxCounts, error) {
	rows, err := r.q.CountOutboxByStatusSince(ctx, platformadmindb.CountOutboxByStatusSinceParams{SchoolID: schoolID, Since: tsOf(since)})
	if err != nil {
		return OutboxCounts{}, err
	}
	var out OutboxCounts
	for _, row := range rows {
		switch row.Status {
		case "sent":
			out.Sent = row.N
		case "failed":
			out.Failed = row.N
		case "dead":
			out.Dead = row.N
		}
	}
	return out, nil
}

func (r *Repository) ListLastLoginsByRole(ctx context.Context, schoolID int64) ([]LastLoginByRole, error) {
	rows, err := r.q.ListLastLoginsByRole(ctx, schoolID)
	if err != nil {
		return nil, err
	}
	out := make([]LastLoginByRole, 0, len(rows))
	for _, row := range rows {
		if !row.LastLogin.Valid {
			continue
		}
		out = append(out, LastLoginByRole{Role: row.Role, At: row.LastLogin.Time})
	}
	return out, nil
}

// -- P4.4 --

func (r *Repository) ListOutboxAdmin(ctx context.Context, status string, schoolID int64, limit, offset int) ([]OutboxItem, error) {
	rows, err := r.q.ListOutboxAdmin(ctx, platformadmindb.ListOutboxAdminParams{
		Status: status, SchoolID: schoolID, LimitCount: int32(limit), OffsetCount: int32(offset),
	})
	if err != nil {
		return nil, err
	}
	out := make([]OutboxItem, 0, len(rows))
	for _, row := range rows {
		out = append(out, OutboxItem{
			ID: row.ID, SchoolID: row.SchoolID, SchoolName: row.SchoolName, Event: row.Event,
			Channel: row.Channel, UserName: row.UserName, Status: row.Status, Attempts: int(row.Attempts),
			NextRetryAt: tsToPtr(row.NextRetryAt), CreatedAt: row.CreatedAt.Time,
		})
	}
	return out, nil
}

func (r *Repository) CountOutboxAdmin(ctx context.Context, status string, schoolID int64) (int64, error) {
	return r.q.CountOutboxAdmin(ctx, platformadmindb.CountOutboxAdminParams{Status: status, SchoolID: schoolID})
}

func (r *Repository) GetOutboxStatus(ctx context.Context, id int64) (string, error) {
	row, err := r.q.GetOutboxByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrNotFound
		}
		return "", err
	}
	return row.Status, nil
}

func (r *Repository) RetryOutboxRow(ctx context.Context, id int64, now time.Time) error {
	return r.q.RetryOutboxRow(ctx, platformadmindb.RetryOutboxRowParams{ID: id, Now: tsOf(now)})
}

func (r *Repository) RetryAllOutbox(ctx context.Context, status string, schoolID int64, now time.Time) (int64, error) {
	return r.q.RetryAllOutbox(ctx, platformadmindb.RetryAllOutboxParams{Status: status, SchoolID: schoolID, Now: tsOf(now)})
}

// -- P2.2 (fase 13 Gelombang 2) --

func (r *Repository) SchoolExists(ctx context.Context, schoolID int64) (bool, error) {
	return r.q.SchoolExistsByID(ctx, schoolID)
}

func (r *Repository) SchoolOnboardingStatus(ctx context.Context, schoolID int64) (OnboardingRow, error) {
	row, err := r.q.SchoolOnboardingStatus(ctx, schoolID)
	if err != nil {
		return OnboardingRow{}, err
	}
	return OnboardingRow{
		HasActiveYear: row.HasActiveYear, HasAdmin: row.HasAdmin, HasSubscriptionActive: row.HasSubscriptionActive,
		HasStudents: row.HasStudents, HasSchedule: row.HasSchedule,
	}, nil
}

// -- P5 (fase 13 Gelombang 2) --

func platformAnnouncementFromDB(a platformadmindb.PlatformAnnouncement) PlatformAnnouncementRecord {
	var createdBy int64
	if a.CreatedBy.Valid {
		createdBy = a.CreatedBy.Int64
	}
	return PlatformAnnouncementRecord{
		ID: a.ID, Title: a.Title, Body: a.Body, StartsAt: a.StartsAt.Time, EndsAt: a.EndsAt.Time,
		CreatedBy: createdBy, CreatedAt: a.CreatedAt.Time,
	}
}

func (r *Repository) InsertPlatformAnnouncement(ctx context.Context, title, body string, startsAt, endsAt time.Time, createdBy int64) (PlatformAnnouncementRecord, error) {
	row, err := r.q.InsertPlatformAnnouncement(ctx, platformadmindb.InsertPlatformAnnouncementParams{
		Title: title, Body: body, StartsAt: dateOf(startsAt), EndsAt: dateOf(endsAt), CreatedBy: createdBy,
	})
	if err != nil {
		return PlatformAnnouncementRecord{}, err
	}
	return platformAnnouncementFromDB(row), nil
}

func (r *Repository) ListPlatformAnnouncements(ctx context.Context) ([]PlatformAnnouncementRecord, error) {
	rows, err := r.q.ListPlatformAnnouncements(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]PlatformAnnouncementRecord, 0, len(rows))
	for _, row := range rows {
		out = append(out, platformAnnouncementFromDB(row))
	}
	return out, nil
}

func (r *Repository) ListActivePlatformAnnouncements(ctx context.Context, date time.Time) ([]PlatformAnnouncementRecord, error) {
	rows, err := r.q.ListActivePlatformAnnouncements(ctx, dateOf(date))
	if err != nil {
		return nil, err
	}
	out := make([]PlatformAnnouncementRecord, 0, len(rows))
	for _, row := range rows {
		out = append(out, platformAnnouncementFromDB(row))
	}
	return out, nil
}

func (r *Repository) UpdatePlatformAnnouncement(ctx context.Context, id int64, title, body string, startsAt, endsAt time.Time) (PlatformAnnouncementRecord, error) {
	row, err := r.q.UpdatePlatformAnnouncement(ctx, platformadmindb.UpdatePlatformAnnouncementParams{
		ID: id, Title: title, Body: body, StartsAt: dateOf(startsAt), EndsAt: dateOf(endsAt),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return PlatformAnnouncementRecord{}, ErrNotFound
		}
		return PlatformAnnouncementRecord{}, err
	}
	return platformAnnouncementFromDB(row), nil
}

func (r *Repository) DeletePlatformAnnouncement(ctx context.Context, id int64) error {
	n, err := r.q.DeletePlatformAnnouncement(ctx, id)
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
