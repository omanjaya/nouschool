-- +goose Up
-- Fase 11 — branding runtime & PWA manifest dinamis (docs/01-tenant.md),
-- custom domain end-to-end, dan registrasi minat sekolah (landing page).
--
-- Catatan skema:
--  * Branding (app_name/primary_color/logo_path) TETAP disimpan di
--    school_settings module='branding' (jsonb, pola yang sudah ada sejak
--    Fase 1) — TIDAK butuh kolom baru di sini, hanya struct Go
--    (tenant.BrandingSettings) yang diperluas dengan field LogoPath.
--  * schools.pending_domain — domain yang sudah diisi admin sekolah tapi
--    BELUM lolos verifikasi DNS (docs/01: "app tampilkan instruksi ... app
--    cek DNS berkala ... verified"). Setelah verifikasi sukses, nilainya
--    dipindah ke custom_domain (yang sudah ada sejak migrasi 00001) dan
--    kolom ini dikosongkan lagi.
--  * interest_leads — SENGAJA TANPA school_id (platform-level): registrasi
--    minat dari calon sekolah yang BELUM jadi tenant NouSchool sama sekali,
--    diisi dari landing page publik nouschool.id (host platform).

ALTER TABLE schools ADD COLUMN pending_domain text;

-- Unik lintas sekolah (partial index — banyak sekolah boleh punya
-- pending_domain NULL sekaligus). Uniknya SILANG dengan custom_domain juga
-- ditegakkan di service layer (satu query EXISTS mengecek kedua kolom),
-- karena partial unique index Postgres tidak bisa menyilang dua kolom
-- berbeda secara langsung.
CREATE UNIQUE INDEX schools_pending_domain_unique ON schools (pending_domain) WHERE pending_domain IS NOT NULL;

CREATE TABLE interest_leads (
    id           bigserial PRIMARY KEY,
    school_name  text NOT NULL,
    contact_name text NOT NULL,
    phone        text NOT NULL,
    email        text,
    note         text,
    created_at   timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE interest_leads;
DROP INDEX schools_pending_domain_unique;
ALTER TABLE schools DROP COLUMN pending_domain;
