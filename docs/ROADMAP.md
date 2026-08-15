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

## Fase 1 — Tenant + Identity (tulang punggung) ✅
Terverifikasi end-to-end di Docker dev: login super admin (host platform) & admin sekolah (`demo.localhost`), CRUD sekolah, settings branding, 401/403 benar. UI `web/`: login, panel super admin, pengaturan branding, profil — build hijau.
Dev: `make docker-up` → `make docker-migrate` → bootstrap `go run ./cmd/bootstrap ... -demo` (admin demo: `admin`/`admin12345` di `demo.localhost:5173`). Catatan: Air wajib `poll = true` (bind mount Windows tidak meneruskan file event).
- ✅ Middleware resolusi tenant (Host → school_id, cache) + `/internal/check-domain`
- ✅ Auth: login/logout, argon2id, session cookie, rate limit
- ✅ RBAC: permission map, `requireAuth`, `requirePerm`
- ✅ Panel super admin minimal: CRUD sekolah, tahun ajaran (endpoint backend `/api/admin/schools`)
- ✅ Pola school_settings jalan end-to-end (`GET/PUT /api/settings/{module}`, module `branding`) + halaman UI `/pengaturan`
- ✅ Audit log helper (`identity.Service.Log`, dipanggil dari create/update school, activate tahun ajaran, put settings)

## Fase 2 — Student ✅
Terverifikasi end-to-end di Docker dev (`demo.localhost`): list rombel (2) & siswa (8) demo, buat siswa baru, import CSV siswa (preview+commit: create/update/error rombel salah), import CSV guru (preview+commit), generate undangan kelas (idempoten — panggil ulang menghasilkan kode sama), aktivasi kode wali → akun orang tua baru + auto-login, GET siswa sendiri (200) vs siswa lain (403). `go test ./...` hijau.
- ✅ CRUD siswa, rombel, enrollment, guru (profil), mapel
- ✅ Import Excel/CSV siswa & guru (preview → commit, idempotent)
- ✅ Undangan akun siswa/ortu/guru (generate massal + aktivasi)
- ✅ Object-level access ortu/siswa + test

## Fase 3 — Attendance mode daily (nilai tercepat) ✅
Backend terverifikasi end-to-end di Docker dev (`demo.localhost`, admin): GET
rombel+status sesi hari ini, buat sesi (idempoten), bulk isi absen (hadir/
sakit/terlambat), GET sesi, finalize (menolak bila ada yang belum diabsen),
edit setelah finalize sebagai admin (audit_log old/new tercatat), rekap
harian per rombel (angka benar), aktivasi akun orang tua dari kode undangan +
`GET /api/me/children` + riwayat absen anak sendiri (200) vs siswa lain (403).
`go build/vet/test ./...` hijau. UI guru mobile-first (Fase 3 lanjutan) belum
dikerjakan — backend siap dipakai frontend.
- ✅ Settings attendance (`GET/PUT /api/settings/attendance`) + skema
  `attendance_sessions`/`attendance_records` (migrasi `00004_attendance.sql`,
  partial unique daily/subject, `schedule_slot_id` tanpa FK — ditambah Fase 5)
- ✅ Absen manual — bulk upsert transaksional (`PUT /api/attendance/sessions/{id}/records`)
- ✅ Jendela edit (`edit_window_hours`, default admin bypass) + finalize (menolak jika ada belum diabsen) + audit (hanya perubahan record yang SUDAH ada nilainya)
- ✅ Rekap harian kelas (`GET /api/attendance/summary`) & riwayat per siswa (`GET /api/students/{id}/attendance`, perm `attendance:report` ATAU object-level ortu/siswa via `student.Service.CanViewStudent`) + `GET /api/me/children`
- ✅ UI guru mobile-first: `/absensi` (daftar kelas), `/absensi/sesi/:id` (tap-siklus status, default hadir, catatan, simpan bulk, kunci sesi, guard dirty-state), `/kehadiran` (siswa & ortu, pilih anak), `/absensi/rekap` (admin/kepsek)
- ✅ `/api/me` menyertakan `student_id` untuk role siswa; akun siswa demo `siswa`/`siswa12345` (NIS 22101) di bootstrap
- ✅ Fix proxy dev Vite: `changeOrigin: false` supaya Host `*.localhost` diteruskan (resolusi tenant di browser jalan)
- ✅ **Milestone: sekolah pertama bisa pakai absensi harian**

