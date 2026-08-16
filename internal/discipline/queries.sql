-- Query modul discipline (sqlc). Semua query tenant-scoped WAJIB filter
-- school_id. Join read-only ke students/classes/enrollments/violation_types/
-- users (dimiliki modul lain) dipakai untuk menampilkan nama tanpa N+1 call
-- lintas modul — pola yang sama dengan internal/leave/queries.sql (join ke
-- users) & internal/attendance/queries.sql (join ke classes/students).
--
-- Catatan lingkungan (lihat internal/discipline/disciplinedb/*.go): sqlc CLI
-- TIDAK tersedia di lingkungan build/dev/container ini (dikonfirmasi ulang
-- sesi ini, sama seperti dicatat Fase 12/13) — file *.sql.go di
-- internal/discipline/disciplinedb/ ditulis TANGAN mengikuti idiom PERSIS
-- sqlc v1.31.1 (lihat internal/leave/leavedb, internal/billing/billingdb
-- sebagai acuan), BUKAN hasil generate otomatis. sqlc.yaml tetap didaftarkan
-- supaya `make sqlc` regenerate identik begitu sqlc CLI tersedia.

-- -- violation_types --

-- name: CreateViolationType :one
INSERT INTO violation_types (school_id, name, points, category, active)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: UpdateViolationType :one
UPDATE violation_types SET
    name = COALESCE(sqlc.narg(name), name),
    points = COALESCE(sqlc.narg(points), points),
    category = COALESCE(sqlc.narg(category), category),
    active = COALESCE(sqlc.narg(active), active)
WHERE id = sqlc.arg(id) AND school_id = sqlc.arg(school_id)
RETURNING *;

-- name: GetViolationTypeByID :one
SELECT * FROM violation_types WHERE id = $1 AND school_id = $2;

-- name: ListViolationTypes :many
SELECT * FROM violation_types WHERE school_id = $1 ORDER BY name;

-- name: DeleteViolationType :execrows
DELETE FROM violation_types WHERE id = $1 AND school_id = $2;

-- name: CountViolationsByType :one
-- Dipakai guard DELETE /api/discipline/types/{id} (409 bila sudah dipakai
-- record — sarankan nonaktifkan active=false, bukan hapus).
SELECT COUNT(*) FROM student_violations WHERE school_id = $1 AND violation_type_id = $2;

-- -- student_violations --

-- name: CreateStudentViolation :one
INSERT INTO student_violations (school_id, academic_year_id, student_id, violation_type_id, attendance_session_id, noted_by, note, occurred_on)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: GetStudentViolationByID :one
-- Dipakai (a) DELETE (audit old value) DAN (b) memuat ulang RecordView
-- lengkap setelah CreateStudentViolation (pola yang sama dengan
-- leave.buildRequestView: insert lalu reload SEKALI dengan join lengkap,
-- bukan compose manual dari beberapa fetch terpisah). Kelas diresolusi dari
-- enrollment PADA TAHUN AJARAN baris ini dibuat (sv.academic_year_id) —
-- BEDA dari ListStudentViolations yang selalu memakai TA AKTIF sekarang.
SELECT sv.id, sv.school_id, sv.academic_year_id, sv.student_id, sv.violation_type_id,
       sv.attendance_session_id, sv.noted_by, sv.note, sv.occurred_on, sv.created_at,
       s.name AS student_name, s.nis AS student_nis, COALESCE(c.name, '') AS class_name,
       vt.name AS violation_name, vt.points AS violation_points, u.name AS noted_by_name
FROM student_violations sv
JOIN students s ON s.id = sv.student_id
JOIN violation_types vt ON vt.id = sv.violation_type_id
JOIN users u ON u.id = sv.noted_by
LEFT JOIN enrollments e ON e.student_id = s.id
LEFT JOIN classes c ON c.id = e.class_id AND c.academic_year_id = sv.academic_year_id
WHERE sv.id = $1 AND sv.school_id = $2;

-- name: DeleteStudentViolation :execrows
-- Menghapus CATATAN saja — surat yang sudah terbit TIDAK disentuh (snapshot
-- immutable, lihat discipline_warning_letters).
DELETE FROM student_violations WHERE id = $1 AND school_id = $2;

-- name: ListStudentViolations :many
-- GET /api/discipline/records?student_id=&class_id=&from=&to=&page= — class
-- SELALU diresolusi dari enrollment TAHUN AJARAN AKTIF (sqlc.arg(academic_year_id)),
-- BUKAN historis per tanggal pelanggaran (keputusan sendiri, sama pola dgn
-- modul lain yang menampilkan "kelas siswa saat ini").
SELECT
    sv.id, sv.occurred_on, sv.note, sv.attendance_session_id, sv.created_at,
    s.id AS student_id, s.name AS student_name, s.nis AS student_nis,
    COALESCE(c.name, '') AS class_name,
    vt.id AS violation_type_id, vt.name AS violation_name, vt.points AS violation_points,
    sv.noted_by, u.name AS noted_by_name
FROM student_violations sv
JOIN students s ON s.id = sv.student_id
JOIN violation_types vt ON vt.id = sv.violation_type_id
JOIN users u ON u.id = sv.noted_by
LEFT JOIN enrollments e ON e.student_id = s.id
LEFT JOIN classes c ON c.id = e.class_id AND c.academic_year_id = sqlc.arg(academic_year_id)::bigint
WHERE sv.school_id = sqlc.arg(school_id)::bigint
  AND (sqlc.narg(student_id)::bigint IS NULL OR sv.student_id = sqlc.narg(student_id)::bigint)
  AND (sqlc.narg(class_id)::bigint IS NULL OR c.id = sqlc.narg(class_id)::bigint)
  AND (sqlc.narg(from_date)::date IS NULL OR sv.occurred_on >= sqlc.narg(from_date)::date)
  AND (sqlc.narg(to_date)::date IS NULL OR sv.occurred_on <= sqlc.narg(to_date)::date)
ORDER BY sv.occurred_on DESC, sv.id DESC
LIMIT sqlc.arg(limit_rows)::int OFFSET sqlc.arg(offset_rows)::int;

-- name: CountStudentViolations :one
SELECT COUNT(*) FROM student_violations sv
LEFT JOIN enrollments e ON e.student_id = sv.student_id
LEFT JOIN classes c ON c.id = e.class_id AND c.academic_year_id = sqlc.arg(academic_year_id)::bigint
WHERE sv.school_id = sqlc.arg(school_id)::bigint
  AND (sqlc.narg(student_id)::bigint IS NULL OR sv.student_id = sqlc.narg(student_id)::bigint)
  AND (sqlc.narg(class_id)::bigint IS NULL OR c.id = sqlc.narg(class_id)::bigint)
  AND (sqlc.narg(from_date)::date IS NULL OR sv.occurred_on >= sqlc.narg(from_date)::date)
  AND (sqlc.narg(to_date)::date IS NULL OR sv.occurred_on <= sqlc.narg(to_date)::date);

-- name: ListViolationsForStudentYear :many
-- Dipakai GET /api/students/{id}/discipline DAN penyusunan snapshot surat
-- (Service.recordViolation) — SATU sumber, TIDAK diduplikasi. Total poin
-- dihitung MURNI di Go dari hasil query ini (jumlah baris kecil per siswa per
-- TA, tidak butuh query SUM terpisah).
SELECT sv.id, sv.occurred_on, sv.note, sv.attendance_session_id, sv.created_at,
       vt.id AS violation_type_id, vt.name AS violation_name, vt.points AS violation_points,
       sv.noted_by, u.name AS noted_by_name
FROM student_violations sv
JOIN violation_types vt ON vt.id = sv.violation_type_id
JOIN users u ON u.id = sv.noted_by
WHERE sv.school_id = $1 AND sv.academic_year_id = $2 AND sv.student_id = $3
ORDER BY sv.occurred_on, sv.id;

-- -- validasi lintas modul (join read-only ke tabel bersama, pola yang sama
-- -- dengan internal/attendance/queries.sql "SELECT id, name, nis FROM students") --

-- name: GetStudentBasic :one
SELECT id, name, nis FROM students WHERE id = $1 AND school_id = $2;

-- name: AttendanceSessionExists :one
SELECT id FROM attendance_sessions WHERE id = $1 AND school_id = $2;

-- name: GetStudentClassName :one
-- Dipakai GET /api/students/{id}/discipline (nama kelas siswa pada TA yang
-- direquest, dipasangkan dgn ListViolationsForStudentYear yang TIDAK
-- membawa info kelas).
SELECT c.name FROM enrollments e JOIN classes c ON c.id = e.class_id
WHERE e.student_id = $1 AND c.school_id = $2 AND c.academic_year_id = $3
LIMIT 1;

-- name: GetHomeroomTeacherUserID :one
-- Dipakai notifikasi "discipline.sp_issued" ke wali kelas (docs tugas: "ke
-- ortu + wali kelas"). Kelas diresolusi dari enrollment TAHUN AJARAN yang
-- SAMA dengan surat diterbitkan. pgx.ErrNoRows (siswa tidak terdaftar di
-- kelas manapun, atau kelasnya belum punya wali) ditangani Service sebagai
-- "tidak ada wali kelas untuk dinotifikasi" — bukan error.
SELECT t.user_id
FROM enrollments e
JOIN classes c ON c.id = e.class_id
JOIN teachers t ON t.id = c.homeroom_teacher_id
WHERE e.student_id = $1 AND c.school_id = $2 AND c.academic_year_id = $3
LIMIT 1;

-- -- discipline_sp_settings --

-- name: GetSPSettings :one
SELECT * FROM discipline_sp_settings WHERE school_id = $1 AND academic_year_id = $2;

-- name: UpsertSPSettings :one
INSERT INTO discipline_sp_settings (school_id, academic_year_id, sp1, sp2, sp3)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (school_id, academic_year_id) DO UPDATE SET sp1 = $3, sp2 = $4, sp3 = $5
RETURNING *;

-- -- nomor surat (counter atomik per sekolah+tahun) --

-- name: NextLetterSeq :one
INSERT INTO discipline_letter_number_counters (school_id, year, seq) VALUES ($1, $2, 1)
ON CONFLICT (school_id, year) DO UPDATE SET seq = discipline_letter_number_counters.seq + 1
RETURNING seq;

-- -- discipline_warning_letters --

-- name: CreateWarningLetter :one
INSERT INTO discipline_warning_letters (school_id, academic_year_id, student_id, level, number, points_snapshot, snapshot, created_by)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: GetWarningLetterByID :one
SELECT * FROM discipline_warning_letters WHERE id = $1 AND school_id = $2;

-- name: ListWarningLettersForStudentYear :many
SELECT * FROM discipline_warning_letters WHERE school_id = $1 AND academic_year_id = $2 AND student_id = $3 ORDER BY level;

-- name: ListWarningLetters :many
SELECT dwl.id, dwl.school_id, dwl.academic_year_id, dwl.student_id, dwl.level, dwl.number,
       dwl.points_snapshot, dwl.snapshot, dwl.created_by, dwl.created_at,
       s.name AS student_name, s.nis AS student_nis
FROM discipline_warning_letters dwl
JOIN students s ON s.id = dwl.student_id
WHERE dwl.school_id = sqlc.arg(school_id)::bigint
  AND (sqlc.narg(level)::int IS NULL OR dwl.level = sqlc.narg(level)::int)
ORDER BY dwl.created_at DESC
LIMIT sqlc.arg(limit_rows)::int OFFSET sqlc.arg(offset_rows)::int;

-- name: CountWarningLetters :one
SELECT COUNT(*) FROM discipline_warning_letters
WHERE school_id = sqlc.arg(school_id)::bigint
  AND (sqlc.narg(level)::int IS NULL OR level = sqlc.narg(level)::int);

-- -- rekap --

-- name: DisciplineSummary :many
-- GET /api/discipline/summary?class_id=&from=&to= — kelas SELALU dari
-- enrollment TA AKTIF (sama keputusan dgn ListStudentViolations).
SELECT s.id AS student_id, s.name AS student_name, s.nis AS student_nis,
       COALESCE(c.name, '') AS class_name,
       COALESCE(SUM(vt.points), 0)::bigint AS total_points,
       COUNT(sv.id) AS record_count,
       MAX(sv.occurred_on) AS last_at
FROM student_violations sv
JOIN students s ON s.id = sv.student_id
JOIN violation_types vt ON vt.id = sv.violation_type_id
LEFT JOIN enrollments e ON e.student_id = s.id
LEFT JOIN classes c ON c.id = e.class_id AND c.academic_year_id = sqlc.arg(academic_year_id)::bigint
WHERE sv.school_id = sqlc.arg(school_id)::bigint
  AND (sqlc.narg(class_id)::bigint IS NULL OR c.id = sqlc.narg(class_id)::bigint)
  AND (sqlc.narg(from_date)::date IS NULL OR sv.occurred_on >= sqlc.narg(from_date)::date)
  AND (sqlc.narg(to_date)::date IS NULL OR sv.occurred_on <= sqlc.narg(to_date)::date)
GROUP BY s.id, s.name, s.nis, c.name
ORDER BY total_points DESC, s.name;
