-- +goose Up
-- Modul substitution (Fase 14 Gelombang D, docs/12-sion-parity.md): guru
-- pengganti — request/accept/reject/cancel per slot+tanggal, dikonsumsi
-- modul teaching (scan QR pengganti diizinkan) & monitoring/TV (teacher_name
-- "{pengganti} (pengganti)") lewat consumer-side interface SubstitutionLookup.
CREATE TABLE teacher_substitution_requests (
    id                  bigserial PRIMARY KEY,
    school_id           bigint NOT NULL REFERENCES schools (id),
    schedule_slot_id    bigint NOT NULL REFERENCES schedule_slots (id),
    date                date NOT NULL,
    requested_by        bigint NOT NULL REFERENCES users (id),
    substitute_user_id  bigint NOT NULL REFERENCES users (id),
    reason              text NOT NULL DEFAULT '',
    status              text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'accepted', 'rejected', 'canceled')),
    decided_at          timestamptz,
    created_at          timestamptz NOT NULL DEFAULT now()
);
-- Hanya SATU permintaan aktif (pending/accepted) per (slot, tanggal) — request
-- lain boleh dibuat lagi setelah yang lama rejected/canceled.
CREATE UNIQUE INDEX teacher_substitution_requests_active_unique
    ON teacher_substitution_requests (schedule_slot_id, date)
    WHERE status IN ('pending', 'accepted');
CREATE INDEX teacher_substitution_requests_school ON teacher_substitution_requests (school_id);
CREATE INDEX teacher_substitution_requests_substitute ON teacher_substitution_requests (school_id, substitute_user_id);
CREATE INDEX teacher_substitution_requests_requester ON teacher_substitution_requests (school_id, requested_by);

-- +goose Down
DROP TABLE teacher_substitution_requests;
