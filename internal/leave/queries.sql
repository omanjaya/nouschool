-- Query modul leave (sqlc). Semua query tenant-scoped WAJIB filter school_id.
-- Catatan: join read-only ke users (dimiliki modul identity) dipakai untuk
-- menampilkan nama pengaju/pemutus tanpa N+1 call lintas modul — pola yang
-- sama dengan internal/attendance/queries.sql (join ke classes/users).

-- name: GetLeaveSettingsRaw :one
SELECT settings FROM school_settings WHERE school_id = $1 AND module = 'leave';

-- name: CreateLeaveRequest :one
INSERT INTO leave_requests (
    school_id, teacher_id, type, date_start, date_end, reason,
    attachment, attachment_name, attachment_mime, status
) VALUES (
    sqlc.arg(school_id)::bigint, sqlc.arg(teacher_id)::bigint, sqlc.arg(type)::text,
    sqlc.arg(date_start)::date, sqlc.arg(date_end)::date, sqlc.arg(reason)::text,
    sqlc.narg(attachment)::text, sqlc.narg(attachment_name)::text, sqlc.narg(attachment_mime)::text,
    sqlc.arg(status)::text
) RETURNING *;

-- name: CreateLeaveApprovalStep :one
INSERT INTO leave_approval_steps (school_id, leave_request_id, step_order, approver_role, approver_user_id)
VALUES (
    sqlc.arg(school_id)::bigint, sqlc.arg(leave_request_id)::bigint, sqlc.arg(step_order)::int,
    sqlc.arg(approver_role)::text, sqlc.narg(approver_user_id)::bigint
)
RETURNING *;

-- name: GetLeaveRequestByID :one
SELECT lr.*, u.name AS teacher_name
FROM leave_requests lr
JOIN users u ON u.id = lr.teacher_id
WHERE lr.id = $1 AND lr.school_id = $2;

-- name: ListLeaveRequests :many
SELECT lr.*, u.name AS teacher_name
FROM leave_requests lr
JOIN users u ON u.id = lr.teacher_id
WHERE lr.school_id = sqlc.arg(school_id)::bigint
  AND (sqlc.narg(teacher_id)::bigint IS NULL OR lr.teacher_id = sqlc.narg(teacher_id)::bigint)
  AND (sqlc.narg(status)::text IS NULL OR lr.status = sqlc.narg(status)::text)
ORDER BY lr.created_at DESC;

-- name: ListLeaveStepsByRequestIDs :many
SELECT s.*, u.name AS decided_by_name
FROM leave_approval_steps s
LEFT JOIN users u ON u.id = s.decided_by
WHERE s.leave_request_id = ANY(sqlc.arg(request_ids)::bigint[])
ORDER BY s.leave_request_id, s.step_order;

-- name: ListLeaveStepsForRequest :many
SELECT s.*, u.name AS decided_by_name
FROM leave_approval_steps s
LEFT JOIN users u ON u.id = s.decided_by
WHERE s.leave_request_id = $1
ORDER BY s.step_order;

-- name: GetLeaveStepByID :one
SELECT * FROM leave_approval_steps WHERE id = $1 AND school_id = $2;

-- name: DecideLeaveStepGuarded :execrows
-- Guard "WHERE decision IS NULL" mencegah dua keputusan menimpa step yang sama
-- secara bersamaan (optimistic guard, bukan lock — pola yang sama dipakai
-- CancelLeaveRequestGuarded di bawah).
UPDATE leave_approval_steps
SET decision = sqlc.arg(decision)::text, decided_by = sqlc.arg(decided_by)::bigint,
    decided_at = now(), comment = sqlc.narg(comment)::text
WHERE id = sqlc.arg(id)::bigint AND school_id = sqlc.arg(school_id)::bigint AND decision IS NULL;

-- name: UpdateLeaveRequestStatusGuarded :execrows
UPDATE leave_requests SET status = sqlc.arg(status)::text
WHERE id = sqlc.arg(id)::bigint AND school_id = sqlc.arg(school_id)::bigint AND status = 'pending';

-- name: CancelLeaveRequestGuarded :execrows
UPDATE leave_requests SET status = 'canceled'
WHERE id = sqlc.arg(id)::bigint AND school_id = sqlc.arg(school_id)::bigint
  AND teacher_id = sqlc.arg(teacher_id)::bigint AND status = 'pending';

-- name: ListLeaveApprovalQueue :many
-- Antrian step AKTIF (step_order terendah tanpa decision pada request pending)
-- yang cocok dengan user ini (approver_user_id = user ATAU (approver_user_id
-- NULL dan role user = approver_role) — lihat docs/07-leave.md).
SELECT s.*
FROM leave_approval_steps s
JOIN leave_requests lr ON lr.id = s.leave_request_id
WHERE s.school_id = sqlc.arg(school_id)::bigint
  AND lr.status = 'pending'
  AND s.decision IS NULL
  AND s.step_order = (
      SELECT MIN(s2.step_order) FROM leave_approval_steps s2
      WHERE s2.leave_request_id = s.leave_request_id AND s2.decision IS NULL
  )
  AND (
      s.approver_user_id = sqlc.arg(user_id)::bigint
      OR (s.approver_user_id IS NULL AND s.approver_role = sqlc.arg(role)::text)
  )
ORDER BY lr.created_at;

-- name: ListLeaveRequestsInRange :many
-- Dipakai export Excel (Fase 11, GET /api/leave/export?from=&to=): SEMUA
-- status (bukan hanya approved seperti LeaveSummaryByRange), request yang
-- rentangnya beririsan [from,to].
SELECT lr.*, u.name AS teacher_name
FROM leave_requests lr
JOIN users u ON u.id = lr.teacher_id
WHERE lr.school_id = sqlc.arg(school_id)::bigint
  AND lr.date_start <= sqlc.arg(to_date)::date AND lr.date_end >= sqlc.arg(from_date)::date
ORDER BY lr.date_start, u.name;

-- name: LeaveSummaryByRange :many
-- Rekap per guru per tipe dari request approved yang beririsan [from,to] —
-- hari dihitung terpotong (clipped) ke rentang query.
SELECT
    u.id AS teacher_id,
    u.name AS teacher_name,
    lr.type AS type,
    (LEAST(lr.date_end, sqlc.arg(to_date)::date) - GREATEST(lr.date_start, sqlc.arg(from_date)::date) + 1)::bigint AS days
FROM leave_requests lr
JOIN users u ON u.id = lr.teacher_id
WHERE lr.school_id = sqlc.arg(school_id)::bigint
  AND lr.status = 'approved'
  AND lr.date_start <= sqlc.arg(to_date)::date
  AND lr.date_end >= sqlc.arg(from_date)::date
ORDER BY u.name, lr.type;

-- name: LeaveApprovedOn :one
-- Dipakai interface publik Service.ApprovedOn (konsumsi modul teaching, fase 6).
SELECT EXISTS (
    SELECT 1 FROM leave_requests
    WHERE school_id = sqlc.arg(school_id)::bigint AND teacher_id = sqlc.arg(teacher_id)::bigint AND status = 'approved'
      AND date_start <= sqlc.arg(on_date)::date AND date_end >= sqlc.arg(on_date)::date
) AS approved;
