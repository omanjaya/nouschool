-- Query modul identity (sqlc).

-- name: GetUserByEmail :one
SELECT * FROM users WHERE email = $1;

-- name: GetUserByUsername :one
SELECT * FROM users WHERE username = $1;

-- name: GetUserByID :one
SELECT * FROM users WHERE id = $1;

-- name: CreateUser :one
INSERT INTO users (email, username, password_hash, name, phone, is_super_admin)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: UpdateUserPassword :exec
UPDATE users SET password_hash = $2 WHERE id = $1;

-- name: SetUserName :exec
UPDATE users SET name = $2 WHERE id = $1;

-- name: SetSuperAdmin :exec
UPDATE users SET is_super_admin = $2 WHERE id = $1;

-- name: ListActiveMembershipsByUserSchool :many
SELECT * FROM memberships WHERE user_id = $1 AND school_id = $2 AND status = 'active';

-- name: CreateMembership :one
INSERT INTO memberships (user_id, school_id, role, status)
VALUES ($1, $2, $3, 'active')
ON CONFLICT (user_id, school_id, role) DO UPDATE SET status = 'active'
RETURNING *;

-- name: ListUserIDsByRole :many
-- Dipakai interface publik Service.UsersWithRole (fase 9, docs/08-notification.md)
-- — resolusi penerima notifikasi utk step approval leave dengan role-only
-- (approver_user_id NULL: "siapa pun yang sedang memegang role ini").
SELECT DISTINCT user_id FROM memberships
WHERE school_id = sqlc.arg(school_id)::bigint AND role = sqlc.arg(role)::text AND status = 'active';

-- name: CreateSession :one
INSERT INTO sessions (user_id, school_id, token_hash, role, expires_at, ip, user_agent, impersonator_user_id)
VALUES ($1, $2, $3, $4, $5, $6, $7, sqlc.narg(impersonator_user_id)::bigint)
RETURNING *;

-- name: GetSessionByTokenHash :one
SELECT
    s.id, s.user_id, s.school_id, s.token_hash, s.role, s.expires_at, s.created_at, s.ip, s.user_agent,
    s.impersonator_user_id,
    u.name AS user_name,
    u.is_super_admin AS user_is_super_admin
FROM sessions s
JOIN users u ON u.id = s.user_id
WHERE s.token_hash = $1;

-- name: GetUserBasic :one
-- Dipakai render field impersonated_by pada GET /api/me (Fase 14 Gelombang D)
-- — nama saja, TANPA data sensitif lain.
SELECT id, name FROM users WHERE id = $1;

-- name: ExtendSession :exec
UPDATE sessions SET expires_at = $2 WHERE id = $1;

-- name: DeleteSessionByTokenHash :exec
DELETE FROM sessions WHERE token_hash = $1;

-- name: InsertAuditLog :exec
INSERT INTO audit_log (school_id, user_id, action, entity, entity_id, old_value, new_value)
VALUES ($1, $2, $3, $4, $5, $6, $7);

-- -- invitations --

-- name: CreateInvitation :one
INSERT INTO invitations (school_id, code, role, target_id, expires_at)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetInvitationByCode :one
SELECT * FROM invitations WHERE code = $1;

-- name: GetActiveInvitationByRoleTarget :one
-- Dipakai generator kode undangan supaya idempoten: kode lama yang belum
-- terpakai & belum kedaluwarsa dipakai ulang, bukan dibuat baru.
SELECT * FROM invitations
WHERE school_id = $1 AND role = $2 AND target_id = $3
  AND used_at IS NULL AND expires_at > $4
ORDER BY id DESC LIMIT 1;

-- name: MarkInvitationUsed :exec
UPDATE invitations SET used_at = $2 WHERE code = $1;

-- name: GetStudentIDByUser :one
-- Read-only lookup ke tabel students (milik modul student) untuk mengisi
-- student_id di /api/me & login response role siswa. Preseden: join read-only
-- lintas tabel diperbolehkan (lihat student/queries.sql join ke users).
SELECT id FROM students WHERE user_id = $1 AND school_id = $2;

-- -- fase 13, docs/11-superadmin.md P4 "Operasional" --
-- P4.1 (daftar anggota), P4.2 (reset password), P4.3 (audit log viewer) HANYA
-- menyentuh tabel milik modul identity sendiri (users/memberships/sessions/
-- audit_log) — makanya ditempatkan di sini (perluasan modul existing) alih-
-- alih modul agregator baru (lihat catatan keputusan di internal/platformadmin).

-- name: DeleteSessionsByUser :exec
-- Dipakai AdminResetPassword — hapus SEMUA sesi user target supaya password
-- lama langsung tidak berlaku lagi di device manapun.
DELETE FROM sessions WHERE user_id = $1;

-- name: ListMembersForSchool :many
-- Satu baris per membership (user muncul >1 kali bila punya >1 role, mis.
-- guru + orang_tua) — last_login = sesi TERAKHIR user itu DI SEKOLAH ini
-- (lintas role).
SELECT
    m.user_id, u.name, u.email, u.username, m.role, m.status,
    (SELECT MAX(sess.created_at) FROM sessions sess WHERE sess.user_id = m.user_id AND sess.school_id = m.school_id)::timestamptz AS last_login
FROM memberships m
JOIN users u ON u.id = m.user_id
WHERE m.school_id = $1
ORDER BY u.name, m.role;

-- name: ListAuditLogForSchool :many
-- action_prefix kosong ("") berarti tanpa filter aksi.
SELECT a.id, u.name AS user_name, a.action, a.entity, a.entity_id, a.at
FROM audit_log a
LEFT JOIN users u ON u.id = a.user_id
WHERE a.school_id = sqlc.arg(school_id)::bigint
  AND (sqlc.arg(action_prefix)::text = '' OR a.action LIKE sqlc.arg(action_prefix)::text || '%')
ORDER BY a.at DESC
LIMIT sqlc.arg(limit_count)::int OFFSET sqlc.arg(offset_count)::int;

-- name: CountAuditLogForSchool :one
SELECT COUNT(*) FROM audit_log a
WHERE a.school_id = sqlc.arg(school_id)::bigint
  AND (sqlc.arg(action_prefix)::text = '' OR a.action LIKE sqlc.arg(action_prefix)::text || '%');
