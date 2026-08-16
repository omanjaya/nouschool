-- +goose Up
-- Modul period_day_overrides (Fase 14 Gelombang D, docs/12-sion-parity.md):
-- jam khusus per hari (mis. Jumat lebih pendek) — set periods ALTERNATIF utk
-- satu day_of_week tertentu, dipakai loader periods day-aware
-- (internal/schedule) sebagai pengganti periods default HANYA pada hari itu.
CREATE TABLE period_day_overrides (
    id          bigserial PRIMARY KEY,
    school_id   bigint NOT NULL REFERENCES schools (id),
    day_of_week int NOT NULL CHECK (day_of_week BETWEEN 0 AND 6),
    number      int NOT NULL CHECK (number > 0),
    starts_at   time NOT NULL,
    ends_at     time NOT NULL,
    label       text NOT NULL DEFAULT '',
    UNIQUE (school_id, day_of_week, number)
);
CREATE INDEX period_day_overrides_school_day ON period_day_overrides (school_id, day_of_week);

-- +goose Down
DROP TABLE period_day_overrides;
