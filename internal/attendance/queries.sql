-- Query modul attendance (sqlc). Semua query tenant-scoped WAJIB filter
-- school_id. Catatan: join read-only ke classes/enrollments/students
-- (dimiliki modul student) dan users (dimiliki modul identity) dipakai untuk
-- menampilkan nama rombel/siswa/pembuka sesi tanpa N+1 call lintas modul —
-- pola yang sama dengan internal/student/queries.sql (join ke users/teachers).
-- Semua TULIS ke tabel modul lain (classes, students, dst) tetap TIDAK PERNAH
-- dilakukan di sini. Object-level access check (ortu/siswa) tetap lewat
-- consumer-side interface StudentAccess (lihat service.go), bukan query SQL.

-- name: ListClassesForDate :many
SELECT
    c.id, c.name,
    COUNT(DISTINCT e.student_id) AS student_count,
    s.id AS session_id,
    s.status AS session_status,
    (SELECT COUNT(*) FROM attendance_records r WHERE r.session_id = s.id) AS marked_count
FROM classes c
LEFT JOIN enrollments e ON e.class_id = c.id
LEFT JOIN attendance_sessions s
    ON s.class_id = c.id AND s.date = sqlc.arg(date)::date AND s.type = 'daily' AND s.school_id = sqlc.arg(school_id)::bigint
WHERE c.school_id = sqlc.arg(school_id)::bigint AND c.academic_year_id = sqlc.arg(academic_year_id)::bigint
GROUP BY c.id, s.id, s.status
ORDER BY c.name;

-- name: GetClassMeta :one
SELECT id, name, academic_year_id FROM classes WHERE id = $1 AND school_id = $2;

-- name: ListClassStudents :many
SELECT s.id, s.name, s.nis
FROM students s
JOIN enrollments e ON e.student_id = s.id
WHERE e.class_id = $1 AND s.school_id = $2
ORDER BY s.name;

-- name: CreateSession :one
INSERT INTO attendance_sessions (school_id, academic_year_id, class_id, date, type, opened_by, status)
VALUES ($1, $2, $3, $4, 'daily', $5, 'open')
RETURNING *;

-- name: GetSessionByClassDate :one
SELECT * FROM attendance_sessions
WHERE school_id = $1 AND class_id = $2 AND date = $3 AND type = 'daily';

-- name: GetSessionByID :one
SELECT * FROM attendance_sessions WHERE id = $1 AND school_id = $2;

-- name: GetSessionDetail :one
SELECT s.*, c.name AS class_name, u.name AS opened_by_name
FROM attendance_sessions s
JOIN classes c ON c.id = s.class_id
JOIN users u ON u.id = s.opened_by
WHERE s.id = $1 AND s.school_id = $2;

-- name: ListSessionRecords :many
SELECT student_id, status, method, note, marked_at FROM attendance_records WHERE session_id = $1;

-- name: UpsertRecord :one
INSERT INTO attendance_records (school_id, session_id, student_id, status, method, marked_by, note)
VALUES ($1, $2, $3, $4, $5, $6, sqlc.narg(note)::text)
ON CONFLICT (session_id, student_id) DO UPDATE SET
    status     = EXCLUDED.status,
    method     = EXCLUDED.method,
    marked_by  = EXCLUDED.marked_by,
    marked_at  = now(),
    note       = EXCLUDED.note
RETURNING *;

-- name: FinalizeSession :one
UPDATE attendance_sessions SET status = 'finalized' WHERE id = $1 AND school_id = $2 RETURNING *;

-- name: CountRecordsForSession :one
SELECT COUNT(*) FROM attendance_records WHERE session_id = $1;

-- name: CountEnrolledForClass :one
SELECT COUNT(*) FROM enrollments WHERE class_id = $1;

-- name: SummaryByDate :many
SELECT
    c.id AS class_id,
    c.name AS class_name,
    COUNT(DISTINCT e.student_id) AS total,
    COUNT(*) FILTER (WHERE r.status = 'hadir')     AS hadir,
    COUNT(*) FILTER (WHERE r.status = 'terlambat') AS terlambat,
    COUNT(*) FILTER (WHERE r.status = 'izin')      AS izin,
    COUNT(*) FILTER (WHERE r.status = 'sakit')     AS sakit,
    COUNT(*) FILTER (WHERE r.status = 'alpa')      AS alpa,
    COALESCE(s.status, '')::text AS session_status
FROM classes c
LEFT JOIN enrollments e ON e.class_id = c.id
LEFT JOIN attendance_sessions s
    ON s.class_id = c.id AND s.date = sqlc.arg(date)::date AND s.type = 'daily' AND s.school_id = sqlc.arg(school_id)::bigint
LEFT JOIN attendance_records r ON r.session_id = s.id AND r.student_id = e.student_id
WHERE c.school_id = sqlc.arg(school_id)::bigint AND c.academic_year_id = sqlc.arg(academic_year_id)::bigint
  AND (sqlc.narg(class_id)::bigint IS NULL OR c.id = sqlc.narg(class_id)::bigint)
GROUP BY c.id, s.status
ORDER BY c.name;

-- name: StudentAttendanceHistory :many
SELECT s.date, r.status, r.note
FROM attendance_records r
JOIN attendance_sessions s ON s.id = r.session_id
WHERE r.student_id = sqlc.arg(student_id)::bigint AND r.school_id = sqlc.arg(school_id)::bigint
  AND s.date >= sqlc.arg(from_date)::date AND s.date <= sqlc.arg(to_date)::date
ORDER BY s.date DESC
LIMIT 200;

-- name: StudentAttendanceCounts :one
SELECT
    COUNT(*) FILTER (WHERE r.status = 'hadir')     AS hadir,
    COUNT(*) FILTER (WHERE r.status = 'terlambat') AS terlambat,
    COUNT(*) FILTER (WHERE r.status = 'izin')      AS izin,
    COUNT(*) FILTER (WHERE r.status = 'sakit')     AS sakit,
    COUNT(*) FILTER (WHERE r.status = 'alpa')      AS alpa
FROM attendance_records r
JOIN attendance_sessions s ON s.id = r.session_id
WHERE r.student_id = sqlc.arg(student_id)::bigint AND r.school_id = sqlc.arg(school_id)::bigint
  AND s.date >= sqlc.arg(from_date)::date AND s.date <= sqlc.arg(to_date)::date;

-- -- settings (dibaca langsung dari school_settings module='attendance' —
-- -- tabel & pola yang sama dipakai GET/PUT /api/settings/attendance lewat
-- -- tenant.SettingsService; ini hanya baca untuk kebutuhan aturan bisnis
-- -- internal, mis. edit_window_hours — lihat CLAUDE.md "satu pola settings").

-- name: GetAttendanceSettingsRaw :one
SELECT settings FROM school_settings WHERE school_id = $1 AND module = 'attendance';
