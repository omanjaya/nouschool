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

-- name: CreateSession :one
INSERT INTO sessions (user_id, school_id, token_hash, role, expires_at, ip, user_agent)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: GetSessionByTokenHash :one
SELECT
    s.id, s.user_id, s.school_id, s.token_hash, s.role, s.expires_at, s.created_at, s.ip, s.user_agent,
    u.name AS user_name,
    u.is_super_admin AS user_is_super_admin
FROM sessions s
JOIN users u ON u.id = s.user_id
WHERE s.token_hash = $1;

-- name: ExtendSession :exec
UPDATE sessions SET expires_at = $2 WHERE id = $1;

-- name: DeleteSessionByTokenHash :exec
DELETE FROM sessions WHERE token_hash = $1;

-- name: InsertAuditLog :exec
INSERT INTO audit_log (school_id, user_id, action, entity, entity_id, old_value, new_value)
VALUES ($1, $2, $3, $4, $5, $6, $7);
