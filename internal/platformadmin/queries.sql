-- Query modul platformadmin (sqlc). SEMUA query di sini adalah agregasi
-- READ-ONLY lintas modul (schools/subscriptions/invoices/students/
-- interest_leads/notification_outbox/sessions/attendance_sessions/
-- teaching_journals) untuk panel super admin (fase 13, docs/11-superadmin.md
-- P1/P3/P4) — DIPERBOLEHKAN secara eksplisit oleh instruksi tugas untuk modul
-- agregator (preseden dashboard/tv-board), lihat catatan desain lengkap di
-- package doc (service.go). Mutasi HANYA menyentuh notification_outbox
-- (status flag sederhana, TIDAK menduplikasi logika worker) — lihat
-- RetryOutboxRow/RetryAllOutbox di bawah.

-- -- P1: GET /api/admin/overview --

-- name: ListSchoolsOverview :many
-- Dipakai Service.Overview membangun bucket schools_active/grace/readonly/
-- suspended (fallback readonly bila tanpa subscription, suspended dari
-- schools.status) DAN daftar perlu-perhatian grace/readonly — lihat
-- effectiveStatus di service.go.
SELECT
    s.id AS school_id, s.name, s.status AS school_status,
    sub.status AS sub_status, sub.ends_on AS sub_ends_on
FROM schools s
LEFT JOIN subscriptions sub ON sub.school_id = s.id
ORDER BY s.name;

-- name: ListInvoicesAwaitingVerification :many
SELECT i.id AS invoice_id, i.number, i.school_id, s.name AS school_name, i.amount
FROM invoices i
JOIN schools s ON s.id = i.school_id
WHERE i.status = 'awaiting_verification'
ORDER BY i.created_at;

-- name: ListSchoolsWithoutActiveYear :many
SELECT s.id AS school_id, s.name
FROM schools s
WHERE NOT EXISTS (
    SELECT 1 FROM academic_years ay WHERE ay.school_id = s.id AND ay.is_active
)
ORDER BY s.name;

-- name: ListSchoolsWithDeadOutbox :many
SELECT o.school_id, s.name AS school_name, COUNT(*) AS dead_count
FROM notification_outbox o
JOIN schools s ON s.id = o.school_id
WHERE o.status = 'dead'
GROUP BY o.school_id, s.name
ORDER BY COUNT(*) DESC;

-- name: ListSchoolLastActivity :many
-- Urut last_login ASC NULLS FIRST (paling tidak aktif dulu, docs P1).
SELECT
    s.id AS school_id, s.name,
    (SELECT MAX(sess.created_at) FROM sessions sess WHERE sess.school_id = s.id)::timestamptz AS last_login,
    (SELECT MAX(a.created_at) FROM attendance_sessions a WHERE a.school_id = s.id)::timestamptz AS last_attendance_session
FROM schools s
ORDER BY last_login ASC NULLS FIRST
LIMIT sqlc.arg(limit_count)::int;

