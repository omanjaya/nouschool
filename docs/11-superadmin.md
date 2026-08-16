# 11 — Super Admin: Sistem Operator Platform

Super admin = **operator platform NouSchool** (pemilik SaaS), bukan pengguna operasional sekolah. Prinsip pemisahan (docs/00 #2, #4): data operasional sekolah hidup di domain sekolah; super admin mengelola *sekolah sebagai pelanggan* — onboarding, langganan, kesehatan, dukungan. Host: `admin.{BASE_DOMAIN}` / apex (dev: `localhost:5173`). Endpoint host platform dibatasi allowlist (`tenant.platformPathAllowed`) — endpoint tenant dari host platform = 404 `tenant_only_endpoint`.

## Fitur yang SUDAH ada

| Area | Fitur |
|---|---|
| Sekolah | CRUD sekolah (slug, timezone, status active/suspended), tahun ajaran (buat + aktivasi eksklusif) |
| Domain | Lihat custom/pending domain (pengajuan & verifikasi oleh admin sekolah) |
| Notifikasi | Channel per sekolah (in_app/web_push/whatsapp/email) — module settings superadmin-only |
| Billing | Plans & harga bracket (`/admin/plans`), buat/perpanjang langganan, verifikasi transfer manual, void invoice, goodwill extend, lihat bukti transfer |
| Penjualan | Leads form minat landing page (`/admin/minat`) |
| **Support** | **Impersonation "Masuk sebagai Sekolah Ini"** — token sekali-pakai 2 menit → session `admin_sekolah` atas nama super admin di domain sekolah, TTL 2 jam, tercatat audit (`admin.impersonate_issued`/`_started`), banner "Mode support" di UI sekolah, API `/api/admin/*` diblok dari konteks tenant |

## Rencana pemantapan (urutan prioritas — bangun bertahap)

### P1 — Dashboard platform (beranda `/admin`)
Halaman pertama yang dilihat super admin: ringkasan kesehatan bisnis & operasional dalam satu fetch (`GET /api/admin/overview`):
- StatTile: sekolah aktif · grace · readonly · suspended; total siswa aktif; pendapatan tahun berjalan (invoice paid); leads baru 7 hari.
- **Daftar "perlu perhatian"** (inti dashboard): invoice `awaiting_verification` (aksi langsung verifikasi), sekolah grace/readonly (aksi buka billing), sekolah tanpa tahun ajaran aktif, outbox notifikasi `dead` menumpuk per sekolah.
- Aktivitas terakhir per sekolah (login/sesi absen terakhir) — deteksi pelanggan tidak aktif.

### P2 — Onboarding wizard sekolah baru
Sekarang onboarding = 5 langkah manual tersebar. Jadikan satu alur di `/admin/schools/new`:
1. Data sekolah (nama, slug, timezone) → 2. Tahun ajaran pertama (langsung aktif) → 3. **Akun admin sekolah pertama** (nama+email/username, password sementara di-generate — endpoint baru `POST /api/admin/schools/{id}/admins`) → 4. Pilih plan → invoice pertama → 5. Ringkasan + tautan yang bisa disalin untuk dikirim ke sekolah (URL subdomain + kredensial sementara + kode aktivasi). Checklist status onboarding tampil di detail sekolah.

### P3 — Kesehatan & statistik per sekolah (di detail sekolah)
`GET /api/admin/schools/{id}/stats`: jumlah guru/siswa/rombel, sesi absen 7 hari terakhir, jurnal mengajar 7 hari, notifikasi sent/failed/dead 30 hari, login terakhir per role, storage terpakai (upload). Sekolah yang berhenti memakai produk kelihatan sebelum churn.

### P4 — Operasional & darurat
- **Reset password user sekolah** (`POST /api/admin/users/{id}/reset-password` → password sementara) — kasus paling sering: admin sekolah lupa password.
- **Audit log viewer** per sekolah (filter action/rentang) — menjawab "siapa mengubah apa", termasuk jejak impersonation.
- **Outbox notifikasi global**: daftar `failed`/`dead` lintas sekolah + retry manual.

### P5 — Komunikasi platform
- Pengumuman platform → semua sekolah (tampil sebagai announcement berlabel "NouSchool" di beranda/TV sekolah; opt-out per sekolah tidak perlu di v1).
- Broadcast email/WA ke admin sekolah (info rilis/maintenance) — memakai modul notification existing, event `platform.announcement`.

### P6 — Kontrol lanjutan
- **Feature override per sekolah** (kill switch / unlock fitur tanpa ganti plan — kolom `feature_overrides jsonb` di subscriptions, merge saat baca).
- Laporan pendapatan (per bulan, per plan, export Excel — pola export existing).
- Konfigurasi kredensial platform (SMTP/WA gateway) tetap via env — TIDAK dipindah ke UI (secret di DB = risiko; keputusan sadar).

## Aturan desain yang mengikat

1. Semua endpoint super admin di prefix `/api/admin/*`, host platform saja, `RequireSuperAdmin` (+ cek konteks platform).
2. Setiap aksi super admin yang menyentuh data sekolah → `audit_log` dengan school_id target.
3. Super admin TIDAK pernah membaca data operasional siswa langsung dari panel platform — jalur resminya impersonation (terlihat oleh sekolah). Statistik (P3) hanya agregat/angka, bukan data per-siswa.
4. UI panel admin memakai design system Rapor yang sama (tanpa branding sekolah).
