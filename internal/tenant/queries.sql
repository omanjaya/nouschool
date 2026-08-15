-- Query modul tenant (sqlc). Semua query tenant-scoped WAJIB filter school_id.

-- name: GetSchoolBySlug :one
SELECT * FROM schools WHERE slug = $1 AND status = 'active';

-- name: GetSchoolByCustomDomain :one
SELECT * FROM schools WHERE custom_domain = $1 AND status = 'active';

-- name: GetSchoolBySlugAny :one
-- Dipakai admin/bootstrap: tanpa filter status (untuk cek idempoten & manajemen).
SELECT * FROM schools WHERE slug = $1;

-- name: GetSchoolByID :one
SELECT * FROM schools WHERE id = $1;

-- name: CreateSchool :one
INSERT INTO schools (name, slug, timezone)
VALUES ($1, $2, $3)
RETURNING *;

-- name: UpdateSchool :one
UPDATE schools SET
    name          = COALESCE(sqlc.narg(name), name),
    custom_domain = COALESCE(sqlc.narg(custom_domain), custom_domain),
    timezone      = COALESCE(sqlc.narg(timezone), timezone),
    status        = COALESCE(sqlc.narg(status), status)
WHERE id = sqlc.arg(id)
RETURNING *;

-- name: ListSchools :many
SELECT * FROM schools ORDER BY created_at DESC;

-- name: CreateAcademicYear :one
INSERT INTO academic_years (school_id, name, starts_on, ends_on, is_active)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: ListAcademicYears :many
SELECT * FROM academic_years WHERE school_id = $1 ORDER BY starts_on DESC;

-- name: GetAcademicYear :one
SELECT * FROM academic_years WHERE id = $1 AND school_id = $2;

-- name: GetActiveAcademicYear :one
SELECT * FROM academic_years WHERE school_id = $1 AND is_active = true;

-- name: DeactivateAcademicYears :exec
UPDATE academic_years SET is_active = false WHERE school_id = $1 AND is_active = true;

-- name: ActivateAcademicYear :one
UPDATE academic_years SET is_active = true WHERE id = $1 AND school_id = $2
RETURNING *;

-- name: GetSchoolSetting :one
SELECT * FROM school_settings WHERE school_id = $1 AND module = $2;

-- name: UpsertSchoolSetting :one
INSERT INTO school_settings (school_id, module, settings, updated_by)
VALUES ($1, $2, $3, $4)
ON CONFLICT (school_id, module) DO UPDATE
    SET settings = EXCLUDED.settings, updated_by = EXCLUDED.updated_by, updated_at = now()
RETURNING *;
