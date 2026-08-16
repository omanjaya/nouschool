-- Query modul counseling (sqlc). Semua query tenant-scoped WAJIB filter
-- school_id. Join read-only ke students/classes/enrollments/users (dimiliki
-- modul lain) dipakai untuk menampilkan nama tanpa N+1 call lintas modul —
-- pola yang sama dengan internal/discipline/queries.sql. Kelas diresolusi
-- dari enrollment PADA academic_year_id baris counseling itu sendiri (bukan
-- TA aktif sekarang) — sama keputusan dengan
-- discipline.GetStudentViolationByID (potret kelas saat sesi dicatat).

-- name: CreateCounseling :one
INSERT INTO counselings (school_id, academic_year_id, student_id, counselor_id, career_goals, problem_description, follow_up_plan, evidence, evidence_name, evidence_mime)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING *;

-- name: GetCounselingRow :one
SELECT co.id, co.school_id, co.academic_year_id, co.student_id, co.counselor_id,
       co.career_goals, co.problem_description, co.follow_up_plan,
       co.evidence, co.evidence_name, co.evidence_mime, co.created_at, co.updated_at,
       s.name AS student_name, s.nis AS student_nis, COALESCE(c.name, '') AS class_name,
       u.name AS counselor_name
FROM counselings co
JOIN students s ON s.id = co.student_id
JOIN users u ON u.id = co.counselor_id
LEFT JOIN enrollments e ON e.student_id = s.id
LEFT JOIN classes c ON c.id = e.class_id AND c.academic_year_id = co.academic_year_id
WHERE co.id = $1 AND co.school_id = $2;

-- name: ListCounselings :many
SELECT co.id, co.school_id, co.academic_year_id, co.student_id, co.counselor_id,
       co.career_goals, co.problem_description, co.follow_up_plan,
       co.evidence, co.evidence_name, co.evidence_mime, co.created_at, co.updated_at,
       s.name AS student_name, s.nis AS student_nis, COALESCE(c.name, '') AS class_name,
       u.name AS counselor_name
FROM counselings co
JOIN students s ON s.id = co.student_id
JOIN users u ON u.id = co.counselor_id
LEFT JOIN enrollments e ON e.student_id = s.id
LEFT JOIN classes c ON c.id = e.class_id AND c.academic_year_id = co.academic_year_id
WHERE co.school_id = sqlc.arg(school_id)::bigint
  AND (sqlc.narg(student_id)::bigint IS NULL OR co.student_id = sqlc.narg(student_id)::bigint)
ORDER BY co.created_at DESC, co.id DESC
LIMIT sqlc.arg(limit_rows)::int OFFSET sqlc.arg(offset_rows)::int;

-- name: CountCounselings :one
SELECT COUNT(*) FROM counselings co
WHERE co.school_id = sqlc.arg(school_id)::bigint
  AND (sqlc.narg(student_id)::bigint IS NULL OR co.student_id = sqlc.narg(student_id)::bigint);

-- name: UpdateCounseling :one
UPDATE counselings SET
    career_goals = $3,
    problem_description = $4,
    follow_up_plan = $5,
    updated_at = now()
WHERE id = $1 AND school_id = $2
RETURNING *;

-- name: UpdateCounselingEvidence :exec
UPDATE counselings SET evidence = $3, evidence_name = $4, evidence_mime = $5, updated_at = now()
WHERE id = $1 AND school_id = $2;

-- name: DeleteCounseling :execrows
DELETE FROM counselings WHERE id = $1 AND school_id = $2;

-- name: GetCounselingByID :one
SELECT * FROM counselings WHERE id = $1 AND school_id = $2;

-- -- validasi lintas modul (join read-only ke tabel bersama, pola yang sama
-- -- dengan internal/discipline/queries.sql "GetStudentBasic") --

-- name: GetStudentBasic :one
SELECT id, name, nis FROM students WHERE id = $1 AND school_id = $2;
