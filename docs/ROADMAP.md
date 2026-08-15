# ROADMAP — Urutan Build & Status

Status: ⬜ belum · 🚧 sedang dikerjakan · ✅ selesai. **Update file ini setiap menyelesaikan pekerjaan.** Setiap fase menghasilkan sesuatu yang bisa dipakai/didemokan.

## Fase 0 — Fondasi teknis ✅
- ✅ Scaffold repo: struktur folder sesuai CLAUDE.md, Makefile, .env.example, git init + remote GitHub (omanjaya/nouschool)
- ✅ `platform/`: config, database (pgx pool), httpx (error & response), clock, middleware (Recover, Logger, SecurityHeaders)
- ✅ Setup goose + migrasi 00001 (schools, academic_years, school_settings, users, memberships, sessions, invitations, audit_log) — belum dijalankan ke DB (Postgres belum disiapkan di mesin dev)
- ✅ Setup sqlc + query pertama (tenant) — generate via `make sqlc`
- ✅ Scaffold `web/`: Vite + React 19 + TS + Tailwind v4 (token Rapor) + TanStack Query + vite-plugin-pwa + AppShell/Button/Card/ListRow/EmptyState/Skeleton; `npm run build` hijau
- ✅ Caddyfile dev & prod (on-demand TLS, template)

Catatan mesin dev (Windows): port 7929–8171 direserve Hyper-V → backend dev pakai PORT=8210; Vite proxy `/api` → 8210.

## Fase 1 — Tenant + Identity (tulang punggung) ⬜
- ⬜ Middleware resolusi tenant (Host → school_id, cache) + `/internal/check-domain`
- ⬜ Auth: login/logout, argon2id, session cookie, rate limit
- ⬜ RBAC: permission map, `requireAuth`, `requirePerm`
- ⬜ Panel super admin minimal: CRUD sekolah, tahun ajaran
- ⬜ Halaman pengaturan sekolah (kerangka) + pola school_settings jalan end-to-end
- ⬜ Audit log helper

## Fase 2 — Student ⬜
- ⬜ CRUD siswa, rombel, enrollment, guru (profil), mapel
- ⬜ Import Excel/CSV siswa & guru (preview → commit, idempotent)
- ⬜ Undangan akun siswa/ortu/guru (generate massal + aktivasi)
- ⬜ Object-level access ortu/siswa + test

## Fase 3 — Attendance mode daily (nilai tercepat) ⬜
- ⬜ Settings attendance + sesi/record skema
- ⬜ Absen manual (UI guru mobile-first, bulk save)
- ⬜ Jendela edit + finalize + audit
- ⬜ Rekap harian kelas & riwayat per siswa (view ortu/siswa)
- ⬜ **Milestone: sekolah pertama bisa pakai absensi harian**

## Fase 4 — Leave (izin guru) ⬜
- ⬜ Settings chain + snapshot approval steps
- ⬜ Pengajuan (+ lampiran), antrian approver, keputusan berurutan
- ⬜ Rekap izin per guru

## Fase 5 — Schedule ⬜
- ⬜ Periods, rooms (+ QR token & cetak QR ruangan), subjects
- ⬜ Import jadwal Excel (preview + deteksi bentrok)
- ⬜ Builder UI grid (per kelas & per guru) + deteksi bentrok real-time
- ⬜ Query SlotNow/SlotsToday/CurrentPeriod

## Fase 6 — Attendance per-mapel + Teaching ⬜
- ⬜ Mode per_subject (sesi dari slot jadwal)
- ⬜ Scan QR ruangan → jurnal mengajar + buka sesi absen (satu aksi)
- ⬜ Jurnal mengajar (materi, flags, riwayat)
- ⬜ Status mengajar derivasi (mengajar/belum masuk/izin/belum mulai)

## Fase 7 — Dashboard TV + Kepsek ⬜
- ⬜ Akun display (role, session panjang) + halaman TV fullscreen `/tv`
- ⬜ Panel: status guru mengajar, jam & countdown, pengumuman, rekap absen hari ini (`/api/tv/board` satu fetch, polling)
- ⬜ Modul announcements (CRUD admin/kepsek)
- ⬜ Dashboard kepsek interaktif + drill-down + rekap ketertiban mengajar

## Fase 8 — Metode absen lanjutan ⬜
- ⬜ QR kartu siswa: token per siswa, mode scan guru, generator kartu PDF
- ⬜ Self check-in GPS: rule settings, jendela waktu, validasi radius, deteksi anomali untuk wali kelas

## Fase 9 — Notification ⬜
- ⬜ Outbox + worker + retry; in-app inbox (baseline)
- ⬜ Web Push (VAPID + service worker)
- ⬜ WhatsApp gateway (implementasi pertama: HTTP gateway sederhana)
- ⬜ Email SMTP (+ reset password)
- ⬜ Konfigurasi channel per sekolah di panel super admin; event awal terpasang (absent → ortu, leave → guru)

## Fase 10 — Billing ⬜
- ⬜ Plans + plan_prices + seeding Basic/Pro & bracket
- ⬜ Subscriptions (snapshot), lifecycle job (grace → readonly), enforcement middleware
- ⬜ Feature gating `requireFeature` + daftar fitur di `/api/me`
- ⬜ Invoice + PDF, transfer manual (upload bukti + verifikasi super admin)
- ⬜ Payment gateway (satu provider dulu) + webhook idempotent
- ⬜ Panel billing super admin

## Fase 11 — Branding & polish SaaS ⬜
- ⬜ Branding per sekolah (logo, warna, nama) + PWA manifest dinamis + CSS variables
- ⬜ Custom domain end-to-end (form, verifikasi DNS, on-demand TLS production)
- ⬜ Import Dapodik (parser adaptor)
- ⬜ Export Excel laporan (absensi bulanan, izin)
- ⬜ Backup otomatis pg_dump harian + restore drill
- ⬜ Landing page nouschool.id + registrasi minat sekolah

## Ide tertunda (JANGAN dikerjakan tanpa keputusan user)
- Surat izin siswa dari ortu → status absen; kuota cuti guru; custom role/permission di DB; WebSocket realtime TV; opt-out notifikasi per user; RLS Postgres; PKL/magang SMK; SPP/pembayaran siswa; rapor.
