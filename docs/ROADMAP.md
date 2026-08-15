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

## Fase 6 — Attendance per-mapel + Teaching ✅ (backend)
Backend terverifikasi end-to-end di Docker dev (`demo.localhost`): guru login
→ scan QR di luar jam period (dini hari) → `needs_manual:true` + room terisi,
TANPA journal dibuat → POST jurnal unscheduled (room_id+class_id+subject_id)
→ 201, flag `unscheduled`, status `ongoing` → PATCH material+note → 200 →
POST end → `ended_at` terisi, status `done` → GET journals scope=mine →
1 baris → GET `/api/teaching/status?date=2026-08-14` (Jumat lalu) sebagai
admin & kepsek → 200, 4 slot XII RPL 1/XI RPL 2 SEMUA `belum_masuk` (tanggal
lampau tanpa journal, sesuai "tetap merah di rekap") → sebagai guru → 403
(guru TIDAK punya `teaching:monitor`, dikonfirmasi rbac docs/02) → GET
`/api/attendance/slots-today` (guru Rendi, hari Minggu = 2 slot yang di-seed
`ensureDemoTodaySlots`) → 1 slot miliknya (Sari tidak muncul), `session:null`
→ POST `/api/attendance/sessions {schedule_slot_id}` → 201, sesi type
`subject`, roster 6 siswa → coba buka slot Rendi sebagai guru LAIN (Sari) →
403 (object-level "slot milik guru itu kecuali admin") → PUT records 6 siswa
→ GET slots-today → `marked_count:6/total:6` → GET/PUT `/api/settings/teaching`
(`not_started_after_min`) → 200. Jalur "slot ketemu" (scan saat jam pelajaran
berjalan) tidak bisa diuji live pada sesi ini (dini hari, di luar semua jam
period) — diuji lewat unit test `clock.Fixed` (cukup, sesuai instruksi
tugas). `go build/vet/test ./...` hijau; test service (fake repo/gateway):
scan slot ketemu (journal+sesi dibuka), scan idempoten (journal & sesi sama),
flag `room_mismatch`, `needs_manual` tanpa slot, derivasi status jurnal
(ongoing→done saat lewat period_end), derivasi status monitoring SEMUA
cabang (mengajar/selesai/izin/belum_mulai×2/belum_masuk×2, termasuk grace
period `not_started_after_min` & prioritas izin di atas belum_masuk), jendela
edit jurnal H+2 (guru ditolak lewat H+2, admin bebas), akses PATCH (guru lain
ditolak, pemilik/admin boleh), object-level `attendance.CreateSession`
(schedule_slot_id) & `SlotsToday` guru.
- ✅ Migrasi `00007_teaching.sql` (`teaching_journals`, `flags text[]`,
  UNIQUE `(schedule_slot_id, date)` partial WHERE NOT NULL — unscheduled
  boleh banyak per hari)
- ✅ Settings module `teaching` (`not_started_after_min`, default 10) +
  terdaftar `tenant.NewModuleSettings`
- ✅ `internal/teaching/`: `POST /api/teaching/scan` (resolve room by
  token/id → `SlotNow` → slot ketemu: journal + `attendance.OpenSubjectSession`
  sekaligus, idempoten per slot+tanggal, flag `room_mismatch` bila ruang
  aktual ≠ ruang slot; slot tidak ketemu: `needs_manual` TANPA journal),
  `POST /api/teaching/journals` (entry unscheduled, flag `unscheduled`, TANPA
  sesi absen otomatis), `PATCH .../journals/{id}` (pemilik/admin, jendela
  H+2), `POST .../journals/{id}/end` (pemilik saja), `GET /api/teaching/journals`
  (scope mine/all, filter date/month, max 100), `GET /api/teaching/status`
  (derivasi lintas guru per slot hari itu: mengajar/belum_masuk/izin/
  belum_mulai/selesai + `current_period` + `summary`) — semua konsumsi
  lintas modul lewat consumer-side interface primitif
  (`ScheduleGateway`/`LeaveGateway`/`AttendanceGateway`/`StudentGateway`);
  `teaching.SlotInfo` (tipe lokal, bukan tipe schedule) dijembatani dari
  `*schedule.Service` oleh adapter `scheduleForTeaching` di
  `cmd/server/scheduleadapter.go` (satu-satunya tempat yang boleh mengimpor
  kedua modul — keputusan didokumentasikan di sana: data yang dibutuhkan
  terlalu kaya utk primitif langsung seperti modul lain)
