-- +goose Up
-- Modul schedule (jadwal pelajaran): periods, rooms (QR per ruangan),
-- schedule_slots (kelas x mapel x guru x hari x jam). Lihat docs/04-schedule.md.
-- Juga menutup FK yang tertunda dari migrasi 00004 (lihat komentar di sana):
-- attendance_sessions.schedule_slot_id -> schedule_slots(id), sekarang tabel
-- schedule_slots sudah ada.

CREATE TABLE periods (
    id         bigserial PRIMARY KEY,
    school_id  bigint NOT NULL REFERENCES schools (id),
    number     int NOT NULL,
    starts_at  time NOT NULL,
    ends_at    time NOT NULL,
    label      text,
    UNIQUE (school_id, number)
);
CREATE INDEX periods_school ON periods (school_id);

CREATE TABLE rooms (
    id         bigserial PRIMARY KEY,
    school_id  bigint NOT NULL REFERENCES schools (id),
    name       text NOT NULL,
    qr_token   text NOT NULL UNIQUE,
    UNIQUE (school_id, name)
);
CREATE INDEX rooms_school ON rooms (school_id);

CREATE TABLE schedule_slots (
    id               bigserial PRIMARY KEY,
    school_id        bigint NOT NULL REFERENCES schools (id),
    academic_year_id bigint NOT NULL REFERENCES academic_years (id),
    class_id         bigint NOT NULL REFERENCES classes (id),
    subject_id       bigint NOT NULL REFERENCES subjects (id),
    teacher_id       bigint NOT NULL REFERENCES teachers (id),
    room_id          bigint REFERENCES rooms (id),
    day_of_week      int NOT NULL,   -- 1=Senin ... 6=Sabtu
    period_start     int NOT NULL,   -- jam ke- mulai
    period_end       int NOT NULL    -- jam ke- selesai (blok 2 JP: start=3,end=4)
);
CREATE INDEX schedule_slots_school_year ON schedule_slots (school_id, academic_year_id);
CREATE INDEX schedule_slots_class_day ON schedule_slots (class_id, day_of_week);
CREATE INDEX schedule_slots_teacher_day ON schedule_slots (teacher_id, day_of_week);
CREATE INDEX schedule_slots_room_day ON schedule_slots (room_id, day_of_week);

-- FK tertunda dari migrasi 00004_attendance.sql (mode per_subject, dibangun
-- penuh fase 6) — sekarang schedule_slots ada, tutup janji itu.
ALTER TABLE attendance_sessions
    ADD CONSTRAINT attendance_sessions_schedule_slot_id_fkey
    FOREIGN KEY (schedule_slot_id) REFERENCES schedule_slots (id);

-- +goose Down
ALTER TABLE attendance_sessions DROP CONSTRAINT attendance_sessions_schedule_slot_id_fkey;
DROP TABLE schedule_slots;
DROP TABLE rooms;
DROP TABLE periods;
