# 01 — Tenant: Sekolah, Domain, Tahun Ajaran, Settings, Branding

Modul: `internal/tenant/`

## Skema

```sql
schools (
  id            bigserial PRIMARY KEY,
  name          text NOT NULL,              -- "SMKN 2 Malang"
  slug          text UNIQUE NOT NULL,       -- subdomain default: {slug}.nouschool.id
  custom_domain text UNIQUE,                -- NULL = belum pakai domain sendiri
  timezone      text NOT NULL DEFAULT 'Asia/Jakarta',  -- Asia/Jakarta|Makassar|Jayapura
  status        text NOT NULL DEFAULT 'active',        -- active|suspended
  created_at    timestamptz NOT NULL DEFAULT now()
)

academic_years (
  id         bigserial PRIMARY KEY,
  school_id  bigint NOT NULL REFERENCES schools,
  name       text NOT NULL,                 -- "2026/2027"
  starts_on  date NOT NULL,
  ends_on    date NOT NULL,
  is_active  boolean NOT NULL DEFAULT false,
  UNIQUE (school_id, name)
)
-- hanya boleh 1 tahun ajaran aktif per sekolah (partial unique index WHERE is_active)

school_settings (
  school_id  bigint NOT NULL REFERENCES schools,
  module     text NOT NULL,                 -- 'attendance', 'leave', 'notification', 'branding'
  settings   jsonb NOT NULL,
  updated_by bigint,
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (school_id, module)
)
```

## Resolusi tenant (middleware pertama di chain)

1. Baca `Host` header.
2. `*.nouschool.id` → lookup `schools.slug`; selain itu → lookup `schools.custom_domain`.
3. Ketemu → `school_id`, `timezone`, `status` masuk request context. Tidak ketemu → 404 halaman netral.
4. Cache lookup di memori (TTL ~60 dtk) — jangan query DB tiap request.
5. Host `admin.nouschool.id` (atau apex) = panel super admin, bukan tenant.

## Custom domain & Caddy

- Alur: admin sekolah isi domain di pengaturan → app tampilkan instruksi "arahkan A record ke IP …" → app cek DNS berkala (resolve A record) → verified.
- Caddy On-Demand TLS: `ask http://localhost:8080/internal/check-domain?domain=...` → Go menjawab 200 hanya jika domain terdaftar & verified. Endpoint `/internal/*` hanya bind localhost.
- Caddyfile di root repo; perubahan domain TIDAK butuh restart Caddy.

## Pola settings (dipakai semua modul)

- Storage jsonb, tapi tiap modul punya struct typed + `DefaultSettings()` + `Validate()`.
- Baca: `settingsSvc.Get(ctx, schoolID, "attendance", &out)` — jika belum ada baris, kembalikan default.
- Tulis: validasi → simpan → tulis `audit_log`.
- Sekolah baru TIDAK perlu seeding settings — default berlaku implisit.

## Branding per sekolah (module `branding` di school_settings)

```json
{
  "app_name": "SMKN 2 Malang",
  "logo_url": "/uploads/{school_id}/logo.png",
  "icon_url": "/uploads/{school_id}/icon-512.png",
  "primary_color": "#0E6B4E"
}
```

- **PWA manifest dinamis**: endpoint `GET /manifest.webmanifest` menghasilkan manifest dari branding tenant (name, icons, theme_color) — jadi install di HP memakai nama & ikon sekolah. `index.html` juga inject `<meta name="theme-color">` + CSS variable `--primary` dari branding.
- Ikon: minta upload 1 gambar persegi ≥512px, server generate ukuran turunannya (192, 512, maskable).
- Frontend memakai CSS variables untuk warna — komponen tidak hardcode warna brand.
- Upload file disimpan di disk VPS `uploads/{school_id}/...` (abstraksi storage kecil supaya bisa pindah S3-compatible nanti).

## Panel super admin (minimal di fase awal)

- CRUD sekolah, set slug, lihat status domain, suspend/aktifkan.
- Set konfigurasi channel notifikasi per sekolah (lihat 08).
- Kelola langganan & verifikasi pembayaran manual (lihat 09).
- Berjalan di host `admin.nouschool.id`, hanya role `super_admin`.

## Kaitan subscription

Middleware tenant juga membaca status langganan (dari modul billing, via interface): `active` → normal; `grace` → normal + banner; `readonly` → tolak semua mutasi non-billing dengan 402/403 + pesan jelas; `suspended` → halaman info.
