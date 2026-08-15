-- Query modul schedule (sqlc). Semua query tenant-scoped WAJIB filter
-- school_id. Catatan: join read-only ke classes/subjects/teachers (dimiliki
-- modul student) dan users (dimiliki modul identity) dipakai untuk
-- menampilkan nama tanpa N+1 call lintas modul — pola yang sama dengan
-- internal/attendance/queries.sql. Semua TULIS ke tabel modul lain tetap
-- TIDAK PERNAH dilakukan di sini.

-- -- periods --

-- name: ListPeriods :many
SELECT * FROM periods WHERE school_id = $1 ORDER BY number;

-- name: DeletePeriods :exec
DELETE FROM periods WHERE school_id = $1;

-- name: InsertPeriod :one
INSERT INTO periods (school_id, number, starts_at, ends_at, label)
VALUES ($1, $2, $3, $4, sqlc.narg(label)::text)
RETURNING *;

-- name: ListSlotPeriodRanges :many
SELECT DISTINCT period_start, period_end FROM schedule_slots WHERE school_id = $1;

-- -- rooms --

-- name: CreateRoom :one
INSERT INTO rooms (school_id, name, qr_token) VALUES ($1, $2, $3) RETURNING *;

-- name: UpdateRoomName :one
UPDATE rooms SET name = $3 WHERE id = $1 AND school_id = $2 RETURNING *;

-- name: RegenerateRoomQRToken :one
UPDATE rooms SET qr_token = $3 WHERE id = $1 AND school_id = $2 RETURNING *;

-- name: GetRoomByID :one
SELECT * FROM rooms WHERE id = $1 AND school_id = $2;

-- name: ListRooms :many
SELECT * FROM rooms WHERE school_id = $1 ORDER BY name;

-- name: DeleteRoom :exec
DELETE FROM rooms WHERE id = $1 AND school_id = $2;

-- name: CountSlotsForRoom :one
SELECT COUNT(*) FROM schedule_slots WHERE room_id = $1 AND school_id = $2;

-- -- referensi (validasi & lookup by kode/nama, dipakai builder & import) --

-- name: GetClassRef :one
SELECT id, name, academic_year_id FROM classes WHERE id = $1 AND school_id = $2;

-- name: GetSubjectRef :one
SELECT id, code, name FROM subjects WHERE id = $1 AND school_id = $2;

-- name: GetTeacherRef :one
SELECT t.id, u.name FROM teachers t JOIN users u ON u.id = t.user_id WHERE t.id = $1 AND t.school_id = $2;

-- name: GetRoomRef :one
SELECT id, name FROM rooms WHERE id = $1 AND school_id = $2;

-- name: LookupClassIDByName :one
SELECT id FROM classes WHERE school_id = $1 AND academic_year_id = $2 AND name = $3;

-- name: LookupSubjectByCode :one
SELECT id, code, name FROM subjects WHERE school_id = $1 AND code = $2;

-- name: LookupTeacherByEmail :one
SELECT t.id, u.name FROM teachers t JOIN users u ON u.id = t.user_id WHERE t.school_id = $1 AND u.email = $2;

-- name: LookupRoomIDByName :one
SELECT id FROM rooms WHERE school_id = $1 AND name = $2;

-- -- schedule_slots --

-- name: ListSlotsForYear :many
SELECT sl.id, sl.school_id, sl.academic_year_id,
       sl.class_id, c.name AS class_name,
       sl.subject_id, sub.code AS subject_code, sub.name AS subject_name,
       sl.teacher_id, u.name AS teacher_name,
       sl.room_id, COALESCE(r.name, '') AS room_name,
       sl.day_of_week, sl.period_start, sl.period_end
FROM schedule_slots sl
JOIN classes c ON c.id = sl.class_id
JOIN subjects sub ON sub.id = sl.subject_id
JOIN teachers t ON t.id = sl.teacher_id
JOIN users u ON u.id = t.user_id
LEFT JOIN rooms r ON r.id = sl.room_id
WHERE sl.school_id = $1 AND sl.academic_year_id = $2
ORDER BY sl.day_of_week, sl.period_start;

-- name: GetSlotByID :one
SELECT sl.id, sl.school_id, sl.academic_year_id,
       sl.class_id, c.name AS class_name,
       sl.subject_id, sub.code AS subject_code, sub.name AS subject_name,
       sl.teacher_id, u.name AS teacher_name,
       sl.room_id, COALESCE(r.name, '') AS room_name,
       sl.day_of_week, sl.period_start, sl.period_end
FROM schedule_slots sl
JOIN classes c ON c.id = sl.class_id
JOIN subjects sub ON sub.id = sl.subject_id
JOIN teachers t ON t.id = sl.teacher_id
JOIN users u ON u.id = t.user_id
LEFT JOIN rooms r ON r.id = sl.room_id
WHERE sl.id = $1 AND sl.school_id = $2;

-- name: InsertSlot :one
INSERT INTO schedule_slots (school_id, academic_year_id, class_id, subject_id, teacher_id, room_id, day_of_week, period_start, period_end)
VALUES ($1, $2, $3, $4, $5, sqlc.narg(room_id)::bigint, $6, $7, $8)
RETURNING id;

-- name: UpdateSlot :exec
UPDATE schedule_slots SET
    class_id = $3, subject_id = $4, teacher_id = $5, room_id = sqlc.narg(room_id)::bigint,
    day_of_week = $6, period_start = $7, period_end = $8
WHERE id = $1 AND school_id = $2;

-- name: DeleteSlot :exec
DELETE FROM schedule_slots WHERE id = $1 AND school_id = $2;
