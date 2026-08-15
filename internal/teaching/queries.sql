-- Query modul teaching (sqlc). Semua query tenant-scoped WAJIB filter
-- school_id. Catatan: join read-only ke teachers/users (dimiliki modul
-- identity/student), classes/subjects/rooms (dimiliki modul
-- student/schedule) dipakai untuk menampilkan nama tanpa N+1 call lintas
-- modul — pola yang sama dengan internal/attendance/queries.sql &
-- internal/schedule/queries.sql. Semua TULIS ke tabel modul lain tetap TIDAK
-- PERNAH dilakukan di sini.

-- name: InsertJournal :one
INSERT INTO teaching_journals (school_id, teacher_id, schedule_slot_id, class_id, subject_id, room_id, date, started_at, flags)
VALUES (
    sqlc.arg(school_id)::bigint,
    sqlc.arg(teacher_id)::bigint,
    sqlc.narg(schedule_slot_id)::bigint,
    sqlc.arg(class_id)::bigint,
    sqlc.narg(subject_id)::bigint,
    sqlc.narg(room_id)::bigint,
    sqlc.arg(date)::date,
    sqlc.arg(started_at)::timestamptz,
    sqlc.arg(flags)::text[]
)
RETURNING *;

-- name: GetJournalByID :one
SELECT * FROM teaching_journals WHERE id = $1 AND school_id = $2;

-- name: GetJournalBySlotDate :one
SELECT * FROM teaching_journals WHERE school_id = $1 AND schedule_slot_id = $2 AND date = $3;

-- name: GetJournalRow :one
SELECT
    j.*,
    t.user_id AS teacher_user_id,
    u.name AS teacher_name,
    c.name AS class_name,
    sub.code AS subject_code,
    sub.name AS subject_name,
    r.name AS room_name
FROM teaching_journals j
JOIN teachers t ON t.id = j.teacher_id
JOIN users u ON u.id = t.user_id
JOIN classes c ON c.id = j.class_id
LEFT JOIN subjects sub ON sub.id = j.subject_id
LEFT JOIN rooms r ON r.id = j.room_id
WHERE j.id = $1 AND j.school_id = $2;

-- name: ListJournalsRow :many
SELECT
    j.*,
    t.user_id AS teacher_user_id,
    u.name AS teacher_name,
    c.name AS class_name,
    sub.code AS subject_code,
    sub.name AS subject_name,
    r.name AS room_name
FROM teaching_journals j
JOIN teachers t ON t.id = j.teacher_id
JOIN users u ON u.id = t.user_id
JOIN classes c ON c.id = j.class_id
LEFT JOIN subjects sub ON sub.id = j.subject_id
LEFT JOIN rooms r ON r.id = j.room_id
WHERE j.school_id = sqlc.arg(school_id)::bigint
  AND (sqlc.narg(teacher_id)::bigint IS NULL OR j.teacher_id = sqlc.narg(teacher_id)::bigint)
  AND (sqlc.narg(date)::date IS NULL OR j.date = sqlc.narg(date)::date)
  AND (sqlc.narg(month_start)::date IS NULL OR (j.date >= sqlc.narg(month_start)::date AND j.date < sqlc.narg(month_end)::date))
ORDER BY j.started_at DESC
LIMIT 100;

-- name: UpdateJournalNote :one
UPDATE teaching_journals SET material = $3, note = $4 WHERE id = $1 AND school_id = $2 RETURNING *;

-- name: EndJournal :one
UPDATE teaching_journals SET ended_at = $3 WHERE id = $1 AND school_id = $2 RETURNING *;

-- -- referensi lintas modul (READ-ONLY — lihat catatan di atas berkas) --

-- name: LookupRoomByToken :one
SELECT id, name FROM rooms WHERE school_id = $1 AND qr_token = $2;

-- name: LookupRoomByID :one
SELECT id, name FROM rooms WHERE school_id = $1 AND id = $2;

-- name: LookupClassRef :one
SELECT id, name FROM classes WHERE school_id = $1 AND id = $2;

-- name: LookupSubjectRef :one
SELECT id, code, name FROM subjects WHERE school_id = $1 AND id = $2;

-- name: LookupTeacherUserID :one
SELECT t.user_id FROM teachers t WHERE t.id = $1 AND t.school_id = $2;

-- name: LookupTeacherRef :one
SELECT t.id, u.name FROM teachers t JOIN users u ON u.id = t.user_id WHERE t.id = $1 AND t.school_id = $2;

-- -- settings (dibaca langsung dari school_settings module='teaching' —
-- -- tabel & pola yang sama dipakai GET/PUT /api/settings/teaching lewat
-- -- tenant.SettingsService; ini hanya baca untuk kebutuhan aturan bisnis
-- -- internal, mis. not_started_after_min — lihat CLAUDE.md "satu pola settings").

-- name: GetTeachingSettingsRaw :one
SELECT settings FROM school_settings WHERE school_id = $1 AND module = 'teaching';

-- -- rekap ketertiban mengajar (fase 7, docs/06-teaching.md "Dashboard kepala
-- -- sekolah": % slot terlaksana per guru). HANYA menyentuh tabel MILIK
-- -- teaching sendiri (teaching_journals) — jumlah slot TERJADWAL per guru
-- -- dihitung di service layer lewat ScheduleGateway.SlotsForDayOfWeek
-- -- (consumer-side interface yang sudah ada, BUKAN query SQL baru ke
-- -- schedule_slots di sini) supaya logika kepemilikan/derivasi jadwal tetap
-- -- satu tempat (modul schedule), sesuai CLAUDE.md "jangan duplikasi logika".

-- name: JournalComplianceCounts :many
SELECT
    j.teacher_id,
    COUNT(*) FILTER (WHERE j.schedule_slot_id IS NOT NULL)::bigint AS taught,
    COUNT(*) FILTER (WHERE j.schedule_slot_id IS NULL)::bigint AS unscheduled,
    COUNT(*) FILTER (WHERE j.schedule_slot_id IS NOT NULL AND COALESCE(j.material, '') <> '')::bigint AS material_filled
FROM teaching_journals j
WHERE j.school_id = sqlc.arg(school_id)::bigint
  AND j.date >= sqlc.arg(from_date)::date
  AND j.date <= sqlc.arg(to_date)::date
GROUP BY j.teacher_id;
