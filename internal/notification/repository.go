package notification

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/omanjaya/nouschool/internal/notification/notificationdb"
)

// ErrNotFound menandai baris tidak ditemukan di repository modul notification.
var ErrNotFound = errors.New("notification: data tidak ditemukan")

// notificationRepository adalah kontrak yang dibutuhkan Service dari
// repository — dideklarasikan sebagai interface (dipenuhi *Repository secara
// struktural) supaya Service bisa dites dengan fake repository in-memory,
// tanpa DB — pola yang sama dengan internal/leave.
type notificationRepository interface {
	GetSettings(ctx context.Context, schoolID int64) (Settings, error)

	InsertNotification(ctx context.Context, schoolID, userID int64, event, title, body, link string) (int64, error)
	ListNotifications(ctx context.Context, schoolID, userID int64, limit, offset int32) ([]NotificationRow, error)
	CountUnreadNotifications(ctx context.Context, schoolID, userID int64) (int64, error)
	MarkNotificationsReadByIDs(ctx context.Context, schoolID, userID int64, ids []int64) (int64, error)
	MarkAllNotificationsRead(ctx context.Context, schoolID, userID int64) (int64, error)

	InsertOutboxRow(ctx context.Context, schoolID, userID int64, event, channel string, payload []byte) error
	ListDueOutbox(ctx context.Context, limit int32) ([]OutboxRow, error)
	MarkOutboxSent(ctx context.Context, id int64, sentAt time.Time) error
	MarkOutboxRetry(ctx context.Context, id int64, attempts int32, nextRetryAt time.Time) error
	MarkOutboxDead(ctx context.Context, id int64, attempts int32) error
	CountOutboxByChannel(ctx context.Context, schoolID int64, event, channel string) (int64, error)

	GetUserContact(ctx context.Context, userID int64) (phone, email string, err error)

	UpsertPushSubscription(ctx context.Context, schoolID, userID int64, endpoint, p256dh, auth string) error
	ListPushSubscriptionsForUser(ctx context.Context, schoolID, userID int64) ([]PushSubscriptionRow, error)
	DeletePushSubscriptionByEndpoint(ctx context.Context, endpoint string) error
	DeletePushSubscriptionByUserEndpoint(ctx context.Context, schoolID, userID int64, endpoint string) (int64, error)

	GetPlatformConfig(ctx context.Context, key string) (string, bool, error)
	SetPlatformConfig(ctx context.Context, key, value string) error
}

var _ notificationRepository = (*Repository)(nil)

// Repository membungkus akses data modul notification (sqlc + pgx).
type Repository struct {
	pool *pgxpool.Pool
	q    *notificationdb.Queries
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool, q: notificationdb.New(pool)}
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

func tsOrNil(t time.Time) pgtype.Timestamptz {
	if t.IsZero() {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: t, Valid: true}
}

// -- settings --

// GetSettings membaca school_settings module='notification' langsung (tabel
// & pola yang sama dipakai tenant.SettingsService — lihat catatan di
// internal/leave/repository.go) dan mengembalikan default bila sekolah belum
// pernah menyimpan pengaturannya.
func (r *Repository) GetSettings(ctx context.Context, schoolID int64) (Settings, error) {
	raw, err := r.q.GetNotificationSettingsRaw(ctx, schoolID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return DefaultSettings(), nil
		}
		return Settings{}, err
	}
	out := DefaultSettings()
	if err := json.Unmarshal(raw, &out); err != nil {
		return Settings{}, err
	}
	return out, nil
}

// -- inbox in-app --

type NotificationRow struct {
	ID        int64
	Event     string
	Title     string
	Body      string
	Link      string
	ReadAt    *time.Time
	CreatedAt time.Time
}

func notificationFromDB(n notificationdb.Notification) NotificationRow {
	row := NotificationRow{ID: n.ID, Event: n.Event, Title: n.Title, Body: n.Body, Link: n.Link.String, CreatedAt: n.CreatedAt.Time}
	if n.ReadAt.Valid {
		t := n.ReadAt.Time
		row.ReadAt = &t
	}
	return row
}

func (r *Repository) InsertNotification(ctx context.Context, schoolID, userID int64, event, title, body, link string) (int64, error) {
	row, err := r.q.InsertNotification(ctx, notificationdb.InsertNotificationParams{
		SchoolID: schoolID, UserID: userID, Event: event, Title: title, Body: body, Link: textOrNil(link),
	})
	if err != nil {
		return 0, err
	}
	return row.ID, nil
}

func (r *Repository) ListNotifications(ctx context.Context, schoolID, userID int64, limit, offset int32) ([]NotificationRow, error) {
	rows, err := r.q.ListNotifications(ctx, notificationdb.ListNotificationsParams{
		SchoolID: schoolID, UserID: userID, LimitCount: limit, OffsetCount: offset,
	})
	if err != nil {
		return nil, err
	}
	out := make([]NotificationRow, 0, len(rows))
	for _, row := range rows {
		out = append(out, notificationFromDB(row))
	}
	return out, nil
}

func (r *Repository) CountUnreadNotifications(ctx context.Context, schoolID, userID int64) (int64, error) {
	return r.q.CountUnreadNotifications(ctx, notificationdb.CountUnreadNotificationsParams{SchoolID: schoolID, UserID: userID})
}

