-- name: CreateExitPermit :one
INSERT INTO student_exit_permits (school_id, academic_year_id, student_id, reason, status)
VALUES ($1, $2, $3, $4, 'pending_duty_teacher')
RETURNING id;

-- name: CountActiveExitPermits :one
-- Maks 1 permit AKTIF (pending_*/issued) per siswa (docs tugas).
SELECT count(*) FROM student_exit_permits
WHERE school_id = $1 AND student_id = $2
  AND status NOT IN ('exited', 'rejected', 'canceled');

-- name: GetExitPermitDetail :one
SELECT
    p.id, p.school_id, p.academic_year_id, p.student_id, p.reason, p.status,
    p.duty_by, p.duty_at, p.class_by, p.class_at, p.bk_by, p.bk_at,
    p.leadership_by, p.leadership_at, p.rejected_by, p.rejected_at, p.reject_comment,
    p.gate_token, p.gate_expires_at, p.exited_by, p.exited_at, p.created_at,
    s.name AS student_name, s.nis AS student_nis, COALESCE(c.name, '') AS class_name,
    du.name AS duty_by_name, cu.name AS class_by_name, bu.name AS bk_by_name,
    lu.name AS leadership_by_name, ru.name AS rejected_by_name, eu.name AS exited_by_name
FROM student_exit_permits p
JOIN students s ON s.id = p.student_id
LEFT JOIN enrollments en ON en.student_id = p.student_id
LEFT JOIN classes c ON c.id = en.class_id AND c.academic_year_id = p.academic_year_id
LEFT JOIN users du ON du.id = p.duty_by
LEFT JOIN users cu ON cu.id = p.class_by
LEFT JOIN users bu ON bu.id = p.bk_by
LEFT JOIN users lu ON lu.id = p.leadership_by
LEFT JOIN users ru ON ru.id = p.rejected_by
LEFT JOIN users eu ON eu.id = p.exited_by
WHERE p.id = $1 AND p.school_id = $2
LIMIT 1;

-- name: GetExitPermitByGateToken :one
SELECT
    p.id, p.school_id, p.academic_year_id, p.student_id, p.reason, p.status,
    p.duty_by, p.duty_at, p.class_by, p.class_at, p.bk_by, p.bk_at,
    p.leadership_by, p.leadership_at, p.rejected_by, p.rejected_at, p.reject_comment,
    p.gate_token, p.gate_expires_at, p.exited_by, p.exited_at, p.created_at,
    s.name AS student_name, s.nis AS student_nis, COALESCE(c.name, '') AS class_name,
    du.name AS duty_by_name, cu.name AS class_by_name, bu.name AS bk_by_name,
    lu.name AS leadership_by_name, ru.name AS rejected_by_name, eu.name AS exited_by_name
FROM student_exit_permits p
JOIN students s ON s.id = p.student_id
LEFT JOIN enrollments en ON en.student_id = p.student_id
LEFT JOIN classes c ON c.id = en.class_id AND c.academic_year_id = p.academic_year_id
LEFT JOIN users du ON du.id = p.duty_by
LEFT JOIN users cu ON cu.id = p.class_by
LEFT JOIN users bu ON bu.id = p.bk_by
LEFT JOIN users lu ON lu.id = p.leadership_by
LEFT JOIN users ru ON ru.id = p.rejected_by
LEFT JOIN users eu ON eu.id = p.exited_by
WHERE p.gate_token = $1 AND p.school_id = $2
LIMIT 1;

-- name: ListExitPermitsForStudents :many
SELECT
    p.id, p.school_id, p.academic_year_id, p.student_id, p.reason, p.status,
    p.duty_by, p.duty_at, p.class_by, p.class_at, p.bk_by, p.bk_at,
    p.leadership_by, p.leadership_at, p.rejected_by, p.rejected_at, p.reject_comment,
    p.gate_token, p.gate_expires_at, p.exited_by, p.exited_at, p.created_at,
    s.name AS student_name, s.nis AS student_nis, COALESCE(c.name, '') AS class_name,
    du.name AS duty_by_name, cu.name AS class_by_name, bu.name AS bk_by_name,
    lu.name AS leadership_by_name, ru.name AS rejected_by_name, eu.name AS exited_by_name
