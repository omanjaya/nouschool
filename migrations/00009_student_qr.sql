-- +goose Up
-- Modul attendance — Fase 8: QR kartu siswa (docs/05-attendance.md "QR kartu
-- siswa"). Token ACAK per siswa (BUKAN NIS polos) dicetak di kartu fisik;
-- bisa dicabut (mis. kartu hilang) tanpa menghapus riwayat — baris lama tetap
-- ada dengan revoked_at terisi, token baru dibuat sebagai baris baru.

CREATE TABLE student_qr_tokens (
    id         bigserial PRIMARY KEY,
    school_id  bigint NOT NULL REFERENCES schools (id),
    student_id bigint NOT NULL REFERENCES students (id),
    token      text NOT NULL UNIQUE,
    created_at timestamptz NOT NULL DEFAULT now(),
    revoked_at timestamptz
);
CREATE INDEX student_qr_tokens_school ON student_qr_tokens (school_id);
CREATE INDEX student_qr_tokens_student ON student_qr_tokens (student_id);
-- Maksimal SATU token AKTIF (revoked_at IS NULL) per siswa pada satu waktu —
-- generate ulang/revoke selalu mencabut yang lama dulu sebelum menerbitkan
-- baru (lihat internal/attendance/service.go RevokeQRCard).
CREATE UNIQUE INDEX student_qr_tokens_active_unique
    ON student_qr_tokens (student_id) WHERE revoked_at IS NULL;

-- +goose Down
DROP TABLE student_qr_tokens;