func (r *Repository) MarkNotificationsReadByIDs(ctx context.Context, schoolID, userID int64, ids []int64) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	return r.q.MarkNotificationsReadByIDs(ctx, notificationdb.MarkNotificationsReadByIDsParams{SchoolID: schoolID, UserID: userID, Ids: ids})
}

func (r *Repository) MarkAllNotificationsRead(ctx context.Context, schoolID, userID int64) (int64, error) {
	return r.q.MarkAllNotificationsRead(ctx, notificationdb.MarkAllNotificationsReadParams{SchoolID: schoolID, UserID: userID})
}

// -- outbox --

type OutboxRow struct {
	ID       int64
	SchoolID int64
	Event    string
	UserID   int64
	Channel  string
	Payload  []byte
	Status   string
	Attempts int32
}

func outboxFromDB(o notificationdb.NotificationOutbox) OutboxRow {
	return OutboxRow{ID: o.ID, SchoolID: o.SchoolID, Event: o.Event, UserID: o.UserID, Channel: o.Channel, Payload: o.Payload, Status: o.Status, Attempts: o.Attempts}
}

func (r *Repository) InsertOutboxRow(ctx context.Context, schoolID, userID int64, event, channel string, payload []byte) error {
	_, err := r.q.InsertOutboxRow(ctx, notificationdb.InsertOutboxRowParams{
		SchoolID: schoolID, Event: event, UserID: userID, Channel: channel, Payload: payload,
	})
	return err
}

func (r *Repository) ListDueOutbox(ctx context.Context, limit int32) ([]OutboxRow, error) {
	rows, err := r.q.ListDueOutbox(ctx, limit)
	if err != nil {
		return nil, err
	}
	out := make([]OutboxRow, 0, len(rows))
	for _, row := range rows {
		out = append(out, outboxFromDB(row))
	}
	return out, nil
}

func (r *Repository) MarkOutboxSent(ctx context.Context, id int64, sentAt time.Time) error {
	return r.q.MarkOutboxSent(ctx, notificationdb.MarkOutboxSentParams{ID: id, SentAt: tsOrNil(sentAt)})
}

func (r *Repository) MarkOutboxRetry(ctx context.Context, id int64, attempts int32, nextRetryAt time.Time) error {
	return r.q.MarkOutboxRetry(ctx, notificationdb.MarkOutboxRetryParams{ID: id, Attempts: attempts, NextRetryAt: tsOrNil(nextRetryAt)})
}

func (r *Repository) MarkOutboxDead(ctx context.Context, id int64, attempts int32) error {
	return r.q.MarkOutboxDead(ctx, notificationdb.MarkOutboxDeadParams{ID: id, Attempts: attempts})
}

func (r *Repository) CountOutboxByChannel(ctx context.Context, schoolID int64, event, channel string) (int64, error) {
	return r.q.CountOutboxByChannel(ctx, notificationdb.CountOutboxByChannelParams{SchoolID: schoolID, Event: event, Channel: channel})
}

// -- kontak --

func (r *Repository) GetUserContact(ctx context.Context, userID int64) (phone, email string, err error) {
	row, err := r.q.GetUserContact(ctx, userID)
	if err != nil {
		return "", "", mapNoRows(err)
	}
	return row.Phone.String, row.Email.String, nil
}

// -- push subscriptions --

type PushSubscriptionRow struct {
	ID       int64
	Endpoint string
	P256dh   string
	Auth     string
}

func (r *Repository) UpsertPushSubscription(ctx context.Context, schoolID, userID int64, endpoint, p256dh, auth string) error {
	_, err := r.q.UpsertPushSubscription(ctx, notificationdb.UpsertPushSubscriptionParams{
		SchoolID: schoolID, UserID: userID, Endpoint: endpoint, P256dh: p256dh, Auth: auth,
	})
	return err
}

func (r *Repository) ListPushSubscriptionsForUser(ctx context.Context, schoolID, userID int64) ([]PushSubscriptionRow, error) {
	rows, err := r.q.ListPushSubscriptionsForUser(ctx, notificationdb.ListPushSubscriptionsForUserParams{SchoolID: schoolID, UserID: userID})
	if err != nil {
		return nil, err
	}
	out := make([]PushSubscriptionRow, 0, len(rows))
	for _, row := range rows {
		out = append(out, PushSubscriptionRow{ID: row.ID, Endpoint: row.Endpoint, P256dh: row.P256dh, Auth: row.Auth})
	}
	return out, nil
}

func (r *Repository) DeletePushSubscriptionByEndpoint(ctx context.Context, endpoint string) error {
	return r.q.DeletePushSubscriptionByEndpoint(ctx, endpoint)
}

func (r *Repository) DeletePushSubscriptionByUserEndpoint(ctx context.Context, schoolID, userID int64, endpoint string) (int64, error) {
	return r.q.DeletePushSubscriptionByUserEndpoint(ctx, notificationdb.DeletePushSubscriptionByUserEndpointParams{
		SchoolID: schoolID, UserID: userID, Endpoint: endpoint,
	})
}

// -- platform_config --

func (r *Repository) GetPlatformConfig(ctx context.Context, key string) (string, bool, error) {
	v, err := r.q.GetPlatformConfig(ctx, key)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", false, nil
		}
		return "", false, err
	}
	return v, true, nil
}

func (r *Repository) SetPlatformConfig(ctx context.Context, key, value string) error {
	return r.q.UpsertPlatformConfig(ctx, notificationdb.UpsertPlatformConfigParams{Key: key, Value: value})
}
