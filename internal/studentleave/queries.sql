-- name: CreateRequest :one
INSERT INTO student_leave_requests (
    school_id, academic_year_id, student_id, submitted_by, type,
    date_start, date_end, reason, attachment, attachment_name, attachment_mime, status
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, 'pending_homeroom')
RETURNING id;

-- name: GetRequestDetail :one
-- Satu baris lengkap (join siswa+kelas TA request+pengaju+pemutus) — sumber
-- tunggal shape Request (pola sama internal/discipline RecordDetail).
SELECT
    r.id, r.school_id, r.academic_year_id, r.student_id, r.submitted_by, r.type,
    r.date_start, r.date_end, r.reason, r.attachment, r.attachment_name, r.attachment_mime,
    r.status, r.letter_number, r.verify_token,
    r.homeroom_decided_by, r.homeroom_decided_at, r.homeroom_comment,
    r.bk_decided_by, r.bk_decided_at, r.bk_comment, r.created_at,
    s.name AS student_name, s.nis AS student_nis,
    COALESCE(c.name, '') AS class_name,
    su.name AS submitted_by_name,
    hd.name AS homeroom_decided_by_name,
    bd.name AS bk_decided_by_name
FROM student_leave_requests r
JOIN students s ON s.id = r.student_id
LEFT JOIN enrollments en ON en.student_id = r.student_id
LEFT JOIN classes c ON c.id = en.class_id AND c.academic_year_id = r.academic_year_id
JOIN users su ON su.id = r.submitted_by
LEFT JOIN users hd ON hd.id = r.homeroom_decided_by
LEFT JOIN users bd ON bd.id = r.bk_decided_by
WHERE r.id = $1 AND r.school_id = $2
LIMIT 1;

-- name: ListRequestsForStudents :many
-- scope=mine: siswa (satu student_id) / orang tua (banyak student_id, anak-anaknya).
SELECT
    r.id, r.school_id, r.academic_year_id, r.student_id, r.submitted_by, r.type,
    r.date_start, r.date_end, r.reason, r.attachment, r.attachment_name, r.attachment_mime,
    r.status, r.letter_number, r.verify_token,
    r.homeroom_decided_by, r.homeroom_decided_at, r.homeroom_comment,
    r.bk_decided_by, r.bk_decided_at, r.bk_comment, r.created_at,
    s.name AS student_name, s.nis AS student_nis,
    COALESCE(c.name, '') AS class_name,
    su.name AS submitted_by_name,
    hd.name AS homeroom_decided_by_name,
    bd.name AS bk_decided_by_name
FROM student_leave_requests r
JOIN students s ON s.id = r.student_id
LEFT JOIN enrollments en ON en.student_id = r.student_id
LEFT JOIN classes c ON c.id = en.class_id AND c.academic_year_id = r.academic_year_id
JOIN users su ON su.id = r.submitted_by
LEFT JOIN users hd ON hd.id = r.homeroom_decided_by
LEFT JOIN users bd ON bd.id = r.bk_decided_by
WHERE r.school_id = $1 AND r.student_id = ANY(sqlc.arg(student_ids)::bigint[])
  AND (sqlc.narg(status)::text IS NULL OR r.status = sqlc.narg(status))
ORDER BY r.created_at DESC;

-- name: ListRequestsByStatus :many
-- Dipakai queue pemegang flag (SEMUA siswa, bukan hanya rombel sendiri) &
-- scope=all admin (status_filter NULL -> SEMUA status).
SELECT
    r.id, r.school_id, r.academic_year_id, r.student_id, r.submitted_by, r.type,
    r.date_start, r.date_end, r.reason, r.attachment, r.attachment_name, r.attachment_mime,
    r.status, r.letter_number, r.verify_token,
    r.homeroom_decided_by, r.homeroom_decided_at, r.homeroom_comment,
    r.bk_decided_by, r.bk_decided_at, r.bk_comment, r.created_at,
    s.name AS student_name, s.nis AS student_nis,
    COALESCE(c.name, '') AS class_name,
    su.name AS submitted_by_name,
    hd.name AS homeroom_decided_by_name,
    bd.name AS bk_decided_by_name
FROM student_leave_requests r
JOIN students s ON s.id = r.student_id
LEFT JOIN enrollments en ON en.student_id = r.student_id
LEFT JOIN classes c ON c.id = en.class_id AND c.academic_year_id = r.academic_year_id
JOIN users su ON su.id = r.submitted_by
LEFT JOIN users hd ON hd.id = r.homeroom_decided_by
LEFT JOIN users bd ON bd.id = r.bk_decided_by
WHERE r.school_id = $1 AND (sqlc.narg(status)::text IS NULL OR r.status = sqlc.narg(status))
ORDER BY r.created_at DESC;

