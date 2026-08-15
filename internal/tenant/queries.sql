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

-- -- custom domain (Fase 11, docs/01-tenant.md "Custom domain & Caddy") --

-- name: ExistsDomainUsedByOtherSchool :one
-- Unik LINTAS sekolah, menyilang custom_domain (aktif) dan pending_domain
-- (menunggu verifikasi) — partial unique index per kolom tidak bisa
-- menyilang dua kolom berbeda, jadi dicek di sini sebelum SetPendingDomain.
SELECT EXISTS (
    SELECT 1 FROM schools
    WHERE id != sqlc.arg(exclude_id)::bigint
      AND (custom_domain = sqlc.arg(domain)::text OR pending_domain = sqlc.arg(domain)::text)
) AS used;

-- name: SetPendingDomain :one
UPDATE schools SET pending_domain = sqlc.arg(domain)::text WHERE id = sqlc.arg(id)::bigint
RETURNING *;

-- name: VerifyPendingDomain :one
-- Verifikasi sukses: pending_domain pindah jadi custom_domain aktif.
UPDATE schools SET custom_domain = pending_domain, pending_domain = NULL
WHERE id = $1
RETURNING *;

-- name: ClearDomain :one
-- DELETE /api/custom-domain: hapus baik custom_domain aktif maupun yang
-- masih pending sekaligus (satu aksi "lepas domain sendiri").
UPDATE schools SET custom_domain = NULL, pending_domain = NULL
WHERE id = $1
RETURNING *;

-- -- registrasi minat sekolah (Fase 11, landing page host platform) --
-- interest_leads TANPA school_id (platform-level, calon sekolah belum jadi
-- tenant) — lihat catatan migrations/00012_branding_domain_interest.sql.

-- name: CreateInterestLead :one
INSERT INTO interest_leads (school_name, contact_name, phone, email, note)
VALUES (sqlc.arg(school_name)::text, sqlc.arg(contact_name)::text, sqlc.arg(phone)::text,
        sqlc.narg(email)::text, sqlc.narg(note)::text)
RETURNING *;

-- name: ListInterestLeads :many
SELECT * FROM interest_leads ORDER BY created_at DESC;