FROM student_exit_permits p
JOIN students s ON s.id = p.student_id
LEFT JOIN enrollments en ON en.student_id = p.student_id
LEFT JOIN classes c ON c.id = en.class_id AND c.academic_year_id = p.academic_year_id
LEFT JOIN users du ON du.id = p.duty_by
LEFT JOIN users cu ON cu.id = p.class_by
LEFT JOIN users bu ON bu.id = p.bk_by
LEFT JOIN users lu ON lu.id = p.leadership_by
LEFT JOIN users ru ON ru.id = p.rejected_by
LEFT JOIN users eu ON eu.id = p.exited_by
WHERE p.school_id = $1 AND p.student_id = ANY(sqlc.arg(student_ids)::bigint[])
ORDER BY p.created_at DESC;

-- name: ListExitPermitsActiveToday :many
-- scope=active: permit hari ini (created_at lokal sekolah — rentang UTC
-- diteruskan service) yang statusnya BUKAN final.
SELECT
    p.id, p.school_id, p.academic_year_id, p.student_id, p.reason, p.status,
    p.duty_by, p.duty_at, p.class_by, p.class_at, p.bk_by, p.bk_at,
    p.leadership_by, p.leadership_at, p.rejected_by, p.rejected_at, p.reject_comment,
    p.gate_token, p.gate_expires_at, p.exited_by, p.exited_at, p.created_at,
    s.name AS student_name, s.nis AS student_nis, COALESCE(c.name, '') AS class_name,
    du.name AS duty_by_name, cu.name AS class_by_name, bu.name AS bk_by_name,
    lu.name AS leadership_by_name, ru.name AS rejected_by_name, eu.name AS exited_by_name
FROM student_exit_permits p
JOIN students s ON s.id = p.student_id
LEFT JOIN enrollments en ON en.student_id = p.student_id
LEFT JOIN classes c ON c.id = en.class_id AND c.academic_year_id = p.academic_year_id
LEFT JOIN users du ON du.id = p.duty_by
LEFT JOIN users cu ON cu.id = p.class_by
LEFT JOIN users bu ON bu.id = p.bk_by
LEFT JOIN users lu ON lu.id = p.leadership_by
LEFT JOIN users ru ON ru.id = p.rejected_by
LEFT JOIN users eu ON eu.id = p.exited_by
WHERE p.school_id = $1 AND p.created_at >= sqlc.arg(from_at)::timestamptz AND p.created_at < sqlc.arg(to_at)::timestamptz
  AND p.status NOT IN ('exited', 'rejected', 'canceled')
ORDER BY p.created_at DESC;

-- name: ListExitPermitsAll :many
-- scope=all: SEMUA status, opsional rentang tanggal (created_at, lokal
-- sekolah — rentang UTC diteruskan service; NULL = tanpa filter tanggal).
SELECT
    p.id, p.school_id, p.academic_year_id, p.student_id, p.reason, p.status,
    p.duty_by, p.duty_at, p.class_by, p.class_at, p.bk_by, p.bk_at,
    p.leadership_by, p.leadership_at, p.rejected_by, p.rejected_at, p.reject_comment,
    p.gate_token, p.gate_expires_at, p.exited_by, p.exited_at, p.created_at,
    s.name AS student_name, s.nis AS student_nis, COALESCE(c.name, '') AS class_name,
    du.name AS duty_by_name, cu.name AS class_by_name, bu.name AS bk_by_name,
    lu.name AS leadership_by_name, ru.name AS rejected_by_name, eu.name AS exited_by_name
