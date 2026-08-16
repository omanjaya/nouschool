-- +goose Up
-- Modul grading (Fase 14 Gelombang C, docs/12-sion-parity.md): penilaian per
-- kelas+mapel dengan komponen berbobot (tp/sumatif/praktik/lainnya) + KKTP,
-- nilai akhir dinormalisasi HANYA dari komponen yang siswa itu punya nilai
-- (Σ score*weight / Σ weight komponen terisi — BUKAN dibagi 100), publikasi
-- per kelas-mapel, dan bintang kelas (poin non-akademik). Toggle per sekolah
-- lewat school_settings module "grading" (tenant.NewModuleSettings).
--
-- Keputusan desain: kolom score/delta memakai numeric(5,2)/int biasa
-- (BUKAN pgtype.Numeric di sisi Go — query SELECT/INSERT selalu cast
-- eksplisit ::float8 di internal/grading/queries.sql supaya sqlc
-- menghasilkan float64 murni, lihat catatan di repository.go).

-- assessment_components — master komponen penilaian per (kelas, mapel).
CREATE TABLE assessment_components (
    id               bigserial PRIMARY KEY,
    school_id        bigint NOT NULL REFERENCES schools (id),
    academic_year_id bigint NOT NULL REFERENCES academic_years (id),
    class_id         bigint NOT NULL REFERENCES classes (id),
    subject_id       bigint NOT NULL REFERENCES subjects (id),
    name             text NOT NULL,
    type             text NOT NULL CHECK (type IN ('tp', 'sumatif', 'praktik', 'lainnya')),
    weight           int NOT NULL CHECK (weight > 0),
    kktp             int NOT NULL CHECK (kktp BETWEEN 0 AND 100),
    created_by       bigint REFERENCES users (id),
    created_at       timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX assessment_components_school ON assessment_components (school_id);
CREATE INDEX assessment_components_class_subject ON assessment_components (school_id, class_id, subject_id);

-- student_grades — satu baris = satu nilai siswa pada satu komponen. Baris
-- HANYA ada bila nilai terisi (bulk PUT dengan score=null MENGHAPUS baris,
-- bukan menyimpan NULL) — jadi kolom score selalu NOT NULL, dan "komponen
-- terisi" pada normalisasi = "baris ada".
CREATE TABLE student_grades (
    id           bigserial PRIMARY KEY,
    school_id    bigint NOT NULL REFERENCES schools (id),
    component_id bigint NOT NULL REFERENCES assessment_components (id) ON DELETE CASCADE,
    student_id   bigint NOT NULL REFERENCES students (id),
    score        numeric(5, 2) NOT NULL CHECK (score BETWEEN 0 AND 100),
    graded_by    bigint REFERENCES users (id),
    updated_at   timestamptz NOT NULL DEFAULT now(),
    UNIQUE (component_id, student_id)
);
CREATE INDEX student_grades_school ON student_grades (school_id);
CREATE INDEX student_grades_student ON student_grades (student_id);

-- grade_publications — satu baris = satu (kelas, mapel) sudah dipublikasikan
-- pada TA tsb. Publikasi HANYA mengatur visibilitas siswa/ortu (docs/12:
-- "nilai bisa diedit lalu publikasi tetap") — mutasi komponen/nilai TETAP
-- boleh berjalan setelah publikasi.
CREATE TABLE grade_publications (
    school_id        bigint NOT NULL REFERENCES schools (id),
    academic_year_id bigint NOT NULL REFERENCES academic_years (id),
    class_id         bigint NOT NULL REFERENCES classes (id),
    subject_id       bigint NOT NULL REFERENCES subjects (id),
    published_at     timestamptz NOT NULL DEFAULT now(),
    published_by     bigint REFERENCES users (id),
    PRIMARY KEY (school_id, academic_year_id, class_id, subject_id)
);

-- classroom_star_events — bintang kelas (beri/kurangi poin non-akademik +
-- catatan opsional). visibility 'private' = HANYA guru/admin yang lihat;
-- 'student' = siswa/ortu ikut lihat lewat GET /api/my-stars.
CREATE TABLE classroom_star_events (
    id               bigserial PRIMARY KEY,
    school_id        bigint NOT NULL REFERENCES schools (id),
    academic_year_id bigint NOT NULL REFERENCES academic_years (id),
    student_id       bigint NOT NULL REFERENCES students (id),
    given_by         bigint NOT NULL REFERENCES users (id),
    delta            int NOT NULL CHECK (delta != 0),
    note             text NOT NULL DEFAULT '',
    visibility       text NOT NULL CHECK (visibility IN ('private', 'student')),
    created_at       timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX classroom_star_events_school ON classroom_star_events (school_id);
CREATE INDEX classroom_star_events_student ON classroom_star_events (student_id, created_at);

-- +goose Down
DROP TABLE classroom_star_events;
DROP TABLE grade_publications;
DROP TABLE student_grades;
DROP TABLE assessment_components;
