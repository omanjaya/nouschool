-- +goose Up
-- Fase 14 Gelombang B1 (docs/12-sion-parity.md): fondasi tugas tambahan +
-- capability flags (adopsi pola SION — otorisasi alur izin membaca FLAGS,
-- bukan role), role pegawai (staff non-guru), dan izin terencana siswa (alur
-- 1 dari 3 alur izin siswa SION: siswa -> wali kelas -> BK terbitkan surat ->
-- verifikasi publik).

-- duties — tugas tambahan (Wali Kelas, Guru BK, Guru Piket, Pimpinan,
-- Security, dst) per sekolah. flags menentukan kapabilitas apa yang dibawa
-- tugas ini (dibaca modul lain lewat internal/duty.Service.UserHasFlag,
-- BUKAN dicek berdasarkan role langsung).
CREATE TABLE duties (
    id        bigserial PRIMARY KEY,
    school_id bigint NOT NULL REFERENCES schools (id),
    name      text NOT NULL,
    for_role  text NOT NULL CHECK (for_role IN ('guru', 'pegawai')),
    flags     text[] NOT NULL DEFAULT '{}',
    active    boolean NOT NULL DEFAULT true,
    UNIQUE (school_id, name)
);
CREATE INDEX duties_school ON duties (school_id);

-- duty_assignments — siapa memegang tugas apa PER TAHUN AJARAN (tugas bisa
-- berpindah tangan tiap TA, mis. wali kelas ganti tiap tahun).
CREATE TABLE duty_assignments (
    id               bigserial PRIMARY KEY,
    school_id        bigint NOT NULL REFERENCES schools (id),
    duty_id          bigint NOT NULL REFERENCES duties (id),
    user_id          bigint NOT NULL REFERENCES users (id),
    academic_year_id bigint NOT NULL REFERENCES academic_years (id),
    UNIQUE (duty_id, user_id, academic_year_id)
);
CREATE INDEX duty_assignments_school ON duty_assignments (school_id);
CREATE INDEX duty_assignments_user ON duty_assignments (user_id, academic_year_id);
CREATE INDEX duty_assignments_duty_year ON duty_assignments (duty_id, academic_year_id);

-- employees — profil pegawai (staff non-guru, mis. security/tata usaha) —
-- pola yang SAMA dengan `teachers` (profil tipis di atas `users`+membership).
CREATE TABLE employees (
    id        bigserial PRIMARY KEY,
    school_id bigint NOT NULL REFERENCES schools (id),
    user_id   bigint NOT NULL REFERENCES users (id),
    nip       text NOT NULL DEFAULT '',
    UNIQUE (school_id, user_id)
);
CREATE INDEX employees_school ON employees (school_id);

-- student_leave_requests — izin terencana siswa (alur 1 dari 3 alur izin
-- siswa SION, docs/12-sion-parity.md Gelombang B): siswa/ortu ajukan -> wali
-- kelas -> BK terbitkan surat bernomor -> verifikasi publik via verify_token.
CREATE TABLE student_leave_requests (
    id               bigserial PRIMARY KEY,
    school_id        bigint NOT NULL REFERENCES schools (id),
    academic_year_id bigint NOT NULL REFERENCES academic_years (id),
    student_id       bigint NOT NULL REFERENCES students (id),
    submitted_by     bigint NOT NULL REFERENCES users (id),
    type             text NOT NULL CHECK (type IN ('sakit', 'izin')),
    date_start       date NOT NULL,
    date_end         date NOT NULL,
    reason           text NOT NULL,
    attachment       text NOT NULL DEFAULT '',
    attachment_name  text NOT NULL DEFAULT '',
    attachment_mime  text NOT NULL DEFAULT '',
    status           text NOT NULL DEFAULT 'pending_homeroom'
                      CHECK (status IN ('pending_homeroom', 'pending_bk', 'issued', 'rejected', 'canceled')),
    letter_number    text,
    verify_token     text UNIQUE,
    homeroom_decided_by  bigint REFERENCES users (id),
    homeroom_decided_at  timestamptz,
    homeroom_comment     text NOT NULL DEFAULT '',
    bk_decided_by        bigint REFERENCES users (id),
    bk_decided_at        timestamptz,
    bk_comment           text NOT NULL DEFAULT '',
    created_at       timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX student_leave_requests_school ON student_leave_requests (school_id);
CREATE INDEX student_leave_requests_student ON student_leave_requests (student_id);
CREATE INDEX student_leave_requests_status ON student_leave_requests (school_id, status);
-- Nomor surat unik per sekolah (partial — hanya baris issued yang punya nomor).
CREATE UNIQUE INDEX student_leave_requests_letter_number_unique
    ON student_leave_requests (school_id, letter_number)
    WHERE letter_number IS NOT NULL;

-- student_leave_number_counters — counter atomik nomor surat "SI/{tahun}/{seq}"
-- PER SEKOLAH PER TAHUN (pola yang SAMA dengan discipline_letter_number_counters,
-- migrations/00014_discipline.sql).
CREATE TABLE student_leave_number_counters (
    school_id bigint NOT NULL REFERENCES schools (id),
    year      text NOT NULL,
    seq       int NOT NULL,
    PRIMARY KEY (school_id, year)
);

-- +goose Down
DROP TABLE student_leave_number_counters;
DROP TABLE student_leave_requests;
DROP TABLE employees;
DROP TABLE duty_assignments;
DROP TABLE duties;
