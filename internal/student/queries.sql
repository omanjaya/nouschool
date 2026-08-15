-- Query modul student (sqlc). Semua query tenant-scoped WAJIB filter school_id.
-- Catatan: join read-only ke users (dimiliki modul identity) dipakai untuk
-- menampilkan nama/email guru/wali kelas tanpa N+1 call lintas modul. Semua
-- TULIS ke users/memberships/sessions/invitations tetap lewat IdentityGateway
-- (consumer-side interface), tidak pernah lewat query di sini.

-- name: ListStudents :many
SELECT s.id, s.nis, s.nisn, s.name, s.gender, s.birth_date, s.status,
       c.id AS class_id, c.name AS class_name
FROM students s
LEFT JOIN enrollments e ON e.student_id = s.id
LEFT JOIN classes c ON c.id = e.class_id AND c.academic_year_id = sqlc.arg(academic_year_id)::bigint
WHERE s.school_id = sqlc.arg(school_id)::bigint
  AND (sqlc.narg(q)::text IS NULL OR s.name ILIKE '%' || sqlc.narg(q)::text || '%' OR s.nis ILIKE '%' || sqlc.narg(q)::text || '%')
  AND (sqlc.narg(class_id)::bigint IS NULL OR EXISTS (
        SELECT 1 FROM enrollments e2 WHERE e2.student_id = s.id AND e2.class_id = sqlc.narg(class_id)::bigint
      ))
ORDER BY s.name
LIMIT sqlc.arg(limit_)::bigint OFFSET sqlc.arg(offset_)::bigint;

-- name: CountStudents :one
SELECT COUNT(*) FROM students s
WHERE s.school_id = sqlc.arg(school_id)::bigint
  AND (sqlc.narg(q)::text IS NULL OR s.name ILIKE '%' || sqlc.narg(q)::text || '%' OR s.nis ILIKE '%' || sqlc.narg(q)::text || '%')
  AND (sqlc.narg(class_id)::bigint IS NULL OR EXISTS (
        SELECT 1 FROM enrollments e2 WHERE e2.student_id = s.id AND e2.class_id = sqlc.narg(class_id)::bigint
      ));

-- name: GetStudentByID :one
SELECT s.*,
       c.id AS class_id, c.name AS class_name
FROM students s
LEFT JOIN enrollments e ON e.student_id = s.id
LEFT JOIN classes c ON c.id = e.class_id AND c.academic_year_id = sqlc.arg(academic_year_id)::bigint
WHERE s.id = sqlc.arg(id)::bigint AND s.school_id = sqlc.arg(school_id)::bigint;

-- name: GetStudentByNIS :one
SELECT * FROM students WHERE school_id = $1 AND nis = $2;

-- name: GetStudentByUserID :one
SELECT s.*,
       c.id AS class_id, c.name AS class_name
FROM students s
LEFT JOIN enrollments e ON e.student_id = s.id
LEFT JOIN classes c ON c.id = e.class_id AND c.academic_year_id = sqlc.arg(academic_year_id)::bigint
WHERE s.school_id = sqlc.arg(school_id)::bigint AND s.user_id = sqlc.arg(user_id)::bigint;

-- name: ListStudentsByIDs :many
SELECT * FROM students WHERE school_id = $1 AND id = ANY(sqlc.arg(ids)::bigint[]);

-- name: CreateStudent :one
INSERT INTO students (school_id, nis, nisn, name, gender, birth_date, status)
VALUES ($1, $2, $3, $4, $5, $6, 'active')
RETURNING *;

-- name: UpdateStudent :one
UPDATE students SET
    nis        = COALESCE(sqlc.narg(nis), nis),
    nisn       = COALESCE(sqlc.narg(nisn), nisn),
    name       = COALESCE(sqlc.narg(name), name),
    gender     = COALESCE(sqlc.narg(gender), gender),
    birth_date = COALESCE(sqlc.narg(birth_date), birth_date),
    status     = COALESCE(sqlc.narg(status), status)
WHERE id = sqlc.arg(id) AND school_id = sqlc.arg(school_id)
RETURNING *;

-- name: SetStudentUserID :exec
UPDATE students SET user_id = $2 WHERE id = $1;

-- -- classes --

-- name: CreateClass :one
INSERT INTO classes (school_id, academic_year_id, name, grade, major, homeroom_teacher_id)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: UpdateClass :one
UPDATE classes SET
    name                = COALESCE(sqlc.narg(name), name),
    grade               = COALESCE(sqlc.narg(grade), grade),
    major               = COALESCE(sqlc.narg(major), major),
    homeroom_teacher_id = COALESCE(sqlc.narg(homeroom_teacher_id), homeroom_teacher_id)
WHERE id = sqlc.arg(id) AND school_id = sqlc.arg(school_id)
RETURNING *;

-- name: GetClassByID :one
SELECT * FROM classes WHERE id = $1 AND school_id = $2;

-- name: GetClassByNameYear :one
SELECT * FROM classes WHERE school_id = $1 AND academic_year_id = $2 AND name = $3;

