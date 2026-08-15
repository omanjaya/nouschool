-- Query modul notification (sqlc). Semua query tenant-scoped WAJIB filter
-- school_id. Join read-only ke users (dimiliki modul identity) dipakai untuk
-- resolusi kontak (phone/email) provider — pola yang sama dengan
-- internal/leave/queries.sql & internal/attendance/queries.sql.

-- name: GetNotificationSettingsRaw :one
SELECT settings FROM school_settings WHERE school_id = $1 AND module = 'notification';

-- -- inbox in-app --

-- name: InsertNotification :one
INSERT INTO notifications (school_id, user_id, event, title, body, link)
VALUES ($1, $2, $3, $4, $5, sqlc.narg(link)::text)
RETURNING *;

-- name: ListNotifications :many
SELECT * FROM notifications
WHERE school_id = sqlc.arg(school_id)::bigint AND user_id = sqlc.arg(user_id)::bigint
ORDER BY created_at DESC
LIMIT sqlc.arg(limit_count)::int OFFSET sqlc.arg(offset_count)::int;

-- name: CountUnreadNotifications :one
SELECT COUNT(*) FROM notifications
WHERE school_id = $1 AND user_id = $2 AND read_at IS NULL;

-- name: MarkNotificationsReadByIDs :execrows
UPDATE notifications SET read_at = now()
WHERE school_id = sqlc.arg(school_id)::bigint AND user_id = sqlc.arg(user_id)::bigint
  AND id = ANY(sqlc.arg(ids)::bigint[]) AND read_at IS NULL;

-- name: MarkAllNotificationsRead :execrows
UPDATE notifications SET read_at = now()
WHERE school_id = $1 AND user_id = $2 AND read_at IS NULL;

-- -- outbox --

-- name: InsertOutboxRow :one
INSERT INTO notification_outbox (school_id, event, user_id, channel, payload, status)
VALUES ($1, $2, $3, $4, $5, 'pending')
RETURNING *;

-- name: ListDueOutbox :many
-- Baris pending (belum pernah dicoba) ATAU failed (menunggu retry) yang
-- sudah jatuh tempo (next_retry_at NULL atau <= now()) — lihat backoff di
-- internal/notification/worker.go.
SELECT * FROM notification_outbox
WHERE status IN ('pending', 'failed') AND (next_retry_at IS NULL OR next_retry_at <= now())
ORDER BY created_at
LIMIT sqlc.arg(limit_count)::int;

-- name: MarkOutboxSent :exec
UPDATE notification_outbox SET status = 'sent', sent_at = sqlc.arg(sent_at)::timestamptz WHERE id = sqlc.arg(id)::bigint;

-- name: MarkOutboxRetry :exec
UPDATE notification_outbox
SET status = 'failed', attempts = sqlc.arg(attempts)::int, next_retry_at = sqlc.arg(next_retry_at)::timestamptz
WHERE id = sqlc.arg(id)::bigint;

-- name: MarkOutboxDead :exec
UPDATE notification_outbox SET status = 'dead', attempts = sqlc.arg(attempts)::int WHERE id = sqlc.arg(id)::bigint;

-- name: CountOutboxByChannel :one
-- Dipakai verifikasi e2e/test: jumlah baris outbox utk (school,event,channel).
SELECT COUNT(*) FROM notification_outbox WHERE school_id = $1 AND event = $2 AND channel = $3;

-- -- kontak (join read-only ke users, dimiliki modul identity) --

-- name: GetUserContact :one
SELECT id, phone, email FROM users WHERE id = $1;

-- -- push subscriptions --

-- name: UpsertPushSubscription :one
INSERT INTO push_subscriptions (school_id, user_id, endpoint, p256dh, auth)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (endpoint) DO UPDATE SET
    school_id = EXCLUDED.school_id, user_id = EXCLUDED.user_id,
    p256dh = EXCLUDED.p256dh, auth = EXCLUDED.auth
RETURNING *;

-- name: ListPushSubscriptionsForUser :many
SELECT * FROM push_subscriptions WHERE school_id = $1 AND user_id = $2;

-- name: DeletePushSubscriptionByEndpoint :exec
DELETE FROM push_subscriptions WHERE endpoint = $1;

-- name: DeletePushSubscriptionByUserEndpoint :execrows
DELETE FROM push_subscriptions WHERE school_id = sqlc.arg(school_id)::bigint AND user_id = sqlc.arg(user_id)::bigint AND endpoint = sqlc.arg(endpoint)::text;

-- -- platform_config (VAPID keys — lihat internal/notification/webpush.go) --

-- name: GetPlatformConfig :one
SELECT value FROM platform_config WHERE key = $1;

-- name: UpsertPlatformConfig :exec
INSERT INTO platform_config (key, value) VALUES ($1, $2)
ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value;
