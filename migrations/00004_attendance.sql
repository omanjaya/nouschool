-- +goose Up
-- Modul attendance (absensi siswa) — mode daily. Lihat docs/05-attendance.md.
-- Mode per_subject (schedule_slot_id, kolom "type" bernilai 'subject') dibangun
-- di Fase 6 setelah modul schedule ada. schedule_slot_id SENGAJA dibuat TANPA
-- foreign key sekarang karena tabel schedule_slots belum ada — FK ditambahkan
-- di migrasi Fase 5 (lihat docs/04-schedule.md) via ALTER TABLE.

CREATE TABLE attendance_sessions (
    id               bigserial PRIMARY KEY,
    school_id        bigint NOT NULL REFERENCES schools (id),
    academic_year_id bigint NOT NULL REFERENCES academic_years (id),
    class_id         bigint NOT NULL REFERENCES classes (id),
    date             date NOT NULL,             -- tanggal LOKAL sekolah (docs/05)
    type             text NOT NULL,              -- 'daily' | 'subject' (fase 6)
    schedule_slot_id bigint,                     -- NULL utk daily; FK ditambah fase 5
    opened_by        bigint NOT NULL REFERENCES users (id),
    status           text NOT NULL DEFAULT 'open', -- open|finalized
    created_at       timestamptz NOT NULL DEFAULT now()
);
-- Satu sesi daily per (rombel, tanggal); satu sesi per-mapel per (slot, tanggal).
CREATE UNIQUE INDEX attendance_sessions_daily_unique
    ON attendance_sessions (class_id, date) WHERE type = 'daily';
CREATE UNIQUE INDEX attendance_sessions_subject_unique
    ON attendance_sessions (schedule_slot_id, date) WHERE type = 'subject';
CREATE INDEX attendance_sessions_school_date ON attendance_sessions (school_id, date);
CREATE INDEX attendance_sessions_class ON attendance_sessions (class_id);

CREATE TABLE attendance_records (
    id         bigserial PRIMARY KEY,
    school_id  bigint NOT NULL REFERENCES schools (id),
    session_id bigint NOT NULL REFERENCES attendance_sessions (id),
    student_id bigint NOT NULL REFERENCES students (id),
    status     text NOT NULL,       -- hadir|terlambat|izin|sakit|alpa
    method     text NOT NULL,       -- manual|qr_card|self_checkin
    marked_by  bigint NOT NULL REFERENCES users (id),
    marked_at  timestamptz NOT NULL DEFAULT now(),
    meta       jsonb,               -- self_checkin: {lat,lng,accuracy,device}; qr_card: {scanner_device}
    note       text,
    UNIQUE (session_id, student_id)
);
CREATE INDEX attendance_records_school ON attendance_records (school_id);
CREATE INDEX attendance_records_school_date ON attendance_records (school_id, marked_at);
CREATE INDEX attendance_records_student_session ON attendance_records (student_id, session_id);

-- +goose Down
DROP TABLE attendance_records;
DROP TABLE attendance_sessions;
