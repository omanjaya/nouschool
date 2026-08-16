-- name: CreateLateArrival :one
-- Scan PERTAMA hari itu (docs tugas): create LANGSUNG status
-- 'pending_leadership' dengan duty_by/duty_at terisi (tahap piket sudah
-- lunas saat baris dibuat, TIDAK ADA baris berstatus pending_duty_teacher
-- yang benar-benar tersimpan).
INSERT INTO student_late_arrivals (
    school_id, academic_year_id, student_id, arrived_at, late_count, action, status, duty_by, duty_at
) VALUES (
    $1, $2, $3, sqlc.arg(arrived_at)::timestamptz, $4, $5, 'pending_leadership', $6, sqlc.arg(arrived_at)::timestamptz
)
RETURNING id;

-- name: CountLateArrivalsForStudentYear :one
SELECT count(*) FROM student_late_arrivals
WHERE school_id = $1 AND academic_year_id = $2 AND student_id = $3;

-- name: GetLateArrivalDetail :one
SELECT
    la.id, la.school_id, la.academic_year_id, la.student_id, la.arrived_at, la.late_count, la.action, la.status,
    la.duty_by, la.duty_at, la.leadership_by, la.leadership_at, la.class_by, la.class_at, la.created_at,
    s.name AS student_name, s.nis AS student_nis, COALESCE(c.name, '') AS class_name,
    du.name AS duty_by_name, lu.name AS leadership_by_name, cu.name AS class_by_name
FROM student_late_arrivals la
JOIN students s ON s.id = la.student_id
LEFT JOIN enrollments en ON en.student_id = la.student_id
LEFT JOIN classes c ON c.id = en.class_id AND c.academic_year_id = la.academic_year_id
LEFT JOIN users du ON du.id = la.duty_by
LEFT JOIN users lu ON lu.id = la.leadership_by
LEFT JOIN users cu ON cu.id = la.class_by
WHERE la.id = $1 AND la.school_id = $2
LIMIT 1;

-- name: GetTodayLateArrivalForStudent :one
-- Rentang UTC "hari ini lokal sekolah" diteruskan service (docs tugas: maks
-- 1 record per siswa per hari).
SELECT
    la.id, la.school_id, la.academic_year_id, la.student_id, la.arrived_at, la.late_count, la.action, la.status,
    la.duty_by, la.duty_at, la.leadership_by, la.leadership_at, la.class_by, la.class_at, la.created_at,
    s.name AS student_name, s.nis AS student_nis, COALESCE(c.name, '') AS class_name,
    du.name AS duty_by_name, lu.name AS leadership_by_name, cu.name AS class_by_name
FROM student_late_arrivals la
JOIN students s ON s.id = la.student_id
LEFT JOIN enrollments en ON en.student_id = la.student_id
LEFT JOIN classes c ON c.id = en.class_id AND c.academic_year_id = la.academic_year_id
LEFT JOIN users du ON du.id = la.duty_by
LEFT JOIN users lu ON lu.id = la.leadership_by
LEFT JOIN users cu ON cu.id = la.class_by
WHERE la.school_id = $1 AND la.student_id = $2
  AND la.arrived_at >= sqlc.arg(from_at)::timestamptz AND la.arrived_at < sqlc.arg(to_at)::timestamptz
ORDER BY la.arrived_at DESC
LIMIT 1;

-- name: ListLateArrivalsForStudents :many
SELECT
    la.id, la.school_id, la.academic_year_id, la.student_id, la.arrived_at, la.late_count, la.action, la.status,
    la.duty_by, la.duty_at, la.leadership_by, la.leadership_at, la.class_by, la.class_at, la.created_at,
    s.name AS student_name, s.nis AS student_nis, COALESCE(c.name, '') AS class_name,
    du.name AS duty_by_name, lu.name AS leadership_by_name, cu.name AS class_by_name
