-- name: CreateEmployee :one
INSERT INTO employees (school_id, user_id, nip)
VALUES ($1, $2, $3)
RETURNING id, school_id, user_id, nip;

-- name: UpdateEmployeeNIP :one
UPDATE employees SET nip = $3 WHERE id = $1 AND school_id = $2
RETURNING id, school_id, user_id, nip;

-- name: GetEmployeeByID :one
SELECT e.id, e.school_id, e.user_id, e.nip, u.name, u.email, u.username
FROM employees e JOIN users u ON u.id = e.user_id
WHERE e.id = $1 AND e.school_id = $2;

-- name: ListEmployees :many
SELECT e.id, e.school_id, e.user_id, e.nip, u.name, u.email, u.username
FROM employees e JOIN users u ON u.id = e.user_id
WHERE e.school_id = $1
ORDER BY u.name;
