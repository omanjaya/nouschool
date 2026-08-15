-- Query modul announcement (sqlc). Semua query tenant-scoped WAJIB filter
-- school_id (lihat CLAUDE.md aturan multi-tenant).

-- name: InsertAnnouncement :one
INSERT INTO announcements (school_id, title, body, starts_at, ends_at, created_by)
VALUES (
    sqlc.arg(school_id)::bigint,
    sqlc.arg(title)::text,
    sqlc.arg(body)::text,
    sqlc.arg(starts_at)::date,
    sqlc.arg(ends_at)::date,
    sqlc.arg(created_by)::bigint
)
RETURNING *;

-- name: GetAnnouncementByID :one
SELECT * FROM announcements WHERE id = $1 AND school_id = $2;

-- name: ListAnnouncements :many
SELECT * FROM announcements WHERE school_id = $1 ORDER BY starts_at DESC, id DESC;

-- name: ListActiveAnnouncements :many
SELECT * FROM announcements
WHERE school_id = $1 AND starts_at <= $2 AND ends_at >= $2
ORDER BY starts_at DESC, id DESC;

-- name: UpdateAnnouncement :one
UPDATE announcements
SET title = $3, body = $4, starts_at = $5, ends_at = $6
WHERE id = $1 AND school_id = $2
RETURNING *;

-- name: DeleteAnnouncement :execrows
DELETE FROM announcements WHERE id = $1 AND school_id = $2;
