-- +goose Up
-- Modul teaching (jurnal mengajar via scan QR ruangan + monitoring status
-- mengajar guru). Lihat docs/06-teaching.md. Skema PERSIS docs: journal
-- terikat ke schedule_slots (nullable — NULL utk entry unscheduled/manual).
-- UNIQUE (schedule_slot_id, date) HANYA berlaku utk journal yang punya slot
-- (partial index WHERE schedule_slot_id IS NOT NULL) karena entry unscheduled
-- (guru pengganti/insidental) boleh banyak per kelas per hari.

CREATE TABLE teaching_journals (
    id               bigserial PRIMARY KEY,
    school_id        bigint NOT NULL REFERENCES schools (id),
    teacher_id       bigint NOT NULL REFERENCES teachers (id),
    schedule_slot_id bigint REFERENCES schedule_slots (id),  -- NULL jika unscheduled
    class_id         bigint NOT NULL REFERENCES classes (id),
    subject_id       bigint REFERENCES subjects (id),
    room_id          bigint REFERENCES rooms (id),           -- ruang AKTUAL (dari QR yang discan)
    date             date NOT NULL,                          -- tanggal LOKAL sekolah
    started_at       timestamptz NOT NULL,                   -- waktu scan / entry dibuat
    ended_at         timestamptz,
    material         text,
    note             text,
    flags            text[] NOT NULL DEFAULT '{}'            -- {room_mismatch, unscheduled}
);
CREATE UNIQUE INDEX teaching_journals_slot_date_unique
    ON teaching_journals (schedule_slot_id, date) WHERE schedule_slot_id IS NOT NULL;
CREATE INDEX teaching_journals_school_date ON teaching_journals (school_id, date);
CREATE INDEX teaching_journals_teacher_date ON teaching_journals (teacher_id, date);
CREATE INDEX teaching_journals_class_date ON teaching_journals (class_id, date);

-- +goose Down
DROP TABLE teaching_journals;
