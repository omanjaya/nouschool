-- name: CreateTeacherQRToken :one
INSERT INTO teacher_qr_tokens (school_id, user_id, token, expires_at)
VALUES ($1, $2, $3, $4)
RETURNING id;

-- name: DeleteExpiredTeacherQRTokensForUser :exec
-- Cleanup lazy (docs tugas): token kedaluwarsa milik user ini dihapus setiap
-- kali dia generate token baru.
DELETE FROM teacher_qr_tokens
WHERE school_id = $1 AND user_id = $2 AND expires_at <= sqlc.arg(now)::timestamptz;

-- name: ConsumeTeacherQRToken :one
-- Atomik: hanya baris consumed_at IS NULL & belum kedaluwarsa yang cocok —
-- dua pemanggil bersamaan pada token yang sama, HANYA SATU berhasil (row
-- lock implisit UPDATE Postgres, tidak butuh SELECT FOR UPDATE terpisah).
UPDATE teacher_qr_tokens SET consumed_at = sqlc.arg(now)::timestamptz
WHERE token = $1 AND school_id = $2 AND consumed_at IS NULL AND expires_at > sqlc.arg(now)::timestamptz
RETURNING user_id;
