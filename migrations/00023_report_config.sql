-- +goose Up
-- Fase 15 GAP 1 (rapor lanjutan, docs/12-sion-parity.md — sisa konfigurasi
-- rapor SION di atas modul grading Fase 14 Gelombang C): pemetaan komponen
-- tipe 'tp' -> kode+deskripsi Tujuan Pembelajaran dipakai rapor, dan nilai
-- rapor "manual"/"sebelumnya" yang bisa menimpa nilai akhir dinormalisasi
-- (manual MENANG atas computed final; previous HANYA kolom informasi, lihat
-- internal/grading/report.go resolveReportScore).

-- report_tp_mappings — satu baris = satu komponen tipe 'tp' dipetakan ke satu
-- kode TP (mis. "10.1") + deskripsi bebas dipakai cetak rapor. UNIQUE per
-- component_id (satu komponen HANYA punya satu mapping) — replace penuh per
-- (class,subject) dilakukan di service layer (delete lalu insert ulang).
CREATE TABLE report_tp_mappings (
    id               bigserial PRIMARY KEY,
    school_id        bigint NOT NULL REFERENCES schools (id),
    academic_year_id bigint NOT NULL REFERENCES academic_years (id),
    class_id         bigint NOT NULL REFERENCES classes (id),
    subject_id       bigint NOT NULL REFERENCES subjects (id),
    component_id     bigint NOT NULL REFERENCES assessment_components (id) ON DELETE CASCADE,
    tp_code          text NOT NULL DEFAULT '',
    description      text NOT NULL DEFAULT '',
    UNIQUE (component_id)
);
CREATE INDEX report_tp_mappings_school ON report_tp_mappings (school_id);
CREATE INDEX report_tp_mappings_class_subject ON report_tp_mappings (school_id, class_id, subject_id);

-- report_manual_scores — nilai rapor "manual" (override penuh, menang atas
-- nilai akhir dinormalisasi) atau "previous" (nilai rapor semester
-- sebelumnya, HANYA kolom informasi — tidak pernah ikut resolusi/analisis).
-- UNIQUE per (academic_year_id, class_id, subject_id, student_id, kind) — satu
-- siswa hanya punya SATU baris per kind per kelas-mapel-TA.
CREATE TABLE report_manual_scores (
    id               bigserial PRIMARY KEY,
    school_id        bigint NOT NULL REFERENCES schools (id),
    academic_year_id bigint NOT NULL REFERENCES academic_years (id),
    class_id         bigint NOT NULL REFERENCES classes (id),
    subject_id       bigint NOT NULL REFERENCES subjects (id),
    student_id       bigint NOT NULL REFERENCES students (id),
    kind             text NOT NULL CHECK (kind IN ('previous', 'manual')),
    score            numeric(5, 2) NOT NULL CHECK (score BETWEEN 0 AND 100),
    note             text NOT NULL DEFAULT '',
    set_by           bigint REFERENCES users (id),
    updated_at       timestamptz NOT NULL DEFAULT now(),
    UNIQUE (academic_year_id, class_id, subject_id, student_id, kind)
);
CREATE INDEX report_manual_scores_school ON report_manual_scores (school_id);
CREATE INDEX report_manual_scores_class_subject ON report_manual_scores (school_id, class_id, subject_id);

-- +goose Down
DROP TABLE report_manual_scores;
DROP TABLE report_tp_mappings;