## Fase 4 — Leave (izin guru) ✅ (backend)
Backend terverifikasi end-to-end di Docker dev (`demo.localhost`): guru login
→ GET tipe izin → POST pengajuan (izin, besok, tanpa lampiran) → steps
ber-snapshot 1 step kepala_sekolah → kepsek login → GET antrian approval
(muncul) → decide approved → status request approved, approver_name
terisi → guru lihat scope=mine (approved) → POST pengajuan kedua lalu
cancel → summary via kepsek (1 hari izin, sakit yang dibatalkan tidak
terhitung) → guru GET approvals (200, daftar kosong — guru punya
`leave:approve` sebagai gerbang kasar RBAC docs/02, tapi tidak ada step aktif
miliknya) → upload lampiran PDF + GET file sebagai kepsek (200, Content-Type
& Content-Disposition benar) vs sebagai guru lain (403) → admin scope=all
(200) vs guru scope=all (403, butuh `leave:manage`). `go build/vet/test ./...`
hijau; test service (fake repo): snapshot immune, sequential, rejected
menghentikan chain, derivasi status, self-approval skip & auto-approve,
guard eksplisit larangan approve request sendiri, cancel hanya
pending+pemilik, validasi tanggal & settings.
- ✅ Migrasi `00005_leave.sql` (`leave_requests` + kolom attachment/attachment_name/attachment_mime, `leave_approval_steps`)
- ✅ Settings module `leave` (Types/Chain, validasi snake_case & role chain) terdaftar di `tenant.NewModuleSettings`
- ✅ `internal/leave/`: GET tipe, POST pengajuan (multipart + lampiran pdf/jpg/png max 5MB via `platform/storage`), snapshot chain → steps saat submit, self-approval skip + auto-approve saat chain habis (audit `leave.auto_approved_self_chain`), GET daftar (scope mine/all), cancel (pemilik+pending), antrian approval, decide berurutan (audit `leave.decide`), rekap `/api/leave/summary` (perm `leave:manage` atau role `kepala_sekolah`), serve lampiran `/api/files/leave/{id}/attachment` (pengaju/approver-di-chain/`leave:manage`)
- ✅ Interface publik `Service.ApprovedOn` (dipakai modul teaching, Fase 6)
- ✅ Bootstrap: akun kepala sekolah demo (`kepsek`/`kepsek12345`) + password guru demo (`rendi@demo.sch.id`/`guru12345`) di-upsert idempoten setiap run
- ✅ UI frontend (`web/`, dibangun terhadap kontrak API di `docs/07-leave.md`): `/izin` (daftar pengajuan sendiri + filter status + form ajukan izin dengan lampiran), `/izin/:id` (detail + timeline persetujuan + batalkan), `/izin/persetujuan` + `/izin/persetujuan/:stepId` (antrian approver, setujui/tolak dengan komentar wajib saat tolak), `/izin/rekap` (kepsek/admin, per rentang), seksi "Alur Persetujuan Izin" + "Jenis Izin" di `/pengaturan` (admin). `npm run build` & `npm run lint` hijau. Backend sekarang sudah ada (baris di atas) — UI ini belum diverifikasi ulang end-to-end terhadap backend nyata pada sesi ini (fokus sesi ini backend + verifikasi curl).

## Fase 5 — Schedule ✅
Backend terverifikasi end-to-end di Docker dev (`demo.localhost`): login admin
→ GET periods (9 period bootstrap: 8 KBM + Istirahat jam ke-5) → GET rooms
(qr_token tampil utk admin) + GET rooms/{id}/qr.png (200, image/png, 512x512)
→ GET slots XII RPL 1 (10 slot Senin-Jumat terisi, tanpa bentrok) → POST slot
bentrok guru (422, "Bentrok: Rendi Saputra sudah mengajar XII RPL 1 jam
ke-1–2 Senin.") → POST slot valid baru (201) → PATCH & DELETE slot (200) →
import CSV (1 valid, 1 bentrok, 1 mapel tak dikenal) preview (summary
create=1/error=2) + commit (created=1, skipped=2) → siswa login: GET slots
class_id miliknya (200) vs class_id lain (403) → DELETE ruangan yang masih
dipakai jadwal (409) → PUT periods yang masih dipakai jadwal (409) → POST
copy jadwal (skip-dan-laporkan bentrok bekerja: guru sumber otomatis bentrok
dgn dirinya sendiri saat disalin ke kelas baru di TA yang sama, seluruh baris
terlapor di `skipped[]` dengan alasan). `go build/vet/test ./...` hijau; test
service (fake repo): deteksi bentrok guru/kelas/ruang (termasuk beririsan
sebagian & TA berbeda tidak bentrok), validasi periods replace-all
(overlap/urutan/dipakai-slot → 409), CurrentPeriod (dalam jam/di luar
jam/timezone WIB vs WIT), copy skip-bentrok, parser import (nama hari vs
angka, referensi tak dikenal, header wajib hilang).
- ✅ Migrasi `00006_schedule.sql` (`periods`, `rooms` qr_token unik acak,
  `schedule_slots` dgn FK lengkap) + menutup FK tertunda
  `attendance_sessions.schedule_slot_id → schedule_slots(id)` (dijanjikan di
  migrasi 00004)
- ✅ `internal/schedule/`: periods (GET + PUT replace-all transaksional,
  validasi unik/urut/overlap, tolak hapus period yg dipakai slot → 409),
  rooms (CRUD, qr_token hanya utk `schedule:manage`, regenerate-qr, PNG QR
  512x512 via `github.com/skip2/go-qrcode`, tolak hapus ruangan dipakai slot
  → 409), slots (CRUD + deteksi bentrok guru/kelas/ruang DALAM TRANSAKSI
  repository — `CreateSlotChecked`/`UpdateSlotChecked`; object-level siswa
  hanya rombelnya sendiri; guru bebas baca), copy jadwal (skip bentrok +
  laporan), import Excel/CSV (preview→commit in-memory TTL 15 menit, pola
  sama modul student), query kunci publik `SlotNow`/`SlotsToday`/
  `CurrentPeriod` + endpoint `/api/schedule/today` & `/api/schedule/current-period`
- ✅ Interface publik `student.Service.MyClassID` (baru, dipakai schedule
  object-level siswa lewat consumer-side interface `StudentClassLookup`)
- ✅ Bootstrap idempoten: 9 period demo, 3 ruangan (R-101, R-102, Lab
  Komputer 1), jadwal XII RPL 1 & XI RPL 2 Senin-Jumat (guru Rendi: Basis
  Data & Pemrograman Web; guru Sari: Matematika), tanpa bentrok
- ✅ Builder UI grid per kelas (editable) & per guru (read-only) + error bentrok 422 inline,
  halaman rooms/periods/import di `web/` — belum dikerjakan sesi ini (fokus
  backend + verifikasi curl, sesuai batasan tugas)

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
