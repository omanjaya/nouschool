-- name: CreateComponent :one
INSERT INTO assessment_components (school_id, academic_year_id, class_id, subject_id, name, type, weight, kktp, created_by)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: UpdateComponent :one
UPDATE assessment_components SET
    name   = COALESCE(sqlc.narg(name), name),
    type   = COALESCE(sqlc.narg(type), type),
    weight = COALESCE(sqlc.narg(weight), weight),
    kktp   = COALESCE(sqlc.narg(kktp), kktp)
WHERE id = sqlc.arg(id) AND school_id = sqlc.arg(school_id)
RETURNING *;

-- name: GetComponentByID :one
SELECT * FROM assessment_components WHERE id = $1 AND school_id = $2;

-- name: ListComponentsByClassSubject :many
SELECT c.*,
    (SELECT COUNT(*) FROM student_grades sg WHERE sg.component_id = c.id) AS graded_count,
    (SELECT COUNT(*) FROM enrollments e WHERE e.class_id = c.class_id) AS student_count
FROM assessment_components c
WHERE c.school_id = $1 AND c.class_id = $2 AND c.subject_id = $3
ORDER BY c.id;

-- name: ListComponentsRawByClassSubject :many
SELECT * FROM assessment_components
WHERE school_id = $1 AND class_id = $2 AND subject_id = $3
ORDER BY id;

-- name: DeleteComponent :execrows
DELETE FROM assessment_components WHERE id = $1 AND school_id = $2;

-- name: UpsertGrade :one
INSERT INTO student_grades (school_id, component_id, student_id, score, graded_by)
VALUES ($1, $2, $3, sqlc.arg(score)::float8, $4)
ON CONFLICT (component_id, student_id) DO UPDATE SET
    score = EXCLUDED.score, graded_by = EXCLUDED.graded_by, updated_at = now()
RETURNING id, school_id, component_id, student_id, score::float8 AS score, graded_by, updated_at;

-- name: DeleteGrade :exec
DELETE FROM student_grades WHERE component_id = $1 AND student_id = $2;

-- name: ListGradesForComponent :many
SELECT student_id, score::float8 AS score FROM student_grades WHERE component_id = $1;

-- name: ListGradesForComponents :many
SELECT component_id, student_id, score::float8 AS score FROM student_grades
WHERE component_id = ANY(sqlc.arg(component_ids)::bigint[]);

-- name: UpsertPublication :exec
INSERT INTO grade_publications (school_id, academic_year_id, class_id, subject_id, published_by)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (school_id, academic_year_id, class_id, subject_id) DO UPDATE SET
    published_at = now(), published_by = EXCLUDED.published_by;

-- name: DeletePublication :exec
DELETE FROM grade_publications WHERE school_id = $1 AND academic_year_id = $2 AND class_id = $3 AND subject_id = $4;

-- name: GetPublication :one
SELECT * FROM grade_publications WHERE school_id = $1 AND academic_year_id = $2 AND class_id = $3 AND subject_id = $4;

-- name: ListPublishedSubjectsForClass :many
SELECT subject_id FROM grade_publications WHERE school_id = $1 AND academic_year_id = $2 AND class_id = $3;

-- name: CreateStar :one
INSERT INTO classroom_star_events (school_id, academic_year_id, student_id, given_by, delta, note, visibility)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: GetStarByID :one
SELECT * FROM classroom_star_events WHERE id = $1 AND school_id = $2;

-- name: DeleteStar :execrows
DELETE FROM classroom_star_events WHERE id = $1 AND school_id = $2;

-- name: ListStarsForStudent :many
SELECT e.id, e.delta, e.note, e.visibility, e.created_at, u.name AS given_by_name
FROM classroom_star_events e
JOIN users u ON u.id = e.given_by
WHERE e.school_id = $1 AND e.student_id = $2
ORDER BY e.created_at DESC;

-- name: ListStarsForStudentVisible :many
SELECT e.id, e.delta, e.note, e.visibility, e.created_at, u.name AS given_by_name
FROM classroom_star_events e
JOIN users u ON u.id = e.given_by
WHERE e.school_id = $1 AND e.student_id = $2 AND e.visibility = 'student'
ORDER BY e.created_at DESC;

-- name: ListStarTotalsForClass :many
SELECT s.id AS student_id, s.name AS student_name, s.nis AS student_nis,
    COALESCE((SELECT SUM(e.delta) FROM classroom_star_events e WHERE e.student_id = s.id AND e.school_id = $1), 0)::int AS total
FROM students s
JOIN enrollments en ON en.student_id = s.id
WHERE en.class_id = $2 AND s.school_id = $1
ORDER BY s.name;

-- -- lookup & validasi lintas modul (join read-only) --

-- name: GetClassByID :one
SELECT id, name, academic_year_id FROM classes WHERE id = $1 AND school_id = $2;

-- name: GetSubjectByID :one
SELECT id, code, name FROM subjects WHERE id = $1 AND school_id = $2;

-- name: ListClassRoster :many
SELECT s.id, s.name, s.nis, COALESCE(s.user_id, 0)::bigint AS user_id
FROM students s
JOIN enrollments e ON e.student_id = s.id
WHERE e.class_id = $1 AND s.school_id = $2
ORDER BY s.name;

-- name: GetStudentBasic :one
SELECT name, nis FROM students WHERE id = $1 AND school_id = $2;

-- name: StudentClassID :one
SELECT e.class_id FROM enrollments e
JOIN classes c ON c.id = e.class_id
WHERE e.student_id = $1 AND c.school_id = $2 AND c.academic_year_id = $3;
