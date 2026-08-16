-- +goose Up
-- Fase 14 Gelombang B2 (docs/12-sion-parity.md): QR token guru (kebalikan QR
-- kartu siswa/ruangan) + izin dispensasi keluar (rantai QR 4 tahap: piket ->
-- guru pengajar jam berjalan -> BK -> pimpinan -> issued -> gate security ->
-- exited) + izin terlambat (siswa scan QR -> piket -> pimpinan -> guru kelas
-- -> completed, aksi otomatis by hitungan telat).

-- teacher_qr_tokens — token pendek SEKALI PAKAI (TTL 60 detik) ditampilkan
-- guru di layar sendiri; siswa memindainya untuk approval alur exit-permit/
-- late-arrival (internal/teacherqr.Service.ConsumeToken, dipakai lintas
-- modul).
CREATE TABLE teacher_qr_tokens (
    id          bigserial PRIMARY KEY,
    school_id   bigint NOT NULL REFERENCES schools (id),
    user_id     bigint NOT NULL REFERENCES users (id),
    token       text NOT NULL UNIQUE,
    expires_at  timestamptz NOT NULL,
    consumed_at timestamptz,
    created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX teacher_qr_tokens_school ON teacher_qr_tokens (school_id);
CREATE INDEX teacher_qr_tokens_user ON teacher_qr_tokens (user_id);

-- student_exit_permits — izin dispensasi keluar (alur 2 dari 3 alur izin
-- siswa SION, docs/12-sion-parity.md Gelombang B): rantai QR 4 tahap ->
-- issued (gate_token kedaluwarsa otomatis akhir jam) -> scan gerbang
-- security -> exited.
CREATE TABLE student_exit_permits (
    id               bigserial PRIMARY KEY,
    school_id        bigint NOT NULL REFERENCES schools (id),
    academic_year_id bigint NOT NULL REFERENCES academic_years (id),
    student_id       bigint NOT NULL REFERENCES students (id),
    reason           text NOT NULL,
    status           text NOT NULL DEFAULT 'pending_duty_teacher'
                      CHECK (status IN ('pending_duty_teacher', 'pending_class_teacher', 'pending_bk',
                                        'pending_leadership', 'issued', 'exited', 'rejected', 'canceled')),
    duty_by          bigint REFERENCES users (id),
    duty_at          timestamptz,
    class_by         bigint REFERENCES users (id),
    class_at         timestamptz,
    bk_by            bigint REFERENCES users (id),
    bk_at            timestamptz,
    leadership_by    bigint REFERENCES users (id),
    leadership_at    timestamptz,
    rejected_by      bigint REFERENCES users (id),
    rejected_at      timestamptz,
    reject_comment   text NOT NULL DEFAULT '',
    gate_token       text UNIQUE,
    gate_expires_at  timestamptz,
    exited_by        bigint REFERENCES users (id),
    exited_at        timestamptz,
    created_at       timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX student_exit_permits_school ON student_exit_permits (school_id);
CREATE INDEX student_exit_permits_student ON student_exit_permits (student_id, status);
CREATE INDEX student_exit_permits_status ON student_exit_permits (school_id, status);
CREATE INDEX student_exit_permits_exited_at ON student_exit_permits (school_id, exited_at);

-- student_late_arrivals — izin terlambat (alur 3 dari 3 alur izin siswa
-- SION): siswa scan QR -> piket -> pimpinan -> guru kelas -> completed.
-- Maks satu record AKTIF per siswa per hari ditegakkan di service layer
-- (tanggal lokal sekolah, bukan lewat CHECK/index DB — lihat catatan
-- internal/latearrival/service.go).
CREATE TABLE student_late_arrivals (
    id               bigserial PRIMARY KEY,
    school_id        bigint NOT NULL REFERENCES schools (id),
    academic_year_id bigint NOT NULL REFERENCES academic_years (id),
    student_id       bigint NOT NULL REFERENCES students (id),
    arrived_at       timestamptz NOT NULL,
    late_count       int NOT NULL,
    action           text NOT NULL DEFAULT 'none' CHECK (action IN ('none', 'call_parent', 'send_home')),
    status           text NOT NULL DEFAULT 'pending_leadership'
                      CHECK (status IN ('pending_duty_teacher', 'pending_leadership', 'pending_class_teacher', 'completed')),
    duty_by          bigint REFERENCES users (id),
    duty_at          timestamptz,
    leadership_by    bigint REFERENCES users (id),
    leadership_at    timestamptz,
    class_by         bigint REFERENCES users (id),
    class_at         timestamptz,
    created_at       timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX student_late_arrivals_school ON student_late_arrivals (school_id);
CREATE INDEX student_late_arrivals_student ON student_late_arrivals (student_id, arrived_at);
CREATE INDEX student_late_arrivals_year ON student_late_arrivals (school_id, academic_year_id, student_id);

-- +goose Down
DROP TABLE student_late_arrivals;
DROP TABLE student_exit_permits;
DROP TABLE teacher_qr_tokens;