FROM student_late_arrivals la
JOIN students s ON s.id = la.student_id
LEFT JOIN enrollments en ON en.student_id = la.student_id
LEFT JOIN classes c ON c.id = en.class_id AND c.academic_year_id = la.academic_year_id
LEFT JOIN users du ON du.id = la.duty_by
LEFT JOIN users lu ON lu.id = la.leadership_by
LEFT JOIN users cu ON cu.id = la.class_by
WHERE la.school_id = $1 AND la.student_id = ANY(sqlc.arg(student_ids)::bigint[])
  AND (sqlc.narg(from_at)::timestamptz IS NULL OR la.arrived_at >= sqlc.narg(from_at))
  AND (sqlc.narg(to_at)::timestamptz IS NULL OR la.arrived_at < sqlc.narg(to_at))
ORDER BY la.arrived_at DESC;

-- name: ListLateArrivalsToday :many
SELECT
    la.id, la.school_id, la.academic_year_id, la.student_id, la.arrived_at, la.late_count, la.action, la.status,
    la.duty_by, la.duty_at, la.leadership_by, la.leadership_at, la.class_by, la.class_at, la.created_at,
    s.name AS student_name, s.nis AS student_nis, COALESCE(c.name, '') AS class_name,
    du.name AS duty_by_name, lu.name AS leadership_by_name, cu.name AS class_by_name
FROM student_late_arrivals la
JOIN students s ON s.id = la.student_id
LEFT JOIN enrollments en ON en.student_id = la.student_id
LEFT JOIN classes c ON c.id = en.class_id AND c.academic_year_id = la.academic_year_id
LEFT JOIN users du ON du.id = la.duty_by
LEFT JOIN users lu ON lu.id = la.leadership_by
LEFT JOIN users cu ON cu.id = la.class_by
WHERE la.school_id = $1 AND la.arrived_at >= sqlc.arg(from_at)::timestamptz AND la.arrived_at < sqlc.arg(to_at)::timestamptz
ORDER BY la.arrived_at DESC;

-- name: ListLateArrivalsAll :many
SELECT
    la.id, la.school_id, la.academic_year_id, la.student_id, la.arrived_at, la.late_count, la.action, la.status,
    la.duty_by, la.duty_at, la.leadership_by, la.leadership_at, la.class_by, la.class_at, la.created_at,
    s.name AS student_name, s.nis AS student_nis, COALESCE(c.name, '') AS class_name,
    du.name AS duty_by_name, lu.name AS leadership_by_name, cu.name AS class_by_name
FROM student_late_arrivals la
JOIN students s ON s.id = la.student_id
LEFT JOIN enrollments en ON en.student_id = la.student_id
LEFT JOIN classes c ON c.id = en.class_id AND c.academic_year_id = la.academic_year_id
LEFT JOIN users du ON du.id = la.duty_by
LEFT JOIN users lu ON lu.id = la.leadership_by
LEFT JOIN users cu ON cu.id = la.class_by
WHERE la.school_id = $1
  AND (sqlc.narg(from_at)::timestamptz IS NULL OR la.arrived_at >= sqlc.narg(from_at))
  AND (sqlc.narg(to_at)::timestamptz IS NULL OR la.arrived_at < sqlc.narg(to_at))
ORDER BY la.arrived_at DESC;

-- name: SummaryLateArrivalsByMonth :many
SELECT s.id AS student_id, s.name AS student_name, s.nis AS student_nis, COALESCE(c.name, '') AS class_name,
       count(la.id) AS cnt
FROM student_late_arrivals la
JOIN students s ON s.id = la.student_id
LEFT JOIN enrollments en ON en.student_id = la.student_id
LEFT JOIN classes c ON c.id = en.class_id AND c.academic_year_id = la.academic_year_id
WHERE la.school_id = $1 AND la.arrived_at >= sqlc.arg(from_at)::timestamptz AND la.arrived_at < sqlc.arg(to_at)::timestamptz
GROUP BY s.id, s.name, s.nis, c.name
ORDER BY cnt DESC, s.name ASC;

-- name: UpdateLateArrivalLeadershipStage :execrows
UPDATE student_late_arrivals SET status = 'pending_class_teacher', leadership_by = $3, leadership_at = now()
WHERE id = $1 AND school_id = $2 AND status = 'pending_leadership';

-- name: UpdateLateArrivalClassStage :execrows
UPDATE student_late_arrivals SET status = 'completed', class_by = $3, class_at = now()
WHERE id = $1 AND school_id = $2 AND status = 'pending_class_teacher';
