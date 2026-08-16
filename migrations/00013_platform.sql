-- +goose Up
-- Fase 13 Gelombang 2 (docs/11-superadmin.md P5 + P6):
--   - platform_announcements: pengumuman platform ("NouSchool") tampil di
--     SEMUA sekolah (P5) — terpisah dari announcements (per-sekolah, migrasi
--     00008) karena TIDAK punya school_id (satu baris = tampil di semua).
--   - subscriptions.feature_overrides: kill switch/unlock fitur per sekolah
--     tanpa ganti plan (P6.1) — di-MERGE dengan snapshot plan.features saat
--     resolusi fitur (menang atas snapshot plan), lihat internal/billing
--     mergeFeatures/snapshotFor. Satu migrasi untuk P5+P6 sesuai instruksi
--     tugas (P5 dan P6 gelombang yang sama).

CREATE TABLE platform_announcements (
    id         bigserial PRIMARY KEY,
    title      text NOT NULL,
    body       text NOT NULL,
    starts_at  date NOT NULL,
    ends_at    date NOT NULL,
    created_by bigint REFERENCES users (id),
    created_at timestamptz NOT NULL DEFAULT now()
);
-- Dipakai query "aktif pada tanggal X" (P5, integrasi /api/announcements &
-- tv/board) — pola index sama dengan announcements (migrasi 00008 tidak
-- punya index rentang eksplisit karena volume kecil per sekolah; di sini
-- volume TOTAL lintas-semua-sekolah relatif kecil juga, tapi query jalan tiap
-- request /api/announcements & tv/board jadi tetap diberi index).
CREATE INDEX platform_announcements_range ON platform_announcements (starts_at, ends_at);

ALTER TABLE subscriptions ADD COLUMN feature_overrides jsonb NOT NULL DEFAULT '{}';

-- +goose Down
ALTER TABLE subscriptions DROP COLUMN feature_overrides;
DROP TABLE platform_announcements;