-- name: CountTotalActiveStudents :one
-- Lintas SEMUA sekolah (agregat platform, bukan data per-siswa — docs/11
-- aturan #3).
SELECT COUNT(*) FROM students WHERE status = 'active';

-- name: SumRevenueInRange :one
SELECT COALESCE(SUM(amount), 0)::bigint FROM invoices
WHERE status = 'paid' AND paid_at >= sqlc.arg(from_at)::timestamptz AND paid_at < sqlc.arg(to_at)::timestamptz;

-- name: CountLeadsSince :one
SELECT COUNT(*) FROM interest_leads WHERE created_at >= sqlc.arg(since)::timestamptz;

-- -- P3: GET /api/admin/schools/{id}/stats --

-- name: CountTeachersForSchool :one
SELECT COUNT(*) FROM teachers WHERE school_id = $1;

-- name: CountActiveStudentsForSchool :one
SELECT COUNT(*) FROM students WHERE school_id = $1 AND status = 'active';

-- name: CountClassesForSchool :one
SELECT COUNT(*) FROM classes WHERE school_id = $1;

-- name: CountAttendanceSessionsSince :one
SELECT COUNT(*) FROM attendance_sessions
WHERE school_id = sqlc.arg(school_id)::bigint AND date >= sqlc.arg(since_date)::date;

-- name: CountJournalsSince :one
SELECT COUNT(*) FROM teaching_journals
WHERE school_id = sqlc.arg(school_id)::bigint AND date >= sqlc.arg(since_date)::date;

-- name: CountOutboxByStatusSince :many
SELECT status, COUNT(*) AS n FROM notification_outbox
WHERE school_id = sqlc.arg(school_id)::bigint AND created_at >= sqlc.arg(since)::timestamptz
GROUP BY status;

-- name: ListLastLoginsByRole :many
SELECT role, MAX(created_at)::timestamptz AS last_login
FROM sessions
WHERE school_id = sqlc.arg(school_id)::bigint
GROUP BY role
ORDER BY role;

-- -- P4.4: outbox global --

-- name: ListOutboxAdmin :many
-- status kosong ("") = semua status; school_id 0 = semua sekolah.
SELECT o.id, o.school_id, s.name AS school_name, o.event, o.channel, u.name AS user_name,
       o.status, o.attempts, o.next_retry_at, o.created_at
FROM notification_outbox o
JOIN schools s ON s.id = o.school_id
JOIN users u ON u.id = o.user_id
WHERE (sqlc.arg(status)::text = '' OR o.status = sqlc.arg(status)::text)
  AND (sqlc.arg(school_id)::bigint = 0 OR o.school_id = sqlc.arg(school_id)::bigint)
ORDER BY o.created_at DESC
LIMIT sqlc.arg(limit_count)::int OFFSET sqlc.arg(offset_count)::int;

-- name: CountOutboxAdmin :one
SELECT COUNT(*) FROM notification_outbox o
WHERE (sqlc.arg(status)::text = '' OR o.status = sqlc.arg(status)::text)
  AND (sqlc.arg(school_id)::bigint = 0 OR o.school_id = sqlc.arg(school_id)::bigint);

-- name: GetOutboxByID :one
SELECT * FROM notification_outbox WHERE id = $1;

-- name: RetryOutboxRow :exec
-- attempts SENGAJA tidak diubah (task: "attempts tetap") — next_retry_at=now
-- supaya ListDueOutbox (worker existing, internal/notification) langsung
-- mengambilnya di poll berikutnya (status IN ('pending','failed') DAN
-- next_retry_at <= now()).
UPDATE notification_outbox SET status = 'pending', next_retry_at = sqlc.arg(now)::timestamptz
WHERE id = sqlc.arg(id)::bigint;

-- name: RetryAllOutbox :execrows
UPDATE notification_outbox SET status = 'pending', next_retry_at = sqlc.arg(now)::timestamptz
WHERE status = sqlc.arg(status)::text
  AND (sqlc.arg(school_id)::bigint = 0 OR school_id = sqlc.arg(school_id)::bigint);

-- -- P2.2 (fase 13 Gelombang 2): GET /api/admin/schools/{id}/onboarding --

-- name: SchoolExistsByID :one
SELECT EXISTS(SELECT 1 FROM schools WHERE id = sqlc.arg(school_id)::bigint);

-- name: SchoolOnboardingStatus :one
-- Satu baris agregasi lintas modul (tenant/identity/billing/student/schedule)
-- — DIPERBOLEHKAN eksplisit utk modul agregator (lihat catatan desain
-- package doc), sama pola dengan ListSchoolsOverview di atas.
SELECT
    EXISTS(SELECT 1 FROM academic_years WHERE school_id = sqlc.arg(school_id)::bigint AND is_active) AS has_active_year,
    EXISTS(SELECT 1 FROM memberships WHERE school_id = sqlc.arg(school_id)::bigint AND role = 'admin_sekolah' AND status = 'active') AS has_admin,
    EXISTS(SELECT 1 FROM subscriptions WHERE school_id = sqlc.arg(school_id)::bigint AND status = 'active') AS has_subscription_active,
    EXISTS(SELECT 1 FROM students WHERE school_id = sqlc.arg(school_id)::bigint AND status = 'active') AS has_students,
    EXISTS(SELECT 1 FROM schedule_slots WHERE school_id = sqlc.arg(school_id)::bigint) AS has_schedule;

-- -- P5 (fase 13 Gelombang 2): platform_announcements, docs/11-superadmin.md
-- "Pengumuman platform" — TIDAK tenant-scoped (satu baris = tampil di SEMUA
-- sekolah), jadi TIDAK ada filter school_id (beda dari announcements
-- per-sekolah di internal/announcement).

-- name: InsertPlatformAnnouncement :one
INSERT INTO platform_announcements (title, body, starts_at, ends_at, created_by)
VALUES (sqlc.arg(title)::text, sqlc.arg(body)::text, sqlc.arg(starts_at)::date, sqlc.arg(ends_at)::date, sqlc.arg(created_by)::bigint)
RETURNING *;

-- name: ListPlatformAnnouncements :many
SELECT * FROM platform_announcements ORDER BY starts_at DESC, id DESC;

-- name: ListActivePlatformAnnouncements :many
SELECT * FROM platform_announcements WHERE starts_at <= sqlc.arg(on_date)::date AND ends_at >= sqlc.arg(on_date)::date
ORDER BY starts_at DESC, id DESC;

-- name: UpdatePlatformAnnouncement :one
UPDATE platform_announcements
SET title = sqlc.arg(title)::text, body = sqlc.arg(body)::text, starts_at = sqlc.arg(starts_at)::date, ends_at = sqlc.arg(ends_at)::date
WHERE id = sqlc.arg(id)::bigint
RETURNING *;

-- name: DeletePlatformAnnouncement :execrows
DELETE FROM platform_announcements WHERE id = sqlc.arg(id)::bigint;