-- name: ListClasses :many
SELECT c.id, c.school_id, c.academic_year_id, c.name, c.grade, c.major, c.homeroom_teacher_id,
       t.id AS teacher_row_id, u.id AS teacher_user_id, u.name AS teacher_name,
       COUNT(e.id) AS student_count
FROM classes c
LEFT JOIN teachers t ON t.id = c.homeroom_teacher_id
LEFT JOIN users u ON u.id = t.user_id
LEFT JOIN enrollments e ON e.class_id = c.id
WHERE c.school_id = $1 AND c.academic_year_id = $2
GROUP BY c.id, t.id, u.id, u.name
ORDER BY c.name;

-- -- enrollments --

-- name: UpsertEnrollment :one
INSERT INTO enrollments (school_id, student_id, class_id)
VALUES ($1, $2, $3)
ON CONFLICT (student_id, class_id) DO UPDATE SET class_id = EXCLUDED.class_id
RETURNING *;

-- name: GetEnrollmentInYear :one
SELECT e.* FROM enrollments e
JOIN classes c ON c.id = e.class_id
WHERE e.student_id = $1 AND c.academic_year_id = $2;

-- name: DeleteEnrollment :exec
DELETE FROM enrollments WHERE id = $1;

-- name: DeleteEnrollmentByStudentClass :exec
DELETE FROM enrollments WHERE student_id = $1 AND class_id = $2;

-- -- guardians --

-- name: CreateGuardian :one
INSERT INTO guardians (school_id, user_id, student_id, relation)
VALUES ($1, $2, $3, $4)
ON CONFLICT (user_id, student_id) DO UPDATE SET relation = EXCLUDED.relation
RETURNING *;

-- name: GetGuardianByUserStudent :one
SELECT * FROM guardians WHERE user_id = $1 AND student_id = $2;

-- name: CountGuardiansForStudent :one
SELECT COUNT(*) FROM guardians WHERE student_id = $1;

-- name: ListGuardianUserIDsForStudent :many
-- Dipakai interface publik Service.GuardianUserIDs (konsumsi modul
-- notification/attendance fase 9 — resolve penerima notifikasi absensi).
SELECT g.user_id
FROM guardians g
JOIN students s ON s.id = g.student_id
WHERE g.student_id = sqlc.arg(student_id)::bigint AND s.school_id = sqlc.arg(school_id)::bigint;

-- name: ListChildrenForGuardian :many
SELECT s.id, s.name, c.name AS class_name
FROM guardians g
JOIN students s ON s.id = g.student_id
LEFT JOIN enrollments e ON e.student_id = s.id
LEFT JOIN classes c ON c.id = e.class_id AND c.academic_year_id = sqlc.arg(academic_year_id)::bigint
WHERE g.user_id = sqlc.arg(user_id)::bigint AND s.school_id = sqlc.arg(school_id)::bigint
ORDER BY s.name;

-- -- teachers --

-- name: CreateTeacher :one
INSERT INTO teachers (school_id, user_id, nip)
VALUES ($1, $2, $3)
RETURNING *;

-- name: UpdateTeacher :one
UPDATE teachers SET nip = COALESCE(sqlc.narg(nip), nip)
WHERE id = sqlc.arg(id) AND school_id = sqlc.arg(school_id)
RETURNING *;

-- name: GetTeacherByID :one
SELECT t.*, u.name AS user_name, u.email AS user_email
FROM teachers t JOIN users u ON u.id = t.user_id
WHERE t.id = $1 AND t.school_id = $2;

-- name: GetTeacherByUserID :one
SELECT t.*, u.name AS user_name, u.email AS user_email
FROM teachers t JOIN users u ON u.id = t.user_id
WHERE t.school_id = $1 AND t.user_id = $2;

-- name: GetTeacherByEmail :one
SELECT t.*, u.name AS user_name, u.email AS user_email
FROM teachers t JOIN users u ON u.id = t.user_id
WHERE t.school_id = $1 AND u.email = $2;

-- name: GetTeacherByNIP :one
SELECT t.*, u.name AS user_name, u.email AS user_email
FROM teachers t JOIN users u ON u.id = t.user_id
WHERE t.school_id = $1 AND t.nip = $2;

-- name: ListTeachers :many
SELECT t.*, u.name AS user_name, u.email AS user_email
FROM teachers t JOIN users u ON u.id = t.user_id
WHERE t.school_id = $1
ORDER BY u.name;

-- -- subjects --

-- name: CreateSubject :one
INSERT INTO subjects (school_id, code, name) VALUES ($1, $2, $3) RETURNING *;

-- name: UpdateSubject :one
UPDATE subjects SET
    code = COALESCE(sqlc.narg(code), code),
    name = COALESCE(sqlc.narg(name), name)
WHERE id = sqlc.arg(id) AND school_id = sqlc.arg(school_id)
RETURNING *;

-- name: GetSubjectByCode :one
SELECT * FROM subjects WHERE school_id = $1 AND code = $2;

-- name: ListSubjects :many
SELECT * FROM subjects WHERE school_id = $1 ORDER BY code;
