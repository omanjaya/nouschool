-- +goose Up
-- Modul announcement (pengumuman ruang guru/dashboard TV & kepsek). Lihat
-- docs/06-teaching.md "Pengumuman". Skema PERSIS docs: starts_at/ends_at
-- adalah DATE (bukan timestamptz) — "aktif" dibandingkan terhadap tanggal
-- LOKAL sekolah (pola sama modul attendance/teaching), bukan jam.

CREATE TABLE announcements (
    id         bigserial PRIMARY KEY,
    school_id  bigint NOT NULL REFERENCES schools (id),
    title      text NOT NULL,
    body       text NOT NULL,
    starts_at  date NOT NULL,
    ends_at    date NOT NULL,
    created_by bigint NOT NULL REFERENCES users (id),
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX announcements_school_active ON announcements (school_id, starts_at, ends_at);

-- +goose Down
DROP TABLE announcements;
