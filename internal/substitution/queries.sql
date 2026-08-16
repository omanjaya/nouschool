-- Query modul substitution (sqlc). Semua query tenant-scoped WAJIB filter
-- school_id. Join read-only ke schedule_slots/classes/subjects/users
-- (dimiliki modul lain) dipakai untuk menampilkan nama tanpa N+1 call lintas
-- modul — pola yang sama dengan internal/discipline/queries.sql.

-- name: CreateSubstitutionRequest :one
INSERT INTO teacher_substitution_requests (school_id, schedule_slot_id, date, requested_by, substitute_user_id, reason)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetSubstitutionRequestByID :one
SELECT * FROM teacher_substitution_requests WHERE id = $1 AND school_id = $2;

-- name: TransitionSubstitutionStatus :one
-- Race guard transaksional (pola sama internal/exitpermit: "UPDATE ... WHERE
-- status = $tahap_lama") — mengembalikan pgx.ErrNoRows bila status saat ini
-- BUKAN from_status (sudah diputuskan pihak lain / dibatalkan duluan).
UPDATE teacher_substitution_requests
SET status = sqlc.arg(to_status)::text, decided_at = sqlc.arg(decided_at)::timestamptz
WHERE id = sqlc.arg(id) AND school_id = sqlc.arg(school_id) AND status = sqlc.arg(from_status)::text
RETURNING *;

-- name: GetSubstitutionRow :one
SELECT tsr.id, tsr.school_id, tsr.schedule_slot_id, tsr.date, tsr.requested_by, tsr.substitute_user_id,
       tsr.reason, tsr.status, tsr.decided_at, tsr.created_at,
       ss.class_id, c.name AS class_name, ss.subject_id, sub.name AS subject_name,
       ss.day_of_week, ss.period_start, ss.period_end,
       ureq.name AS requested_by_name, usub.name AS substitute_name
FROM teacher_substitution_requests tsr
JOIN schedule_slots ss ON ss.id = tsr.schedule_slot_id
JOIN classes c ON c.id = ss.class_id
JOIN subjects sub ON sub.id = ss.subject_id
JOIN users ureq ON ureq.id = tsr.requested_by
JOIN users usub ON usub.id = tsr.substitute_user_id
WHERE tsr.id = $1 AND tsr.school_id = $2;

-- name: ListSubstitutionRequests :many
SELECT tsr.id, tsr.school_id, tsr.schedule_slot_id, tsr.date, tsr.requested_by, tsr.substitute_user_id,
       tsr.reason, tsr.status, tsr.decided_at, tsr.created_at,
       ss.class_id, c.name AS class_name, ss.subject_id, sub.name AS subject_name,
       ss.day_of_week, ss.period_start, ss.period_end,
       ureq.name AS requested_by_name, usub.name AS substitute_name
FROM teacher_substitution_requests tsr
JOIN schedule_slots ss ON ss.id = tsr.schedule_slot_id
JOIN classes c ON c.id = ss.class_id
JOIN subjects sub ON sub.id = ss.subject_id
JOIN users ureq ON ureq.id = tsr.requested_by
JOIN users usub ON usub.id = tsr.substitute_user_id
WHERE tsr.school_id = sqlc.arg(school_id)::bigint
  AND (sqlc.narg(requested_by)::bigint IS NULL OR tsr.requested_by = sqlc.narg(requested_by)::bigint)
  AND (sqlc.narg(substitute_user_id)::bigint IS NULL OR tsr.substitute_user_id = sqlc.narg(substitute_user_id)::bigint)
  AND (sqlc.narg(date)::date IS NULL OR tsr.date = sqlc.narg(date)::date)
ORDER BY tsr.date DESC, tsr.id DESC;

-- -- validasi lintas modul (join read-only ke tabel bersama) --

-- name: GetSlotBasic :one
SELECT id, school_id, teacher_id, day_of_week FROM schedule_slots WHERE id = $1 AND school_id = $2;

-- name: GetTeacherUserID :one
-- Dipakai validasi "guru pemilik slot" (bandingkan user_id pemilik profil
-- teacher_id slot dgn actor yang mengajukan) — join langsung ke teachers
-- (dimiliki modul student), pola sama internal/discipline "GetHomeroomTeacherUserID".
SELECT user_id FROM teachers WHERE id = $1 AND school_id = $2;

-- name: IsActiveGuru :one
-- Dipakai validasi substitute_user_id: harus anggota AKTIF sekolah ini
-- dengan role guru — join langsung ke memberships (dimiliki modul identity).
SELECT EXISTS (SELECT 1 FROM memberships WHERE user_id = $1 AND school_id = $2 AND role = 'guru' AND status = 'active');

-- name: ActiveSubstituteForSlotDate :one
-- Dipakai interface publik Service.SubstituteFor (docs tugas Gelombang D:
-- dikonsumsi modul teaching lewat consumer-side interface SubstitutionLookup)
-- — HANYA status accepted yang berlaku sbg pengganti sah.
SELECT usub.id AS user_id, usub.name AS name
FROM teacher_substitution_requests tsr
JOIN users usub ON usub.id = tsr.substitute_user_id
WHERE tsr.schedule_slot_id = $1 AND tsr.school_id = $2 AND tsr.date = $3 AND tsr.status = 'accepted';

-- name: SlotIDsAcceptedSubstituteForDate :many
-- Dipakai interface publik Service.IsSubstituteToday.
SELECT schedule_slot_id FROM teacher_substitution_requests
WHERE school_id = $1 AND substitute_user_id = $2 AND date = $3 AND status = 'accepted';
