-- Query modul tenant (sqlc). Semua query tenant-scoped WAJIB filter school_id.

-- name: GetSchoolBySlug :one
SELECT * FROM schools WHERE slug = $1 AND status = 'active';

-- name: GetSchoolByCustomDomain :one
SELECT * FROM schools WHERE custom_domain = $1 AND status = 'active';

-- name: CreateSchool :one
INSERT INTO schools (name, slug, timezone)
VALUES ($1, $2, $3)
RETURNING *;

-- name: ListSchools :many
SELECT * FROM schools ORDER BY created_at DESC;
