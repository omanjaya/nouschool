-- name: CreateDuty :one
INSERT INTO duties (school_id, name, for_role, flags, active)
VALUES ($1, $2, $3, $4, true)
RETURNING id, school_id, name, for_role, flags, active;

-- name: UpdateDuty :one
-- PATCH semantics: sqlc.narg NULL berarti field tidak diubah (pola sama
-- internal/discipline UpdateViolationType).
UPDATE duties SET
    name     = COALESCE(sqlc.narg(name), name),
    for_role = COALESCE(sqlc.narg(for_role), for_role),
    flags    = COALESCE(sqlc.narg(flags), flags),
    active   = COALESCE(sqlc.narg(active), active)
WHERE id = $1 AND school_id = $2
RETURNING id, school_id, name, for_role, flags, active;

-- name: GetDutyByID :one
SELECT id, school_id, name, for_role, flags, active FROM duties WHERE id = $1 AND school_id = $2;

-- name: ListDutiesWithAssigneeCount :many
-- assignee_count = jumlah pemegang tugas pada TA AKTIF (academic_year_id
-- parameter, 0 bila sekolah belum punya TA aktif -> tidak match apa pun).
SELECT d.id, d.school_id, d.name, d.for_role, d.flags, d.active,
       COUNT(a.id) AS assignee_count
FROM duties d
LEFT JOIN duty_assignments a ON a.duty_id = d.id AND a.academic_year_id = $2
WHERE d.school_id = $1
GROUP BY d.id
ORDER BY d.name;

-- name: DeleteDuty :execrows
DELETE FROM duties WHERE id = $1 AND school_id = $2;

-- name: CountAssignmentsForDuty :one
-- Dipakai DeleteDuty (409 bila SUDAH punya assignment di TA MANA PUN).
SELECT COUNT(*) FROM duty_assignments WHERE duty_id = $1 AND school_id = $2;

-- name: CreateAssignment :one
INSERT INTO duty_assignments (school_id, duty_id, user_id, academic_year_id)
VALUES ($1, $2, $3, $4)
RETURNING id;

-- name: DeleteAssignmentsForDutyYear :exec
DELETE FROM duty_assignments WHERE duty_id = $1 AND school_id = $2 AND academic_year_id = $3;

-- name: ListAssignmentsForDutyYear :many
SELECT u.id AS user_id, u.name
FROM duty_assignments a
JOIN users u ON u.id = a.user_id
WHERE a.duty_id = $1 AND a.school_id = $2 AND a.academic_year_id = $3
ORDER BY u.name;

-- name: UserHasFlag :one
-- Dipakai Service.UserHasFlag (interface publik konsumsi modul lain, mis.
-- studentleave): true bila user memegang SETIDAKNYA SATU tugas AKTIF pada TA
-- yang diberikan yang membawa flag tsb.
SELECT EXISTS (
    SELECT 1 FROM duty_assignments a
    JOIN duties d ON d.id = a.duty_id
    WHERE a.school_id = $1 AND a.user_id = $2 AND a.academic_year_id = $3
      AND d.active AND $4::text = ANY (d.flags)
) AS has_flag;

-- name: ListUserIDsWithFlag :many
-- Dipakai Service.UserIDsWithFlag (interface publik, mis. resolusi target
-- notifikasi/antrian): distinct user_id pemegang tugas AKTIF pada TA yang
-- diberikan yang membawa flag tsb.
SELECT DISTINCT a.user_id
FROM duty_assignments a
JOIN duties d ON d.id = a.duty_id
WHERE a.school_id = $1 AND a.academic_year_id = $2
  AND d.active AND $3::text = ANY (d.flags);
