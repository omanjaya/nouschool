-- +goose Up
-- Modul counseling (Fase 14 Gelombang D, docs/12-sion-parity.md): sesi
-- konseling BK — tujuan karir, deskripsi masalah, rencana tindak lanjut, +
-- bukti foto/dokumen opsional. Privat BK: siswa/ortu TIDAK bisa mengakses
-- (beda dari discipline yang siswa/ortu boleh lihat poin sendiri).
CREATE TABLE counselings (
    id                   bigserial PRIMARY KEY,
    school_id            bigint NOT NULL REFERENCES schools (id),
    academic_year_id     bigint NOT NULL REFERENCES academic_years (id),
    student_id           bigint NOT NULL REFERENCES students (id),
    counselor_id         bigint NOT NULL REFERENCES users (id),
    career_goals         text NOT NULL DEFAULT '',
    problem_description  text NOT NULL DEFAULT '',
    follow_up_plan       text NOT NULL DEFAULT '',
    evidence             text,
    evidence_name        text,
    evidence_mime        text,
    created_at           timestamptz NOT NULL DEFAULT now(),
    updated_at           timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX counselings_school ON counselings (school_id);
CREATE INDEX counselings_student ON counselings (school_id, student_id);
CREATE INDEX counselings_counselor ON counselings (school_id, counselor_id);

-- +goose Down
DROP TABLE counselings;
