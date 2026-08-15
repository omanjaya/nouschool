# 00 — Overview & Keputusan Arsitektur

## Visi

NouSchool: superapp sekolah multi-tenant (SaaS langganan tahunan) di bawah brand **Nouma**. Target awal SMA/SMK/MA. Fitur inti: absensi siswa, monitoring guru mengajar (jurnal + dashboard TV ruang guru), izin guru, dashboard kepala sekolah. Prioritas produk: **sangat ringan, sangat cepat, mobile-first (PWA)**.

Pengguna yang login: super admin (pemilik platform), admin sekolah, kepala sekolah, guru, siswa, orang tua, + akun display (TV).

## Arsitektur tingkat tinggi

```
{slug}.nouschool.id  ─┐
custom-domain.sch.id ─┼─> Caddy (On-Demand TLS) ──> Go binary (modular monolith) ──> PostgreSQL
                      │        └─ serve web/dist (React PWA)
                      └─ wildcard DNS *.nouschool.id → 1 IP VPS
```

- **Satu binary Go**, modul per domain bisnis dengan batas tegas (lihat CLAUDE.md untuk aturan).
- **Satu database PostgreSQL, shared schema** — semua tabel tenant-scoped punya kolom `school_id`. Bukan schema-per-tenant. (Postgres RLS bisa ditambah nanti sebagai lapisan kedua.)
- Tenant di-resolve dari **Host header** oleh middleware pertama → `school_id` masuk request context.
- Frontend React PWA statis, di-serve Caddy/Go; komunikasi via REST JSON `/api/...`.

## Decision log (final — jangan didesain ulang tanpa persetujuan user)

| # | Keputusan | Alasan singkat |
|---|---|---|
| 1 | Modular monolith, bukan microservices | Skala sekolah; kecepatan berkembang; batas modul disiapkan untuk pemecahan nanti |
| 2 | Multi-tenant shared schema + `school_id` | Migrasi & maintain paling gampang; RLS menyusul sebagai defense-in-depth |
| 3 | Custom domain via Caddy On-Demand TLS | SSL otomatis per domain; sekolah cukup arahkan A record; tanpa mengelola DNS sekolah |
| 4 | Cookie session per domain, bukan JWT | Lebih aman & simpel; isolasi login antar sekolah justru diinginkan |
| 5 | Role = atribut membership, bukan user | Satu orang bisa guru di sekolah A + orang tua di sekolah B |
| 6 | Permission-based authorization | `requirePerm("attendance:write")`, bukan cek role hardcoded |
| 7 | React + Vite PWA | Ekosistem terbesar; bisa turun ke Preact via alias kalau butuh bundle lebih kecil |
| 8 | pgx + sqlc, tanpa ORM | Kecepatan raw SQL + type-safety compile time |
| 9 | Absensi = konsep **sesi** (`attendance_sessions` + `attendance_records`) | Satu skema menampung mode per-hari DAN per-mapel |
| 10 | Mode absensi & metode input **konfigurable per sekolah** | Via pola tunggal `school_settings` (jsonb + struct typed) |
| 11 | Metode absen siswa: manual (wajib), QR kartu siswa, self check-in GPS | Sekolah memilih sendiri; self check-in = alat bantu, bukan anti-curang |
| 12 | Monitoring guru = jurnal mengajar via **scan QR per kelas** | Guru scan QR fisik di kelas → jurnal + buka sesi absen sekaligus; TANPA absensi jam datang/pulang guru |
| 13 | TV ruang guru pakai **akun display** (role `display`, read-only) | Pilihan user; session panjang khusus role ini |
| 14 | Approval izin guru konfigurable, steps di-**snapshot** saat pengajuan | Perubahan config tidak merusak request berjalan |
| 15 | Notifikasi: in-app, web push, WhatsApp gateway, email — pluggable, diaktifkan per sekolah oleh super admin | Outbox pattern + channel provider |
| 16 | Jadwal: import Excel **dan** builder UI dengan deteksi bentrok | Kebiasaan sekolah (Excel) + kemudahan revisi |
| 17 | Import data awal: template Excel/CSV + file export Dapodik | Tidak ada API resmi Dapodik; via file |
| 18 | Billing: langganan tahunan, **tier fitur (Basic/Pro) × bracket jumlah siswa** | Butuh feature gating + hitung siswa aktif |
| 19 | Pembayaran: transfer manual (verifikasi super admin) **dan** payment gateway (Midtrans/Xendit) dari awal | Satu model invoice, dua channel pembayaran |
| 20 | Expired: grace period 14 hari → **read-only** (bisa lihat & export, tidak bisa input) | Data tidak pernah disandera; reputasi |
| 21 | Online-only + retry queue; TANPA offline-first | Internet sekolah target stabil; sederhanakan PWA |
| 22 | Branding penuh per sekolah: logo, warna, nama app; PWA manifest per tenant | Nilai jual custom domain; install di HP = identitas sekolah |
| 23 | Timezone per sekolah (WIB/WITA/WIT); DB simpan UTC | Tanggal absensi = tanggal lokal sekolah |
| 24 | Konsep `academic_year` + `enrollments` dari hari pertama | Siswa pindah kelas tiap tahun; data historis aman |
| 25 | Audit log semua mutasi penting dari hari pertama | "Kok absen anak saya berubah?" harus terjawab |

## Role

`super_admin` (lintas sekolah, platform), `admin_sekolah`, `kepala_sekolah`, `guru` (varian tugas: wali kelas via flag di rombel, bukan role baru), `siswa`, `orang_tua`, `display` (TV, read-only).

## Urutan build

Lihat `docs/ROADMAP.md`. Prinsip: fondasi tenant+identity dulu, lalu fitur bernilai tertinggi (absensi mode daily) secepatnya bisa dipakai, mode per-mapel setelah modul jadwal.