-- name: ListRequestsForHomeroomTeacher :many
-- Dipakai queue wali kelas: HANYA siswa di rombel perwaliannya sendiri (TA
-- SAMA dengan academic_year_id request), status tertentu.
SELECT
    r.id, r.school_id, r.academic_year_id, r.student_id, r.submitted_by, r.type,
    r.date_start, r.date_end, r.reason, r.attachment, r.attachment_name, r.attachment_mime,
    r.status, r.letter_number, r.verify_token,
    r.homeroom_decided_by, r.homeroom_decided_at, r.homeroom_comment,
    r.bk_decided_by, r.bk_decided_at, r.bk_comment, r.created_at,
    s.name AS student_name, s.nis AS student_nis,
    COALESCE(c.name, '') AS class_name,
    su.name AS submitted_by_name,
    hd.name AS homeroom_decided_by_name,
    bd.name AS bk_decided_by_name
FROM student_leave_requests r
JOIN students s ON s.id = r.student_id
JOIN enrollments en ON en.student_id = r.student_id
JOIN classes c ON c.id = en.class_id AND c.academic_year_id = r.academic_year_id
JOIN teachers t ON t.id = c.homeroom_teacher_id
JOIN users su ON su.id = r.submitted_by
LEFT JOIN users hd ON hd.id = r.homeroom_decided_by
LEFT JOIN users bd ON bd.id = r.bk_decided_by
WHERE r.school_id = $1 AND t.user_id = $2 AND r.status = $3
ORDER BY r.created_at DESC;

-- name: UpdateHomeroomDecision :execrows
UPDATE student_leave_requests SET
    status = $3, homeroom_decided_by = $4, homeroom_decided_at = now(), homeroom_comment = $5
WHERE id = $1 AND school_id = $2 AND status = 'pending_homeroom';

-- name: UpdateBKDecision :execrows
UPDATE student_leave_requests SET
    status = $3, bk_decided_by = $4, bk_decided_at = now(), bk_comment = $5,
    letter_number = $6, verify_token = $7
WHERE id = $1 AND school_id = $2 AND status = 'pending_bk';

-- name: CancelRequestExec :execrows
UPDATE student_leave_requests SET status = 'canceled'
WHERE id = $1 AND school_id = $2 AND submitted_by = $3
  AND status IN ('pending_homeroom', 'pending_bk');

-- name: NextLetterSeq :one
-- Counter atomik nomor surat "SI/{tahun}/{seq}" PER SEKOLAH PER TAHUN (pola
-- sama discipline_letter_number_counters).
INSERT INTO student_leave_number_counters (school_id, year, seq)
VALUES ($1, $2, 1)
ON CONFLICT (school_id, year) DO UPDATE SET seq = student_leave_number_counters.seq + 1
RETURNING seq;

-- name: GetByVerifyToken :one
-- Dipakai GET /api/public/leave-verify (PUBLIK, host tenant) — token GLOBAL
-- unik tapi TETAP difilter school_id (tenant saat ini) sesuai aturan
-- multi-tenant CLAUDE.md.
SELECT
    r.id, r.letter_number, r.type, r.date_start, r.date_end, r.bk_decided_at,
    s.name AS student_name, COALESCE(c.name, '') AS class_name
FROM student_leave_requests r
JOIN students s ON s.id = r.student_id
LEFT JOIN enrollments en ON en.student_id = r.student_id
LEFT JOIN classes c ON c.id = en.class_id AND c.academic_year_id = r.academic_year_id
WHERE r.verify_token = $1 AND r.school_id = $2 AND r.status = 'issued'
LIMIT 1;

-- -- lookup & validasi lintas modul (join read-only ke tabel bersama, pola
-- sama internal/discipline/queries.sql) --

-- name: StudentBasic :one
SELECT name, nis FROM students WHERE id = $1 AND school_id = $2;

-- name: HomeroomTeacherUserID :one
-- Dipakai otorisasi tahap wali kelas & resolusi notifikasi surat terbit.
SELECT t.user_id
FROM enrollments e
JOIN classes c ON c.id = e.class_id
JOIN teachers t ON t.id = c.homeroom_teacher_id
WHERE e.student_id = $1 AND c.school_id = $2 AND c.academic_year_id = $3
LIMIT 1;