- ✅ Interface publik baru `attendance.Service.OpenSubjectSession` (create-or-get
  sesi subject dari slot, dipakai teaching), `attendance.Service.CreateSession`
  diperluas terima `{schedule_slot_id}` (alternatif `{class_id}`, object-level
  "slot milik guru kecuali admin" via consumer-side interface
  `ScheduleSlotLookup`/`TeacherLookup`), `GET /api/attendance/slots-today`
  (slot jadwal guru hari ini + status sesi subject, via
  `ScheduleSlotLookup.SlotsTodayForTeacher` dijembatani adapter
  `scheduleForAttendance`)
- ✅ Interface publik baru `schedule.Service`: `SlotByID`,
  `SlotsForDayOfWeek` (semua slot lintas guru pada satu hari, dipakai
  monitoring), `SlotOwnership` (primitif langsung, tanpa adapter)
- ✅ **Keputusan: hari Minggu (day_of_week=0) kini valid** di `schedule`
  (sebelumnya hanya Senin–Sabtu 1–6) — diotorisasi eksplisit di scope kerja
  fase 6 supaya bootstrap bisa menyeed slot pada HARI INI apa pun harinya
  untuk verifikasi e2e reproducible; lihat komentar `dayNames` di
  `internal/schedule/model.go`
- ✅ **Keputusan: mode absensi tidak berubah otomatis** — sesi `subject`
  boleh dibuat server-side apapun mode `attendance.Settings.Mode` sekolah
  (daily/per_subject); mode hanya menentukan default UI, bukan gerbang
  server. Demo tetap `daily`.
- ✅ Bootstrap idempoten: `ensureDemoTodaySlots` — 2 slot tambahan pada
  day_of_week HARI INI (waktu lokal sekolah, termasuk Minggu) utk XII RPL 1,
  skip bila kelas itu sudah punya slot hari itu (mis. Senin–Jumat sudah
  terisi `ensureDemoSchedule`)
- ⬜ UI frontend (belum dikerjakan sesi ini — fokus backend + verifikasi
  curl, sesuai batasan tugas)

## Fase 7 — Dashboard TV + Kepsek ✅
Backend terverifikasi end-to-end di Docker dev (`demo.localhost`): bootstrap
ulang (idempoten, akun `display`/`display12345` dibuat) → login display →
`GET /api/tv/board` → 200, berisi teaching summary (`belum_mulai:2` — 2 slot
hari ini blm mulai jam 03:10 WIB) + `current_period:null` + `next_starts_at`
+ rekap absen hari ini per kelas (3 kelas, termasuk sesi finalized & open) +
`announcements:[]` → display coba `GET /api/students` → 403 dan
`POST /api/announcements` → 403 (read-only benar, `teaching:monitor`/
`schedule:read` saja) → display `GET /api/announcements?active=1` → 200
kosong; tanpa `active` → 403 (butuh `announcement:manage`) → login kepsek →
`POST /api/announcements` pengumuman aktif hari ini → 201 → `GET /api/tv/board`
→ pengumuman muncul (`{id,title,body}`) → `PATCH` → 200 (judul/isi/rentang
berubah) → `DELETE` → 200, hilang dari `?active=1` → `GET /api/teaching/compliance?from=2026-08-09&to=2026-08-16`
→ 200, 2 guru (Rendi `scheduled:13 taught:0 unscheduled:1`, Sari
`scheduled:12 taught:0`) — cocok dgn data riil `teaching_journals` (1 baris
journal `unscheduled` tanggal hari ini, TIDAK ada journal berjadwal karena
jalur "slot ketemu" fase 6 hanya diuji lewat unit test, bukan curl live —
lihat catatan fase 6) → cek `sessions.expires_at` display via psql: ~365 hari
ke depan vs kepsek ~30 hari (`sessionTTLForRole`/`sessionRenewWindowForRole`
baru, dipakai Login/IssueSession/RequireAuth renewal — ketiganya HARUS pakai
fungsi per-role, bukan konstanta `sessionTTL` langsung, supaya sesi display
tidak diam-diam terpotong jadi 30 hari saat sliding renewal). `go build/vet/test
./...` hijau; test service (fake repo/gateway): announcement aktif by tanggal
LOKAL sekolah (`clock.Fixed` + timezone WIB/WIT, termasuk kasus tanggal lokal
beda dari tanggal UTC), validasi CRUD & permission `announcement:manage`
(admin/kepsek boleh, guru/display 403), TTL sesi per role, compliance
(scheduled dihitung SEKALI per kemunculan day_of_week dalam rentang — mis.
slot Senin dihitung 2x utk rentang 2 Senin — taught/unscheduled/material dari
teaching_journals sendiri, guru tanpa data sama sekali di-skip, urutan pct
asc), tv board menyusun bagian (current = slot jam berjalan, upcoming = slot
starts_at berikutnya, bukan seluruh sisa hari).
- ✅ Migrasi `00008_announcement.sql` (`announcements` — title/body/starts_at/
  ends_at date, created_by)
