-- +goose Up
-- Modul leave (izin guru dengan approval engine konfigurable). Lihat docs/07-leave.md.
-- Steps di-SNAPSHOT dari school_settings module='leave' saat pengajuan dibuat —
-- perubahan chain setelahnya TIDAK mengubah request yang sedang berjalan.

CREATE TABLE leave_requests (
    id               bigserial PRIMARY KEY,
    school_id        bigint NOT NULL REFERENCES schools (id),
    teacher_id       bigint NOT NULL REFERENCES users (id),  -- user_id pengaju (guru)
    type             text NOT NULL,             -- key dari Settings.Types saat pengajuan
    date_start       date NOT NULL,
    date_end         date NOT NULL,
    reason           text NOT NULL,
    attachment       text,                      -- path relatif (disimpan lewat platform/storage), NULL = tanpa lampiran
    attachment_name  text,                      -- nama file asli, dipakai Content-Disposition
    attachment_mime  text,                      -- content-type asli, dipakai saat serve file
    status           text NOT NULL DEFAULT 'pending', -- pending|approved|rejected|canceled
    created_at       timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX leave_requests_school ON leave_requests (school_id);
CREATE INDEX leave_requests_teacher ON leave_requests (teacher_id);
CREATE INDEX leave_requests_school_status ON leave_requests (school_id, status);
CREATE INDEX leave_requests_school_dates ON leave_requests (school_id, date_start, date_end);

CREATE TABLE leave_approval_steps (
    id               bigserial PRIMARY KEY,
    school_id        bigint NOT NULL REFERENCES schools (id),
    leave_request_id bigint NOT NULL REFERENCES leave_requests (id),
    step_order       int NOT NULL,          -- 1, 2, ... (hanya step yang TIDAK di-skip self-approval)
    approver_role    text NOT NULL,
    approver_user_id bigint REFERENCES users (id), -- terisi jika chain menunjuk orang spesifik
    decided_by       bigint REFERENCES users (id),  -- NULL = belum diputuskan
    decision         text,                  -- approved|rejected
    decided_at       timestamptz,
    comment          text,
    UNIQUE (leave_request_id, step_order)
);
CREATE INDEX leave_approval_steps_request ON leave_approval_steps (leave_request_id);
CREATE INDEX leave_approval_steps_school ON leave_approval_steps (school_id);

-- +goose Down
DROP TABLE leave_approval_steps;
DROP TABLE leave_requests;
