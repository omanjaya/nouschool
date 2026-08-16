-- +goose Up
-- Modul discipline (Fase 14 Gelombang A, docs/12-sion-parity.md): kedisiplinan
-- siswa — master jenis pelanggaran+poin per sekolah, catat pelanggaran siswa
-- (opsional tertaut satu sesi presensi; unik per sesi+siswa+jenis), ambang
-- SP1/SP2/SP3 per tahun ajaran, dan surat peringatan OTOMATIS (snapshot
-- immutable + nomor surat unik per (sekolah, tahun ajaran, siswa, level)).

-- violation_types — master jenis pelanggaran + poin per sekolah.
CREATE TABLE violation_types (
    id        bigserial PRIMARY KEY,
    school_id bigint NOT NULL REFERENCES schools (id),
    name      text NOT NULL,
    points    int NOT NULL CHECK (points > 0),
    category  text NOT NULL DEFAULT '',
    active    boolean NOT NULL DEFAULT true,
    UNIQUE (school_id, name)
);
CREATE INDEX violation_types_school ON violation_types (school_id);

-- student_violations — satu baris = satu catatan pelanggaran siswa.
-- academic_year_id disimpan pada baris (bukan diturunkan dari occurred_on)
-- supaya perhitungan total poin & ambang SP per TA (docs/12: "ambang SP1/2/3
-- per tahun ajaran") tetap benar walau TA aktif berganti setelah baris dibuat.
CREATE TABLE student_violations (
    id                    bigserial PRIMARY KEY,
    school_id             bigint NOT NULL REFERENCES schools (id),
    academic_year_id      bigint NOT NULL REFERENCES academic_years (id),
    student_id            bigint NOT NULL REFERENCES students (id),
    violation_type_id     bigint NOT NULL REFERENCES violation_types (id),
    attendance_session_id bigint REFERENCES attendance_sessions (id),
    noted_by              bigint NOT NULL REFERENCES users (id),
    note                  text NOT NULL DEFAULT '',
    occurred_on           date NOT NULL,
    created_at            timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX student_violations_school ON student_violations (school_id);
CREATE INDEX student_violations_student ON student_violations (student_id, occurred_on);
CREATE INDEX student_violations_year ON student_violations (school_id, academic_year_id);
-- Anti dobel DALAM SATU SESI presensi (perilaku SION, docs/12): partial unique
-- HANYA berlaku bila tertaut sesi (attendance_session_id NOT NULL) — catatan
-- tanpa sesi (pelanggaran di luar jam pelajaran, mis. atribut/merokok) boleh
-- berulang kapan saja.
CREATE UNIQUE INDEX student_violations_session_unique
    ON student_violations (attendance_session_id, student_id, violation_type_id)
    WHERE attendance_session_id IS NOT NULL;

-- discipline_sp_settings — ambang poin SP1/SP2/SP3 per (sekolah, TA).
CREATE TABLE discipline_sp_settings (
    school_id        bigint NOT NULL REFERENCES schools (id),
    academic_year_id bigint NOT NULL REFERENCES academic_years (id),
    sp1              int NOT NULL,
    sp2              int NOT NULL,
    sp3              int NOT NULL,
    PRIMARY KEY (school_id, academic_year_id),
    CHECK (sp1 < sp2 AND sp2 < sp3)
);

-- discipline_letter_number_counters — counter atomik nomor surat, format
-- "SP{level}/{tahun}/{seq}". BUKAN bagian skema docs/12 (tambahan kecil,
-- pola SAMA invoice_number_counters migrations/00011_billing.sql) — BEDA:
-- counter PER SEKOLAH (bukan global lintas tenant), karena nomor surat
-- adalah dokumen resmi masing-masing sekolah, bukan dokumen platform.
CREATE TABLE discipline_letter_number_counters (
    school_id bigint NOT NULL REFERENCES schools (id),
    year      text NOT NULL,
    seq       int NOT NULL,
    PRIMARY KEY (school_id, year)
);

-- discipline_warning_letters — surat peringatan TERBIT OTOMATIS. snapshot
-- (jsonb) adalah POTRET data siswa+pelanggaran+total+ambang SAAT terbit dan
-- TIDAK PERNAH diperbarui lagi (docs/12: "surat = potret saat terbit") — PDF/
-- HTML selalu dirender dari snapshot ini, bukan dari data live. UNIQUE
-- (school_id, academic_year_id, student_id, level) menegakkan "satu surat per
-- level per siswa per TA" (auto-terbit hanya sekali per level).
CREATE TABLE discipline_warning_letters (
    id               bigserial PRIMARY KEY,
    school_id        bigint NOT NULL REFERENCES schools (id),
    academic_year_id bigint NOT NULL REFERENCES academic_years (id),
    student_id       bigint NOT NULL REFERENCES students (id),
    level            int NOT NULL CHECK (level BETWEEN 1 AND 3),
    number           text NOT NULL,
    points_snapshot  int NOT NULL,
    snapshot         jsonb NOT NULL,
    created_by       bigint REFERENCES users (id),
    created_at       timestamptz NOT NULL DEFAULT now(),
    UNIQUE (school_id, academic_year_id, student_id, level),
    UNIQUE (school_id, number)
);
CREATE INDEX discipline_warning_letters_student ON discipline_warning_letters (student_id);

-- +goose Down
DROP TABLE discipline_warning_letters;
DROP TABLE discipline_letter_number_counters;
DROP TABLE discipline_sp_settings;
DROP TABLE student_violations;
DROP TABLE violation_types;