- ✅ `internal/announcement/`: `GET /api/announcements?active=1` (SEMUA role
  termasuk display, aktif = tanggal lokal sekolah di antara starts_at..ends_at)
  vs tanpa `active` (perm `announcement:manage`); `POST/PATCH/DELETE
  /api/announcements/{id}` (admin & kepsek, validasi title/body wajib +
  starts<=ends, replace-all pada PATCH — bukan partial, keputusan sendiri
  konsisten dgn presedan `teaching.PatchInput`); audit `announcement.create/
  update/delete`; interface publik `Service.ActiveOn` (dipakai modul
  dashboard lewat consumer-side interface + adapter)
- ✅ Session TTL PER ROLE (`internal/identity/session.go`
  `sessionTTLForRole`/`sessionRenewWindowForRole`): role `display` 365 hari
  (renew window 60 hari), role lain tetap 30 hari (renew window 15 hari) —
  dipakai `Login`, `IssueSession` (gateway.go), DAN sliding renewal
  `RequireAuth` (middleware.go, sebelumnya bug laten: renewal selalu pakai
  `sessionTTL` 30 hari fixed, akan memotong sesi display jadi 30 hari saat
  pertama kali diperpanjang)
- ✅ Role `display` di `rolePermissions` (rbac.go) — SUDAH ada sejak fase
  sebelumnya (`teaching:monitor` + `schedule:read`), diverifikasi ulang sesuai
  docs/02, tidak perlu ditambah
- ✅ Bootstrap idempoten: akun `display`/`display12345` (membership role
  `display`) — password & membership diupsert tiap run
- ✅ `internal/dashboard/` (modul baru, tanpa tabel sendiri): `GET /api/tv/board`
  (perm `teaching:monitor`) menyusun payload gabungan SATU fetch (docs/06:
  polling 15-30dtk di sisi klien) dari teaching status, attendance summary,
  announcement aktif, DAN schedule CurrentPeriod (utk `next_starts_at`
  akurat di level period, bukan cuma slot) — semua lewat consumer-side
  interface primitif kecuali beberapa method BUTUH adapter kecil
  (`cmd/server/dashboardadapter.go`, satu-satunya tempat yg boleh mengimpor
  teaching+attendance+announcement+schedule+dashboard sekaligus, pola sama
  `scheduleadapter.go` fase 6) karena producer mengembalikan struct bernama
  masing-masing package (bukan primitif) — TIDAK menduplikasi derivasi status
  mengajar/rekap absen/tanggal aktif pengumuman, murni menyusun ulang
- ✅ `GET /api/teaching/compliance?from=&to=` (`internal/teaching`, perm
  `teaching:monitor`, default rentang 30 hari): per guru `{teacher,
  scheduled, taught, pct, unscheduled, material_filled, material_pct}` —
  `scheduled` dihitung service-layer via `ScheduleGateway.SlotsForDayOfWeek`
  yang SUDAH ada (bukan query SQL baru ke schedule_slots, supaya tidak
  duplikasi logika), diiterasi per tanggal dalam rentang (slot Senin dihitung
  sekali per Senin); `taught/unscheduled/material_filled` dari query SQL baru
  `JournalComplianceCounts` yang HANYA menyentuh tabel `teaching_journals`
  milik modul sendiri; `pct`/`material_pct` dibulatkan 1 desimal, guru tanpa
  scheduled+journal sama sekali di-skip dari hasil; urut pct ASC
- ⬜ Halaman TV fullscreen `/tv` (frontend) & dashboard kepsek interaktif +
  drill-down (belum dikerjakan sesi ini — fokus backend + verifikasi curl,
  sesuai batasan tugas)

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