FROM student_exit_permits p
JOIN students s ON s.id = p.student_id
LEFT JOIN enrollments en ON en.student_id = p.student_id
LEFT JOIN classes c ON c.id = en.class_id AND c.academic_year_id = p.academic_year_id
LEFT JOIN users du ON du.id = p.duty_by
LEFT JOIN users cu ON cu.id = p.class_by
LEFT JOIN users bu ON bu.id = p.bk_by
LEFT JOIN users lu ON lu.id = p.leadership_by
LEFT JOIN users ru ON ru.id = p.rejected_by
LEFT JOIN users eu ON eu.id = p.exited_by
WHERE p.school_id = $1
  AND (sqlc.narg(from_at)::timestamptz IS NULL OR p.created_at >= sqlc.narg(from_at))
  AND (sqlc.narg(to_at)::timestamptz IS NULL OR p.created_at < sqlc.narg(to_at))
ORDER BY p.created_at DESC;

-- name: ListExitPermitsExitedOnDate :many
-- gate-history: permit exited pada rentang tanggal (lokal sekolah, rentang
-- UTC diteruskan service), by exited_at.
SELECT
    p.id, p.school_id, p.academic_year_id, p.student_id, p.reason, p.status,
    p.duty_by, p.duty_at, p.class_by, p.class_at, p.bk_by, p.bk_at,
    p.leadership_by, p.leadership_at, p.rejected_by, p.rejected_at, p.reject_comment,
    p.gate_token, p.gate_expires_at, p.exited_by, p.exited_at, p.created_at,
    s.name AS student_name, s.nis AS student_nis, COALESCE(c.name, '') AS class_name,
    du.name AS duty_by_name, cu.name AS class_by_name, bu.name AS bk_by_name,
    lu.name AS leadership_by_name, ru.name AS rejected_by_name, eu.name AS exited_by_name
FROM student_exit_permits p
JOIN students s ON s.id = p.student_id
LEFT JOIN enrollments en ON en.student_id = p.student_id
LEFT JOIN classes c ON c.id = en.class_id AND c.academic_year_id = p.academic_year_id
LEFT JOIN users du ON du.id = p.duty_by
LEFT JOIN users cu ON cu.id = p.class_by
LEFT JOIN users bu ON bu.id = p.bk_by
LEFT JOIN users lu ON lu.id = p.leadership_by
LEFT JOIN users ru ON ru.id = p.rejected_by
LEFT JOIN users eu ON eu.id = p.exited_by
WHERE p.school_id = $1 AND p.status = 'exited' AND p.exited_at >= sqlc.arg(from_at)::timestamptz AND p.exited_at < sqlc.arg(to_at)::timestamptz
ORDER BY p.exited_at DESC;

-- name: UpdateExitPermitDutyStage :execrows
UPDATE student_exit_permits SET status = 'pending_class_teacher', duty_by = $3, duty_at = now()
WHERE id = $1 AND school_id = $2 AND status = 'pending_duty_teacher';

-- name: UpdateExitPermitClassStage :execrows
UPDATE student_exit_permits SET status = 'pending_bk', class_by = $3, class_at = now()
WHERE id = $1 AND school_id = $2 AND status = 'pending_class_teacher';

-- name: UpdateExitPermitBKStage :execrows
UPDATE student_exit_permits SET status = 'pending_leadership', bk_by = $3, bk_at = now()
WHERE id = $1 AND school_id = $2 AND status = 'pending_bk';

-- name: UpdateExitPermitLeadershipStage :execrows
UPDATE student_exit_permits SET status = 'issued', leadership_by = $3, leadership_at = now(),
    gate_token = $4, gate_expires_at = $5
WHERE id = $1 AND school_id = $2 AND status = 'pending_leadership';

-- name: RejectExitPermit :execrows
UPDATE student_exit_permits SET status = 'rejected', rejected_by = $3, rejected_at = now(), reject_comment = $4
WHERE id = $1 AND school_id = $2 AND status NOT IN ('exited', 'rejected', 'canceled');

-- name: CancelExitPermit :execrows
UPDATE student_exit_permits SET status = 'canceled'
WHERE id = $1 AND school_id = $2
  AND status IN ('pending_duty_teacher', 'pending_class_teacher', 'pending_bk', 'pending_leadership');

-- name: GateScanExitPermit :execrows
UPDATE student_exit_permits SET status = 'exited', exited_by = $3, exited_at = now()
WHERE gate_token = $1 AND school_id = $2 AND status = 'issued';
