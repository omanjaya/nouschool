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
- ✅ UI frontend izin (dikerjakan agent frontend fase 4 — /izin lengkap dengan timeline & antrian approver)

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
- ✅ Halaman TV fullscreen `/tv` & dashboard kepsek (dikerjakan agent frontend fase 7)

## Fase 8 — Metode absen lanjutan ✅ (backend)
- ✅ QR kartu siswa (backend): token per siswa (`student_qr_tokens`), generate/list/revoke, scan guru (`POST /api/attendance/sessions/{id}/scan`), PNG QR — generator kartu PDF grid utk dicetak (frontend/print) BELUM dikerjakan (di luar scope backend)
- ✅ Self check-in GPS (backend): rule settings (sudah ada sejak fase sebelumnya), jendela waktu, validasi radius (haversine), status hadir/terlambat via `late_after_min`, deteksi anomali (`GET /api/attendance/anomalies`) untuk wali kelas

## Fase 9 — Notification ✅ (backend)
Backend terverifikasi end-to-end di Docker dev (`demo.localhost` + host
platform): admin PUT record siswa (Budi Santoso, NIS 22103, XII RPL 1 —
guardian ter-aktivasi ada di DB dari sesi sebelumnya, BUKAN NIS 22101/XI RPL
2 seperti perkiraan awal tugas; password ortu di-reset via script sekali
pakai karena tidak terdokumentasi) status `sakit` → login ortu → GET
`/api/notifications` (item "sakit" muncul, `unread_count:1`) → POST read
`{all:true}` (unread 0) → PUT record sama `sakit` lagi (TIDAK ada notif baru
— dedup terverifikasi) → `alpa` (notif baru muncul) → guru submit izin →
kepsek GET notifications (`leave.submitted` muncul) → kepsek approve → guru
GET notifications (`leave.decided` muncul) → GET `/api/push/public-key` (200,
key non-kosong, stabil antar-restart container — diverifikasi lewat
`docker compose restart`) → POST subscribe endpoint dummy (200) → cek outbox
DB: baris `web_push` utk tiap notifikasi ADA (`status=failed`, `attempts`
naik, `next_retry_at` terjadwal — gateway dummy gagal itu WAJAR, backoff
jalan; TIDAK ADA baris `whatsapp`/`email` karena `WA_GATEWAY_URL`/`SMTP_HOST`
kosong di env dev) → super admin (host platform) PUT
`/api/admin/schools/{id}/settings/notification` `{channels:["in_app"]}` →
ulangi 1 notifikasi (`terlambat`) → outbox TIDAK bertambah baris `web_push`
(in-app tetap tertulis) → admin sekolah PUT `/api/settings/notification`
biasa → 403 (module superadmin-only) → restore channels
`["in_app","web_push"]`. `go build/vet/test ./...` hijau; test: render
template (substitusi data), resolusi channel (settings sekolah × config
platform kosong), jadwal backoff (1m/5m/30m/2h lalu dead), worker menandai
sent/failed/dead (fake provider, termasuk kasus `ErrNoContact` → dead
langsung attempts=0), dedup notifikasi absensi (status sama tidak kirim
ulang, status hadir tidak pernah kirim), subscription push 410 dihapus (fake
pushSender), notify wiring leave (submit → approver step aktif, decide →
pengaju + approver berikutnya bila ada).
- ✅ Migrasi `00010_notification.sql`: `notification_outbox`, `notifications`
  (inbox in-app), `push_subscriptions` (persis docs/08) + `platform_config`
  (key/value kecil, tambahan scope tugas — dipakai simpan kunci VAPID supaya
  stabil antar-restart)
- ✅ `internal/notification/`: `Service.Send`/`Notify` (API internal —
  Notify adalah wrapper primitif-friendly utk consumer-side interface
  `Notifier` di modul pemakai) menulis inbox in-app SELALU + baris outbox per
  (user × channel default event YANG aktif di settings sekolah DAN
  dikonfigurasi platform); title/body dirender SEKALI saat Send lewat
  `text/template`, disimpan di payload outbox (worker tidak pernah merender
  ulang); registry event: `attendance.student_absent` (→ ortu, wa+push),
  `leave.submitted` (→ approver, push), `leave.decided` (→ pengaju,
  push+email) — `announcement.published` SENGAJA TIDAK didaftarkan (scope
  tugas: SKIP, modul announcement belum diubah memanggil Notify)
- ✅ Worker outbox (`notification.StartWorker`, goroutine di main.go): poll
  tiap 10 detik batch 50, backoff 1m/5m/30m/2h lalu `dead`; graceful shutdown
  lewat ctx (log "worker outbox berhenti" saat context dibatalkan)
- ✅ Provider: `web_push` (lib `github.com/SherClockHolmes/webpush-go`, VAPID
  dari env atau di-generate saat startup & disimpan `platform_config`, log
  public key; subscription 404/410 dihapus dari `push_subscriptions`; SELALU
  dianggap configured); `whatsapp` (HTTP gateway gaya Fonnte, env
  `WA_GATEWAY_URL`/`WA_GATEWAY_TOKEN`, form POST target+message); `email`
  (net/smtp plain text, env `SMTP_HOST/PORT/USERNAME/PASSWORD/FROM`) —
  keduanya `Configured()` mengecek env platform kosong (outbox row TIDAK
  dibuat + log debug sekali per channel bila kosong)
- ✅ **Keputusan desain (didokumentasikan di kode)**: whatsapp/email tanpa
  kontak tersimpan (users.phone/email kosong) → `ErrNoContact` → worker
  menandai outbox `dead` LANGSUNG dengan `attempts` TETAP 0 (bukan backoff —
  data profil kosong bukan kegagalan sementara); web_push TANPA subscription
  sama sekali → kegagalan biasa (backoff normal, BEDA dari wa/email —
  subscription bisa muncul kapan saja lewat aksi user sendiri, install
  PWA/izinkan notifikasi)
- ✅ Settings module `notification` (`{channels:[...]}`, default
  `[in_app, web_push]`) — **superadmin-only**: `tenant.IsSuperAdminOnlyModule`
  (mekanisme flag baru di settings service tenant) menolak
  `PUT /api/settings/notification` di endpoint tenant umum (403) apa pun
  role-nya; mutasi HANYA lewat
  `GET/PUT /api/admin/schools/{id}/settings/{module}` (baru, host platform,
  `requireSuperAdmin` — generik utk module mana pun, bukan cuma
  "notification")
- ✅ Endpoint tenant (auth semua role, tanpa `requirePerm` — inbox & push
  subscription selalu milik diri sendiri): `GET /api/notifications?page=`,
  `POST /api/notifications/read` (`{ids:[]}` atau `{all:true}`),
  `GET /api/push/public-key`, `POST /api/push/subscribe` (upsert by
  endpoint), `POST /api/push/unsubscribe`
- ✅ Wiring event: `attendance.Notifier`/`leave.Notifier` (consumer-side
  interface, disuntik lewat `SetNotifier` SETELAH konstruksi — setter, bukan
  parameter constructor, supaya call site test attendance/leave yang sudah
  ada tidak berubah) — attendance notify ortu dari `UpdateRecords` (bulk PUT
  manual)/`SelfCheckin`/`ScanQRCard` (ketiganya di-hook sesuai scope tugas;
  scan QR & self-checkin `hadir`/pertama kali tidak pernah memicu notifikasi
  karena statusnya, kecuali self-checkin `terlambat`) — SEKALI per siswa per
  sesi per status (status baru ≠ status lama DAN bukan hadir); leave notify
  approver step aktif saat submit, pengaju + approver berikutnya (bila ada)
  saat decide
- ✅ Interface publik baru: `student.Service.GuardianUserIDs` (dipakai
  `attendance.StudentAccess`), `identity.Service.UsersWithRole` (dipakai
  `leave.IdentityGateway`, resolusi step approval role-only)
- ✅ Service worker Web Push frontend (injectManifest custom SW, dikerjakan agent frontend fase 9)

## Fase 10 — Billing ✅
- ✅ Plans + plan_prices + seeding Basic/Pro & bracket (`migrations/00011_billing.sql`) —
  basic {tv_dashboard:false, whatsapp:false, dapodik_import:false, qr_card:true,
  self_checkin:true}, pro semua true; bracket ≤300/≤600/≤99999 (basic
  2jt/3.5jt/5jt, pro 4jt/7jt/10jt per tahun — placeholder, diedit via
  `PUT /api/admin/plans/{code}`)
- ✅ Subscriptions (satu baris per sekolah, UNIQUE school_id — snapshot
  plan_code/features/max_students saat aktivasi), lifecycle job
  (`billing.StartLifecycleWorker`, ticker 1 jam + `TickOnce` saat startup,
  idempoten), `SubscriptionGuard` (`billing.Service.Middleware`) dipasang di
  chain setelah ResolveTenant — mutasi non-GET ditolak 402
  `subscription_readonly` kecuali `/api/auth/`, `/api/billing/`,
  `/api/webhooks/` (exemption terakhir keputusan implementasi, lihat catatan
  di `internal/billing/guard.go`)
- ✅ Feature gating `RequireFeature` (per-route: `/api/tv/board`→tv_dashboard,
  qr-cards→qr_card, self-checkin→self_checkin) + `HasFeature` (whatsapp,
  dicek `notification.Service` sebelum insert outbox) + `features`/`subscription`
  di `/api/me` (identity.BillingGateway consumer interface)
- ✅ Invoice (nomor `INV/YYYY/NNNN` global per tahun, counter atomik) + PDF
  (`github.com/go-pdf/fpdf`), transfer manual (upload bukti multipart ≤5MB
  pdf/jpg/png + verifikasi super admin — verify juga bisa langsung dari
  status `unpaid` tanpa bukti, mis. konfirmasi manual dari mutasi rekening)
- ✅ Payment gateway Midtrans Snap (`PaymentProvider` interface, provider kedua
  murah ditambahkan) + webhook `/api/webhooks/midtrans` idempotent by
  order_id, verifikasi signature sha512
- ✅ Panel billing super admin: `/api/admin/plans`, `/api/admin/schools/{id}/billing`,
  `/api/admin/schools/{id}/subscriptions[/extend]`, `/api/admin/invoices/{id}/verify|void|pdf|proof`
- ✅ Bootstrap: sekolah demo dapat subscription Pro aktif 1 tahun (invoice
  manual_transfer, verified) — idempoten, dijalankan
- ✅ Test: lifecycle transitions, ActivateSubscription (renewal + snapshot
  tak berubah saat plan diedit), bracket, guard readonly, RequireFeature,
  webhook signature (valid/invalid/idempotent), nomor invoice berurutan —
  `internal/billing/service_test.go`
- ✅ Verifikasi end-to-end via curl (server dev `localhost:8210`): bootstrap →
  login admin/kepsek demo → `/api/billing` (pro, invoice paid) → `/api/me`
  (subscription+features) → `/api/tv/board` 200 → RBAC billing:view (guru
  403) → super admin: sekolah baru "SMA Uji Billing" (`ujibilling`, tanpa
  subscription → `GET .../billing` = null, tenant `/api/health` tetap 200)
  → subscribe basic → invoice unpaid → PDF (`application/pdf`) → verify →
  active basic → extend goodwill (+30 hari) → void invoice. Readonly
  lifecycle: `UPDATE subscriptions SET ends_on = ends_on - 400 hari` (psql
  container) + restart container (`TickOnce` saat startup) → status
  `readonly` di `GET /api/admin/schools/{id}/billing`. Webhook signature
  end-to-end HANYA lewat unit test (`MIDTRANS_SERVER_KEY` tidak di-set di
  container dev, tidak boleh sentuh docker-compose) — endpoint live dicek
  mengembalikan `gateway_not_configured` sesuai kontrak.
- ✅ **Kontrak API disilangcek terhadap `web/` yang sudah dibangun sesi
  sebelumnya** (`web/src/lib/types.ts`, `features/billing/`, `features/admin/`)
  — ditemukan & diperbaiki 3 bug nyata sebelum dianggap selesai:
  `PriceBracket` tanpa json tag (field PascalCase bocor ke API, akan merusak
  editor harga plan total), `SubscriptionView.features` semula `map[string]bool`
  (kontrak minta `string[]`, sama seperti `Me.features`), dan
  `VerifyManualPayment` semula menolak invoice berstatus `unpaid` (kontrak
  admin UI mengizinkan verifikasi tanpa bukti transfer diunggah dulu).
- ✅ UI frontend (`web/`, dibangun terhadap kontrak API pasti di deskripsi tugas —
  DISILANGCEK & backend disesuaikan terhadap kontrak `web/` pada sesi backend
  ini (lihat poin di atas), belum dijalankan `npm run dev` langsung end-to-end
  di browser sesi ini (batasan tugas: jangan sentuh `web/`), fokus
  sesi ini frontend saja sesuai batasan tugas `hanya web/src/`): banner status
  langganan global (`features/billing/SubscriptionBanner.tsx`, kuning saat
  `grace`/merah saat `readonly`, tautan `/tagihan` untuk admin & kepsek,
  tersembunyi otomatis di rute `/tv` karena dirender di dalam
  `AuthenticatedShell`); halaman `/tagihan` (admin & kepsek — kartu langganan
  dgn Tag status/periode/harga `formatRupiah`/pemakaian siswa/daftar fitur,
  daftar invoice dgn aksi Bayar Online (redirect gateway, 422 → Toast pesan
  server)/Upload Bukti Transfer (Dialog, validasi pdf/jpg/png ≤5MB)/Unduh PDF,
  EmptyState saat subscription `null`); kartu "Tagihan & Langganan" di
  Beranda admin & KepsekHomePage; helper `lib/features.ts#hasFeature` +
  penerapan gating client (kartu Check-in Kehadiran & rute `/checkin` →
  `self_checkin`, rute `/tv` → `tv_dashboard` via `TvRouteGuard`, tombol
  Kartu QR di `ClassDetailPage` + rute cetak kartu + tombol Scan Kartu di
  sesi absensi → `qr_card`); panel super admin: seksi "Langganan & Tagihan"
  di `SchoolDetailPage` (buat/perpanjang langganan dgn estimasi bracket dari
  `student_count`, verifikasi/void per invoice, perpanjang manual goodwill)
  + halaman baru `/admin/plans` (editor fitur & 3 bracket harga per plan,
  link dari header daftar sekolah). `npm run build` & `npm run lint` hijau.

## Fase 11 — Branding & polish SaaS ✅
Backend terverifikasi end-to-end via curl (server dev `localhost:8210`, Docker
hot reload): `GET /api/public/context` host platform (`{"platform":true}`) &
`demo.localhost` (`{"platform":false,"school":{...},"branding":{...}}`) → login
admin demo → upload logo PNG 1x1 dummy (`POST /api/settings/branding/logo`) →
`GET /api/public/branding/logo` 200 `image/png` → context & `GET
/manifest.webmanifest` ikut berubah (`app_name`/`theme_color`/icon logo) →
manifest host platform tetap default NouSchool → `PUT /api/custom-domain
{"domain":"sekolahdemo.sch.id"}` → `GET` status `pending` + instructions →
`POST /api/custom-domain/verify` → gagal jelas ("SERVER_IP belum diset...")
→ `PUT` ulang domain yang sama (resave sendiri diizinkan) → `DELETE
/api/custom-domain` → status kosong lagi → import Dapodik: CSV header gaya
Dapodik ("No. Induk", "NISN", "Nama Peserta Didik", "JK", "Tanggal Lahir",
"Rombongan Belajar") → preview 2 baris `create` (header dikenali walau beda
dari template NouSchool) → commit → `created:2` → export absensi bulan
2026-08 kelas XII RPL 1 → 200 xlsx (`Content-Type`
`application/vnd.openxmlformats...`, size 7235 bytes) → export izin rentang
Agustus → 200 xlsx (6466 bytes) → `POST /api/public/interest` 2x host
platform → login super admin (password di-reset sekali pakai via
`cmd/bootstrap`, tidak terdokumentasi sebelumnya — sama pola dengan catatan
Fase 9) → `GET /api/admin/interest` → 2 lead muncul → `scripts/backup.sh`
dijalankan → `backups/nouschool-2026-08-16.sql.gz` terbentuk (20K, `gunzip -t`
OK, isi valid pg_dump). `go build/vet/test ./...` hijau. Restore TIDAK
dijalankan (sesuai batasan tugas — hanya dibuat & didokumentasikan).
- ✅ Migrasi `00012_branding_domain_interest.sql`: `schools.pending_domain`
  (+ partial unique index) dan tabel `interest_leads` (platform-level, TANPA
  `school_id` — landing page host platform, calon sekolah belum jadi tenant)
- ✅ Branding: `BrandingSettings` (module `branding`, school_settings existing
  sejak Fase 1) diperluas field `logo_path` — `POST
  /api/settings/branding/logo` (perm `settings:manage`, multipart field
  `file`, png/jpg ≤2MB, sniff via `http.DetectContentType`) menyimpan file
  via `platform/storage` di `{school_id}/branding/logo.{ext}` lalu MENIMPA
  HANYA `logo_path` pada settings tersimpan (memuat ulang lewat
  `SettingsService.Get` dulu) — `app_name`/`primary_color` yang sudah
  tersimpan TIDAK ikut ter-reset, beda dari `PUT /api/settings/branding`
  generik (replace-all seperti modul settings lain)
- ✅ `GET /api/public/context` (PUBLIK): host platform →
  `{"platform":true}`; host tenant → `{"platform":false,"school":{"name",
  "slug"},"branding":{"app_name","primary_color","logo_url"}}` (`logo_url`
  null bila belum upload logo) — `GET /api/public/branding/logo` (PUBLIK,
  404 host platform / belum ada logo) meng-stream file lewat
  `platform/storage`
- ✅ `GET /manifest.webmanifest` (PUBLIK): host tenant → manifest dari
  branding sekolah (name/short_name = app_name, theme_color = primary_color,
  icon = logo satu entri `sizes:"any"` bila ada else `/pwa-icon.svg`); host
  platform → manifest default NouSchool. Logika penyusunan manifest
  (`manifestForBranding`/`defaultManifest`) SENGAJA dipisah jadi fungsi murni
  dari I/O (settings/HTTP) supaya bisa dites tanpa DB — lihat handler_test.go
- ✅ Caddyfile blok PROD (masih komentar, TIDAK diaktifkan): ditambah rute
  `handle /manifest.webmanifest { reverse_proxy localhost:8080 }` di kedua
  contoh (`*.nouschool.id` & `https://` on-demand) — `/api/public/*` sudah
  otomatis tercakup blok `/api/*` yang sudah ada
- ✅ Custom domain end-to-end (`internal/tenant/domain.go`, modul baru
  `DomainService` — dipisah dari `Service` CRUD sekolah supaya berdiri
  sendiri, tetap pakai `Repository`/`AuditLogger` yang sama): `GET
  /api/custom-domain` → `{domain,verified,server_ip,instructions}` (domain
  aktif ATAU pending, salah satu kosong); `PUT /api/custom-domain {domain}`
  → validasi format hostname (regex longgar, terima TLD apa pun) + unik
  LINTAS SEKOLAH menyilang `custom_domain` DAN `pending_domain` (409
  `domain_taken` bila dipakai sekolah lain, BUKAN 422 — keputusan sendiri:
  ini konflik data, bukan salah format) → simpan sebagai PENDING; `POST
  /api/custom-domain/verify` → `net.LookupHost` (dibungkus field
  `lookupHost`, diganti fake di test) dibandingkan config baru `SERVER_IP`
  (kosong di dev = gagal SEGERA dengan pesan jelas TANPA menyentuh jaringan,
  sesuai scope tugas) → sukses: `custom_domain` = `pending_domain`,
  `pending_domain` dikosongkan, **cache `HostResolver` di-invalidate**
  (method baru `HostResolver.Invalidate(host)`, dipanggil utk domain yang
  baru diverifikasi DAN saat `DELETE` melepas domain — supaya perubahan
  langsung berlaku tanpa menunggu TTL 60 detik habis, docs/01: "perubahan
  domain TIDAK butuh restart Caddy"); `DELETE /api/custom-domain` → lepas
  `custom_domain`+`pending_domain` sekaligus. `/internal/check-domain`
  (Caddy On-Demand TLS) TIDAK diubah — sudah benar sejak Fase 1 (hanya
  domain `custom_domain` terverifikasi yang lolos)
- ✅ **Keputusan desain**: `DomainService`/`InterestService` mendeklarasikan
  interface kecil (`domainRepository`/`interestRepository`) dipenuhi
  `*Repository` secara struktural — BEDA dari `tenant.Service`/
  `SettingsService` yang sudah ada sebelumnya (langsung pegang `*Repository`
  konkret, tanpa interface) — supaya keduanya bisa dites dengan fake
  in-memory tanpa DB, konsisten dengan pola `attendanceRepository`/
  `leaveRepository`/`studentRepository` di modul lain (CLAUDE.md: "Test:
  minimal service-level test untuk aturan bisnis")
- ✅ Import Dapodik (`internal/student/dapodik.go`): parser adaptor toleran
  header by NAMA (case/spasi-insensitive, buang tanda titik, sinonim: "Nama
  Peserta Didik"/"Nama Siswa"→nama, "No. Induk"/"Nomor Induk Sekolah"→nis,
  "JK"/"Jenis Kelamin"→jenis_kelamin terima L/P/Laki-laki/Perempuan berbagai
  variasi spasi-hubung, "Tanggal Lahir"/"Tgl Lahir" terima 4 format tambahan
  di luar yang sudah didukung parser template, "Rombongan Belajar"/"Kelas"→
  rombel) — matching by NISN DULU, fallback NIS (docs/03); baris tanpa NIS
  DAN NISN = error baris; siswa BARU (bukan update) tanpa NIS = error baris
  tersendiri (kolom `nis` NOT NULL di DB, keputusan sendiri: NISN sendirian
  tidak cukup utk membuat baris baru). Hasil parse adalah `[]studentImportRow`
  — TIPE YANG SAMA dipakai pipeline import Excel/CSV template NouSchool
  (docs/03: "Feed ke pipeline ImportRows existing") — `POST
  /api/import/dapodik` (preview, parser+lookup KHUSUS) + `POST
  /api/import/dapodik/commit` (handler KHUSUS tapi memanggil
  `Service.CommitStudentImport` YANG SAMA dengan commit siswa biasa —
  `ImportStore` tidak membedakan asal file)
- ✅ Export Excel (excelize, sudah dependency sejak awal) — dua modul, pola
  dua-lapis di keduanya (fungsi MURNI penyusun struktur data + fungsi MURNI
  penyusun workbook, dipisah dari I/O supaya dites tanpa DB/HTTP):
  - `GET /api/attendance/export?month=YYYY-MM&class_id=` (perm
    `attendance:report`, `internal/attendance/export.go`): query SQL baru
    `MonthlyAttendanceRecords` (LEFT JOIN sessions/records supaya siswa TETAP
    muncul walau belum ada sesi absen bulan itu) →
    `buildMonthlyMatrices` (kelompokkan per rombel/siswa/tanggal) →
    `renderMonthlyXLSX` (satu sheet per rombel — atau satu sheet saja bila
    `class_id` diisi — kolom tanggal 1..N hari dalam bulan berisi kode
    H/T/I/S/A, 5 kolom ringkasan total per status di ujung kanan, header
    bold, freeze pane kolom NIS+Nama & baris header via
    `excelize.Panes{Freeze:true}`)
  - `GET /api/leave/export?from=&to=` (perm `leave:manage` atau
    `kepala_sekolah`, sama gerbang dgn `Summary`, `internal/leave/export.go`):
    query SQL baru `ListLeaveRequestsInRange` (SEMUA status, rentang
    beririsan — beda dari `LeaveSummaryByRange` yg hanya approved) →
    `renderLeaveXLSX` langsung dari `[]Request` (shape yg SAMA dipakai `GET
    /api/leave/requests`, sudah lengkap steps/approver — tidak perlu query
    tambahan) — kolom guru/jenis/tanggal mulai-selesai/jumlah hari/status/
    approver (nama approver TERAKHIR yang memutuskan, atau "-" bila masih
    menunggu)
  - Kedua export: `Content-Disposition: attachment` nama file jelas
    (`"Absensi {rombel|Semua Kelas} {bulan Indonesia} {tahun}.xlsx"` /
    `"Izin {from}_{to}.xlsx"`)
- ✅ Registrasi minat sekolah (landing page, `internal/tenant/interest.go`):
  `POST /api/public/interest` (PUBLIK, host platform, rate-limit 3/jam per
  IP — `interestRateLimiter` kecil duplikat MINIMAL dari pola
  `identity.RateLimiter`, sengaja TIDAK diimpor lintas modul karena identity
  adalah modul bisnis bukan `platform/`) → `GET /api/admin/interest` (super
  admin, host platform, daftar SEMUA lead lintas sekolah — tanpa `school_id`)
- ✅ Config baru `SERVER_IP` (`internal/platform/config`, env `SERVER_IP`,
  didokumentasikan `.env.example`) — dipakai `DomainService` verifikasi DNS
- ✅ Test baru: parser Dapodik (variasi header incl. "No. Induk"/"JK"/
  "Rombongan Belajar", 5 format JK, 5 format tanggal, tanpa NIS+NISN = error,
  matching NISN-dulu-fallback-NIS, siswa baru tanpa NIS = error), manifest
  dinamis tenant vs platform (fungsi murni `manifestForBranding`/
  `defaultManifest`), branding publik (`brandingPublicViewFor`/
  `brandingLogoURLFor`), custom domain (format hostname, unik 409 lintas
  sekolah, resave domain sendiri diizinkan, verify TANPA `SERVER_IP` gagal
  jelas TANPA memanggil `lookupHost`, verify IP cocok/tidak cocok, delete),
  export absensi (matrix builder dari data palsu + baca-balik sel xlsx via
  `excelize.OpenReader`), export izin (approver dari step terakhir
  diputuskan, isi sel dari data palsu), rate limit interest (3/jam per IP,
  IP lain tidak terpengaruh) — `go build/vet/test ./...` hijau
- ✅ `scripts/backup.sh` (pg_dump via `docker compose exec -T db`, gzip,
  retensi 14 file terakhir) + `scripts/restore.sh` (baca file arg → psql,
  PERINGATAN menimpa + konfirmasi ketik "ya" wajib, TIDAK dijalankan sesuai
  batasan tugas) — README seksi "Operasional" (cara backup manual, saran
  cron VPS harian, cara restore + peringatan, catatan `SERVER_IP` produksi)
- ✅ UI landing page (hero + fitur + harga + form minat) di host platform; klien branding dinamis (CSS variables,
  `<meta theme-color>`), form pengaturan custom domain, form registrasi minat
  (`web/` — di luar scope backend sesi ini, sesuai batasan tugas "jangan
  sentuh web/")

## Fase 12 — Realtime WebSocket ✅ (diminta user 16 Agu 2026)
Backend terverifikasi end-to-end di Docker dev (`demo.localhost`) lewat
probe WebSocket sungguhan (`github.com/coder/websocket`, script sementara
di scratchpad — lihat catatan di bawah, BUKAN bagian repo): login REST
(kepsek & ortu.budi) → cookie `ns_session` → connect `ws://localhost:8210/api/ws`
Host `demo.localhost` → keduanya terima `{"type":"hello","data":{"user_id":...}}`
→ kirim `{"type":"ping"}` → balas `{"type":"pong"}` (probe kepsek: user_id 11
dalam ~4ms; probe ortu: user_id 6) → admin PUT
`/api/attendance/sessions/1/records` (Budi Santoso NIS 22103, terlambat→sakit)
→ probe kepsek terima `attendance.session {class_id:1,date:"2026-08-16",session_id:1}`
DAN `attendance.summary {date:"2026-08-16"}`; probe ortu (wali Budi) terima
`notification {}` DAN `attendance.session` (broadcast sekolah, ortu ikut
kebagian) TAPI TIDAK `attendance.summary` (role-only admin/kepsek/display,
ortu bukan salah satunya — targeting role terbukti benar) → admin POST
`/api/announcements` → KEDUA probe terima `announcement {}` (broadcast) →
admin POST `/api/students` (siswa baru) → probe kepsek terima `students {}`,
probe ortu TIDAK (role admin/kepsek/guru saja, orang_tua dikecualikan —
sesuai kontrak) → guru submit `POST /api/leave/requests` (multipart) →
probe kepsek (approver step aktif) terima `notification` DAN
`leave {request_id:5}` → kepsek `POST /api/leave/approvals/5/decide` approved
→ probe kepsek terima `leave {request_id:5}` lagi (dirinya sendiri masuk
roles admin_sekolah/kepala_sekolah target). Semua event dicetak probe TANPA
error, tanpa disconnect prematur; `docker logs` menunjukkan `GET /api/ws
status=101` bersih (upgrade sukses, tanpa panic). `go build/vet/test ./...`
hijau TERMASUK `go test -race ./...` (dijalankan di dalam kontainer
`sekolah-api-1` yang punya gcc/CGO — host Windows dev tidak punya gcc).
schedule/teaching/billing memakai pola SetRealtime IDENTIK (compile +
service test hijau) tapi tidak dipicu live di sesi ini (di luar 3 jalur WAJIB
di deskripsi tugas: absensi/pengumuman/notifikasi) — leave & students dites
live sebagai bonus di atas cakupan minimum.
- ✅ `internal/realtime/` (modul baru, TANPA tabel DB): dependency
  `github.com/coder/websocket` v1.8.15 (+ subpackage `wsjson`) — `Hub`
  (`hub.go`) menyimpan koneksi per `school_id` (`map[int64]map[*Client]struct{}`,
  `RWMutex`), `Register(schoolID,userID,role,closer)`/`Unregister(*Client)`,
  `Publish(schoolID int64, ev Event)` dengan
  `Event{Type string, Data map[string]any, Roles []string, UserIDs []int64}`
  (`Roles`/`UserIDs` kosong dua-duanya = broadcast sekolah; salah satu/keduanya
  terisi = union — `matchesTarget`); kirim NON-BLOCKING per koneksi (channel
  buffer 32, `sendBuffer`), buffer penuh → `Client.Drop()` (panggil `closer`
  sekali via `sync.Once`, klien reconnect sendiri) — TIDAK PERNAH Publish
  blocking/deadlock walau ada klien macet
- ✅ `GET /api/ws` (`ws.go`, `RegisterRoutes` di belakang `requireAuth` SAJA —
  TANPA `requirePerm`, role `display` BOLEH sesuai scope): upgrade via
  `websocket.Accept` → `Hub.Register` → kirim `{"type":"hello","data":{"user_id":...}}`
  → `readPump` (baca pesan klien, HANYA `{"type":"ping"}` dibalas
  `{"type":"pong"}`, batas baca per pesan 90 dtk via context timeout) →
  `writePump` (goroutine terpisah: kirim event dari `Client.Send()` + ping
  level-WebSocket server→klien tiap 30 dtk) — keduanya berhenti bersih lewat
  channel `stop` + `conn.CloseNow()` (idempoten via `sync.Once`), TANPA
  goroutine leak (dibuktikan `<-done` di akhir handler)
- ✅ **Fix bug laten wajib untuk WS**: `internal/platform/middleware/middleware.go`
  `statusWriter` (dipakai `Logger`, dipasang di `middleware.Chain` SEBELUM
  routing) tidak mengekspos `Unwrap() http.ResponseWriter` — tanpa ini
  `websocket.Accept` SELALU gagal "does not implement http.Hijacker" karena
  `http.Hijacker` milik `ResponseWriter` ASLI tersembunyi di balik
  `statusWriter`. Ditambahkan method `Unwrap()` satu baris (konvensi
  `net/http` standar Go 1.20+, `http.ResponseController`) — TIDAK mengubah
  perilaku middleware lain
- ✅ Interface konsumsi per modul (pola SetNotifier, dideklarasikan DI SISI
  PEMAKAI per CLAUDE.md): `attendance.Realtime`/`teaching.Realtime`/
  `notification.Realtime`/`leave.Realtime`/`announcement.Realtime`/
  `schedule.Realtime`/`billing.Realtime`/`student.Realtime` — semua primitif
  (`Publish(schoolID int64, eventType string, data map[string]any)` dan/atau
  `PublishTo(..., roles []string, userIDs []int64)`), disuntik lewat
  `SetRealtime(...)` SETELAH konstruksi (setter, bukan constructor param,
  supaya call site test lama tidak berubah — nil aman/no-op, dibuktikan
  seluruh test service lama tetap hijau TANPA `SetRealtime` dipanggil).
  `*realtime.Hub` TIDAK memenuhi interface ini secara langsung (signature
  `Publish(schoolID,Event)` beda bentuk) — dijembatani adapter tipis
  `cmd/server/realtimeadapter.go` (`realtimeForModules`, SATU instance
  dipakai bersama semua modul karena method set identik), pola sama
  `scheduleadapter.go`/`dashboardadapter.go`; wiring `SetRealtime` di
  `cmd/server/main.go` segera setelah tiap service dikonstruksi
- ✅ **Event catalog final** (tipe → data → target; Data SENGAJA minimal,
  klien SELALU refetch REST ber-authz saat menerima event apa pun):
  - `attendance.session` `{session_id,class_id,date}` — broadcast sekolah;
    dipicu `CreateSession`/`createSubjectSessionFromSlot` (buka sesi, incl.
    dari `schedule_slot_id`)/`UpdateRecords` (PUT bulk)/`Finalize`/
    `ScanQRCard`/`SelfCheckin`
  - `attendance.summary` `{date}` — roles `admin_sekolah`,`kepala_sekolah`,`display`;
    dipicu titik yang SAMA dengan `attendance.session` (satu helper
    `emitSession`)
  - `teaching.status` `{date}` — roles `admin_sekolah`,`kepala_sekolah`,`display`;
    dipicu `Scan` (jalur slot ketemu MAUPUN idempoten re-scan)/
    `CreateUnscheduled`/`UpdateJournal`/`EndJournal`
  - `notification` `{}` — user_ids [penerima]; dipicu `Service.Send` SETIAP
    baris inbox in-app berhasil ditulis (per penerima, di dalam loop
    `n.UserIDs`) — badge & list notifikasi refetch instan
  - `leave` `{request_id}` — user_ids [pengaju, approver step aktif bila ada]
    + roles `admin_sekolah`,`kepala_sekolah`; dipicu `SubmitRequest`/
    `DecideStep` (approver BERIKUTNYA bila chain belum selesai)/`CancelRequest`
  - `announcement` `{}` — broadcast sekolah; dipicu `Create`/`Update`/`Delete`
  - `schedule` `{}` — broadcast sekolah; dipicu `CreateSlot`/`UpdateSlot`/
    `DeleteSlot`/`CopySchedule`/`CommitScheduleImport`/`ReplacePeriods`
  - `billing` `{}` — roles `admin_sekolah`,`kepala_sekolah`; dipicu
    `ActivateSubscription`/`VoidInvoice`/`ExtendSubscriptionGoodwill`, DAN
    `TickOnce` (worker lifecycle 1 jam) HANYA ke sekolah yang BENAR-BENAR
    baru transisi active→grace/grace→readonly (lihat perubahan query di
    bawah) — `VerifyManualPayment` tidak emit terpisah (memanggil
    `ActivateSubscription` yang sudah emit)
  - `students` `{}` — roles `admin_sekolah`,`kepala_sekolah`,`guru`; dipicu
    `CreateStudent`/`UpdateStudent`/`CreateClass`/`UpdateClass`/
    `EnrollStudents`/`UnenrollStudent`/`CommitStudentImport` (termasuk jalur
    Dapodik, memanggil fungsi yang sama)
- ✅ **Perubahan pendukung billing (Fase 12)**: `TransitionActiveToGrace`/
  `TransitionGraceToReadonly` (`internal/billing/queries.sql`) diubah dari
  `:execrows` (jumlah baris) jadi `:many RETURNING school_id` — `TickOnce`
  butuh tahu SEKOLAH MANA yang transisi supaya event ditargetkan per sekolah
  (Hub ter-partisi per sekolah), bukan disiarkan buta. Kode generated
  (`billingdb/queries.sql.go`) ditulis TANGAN mengikuti idiom PERSIS sqlc
  utk query `:many` kolom tunggal (dibandingkan
  `internal/identity/identitydb/queries.sql.go#ListUserIDsByRole`) — sqlc
  CLI tidak tersedia di lingkungan build ini; `repository.go`/fake repo test
  disesuaikan (`[]int64` bukan `int64`)
- ✅ Test `go test -race ./internal/realtime/`: register/publish broadcast,
  targeting by role, by user_id, union role+user_id, buffer penuh → drop
  klien TANPA deadlock (dibuktikan goroutine publisher selesai < timeout),
  unregister bersih (peta sekolah terhapus saat koneksi terakhir lepas, no-op
  dipanggil dua kali), publisher nil-safe (`closer=nil` tidak panic),
  konkurensi register/publish/unregister 50 goroutine sekaligus (murni utk
  `-race`) — SEMUA lolos `-race` (dijalankan di kontainer, gcc tersedia)
- ✅ `go build/vet/test ./...` hijau (host Windows, tanpa `-race`) DAN
  `go build/vet/test -race ./...` hijau di dalam kontainer `sekolah-api-1`
  (host dev Windows tidak punya gcc/CGO_ENABLED — didokumentasikan sebagai
  keterbatasan lingkungan, BUKAN diagnosis kode)
- ✅ Klien WS frontend (backoff+jitter, watchdog 60s, online-event) sebagai bus invalidasi query; TV instan (polling fallback 120s saat connected), badge notif instan, guard dirty-state layar sesi absen, banner "Menyambung ulang"
- ✅ Vite proxy ws:true; e2e Docker via probe WS nyata: role targeting terverifikasi (kepsek dapat summary, ortu tidak; ortu dapat notification), upgrade 101 bersih (browser asli — probe
  Fase 12 backend di atas connect LANGSUNG ke `localhost:8210`, bukan lewat
  Vite, sesuai batasan tugas "jangan sentuh web/")

## Fase 13 — Super Admin ✅ (rencana lengkap: docs/11-superadmin.md; backend + frontend tuntas)
- ✅ Impersonation "Masuk sebagai Sekolah Ini" — backend + frontend (tombol detail sekolah, /impersonate, banner Mode Support)
- ✅ P1 Dashboard platform /admin (overview + daftar perlu perhatian) — backend + frontend
- ✅ P2 Onboarding wizard sekolah baru (+ buat akun admin sekolah) — backend + frontend
- ✅ P3 Statistik kesehatan per sekolah — backend + frontend
- ✅ P4 Reset password user, audit log viewer, outbox global — backend + frontend
- ✅ P5 Pengumuman platform — backend + frontend
- ✅ P6 Feature override per sekolah, laporan pendapatan — backend + frontend

**Fase 13 Gelombang 1 (P1+P3+P4 backend) ✅** — terverifikasi end-to-end di
Docker dev (`localhost:8210`, host platform): login super admin
(`omanjaya53@gmail.com`) → `GET /api/admin/overview` (`schools_active:2`
demo+ujibilling keduanya subscription active, `total_students:13`,
`revenue_year:10000000` dari invoice paid demo, `leads_7d:2`,
`schools_no_active_year` berisi ujibilling, `outbox_dead` berisi demo 8 baris,
`last_activity` terurut null dulu) → `GET /api/admin/schools/1/stats`
(`teachers:4`, `students:13`, `attendance_sessions_7d:3`, `journals_7d:1`,
`notifications_30d.dead:8`, `last_logins` 7 role, `uploads_bytes:135`) →
`GET /api/admin/schools/1/members` (12 baris) → `POST
/api/admin/users/4/reset-password {"school_id":1}` (guru Rendi) → dapat
`temp_password` → login Rendi password lama `guru12345` GAGAL
(`invalid_credentials`), password sementara SUKSES → **dikembalikan**:
bootstrap ulang (`-demo`, idempoten) → password guru demo `guru12345`
disetel ulang, login Rendi dgn `guru12345` SUKSES lagi → `GET
/api/admin/schools/1/audit?action=admin.reset_password` (1 baris) &
`?action=admin.impersonate` (6 baris dari sesi sebelumnya, `issued`+`started`
berpasangan) → `GET /api/admin/outbox?status=dead` (8 baris) → `POST
/api/admin/outbox/8/retry` → baris itu `status:pending` (dead sisa 7) →
`POST /api/admin/outbox/retry-all {"status":"dead"}` → `{"retried":7}`,
outbox dead sisa 0, audit `admin.outbox_retry_all` tercatat (dicek psql
langsung: `school_id` NULL — retry-all dipanggil TANPA `school_id`, action
platform-wide, BUKAN bug). `go build/vet/test ./...` hijau.
- **Keputusan pembagian modul** (dilaporkan sesuai instruksi tugas): DUA
  tempat, bukan satu.
  1. **`internal/identity` diperluas** (`internal/identity/admin.go`) untuk
     P4.1 (`GET /api/admin/schools/{id}/members`), P4.2 (`POST
     /api/admin/users/{id}/reset-password`), P4.3 (`GET
     /api/admin/schools/{id}/audit`) — KETIGANYA hanya menyentuh tabel yang
     SUDAH dimiliki modul identity sendiri (`users`/`memberships`/`sessions`/
     `audit_log`), jadi tidak ada alasan bikin modul baru untuk itu.
  2. **Modul baru `internal/platformadmin`** untuk P1 (`GET
     /api/admin/overview`), P3 (`GET /api/admin/schools/{id}/stats`), P4.4
     (outbox global) — SEMUA butuh JOIN lintas tabel milik BANYAK modul lain
     (schools/subscriptions/invoices/students/interest_leads/
     notification_outbox/sessions/attendance_sessions/teaching_journals),
     dieksekusi sebagai SQL agregasi READ-ONLY langsung di
     `internal/platformadmin/queries.sql` — EKSPLISIT diizinkan instruksi
     tugas ("preseden dashboard/tv-board") supaya tidak perlu puluhan
     consumer-side interface primitif + N+1 query per sekolah. Mutasi
     (retry outbox) HANYA menyentuh `notification_outbox` (status flag,
     dipahami worker existing `internal/notification/worker.go`, TIDAK
     menduplikasi logika bisnis). Catatan desain lengkap ada di package doc
     `internal/platformadmin/model.go`.
- ✅ `internal/platformadmin/` (modul baru, sqlc package `platformadmindb`):
  `GET /api/admin/overview` (`Service.Overview` + `bucketSchools` — status
  efektif per sekolah: `suspended` menang dari `schools.status`, selain itu
  status `subscriptions.status` dgn fallback `readonly` bila TANPA
  subscription ATAU status `canceled`, `grace_until` dihitung
  `ends_on + gracePeriodDays` konst lokal 14 — REDEFINISI nilai
  `billing.GracePeriodDays`, pola sama `billing.PermBillingView` vs
  `identity.PermBillingView`), `GET /api/admin/schools/{id}/stats`
  (`uploads_bytes` via `storage.Store.DirSize` baru, best-effort/0 bila
  belum ada folder), `GET /api/admin/outbox` + `POST
  /api/admin/outbox/{id}/retry` (422 bila status bukan failed/dead, set
  pending+`next_retry_at=now` TANPA reset attempts) + `POST
  /api/admin/outbox/retry-all` (`{school_id?, status}`, audit
  `admin.outbox_retry_all` SAJA — `AuditLogger` consumer-side interface
  dipenuhi `*identity.Service` struktural, diinject `cmd/server/main.go`
  TANPA adapter karena signature primitif langsung cocok)
- ✅ `internal/identity/admin.go` (perluasan modul identity): `ListMembers`
  (join `memberships+users+sessions`, satu baris per membership),
  `AdminResetPassword` (implementasi murni testable `adminResetPassword` —
  tolak `is_super_admin`, wajib member aktif sekolah target, generate
  password 10 char charset `abcdefghijkmnpqrstuvwxyz23456789` TANPA
  `0/o/1/l`, `DeleteSessionsByUser` query baru dipanggil SELALU setelah
  reset, audit `admin.reset_password`), `ListAuditLog` (implementasi murni
  `listAuditLogPage` — page/per_page default 1/50 maks 200, filter
  `action` prefix-match via `LIKE '<prefix>%'`)
- ✅ `platform/storage.Store.DirSize(relPath)` (helper baru, `filepath.WalkDir`
  rekursif, 0+nil bila direktori belum ada)
- ✅ Migrasi: TIDAK ADA migrasi baru — Gelombang 1 murni membaca/menulis tabel
  yang sudah ada (schools/subscriptions/invoices/students/interest_leads/
  notification_outbox/sessions/attendance_sessions/teaching_journals/
  memberships/users/audit_log)
- ✅ Test (fake repo, tanpa DB): `internal/platformadmin/service_test.go`
  (bucket status subscription — active/grace/readonly eksplisit/fallback
  tanpa-subscription/canceled/suspended, `grace_until` = +14 hari; transisi
  outbox dead→pending & failed→pending; pending/sent DITOLAK 422; retry-all
  audit dipanggil tepat sekali), `internal/identity/admin_test.go`
  (`adminResetPassword`: sukses+audit+DeleteSessionsByUser dipanggil sekali,
  tolak super admin TANPA hapus sesi/audit, tolak bukan-member, user tidak
  ditemukan; `generateTempPassword`: panjang 10 & charset tanpa 0/o/1/l lewat
  50 sampel; `listAuditLogPage`: pagination lintas halaman, filter prefix
  action, default & clamp page/per_page)

**Fase 13 Gelombang 2 (P2+P5+P6 backend) ✅** — terverifikasi end-to-end di
Docker dev (`localhost:8210`): login super admin (host `admin.localhost`) →
`POST /api/admin/schools/2/admins {"name","email"}` ke ujibilling → dapat
`user_id:15`+`temp_password` → ulangi dengan email SAMA → 409 `conflict` →
login admin baru itu di host `ujibilling.localhost` → 200 (role
`admin_sekolah`) → `GET /api/admin/schools/2/onboarding` →
`has_admin:true, has_active_year:false, has_students:false,
has_schedule:false, has_subscription_active:true, ready:false` →
`POST /api/admin/platform-announcements` (aktif hari ini) → login admin demo
(host `demo.localhost`, `admin`/`admin12345`) → `GET
/api/announcements?active=1` → pengumuman platform muncul PERTAMA
`is_platform:true`, pengumuman sekolah `is_platform:false` → `GET
/api/tv/board` → `announcements` sama urutannya → `PATCH
/api/admin/platform-announcements/1` (judul+rentang berubah) → tercermin di
`GET /api/announcements?active=1` → `DELETE
/api/admin/platform-announcements/1` → hilang dari kedua endpoint →
`PUT /api/admin/schools/1/feature-overrides {"tv_dashboard":false}` → `GET
/api/me` admin demo → `features` TANPA `tv_dashboard` (cache langsung
ter-invalidate, TIDAK perlu tunggu TTL 60 dtk) → login kepsek demo
(`kepsek`/`kepsek12345`) → `GET /api/tv/board` → 402 `feature_unavailable`
(bukan 403 — mengikuti kode status EXISTING `billing.ErrFeatureUnavailable`,
lihat guard.go) → `PUT .../feature-overrides {"tv_dashboard":null}` (hapus
override) → `GET /api/tv/board` sebagai kepsek → 200 lagi → key tak dikenal
(`fitur_ngasal`) → 422 `validation` → `GET /api/admin/revenue?year=2026` →
`total:10000000` (3 invoice paid existing: 1 basic 2jt + 2 pro 8jt, SEMUA di
Agustus) sesuai invoice paid seed data → `GET
/api/admin/revenue/export?year=2026` → `Content-Type:
application/vnd.openxmlformats-officedocument.spreadsheetml.sheet`, file
terbaca `file(1)` sebagai "Microsoft Excel 2007+" valid. Dicek psql langsung:
audit `admin.create_school_admin` (school_id=2, entity_id=15),
`admin.platform_announcement_create/update/delete` (school_id NULL —
platform-wide, BUKAN bug, entity_id=id pengumuman), `admin.feature_overrides`
(school_id=1 x2 — set lalu hapus) SEMUA tercatat; `subscriptions.
feature_overrides` sekolah 1&2 kembali `{}` bersih setelah test. `go
build/vet/test ./...` hijau.
- **Migrasi**: `migrations/00013_platform.sql` — tabel `platform_announcements`
  (id/title/body/starts_at/ends_at/created_by/created_at, index
  `(starts_at,ends_at)`) + `subscriptions.feature_overrides jsonb NOT NULL
  DEFAULT '{}'` — SATU migrasi untuk P5+P6 sesuai instruksi tugas (gelombang
  yang sama).
- **Keputusan pembagian modul** (dilaporkan sesuai instruksi tugas):
  1. **P2.1** (`POST /api/admin/schools/{id}/admins`) → `internal/identity`
     (perluasan `admin.go`, pola SAMA P4.1/4.2/4.3 Gelombang 1) — hanya
     menyentuh tabel identity sendiri (users/memberships). **P2.2** (`GET
     .../onboarding`) → `internal/platformadmin` — checklist butuh JOIN
     lintas modul (tenant academic_years, identity memberships, billing
     subscriptions, student students, schedule schedule_slots), satu query
     agregasi `SchoolOnboardingStatus` (5 `EXISTS` subquery, pola sama
     `ListSchoolsOverview` Gelombang 1).
  2. **P5** (CRUD pengumuman platform + integrasi tenant) → SELURUHNYA di
     `internal/platformadmin` (CRUD, tabel `platform_announcements` bukan
     milik modul lain) DENGAN **consumer-side interface baru**
     `announcement.PlatformAnnouncements` (method `ActiveOn`) dideklarasikan
     di `internal/announcement` (sisi pemakai) — DIPENUHI
     `*platformadmin.Service` lewat adapter `cmd/server/
     platformadminadapter.go` (pola SAMA `dashboardadapter.go`: producer
     mengembalikan struct bernama beda, butuh adapter tipis). `announcement.
     Service.List/ActiveOn` DIGABUNG dengan hasil gateway ini (platform DULU,
     sekolah KEMUDIAN) — SATU tempat merge dipakai kedua endpoint
     (`/api/announcements?active=1` DAN `/api/tv/board` via
     `dashboard.AnnouncementGateway`, tidak ada duplikasi logika gabung).
  3. **Realtime lintas sekolah** (P5): `Hub.PublishAll(ev)` (baru,
     `internal/realtime/hub.go`) — iterasi SEMUA `school_id` yang PUNYA
     koneksi terdaftar saat ini (bukan query daftar sekolah dari DB) lalu
     panggil `Publish` per sekolah; dipilih KARENA Hub sudah mengelompokkan
     koneksi per sekolah secara alami DAN sekolah tanpa koneksi aktif tidak
     butuh notifikasi real-time (dapat data terbaru saat refetch normal).
     Consumer-side interface baru `platformadmin.PlatformRealtime`
     (`PublishAll(eventType, data)`) dipenuhi adapter `realtimeForModules`
     (`cmd/server/realtimeadapter.go`) lewat method baru dengan nama sama.
  4. **P6 SELURUHNYA di `internal/billing`** (BUKAN platformadmin) — feature
     override menyentuh `subscriptions.feature_overrides` (kolom milik
     billing), revenue report menyentuh `invoices` (tabel milik billing,
     JOIN `schools` untuk nama — preseden SUDAH ada di
     `ListSubscriptionsForAdmin` billing sendiri, Fase 10). Resolusi fitur
     efektif (`mergeFeatures`) ditaruh di SATU tempat (`snapshotFor`, dipakai
     `HasFeature`/`RequireFeature`/`SubscriptionForMe`/`/api/me`/notification
     whatsapp gate) — TIDAK diduplikasi di tempat lain.
- ✅ `internal/identity/admin.go`: `AdminCreateSchoolAdmin` — implementasi
  murni testable `createSchoolAdmin` (nama wajib, minimal satu identifier,
  409 `conflict` bila email/username sudah dipakai — pengecekan pre-insert
  via `UserByEmail`/`UserByUsername`, SAMA pola dengan
  `student.createTeacherAccount`; 404 bila sekolah tidak ada via
  `SchoolGateway.SchoolStatusAndSlug` yang SUDAH ada dari fitur
  impersonation; password sementara 10 char pola P4.2; audit
  `admin.create_school_admin`). `httpx.Conflict(message)` (baru,
  `platform/httpx`) — helper 409 generik dipakai di sini.
- ✅ `internal/platformadmin/`: `Onboarding` (P2.2, 5 checklist +
  `ready = AND semua`), CRUD `PlatformAnnouncement*` (P5: Create/Update/Delete
  validasi title/body/rentang tanggal SAMA `internal/announcement`, audit
  `admin.platform_announcement_create/update/delete` — `school_id` SELALU
  NULL karena platform-wide, method `logPlatformAudit` terpisah dari
  `logAudit` existing karena butuh `entity_id`), `ActivePlatformAnnouncements`
  (tanpa gerbang permission, dipakai lintas modul), `SetRealtime`
  (`PlatformRealtime`, emit `announcement` broadcast SEMUA sekolah setelah
  create/update/delete sukses).
- ✅ `internal/announcement/`: `AnnouncementView`/`BoardItem` +field
  `is_platform` (bool, default false utk pengumuman sekolah); `List`
  (activeOnly) & `ActiveOn` DIGABUNG dengan `platformViews`/
  `platformBoardItems` (platform dulu, lalu sekolah) lewat
  `SetPlatformGateway` opsional (nil = pengumuman platform dilewati, tidak
  error) — `List` (activeOnly=false, daftar kelola sekolah) SENGAJA TIDAK
  ikut pengumuman platform (dikelola terpisah lewat panel super admin).
- ✅ `internal/dashboard/`: `AnnouncementItem` +field `is_platform`
  (passthrough dari `announcement.BoardItem.IsPlatform` lewat
  `dashboardadapter.go` — TV board otomatis dapat pengumuman platform karena
  sumbernya SATU, `announcement.Service.ActiveOn`, tidak ada perubahan logika
  di modul dashboard sendiri).
- ✅ `internal/realtime/hub.go`: `Hub.PublishAll(ev)` (baru) + adapter
  `realtimeForModules.PublishAll` (`cmd/server/realtimeadapter.go`).
- ✅ `internal/billing/`: `SubscriptionRecord.FeatureOverrides` (map, dari
  kolom baru); `mergeFeatures(plan, overrides)` dipakai `snapshotFor` (SEMUA
  gerbang fitur otomatis ikut merge, TANPA sentuh call site lain) &
  `subscriptionView`; `KnownFeatureKeys` (validasi P6.1); `SetFeatureOverrides`
  (merge true/false/set, nil/null=hapus key, 404 tanpa subscription, 422 key
  tak dikenal, invalidate cache SEBELUM return, audit
  `admin.feature_overrides`, return features efektif); `SubscriptionView`
  +field `feature_overrides`+`features_effective` (TAMBAHAN saja, `features`
  lama TIDAK berubah — frontend existing aman); `buildRevenueReport` (murni,
  agregasi per bulan SELALU 12 baris + per plan terurut plan_code) +
  `GetRevenueReport`/`ExportRevenueXLSX` (`revenue_export.go`, pola sama
  `attendance/export.go`: fungsi render murni + 2 sheet "Ringkasan"+"Invoice").
- ✅ Test (fake repo, tanpa DB): `internal/identity/admin_test.go`
  (`createSchoolAdmin`: sukses+audit+membership admin_sekolah, nama wajib,
  identifier wajib, 409 email/username bentrok, 404 sekolah tidak ada),
  `internal/platformadmin/service_test.go` (`Onboarding`: 404 sekolah tidak
  ada, ready hanya bila SEMUA true; platform announcement aktif by tanggal,
  CRUD+audit+`PublishAll` dipanggil tepat sekali per operasi sukses SAJA),
  `internal/announcement/service_test.go` (List/ActiveOn menggabung platform
  DULU lalu sekolah; platform gateway nil dilewati tanpa error),
  `internal/realtime/hub_test.go` (`PublishAll` menjangkau SEMUA sekolah
  dengan koneksi aktif; no-op aman tanpa koneksi), `internal/billing/
  service_test.go` (`SetFeatureOverrides`: merge true/false/null,
  422 key tak dikenal, 404 tanpa subscription, invalidate cache diverifikasi
  lewat `HasFeature` sebelum/sesudah; `buildRevenueReport`: agregasi per
  bulan/plan + kasus kosong; `GetRevenueReport` default tahun berjalan).

## Fase 14 — Paritas SION per-sekolah ✅ (acuan: docs/12-sion-parity.md — SELURUH gelombang A-D tuntas backend+frontend)
- ✅ Gelombang A: Kedisiplinan ✅ (backend) — terverifikasi end-to-end di
  Docker dev (`localhost:8210`, `demo.localhost`): admin login → `GET
  /api/discipline/types` (5 jenis seed: Terlambat 5, Atribut tidak lengkap
  10, Membolos 15, Merokok 50, Berkelahi 75) → `PUT /api/discipline/sp-settings
  {10,20,30}` → `POST /api/discipline/records` siswa NIS 22101 "Membolos" (15
  poin → total 15 ≥ sp1=10 → surat SP1 terbit otomatis `SP1/2026/0001`) →
  record siswa NIS 22103 (Budi, guardian `ortu.budi` — password tidak
  terdokumentasi, direset via script sekali pakai `cmd/resetpw` dijalankan
  lalu DIHAPUS, sama pola catatan Fase 9) "Membolos" → `SP1/2026/0002` →
  `GET /api/notifications` sbg ortu → `discipline.recorded` DAN
  `discipline.sp_issued` muncul → record kedua 22101 "Merokok" 50 poin (total
  65 ≥ sp3=30 → **SP3 terbit langsung, SP2 DILEWATI** sesuai keputusan "hanya
  level tertinggi yang terlampaui & belum ada surat", `SP3/2026/0003`) → `GET
  letters` (3 surat: SP1×2 + SP3) → `letters/{id}/pdf` (`application/pdf`,
  PDF valid 1 halaman) & `/html` (200, render dari snapshot) → `GET
  /api/students/1/discipline` sbg SISWA sendiri (200, `sp_level_reached:3`)
  & siswa lain (403) → `DELETE /api/discipline/types/3` yang sudah dipakai →
  409 → guru login → `POST records` (guru punya `discipline:record`) → total
  70 → **SP2 "menyusul" terbit** (`SP2/2026/0004`, level yang dilewati saat
  lompat SEBELUMNYA otomatis menyusul di panggilan berikutnya selama masih
  di atas ambang & belum pernah bersurat — lihat catatan desain di
  `Service.evaluateAndIssueLetter`) → guru coba `GET sp-settings` → 403
  (butuh `discipline:manage`) → `GET summary` (2 siswa, urut poin desc) +
  `GET export` (200 xlsx valid) → `DELETE records/1` → `GET letters/1/html`
  SEBELUM/SESUDAH delete → snapshot (Total Poin 15) TIDAK berubah (immutable
  terbukti) → sp-settings dikembalikan `{25,50,75}`. `go build/vet/test
  ./...` hijau; test service (fake repo): auto-terbit sekali per level +
  "menyusul" level yang dilewati saat lompat (`TestAutoIssueLetter_OncePerLevel`),
  lompat langsung ke level tertinggi dlm SATU catatan (`TestAutoIssueLetter_LevelJump_HanyaLevelTertinggi`),
  nomor urut per sekolah+tahun lintas siswa (`TestLetterNumber_Sequential`),
  unik per sesi presensi (`TestRecordViolation_UniquePerSession`),
  object-level ortu anak lain 403 (`TestStudentDiscipline_ObjectLevel_GuardianOtherChild403`),
  hapus jenis dipakai → 409 (`TestDeleteViolationType_InUse409`), snapshot
  immutable saat record dihapus (`TestDeleteRecord_SnapshotImmutable`),
  validasi urutan ambang, default sp-settings tanpa baris, gerbang permission
  `discipline:record`.
  - Migrasi `00014_discipline.sql`: `violation_types` (UNIQUE
    school_id+name), `student_violations` (partial UNIQUE
    `attendance_session_id,student_id,violation_type_id` WHERE
    attendance_session_id NOT NULL — anti dobel dalam SATU sesi, boleh
    berulang tanpa sesi), `discipline_sp_settings` (PK school_id+academic_year_id,
    CHECK sp1<sp2<sp3), `discipline_letter_number_counters` (counter atomik
    PER SEKOLAH per tahun — beda dari `invoice_number_counters` billing yang
    global lintas tenant, karena nomor surat dokumen resmi sekolah masing-masing),
    `discipline_warning_letters` (UNIQUE school_id+academic_year_id+student_id+level
    — sekali per level per TA per siswa; kolom `snapshot jsonb` IMMUTABLE)
  - Permission baru (`internal/identity/rbac.go` + docs/02-identity.md):
    `discipline:manage` (admin_sekolah), `discipline:record` (admin_sekolah,
    guru), `discipline:read` (admin_sekolah, kepala_sekolah, guru) — siswa/ortu
    TANPA permission modul ini, akses via object-level `student.CanViewStudent`
    (pola sama `attendance:read_own`)
  - `internal/discipline/` (modul baru, sqlc package `disciplinedb` DITULIS
    TANGAN — **sqlc CLI TIDAK tersedia** di host Windows dev maupun
    kontainer `sekolah-api-1`, dikonfirmasi ulang sesi ini sama seperti
    Fase 12/13; models.go HANYA memuat 4 tabel milik modul ini, bukan
    seluruh skema seperti output sqlc asli, karena Go tidak butuh model
    tabel yang tidak pernah discan langsung — dicatat di komentar
    `disciplinedb/models.go`): `GET/POST/PATCH/DELETE /api/discipline/types`,
    `GET/PUT /api/discipline/sp-settings` (default 25/50/75 dikembalikan
    TANPA menulis baris bila sekolah belum pernah menyimpan), `POST
    /api/discipline/records` (validasi siswa/jenis/sesi milik sekolah,
    default `occurred_on` = hari ini lokal sekolah, hitung ULANG total poin
    TA aktif dari `ListViolationsForStudentYear` — sumber tunggal dipakai
    juga oleh `GET /api/students/{id}/discipline` supaya tidak ada 2 cara
    menghitung total yang bisa berbeda), `DELETE /api/discipline/records/{id}`
    (audit old value, TIDAK menyentuh surat), `GET /api/discipline/records`
    (filter student_id/class_id/from/to/page — kelas SELALU dari enrollment
    TA AKTIF, TIDAK di-scope TA seperti endpoint siswa tunggal — keputusan
    sendiri, rentang tanggal bebas lintas TA), `GET /api/students/{id}/discipline`
    (read ATAU object-level, DI-SCOPE TA aktif: total_points/records/letters
    semuanya dari TA yang sama karena ambang SP per-TA), `GET
    /api/discipline/letters` + `/{id}/pdf` + `/{id}/html` (bentuk URL
    diadaptasi jadi segmen terpisah `.../pdf` & `.../html` — Go net/http
    ServeMux 1.22+ TIDAK mengizinkan wildcard `{id}` menempel ekstensi
    `{id}.pdf` dalam satu segmen, pola yang SAMA dengan
    `/api/rooms/{id}/qr.png` & `/api/billing/invoices/{id}/pdf` yang sudah
    ada), `GET /api/discipline/summary` + `/export` (xlsx, pola dua-lapis
    sama `attendance/export.go`)
  - **Keputusan lompat level** (dilaporkan sesuai instruksi tugas): SATU
    catatan yang melewati >1 ambang sekaligus HANYA menerbitkan level
    TERTINGGI yang terlampaui & belum ada surat SAAT ITU juga (bukan
    SP1+SP2+SP3 sekaligus). NAMUN karena `evaluateAndIssueLetter`
    mengevaluasi ULANG semua level dari nol setiap `POST records` dipanggil,
    level yang sempat terlewati "menyusul" terbit pada catatan BERIKUTNYA
    selama masih di atas ambangnya & belum pernah bersurat (dibuktikan e2e:
    SP2 menyusul setelah SP1+SP3 sudah ada) — begitu semua level relevan
    sudah bersurat, tidak ada surat baru lagi walau poin terus bertambah.
  - Nomor surat `SP{level}/{tahun}/{seq}` — `{tahun}` = tahun kalender saat
    terbit (`clock.Now().Year()`, pola sama `billing.nextInvoiceNumber`
    "INV/YYYY/NNNN"), `{seq}` atomik PER SEKOLAH PER TAHUN lintas SEMUA
    level (bukan counter terpisah per level) — dibuktikan e2e: SP1, SP1, SP3
    dapat seq 0001/0002/0003 berurutan.
  - Notifikasi baru (`internal/notification/model.go` + `templates.go`):
    `discipline.recorded` (→ ortu, webpush) & `discipline.sp_issued` (→
    ortu + wali kelas, webpush+email) — wali kelas diresolusi via join
    langsung `enrollments→classes→teachers` di `internal/discipline/queries.sql`
    (`GetHomeroomTeacherUserID`), BUKAN lewat consumer-side interface baru
    ke modul student (pola sama modul lain yang join langsung ke
    students/classes/users untuk data tampilan read-only)
  - Realtime: event `discipline` `{student_id}` ke roles
    admin_sekolah/kepala_sekolah/guru + `user_ids` ortu (via
    `SetRealtime`/`PublishTo`, pola SAMA modul lain — TIDAK diverifikasi
    lewat probe WebSocket live sesi ini, hanya compile+service test hijau,
    di luar cakupan WAJIB e2e curl tugas ini)
  - Interface publik baru `tenant.Service.BrandingAppName` (kop surat
    PDF/HTML, default "NouSchool") — dipenuhi lewat consumer-side interface
    `discipline.BrandingGateway`
  - Bootstrap idempoten (`cmd/bootstrap/ensureDemoDiscipline`, DIJALANKAN via
    SQL langsung sesi ini karena menjalankan ulang `cmd/bootstrap` penuh akan
    menimpa password super admin/akun demo lain dgn nilai baru — risiko yang
    dihindari, fungsi Go tetap ditambahkan untuk instalasi BARU/`-demo`
    berikutnya): 5 jenis pelanggaran + sp-settings TA aktif 25/50/75,
    diverifikasi idempoten (jalan 2× → 0 baris baru kedua kali)
  - `sqlc.yaml` ditambah blok `disciplinedb` (supaya `make sqlc` regenerate
    identik begitu sqlc CLI tersedia)
- ✅ Gelombang B: Izin siswa 3 alur + duties/capability flags + role pegawai + QR token guru + verifikasi surat publik + gate security
  - ✅ **Gelombang B1 (fondasi duty/pegawai + alur "izin terencana") backend** —
    terverifikasi end-to-end di Docker dev (`localhost:8210`, `demo.localhost`):
    bootstrap ulang (idempoten, `-demo`, super admin password direset sekali
    pakai — tidak terdokumentasi sebelumnya, sama pola catatan Fase 9/13) →
    seed 5 duty (Wali Kelas/Guru BK/Guru Piket/Pimpinan/Security) + pegawai
    `satpam`/`satpam12345` tercipta → admin login → `GET /api/duties` (5
    baris, `assignee_count` benar) → `GET /api/duties/2/assignments` (Guru BK
    = Sari) & `.../1/assignments` (Wali Kelas = Rendi) → kelas XII RPL 1
    (siswa NIS 22101) BELUM punya wali kelas tersimpan (`homeroom_teacher_id`
    kosong di DB, hanya duty assignment yang ada) → `PATCH /api/classes/1
    {"homeroom_teacher_id":1}` (teacher.id Rendi) sebagai admin → wali kelas
    resmi Rendi → siswa (`siswa`/`siswa12345`, NIS 22101) login → `POST
    /api/student-leave` multipart (sakit besok, tanpa lampiran) → 201
    `pending_homeroom` → Rendi `GET ?scope=queue` → muncul (match via join
    langsung `classes.homeroom_teacher_id`, BUKAN fallback flag) → `POST
    .../review {"decision":"approve"}` → `pending_bk` → Sari `GET
    ?scope=queue` → muncul (match via flag `leave_issuance`) → `POST
    .../issue {"decision":"approve"}` → `issued`, `letter_number`
    "SI/2026/0001", `verify_token` 24 karakter → siswa `GET ?scope=mine` →
    issued + nomor terlihat → `GET .../letter/pdf` → `application/pdf`, PDF
    valid 1 halaman (kop, jejak persetujuan, QR embed) → `GET
    /api/public/leave-verify?token=<valid>` → `valid:true` shape lengkap;
    token salah → `valid:false` tanpa detail bocor → jalur notifikasi kedua
    (izin siswa 22101 TIDAK punya ortu terdaftar di DB — sesuai catatan
    instruksi tugas, dialihkan ke siswa Budi Santoso NIS 22103 yang PUNYA
    ortu, sama kelas XII RPL 1 jadi wali kelas SAMA/Rendi): password `ortu.budi`
    direset sekali pakai via script sementara `cmd/resetpw` (dijalankan lalu
    DIHAPUS, pola sama Fase 9/14A) → ortu ajukan izin utk Budi (anaknya) → 201
    → ortu coba ajukan utk siswa 22101 (BUKAN anaknya) → 403 → chain
    disetujui penuh (Rendi approve → Sari issue) → `GET /api/notifications`
    ortu → `studentleave.decided` muncul; Rendi → `studentleave.submitted` +
    `studentleave.decided` (dirinya sendiri di roles admin_sekolah tidak
    relevan, tapi user_ids target ikut dia sbg wali) muncul; Sari →
    `studentleave.forwarded` muncul → jalur reject: siswa 22101 ajukan
    pengajuan ke-3 → Rendi `POST .../review {"decision":"reject"}` → status
    `rejected`, notif `studentleave.decided` ("ditolak wali kelas") ke siswa
    → jalur lampiran+cancel: siswa ajukan ke-4 dgn lampiran PDF → `GET
    .../attachment` sbg pemilik → 200 `application/pdf`; sbg satpam (bukan
    pemilik/reviewer/admin) → 403 → `POST .../cancel` sbg pemilik saat
    pending → 200; cancel lagi → 422 (sudah tidak pending) → login satpam
    (pegawai) → `GET /api/me` → `role:"pegawai"`, `features` tetap muncul
    (subscription tidak digerbang role) → satpam `GET
    /api/student-leave?scope=queue` → 200 `{items:[],total:0}` (TIDAK punya
    flag leave_homeroom_review/leave_issuance, hanya exit_security Gelombang
    B2 yang belum dipakai endpoint mana pun) → satpam `GET /api/duties` →
    403 (tidak punya `duty:manage`) → kepsek `GET /api/duties` → 403 juga
    (keputusan tugas: kepsek TIDAK butuh kelola tugas tambahan) → admin `GET
    ?scope=all` (perm `student:manage`) → 2 baris → `GET /api/employees` →
    satpam muncul → `DELETE /api/duties/1` (Wali Kelas, punya assignment
    Rendi) → 409 (sarankan nonaktifkan). `go build/vet/test ./...` hijau;
    test service (fake repo): `internal/duty` — `UserHasFlag` benar untuk TA
    aktif+duty aktif, salah (false) saat duty dinonaktifkan, salah saat
    assignment ada di TA LAIN (bukan TA aktif); `PutAssignments` menolak 422
    user yang rolenya TIDAK cocok `duty.for_role`, menerima user yang cocok;
    `DeleteDuty` 409 saat masih punya assignment. `internal/studentleave` —
    state machine urutan penuh (pending_homeroom→pending_bk→issued), reject
    menghentikan chain (tidak bisa lanjut ke tahap BK), cancel HANYA pending
    & HANYA pemilik (bukan pemilik → 403; sudah diputuskan → ditolak);
    otorisasi reviewer: wali kelas rombel LAIN → 403, fallback pemegang flag
    `leave_homeroom_review` (kelas belum punya wali) → boleh review; nomor
    surat & verify_token UNIK antar 2 pengajuan berbeda; ortu ajukan izin
    utk anak sendiri OK, anak orang lain 403; shape publik verify (token
    valid vs salah, TANPA membocorkan detail apa pun saat salah).
    - Migrasi `00015_duty_studentleave.sql`: `duties` (UNIQUE
      school_id+name, CHECK for_role IN guru/pegawai), `duty_assignments`
      (UNIQUE duty_id+user_id+academic_year_id — pemegang tugas PER TA),
      `employees` (UNIQUE school_id+user_id — profil pegawai, pola sama
      `teachers`), `student_leave_requests` (CHECK type sakit/izin, CHECK
      status 5 nilai state machine, partial UNIQUE
      school_id+letter_number WHERE NOT NULL, `verify_token` UNIQUE global),
      `student_leave_number_counters` (counter atomik PER SEKOLAH PER TAHUN,
      pola sama `discipline_letter_number_counters`)
    - Permission baru `duty:manage` (HANYA admin_sekolah — docs/02:
      "kepsek tidak butuh kelola tugas tambahan") + role baru **`pegawai`**
      (`internal/identity/rbac.go`, `rolePermissions[pegawai] = {}` SENGAJA
      kosong — otorisasi pegawai lewat capability flags modul duty, BUKAN
      RBAC; `PickActiveRole` prioritas persis setelah guru, sebelum
      orang_tua/siswa)
    - `internal/duty/` (modul baru, sqlc package `dutydb`): capability flags
      kanonik `leave_homeroom_review`/`leave_issuance` (dipakai Gelombang
      B1) + `exit_bk_approval`/`exit_leadership_approval`/`exit_security`/
      `late_arrival_duty`/`late_arrival_leadership`/`all_attendance_reports`
      (DIDEFINISIKAN sekarang sesuai instruksi tugas, dipakai Gelombang B2 —
      belum ada endpoint yang menggerbang dengan flag-flag ini) + label
      Indonesia; `GET/POST/PATCH/DELETE /api/duties` (DELETE 409 bila punya
      assignment DI TA MANA PUN, sarankan nonaktifkan) + `GET
      /api/duties/flags`; `GET/PUT /api/duties/{id}/assignments`
      (replace-all TA AKTIF dalam transaksi delete-all+insert-all, validasi
      role via `IdentityGateway.UsersWithRole` yang SUDAH ada — TIDAK bikin
      query membership baru); **interface publik** (dipakai studentleave via
      consumer-side interface) `Service.UserHasFlag`/`UserIDsWithFlag`
      (SELALU di-scope TA AKTIF + `duties.active`, `[]`/false tanpa error
      bila sekolah belum punya TA aktif — BUKAN error)
    - `internal/employee/` (modul baru, sqlc package `employeedb`): profil
      pegawai (pola tipis sama `student.Teacher`) — `GET/POST/PATCH
      /api/employees` (perm SENGAJA `student:manage`, BUKAN permission baru
      — keputusan sendiri, dilaporkan: admin sekolah yang sama yang kelola
      siswa yang kelola pegawai); `POST` membuat user+membership
      pegawai+profil employees, `temp_password` SEKALI TAMPIL (pola
      IDENTIK `internal/identity/admin.go` P2.1/P4.2 — charset
      `abcdefghijkmnpqrstuvwxyz23456789` tanpa 0/o/1/l, didefinisikan ULANG
      lokal karena employee tidak boleh mengimpor identity)
    - `internal/studentleave/` (modul baru, sqlc package `studentleavedb`):
      state machine `pending_homeroom → pending_bk → issued` | reject di
      tahap mana pun → `rejected` | cancel pemilik selama pending_* →
      `canceled`; **otorisasi reviewer via consumer-side interface DutyGateway
      + join langsung** (BUKAN role/permission RBAC): tahap wali kelas = user
      yang SAMA dengan `classes.homeroom_teacher_id` rombel siswa PADA TA
      request (query join `enrollments→classes→teachers`, pola SAMA
      `discipline.GetHomeroomTeacherUserID`) ATAU pemegang flag
      `leave_homeroom_review` (fallback bila kelas belum punya wali); tahap
      BK = HANYA pemegang flag `leave_issuance` (TANPA fallback role); `POST
      /api/student-leave` (siswa utk dirinya via `MyStudentID`, orang tua utk
      anaknya via `CanViewStudent` — object-level, BUKAN dari body request
      begitu saja); `GET ?scope=mine|queue|all&status=` (mine: siswa diri
      sendiri/ortu SEMUA anaknya via `MyChildStudentIDs`; queue: union rombel
      perwalian sendiri + SEMUA pending_homeroom bila pemegang flag review +
      SEMUA pending_bk bila pemegang flag issuance; all: perm
      `student:manage`); `POST .../review` & `.../issue` `{decision:'approve'
      |'reject', comment?}` (nilai LITERAL beda dari `internal/leave` yang
      pakai 'approved'/'rejected' — sesuai instruksi tugas); `POST
      .../cancel`; `GET .../attachment` & `.../letter/pdf` (otorisasi
      SATU fungsi `authorizeView`: pemilik/siswa-sendiri/ortu-anaknya/
      reviewer-tahap-mana-pun/admin); `GET /api/public/leave-verify?token=`
      (PUBLIK host tenant, TANPA requireAuth — token tetap difilter
      school_id tenant saat ini sesuai aturan multi-tenant CLAUDE.md;
      token tak dikenal → `{"valid":false}` HTTP 200, BUKAN 404 — keputusan
      sendiri: konsisten dgn konvensi envelope `httpx.JSON` yang dipakai
      SELURUH endpoint publik lain di codebase ini, mis. `GET
      /api/public/context`)
    - Nomor surat `SI/{tahun}/{seq}` (counter atomik PER SEKOLAH PER TAHUN,
      pola SAMA `discipline.formatLetterNumber` "SP{level}/{tahun}/{seq}"
      minus level) + `verify_token` 24 karakter alfanumerik acak
      (`crypto/rand`, pola sama `identity.generateTempPassword` tapi charset
      lebih luas & panjang beda)
    - Surat izin PDF (`internal/studentleave/pdf.go`, fpdf — pola dua-lapis
      sama `discipline/pdf.go`): kop app_name (via `tenant.BrandingAppName`),
      nomor surat, identitas siswa+kelas, jenis+rentang+alasan, jejak
      persetujuan (wali & BK + waktu), **QR code verifikasi**
      (`github.com/skip2/go-qrcode`, PNG di-embed via
      `fpdf.RegisterImageOptionsReader` — pola BARU di codebase ini, belum
      ada presedan embed gambar ke fpdf sebelumnya) berisi URL
      `https://{host}/verifikasi-surat?token={verify_token}` (host dari
      `r.Host` request, diteruskan handler→service) + teks URL di bawah QR
    - Notifikasi baru (3 event, `internal/notification/model.go` +
      `templates.go`): `studentleave.submitted` (→ pemegang review tahap 1,
      webpush), `studentleave.forwarded` (→ pemegang flag leave_issuance,
      webpush), `studentleave.decided` (→ pengaju+ortu (+wali kelas bila
      surat terbit), webpush+email) — didaftarkan docs/08-notification.md
    - Realtime: event baru `studentleave` `{request_id}` → target pemilik
      user_ids + roles admin_sekolah + reviewer target user_ids (pola SAMA
      modul lain, `SetRealtime`/`PublishTo` — TIDAK diverifikasi lewat probe
      WebSocket live sesi ini, hanya compile+service test hijau, di luar
      cakupan WAJIB e2e curl tugas)
    - Interface publik baru: `student.Service.MyChildStudentIDs` (signature
      PRIMITIF `[]int64`, BEDA dari `ListMyChildren` yang mengembalikan
      `[]ChildRef` non-primitif — TIDAK bisa dipakai consumer-side interface
      lintas modul per aturan CLAUDE.md "signature primitif", dipakai
      `studentleave.StudentAccess` utk scope=mine ortu)
    - `sqlc.yaml` ditambah 3 blok (`dutydb`/`employeedb`/`studentleavedb`) —
      **sqlc CLI TERSEDIA & DIPAKAI sesi ini** (`go run
      github.com/sqlc-dev/sqlc/cmd/sqlc@latest generate` dari host, BERHASIL
      — beda dari catatan Fase 12-14A yang mengeluhkan CLI tidak tersedia;
      seluruh kode `*db` package Gelombang B1 adalah GENERATED, bukan tulisan
      tangan)
    - Bootstrap idempoten (`cmd/bootstrap`): `ensureDemoEmployee` (pegawai
      `satpam`/`satpam12345`) + `ensureDemoDuties` (5 duty + assignment TA
      aktif Wali Kelas→Rendi, Guru BK→Sari, Security→satpam; Guru Piket &
      Pimpinan dibuat TANPA assignment — belum ada akun contoh Gelombang B2)
      — dijalankan via `*.Repository` langsung (BUKAN `*.Service`, yang
      digerbang `requireManage`/reqctx.Role tidak tersedia di konteks
      bootstrap — pola SAMA `ensureDemoDiscipline`)
    - **Keputusan dilaporkan**: kelas XII RPL 1 (siswa NIS 22101, dipakai
      verifikasi e2e wajib) TIDAK punya `homeroom_teacher_id` tersimpan di DB
      sebelum sesi ini (hanya assignment duty "Wali Kelas" yang ada, tabel
      BEDA) — di-set manual via `PATCH /api/classes/1
      {"homeroom_teacher_id":1}` sebagai admin sebelum verifikasi jalur wali
      kelas asli (bukan fallback flag) bisa diuji, sesuai instruksi tugas
      eksplisit. Siswa NIS 22101 juga TIDAK punya ortu terdaftar — jalur
      notifikasi ortu diverifikasi memakai siswa Budi Santoso (NIS 22103,
      guardian `ortu.budi` sudah ada dari Fase 9, password direset sekali
      pakai via script sementara `cmd/resetpw` yang dibuat lalu DIHAPUS sesi
      ini, sama pola catatan Fase 9/14A).
  - ✅ Gelombang B2 (QR token guru + izin dispensasi keluar + izin terlambat)
    backend — terverifikasi end-to-end di Docker dev (`localhost:8210`,
    `demo.localhost`): bootstrap ulang (idempoten, super admin password
    reset — `admin@nouschool.id`, tidak terdokumentasi sebelumnya, sama pola
    Fase 9/13/14A/B1) → `GET /api/duties/{id}/assignments` konfirmasi
    keputusan demo (lihat "Keputusan dilaporkan" di bawah): Wali
    Kelas=Rendi, Guru BK=Sari, **Guru Piket=Sari, Pimpinan=Sari**,
    Security=Pak Satpam → siswa NIS 22101 (`siswa`/`siswa12345`) TIDAK punya
    ortu (sama seperti B1) → generate kode undangan kelas XII RPL 1 →
    aktivasi kode siswa Budi Santoso (NIS 22103, guardian `ortu.budi` sudah
    ada dari Fase 9) → akun siswa baru `budi`/`budi12345` → super admin
    `POST /api/admin/users/6/reset-password` (endpoint EXISTING, host
    platform) → password `ortu.budi` diketahui tanpa script sekali pakai
    (beda dari catatan Fase 9/14A/B1 — kali ini ada endpoint resmi yang
    cocok) → **QR token guru**: Sari `POST /api/teacher-qr` → `{token
    24 char, expires_at +60dtk}`; pegawai (satpam) `POST /api/teacher-qr` →
    403 (role guru saja) → **dispensasi keluar**: Budi `POST
    /api/exit-permits {reason}` → `pending_duty_teacher` → scan token Sari
    (flag `late_arrival_duty`, label piket) → `pending_class_teacher` →
    Budi coba scan token Sari LAGI utk tahap 2 → **422 "guru pengajar tahap
    ini harus berbeda dari guru piket"** (beda-orang teruji) → scan token
    Rendi (guru pengajar XII RPL 1 jam berjalan, divalidasi via
    `schedule.ClassSlotNowOrNext`) → `pending_bk` → scan token Sari (flag
    `exit_bk_approval`) → `pending_leadership` → scan token Sari (flag
    `exit_leadership_approval`) → `issued` + `gate_token` 24 char +
    `gate_expires_at` (akhir period terakhir hari itu) → `GET
    /api/notifications` Budi & ortu.budi → `exitpermit.issued` muncul
    keduanya → satpam `POST /api/exit-permits/gate-scan {gate_token}` →
    `exited` (`{student,reason,issued_at,gate_expires_at,exited_at}`); guru
    (bukan pemegang flag `exit_security`) coba gate-scan → 403 → gate-scan
    ULANG token yang sama → 409 "sudah pernah keluar gerbang" → `GET
    /api/notifications` ortu.budi → `exitpermit.exited` muncul (jam WIB
    disebut) → `GET .../gate-history` (satpam DAN kepsek via `student:read`,
    dua gerbang OR teruji) → 1 baris → `GET ?scope=mine` (Budi, issued+trail
    lengkap) & `?scope=all` (admin) → cocok → **maks 1 permit aktif**: siswa
    NIS 22101 `POST /api/exit-permits` → 201 → POST lagi (masih pending) →
    409 → `POST .../cancel` (pemilik) → 200 → ajukan lagi → admin `POST
    .../reject {comment}` (perm `student:manage`) → `rejected`; siswa coba
    reject → 403 → **izin terlambat**: Budi `POST /api/late-arrivals/scan`
    token Sari (flag `late_arrival_duty`) → record BARU dibuat langsung
    `pending_leadership`, `late_count:1 action:"none"` (hitungan TA aktif +1)
    → `GET /api/notifications` ortu.budi → `latearrival.recorded` muncul
    ("terlambat ke-1", TANPA kalimat aksi krn action none) → scan token Sari
    LAGI (flag `late_arrival_leadership`) → `pending_class_teacher` → scan
    token Rendi (guru ber-slot XII RPL 1 hari itu, TANPA syarat jam
    berjalan/berikutnya — beda dari exit-permit) → `completed` → scan LAGI
    hari yang sama → **409 "sudah tercatat & selesai untuk hari ini"** (maks
    1 record/hari teruji) → token sudah dipakai (Rendi, reuse) → 410 "QR
    tidak berlaku" → Rendi (bukan pemegang flag `late_arrival_duty`) coba
    jadi piket → 422 → `GET ?scope=mine` (Budi) & `?scope=today` (admin) &
    `?scope=all` (kepsek) & `GET .../summary` (admin & kepsek, 1 siswa count
    1) → siswa coba summary → 403. **Jalur "jam berjalan" exit-permit tahap
    2** DIVERIFIKASI LIVE dengan trik waktu (lihat catatan periode uji di
    bawah — direvert setelah verifikasi, TIDAK permanen). `go build/vet/test
    ./...` hijau; test service (fake repo/gateway) — `internal/teacherqr`:
    generate hanya role guru, consume valid, consume KEDUA pada token sama
    gagal (simulasi race single-use), consume token kedaluwarsa (410),
    terima token dengan/tanpa awalan `nouschool:tqr:`, token tak dikenal;
    `internal/exitpermit`: rantai penuh 4 tahap, tahap 1&2 orang sama
    ditolak, salah tahap (guru tanpa flag duty), maks 1 permit aktif, cancel
    hanya pemilik+pending, reject admin (siswa ditolak), gate-scan expiry
    (`clock.Fixed` sebelum/sesudah) & scan ganda (409) & gerbang wajib flag
    `exit_security`; `internal/latearrival`: `lateArrivalAction` MURNI
    (count 1..7, fungsi terisolasi dari I/O), rantai penuh 3 tahap, salah
    flag piket/pimpinan, salah guru kelas, hitungan lintas hari (`late_count`
    TA-scoped, bukan per-hari) → action `call_parent` pada telat ke-2.
    - Migrasi `00016_exit_late.sql`: `teacher_qr_tokens` (UNIQUE token,
      TTL via `expires_at`, `consumed_at` nullable — sekali pakai),
      `student_exit_permits` (CHECK status 8 nilai state machine, kolom
      `{tahap}_by`/`{tahap}_at` per tahap + `rejected_*`/`gate_token` UNIQUE
      + `gate_expires_at`/`exited_*`), `student_late_arrivals` (CHECK action
      3 nilai, CHECK status 4 nilai — `pending_duty_teacher` TIDAK PERNAH
      benar-benar tersimpan, hanya nilai konseptual "belum ada record",
      lihat catatan `internal/latearrival/model.go`)
    - `internal/teacherqr/` (modul baru, KECIL SENGAJA — tanpa
      IdentityGateway/audit, token ephemeral bukan "mutasi penting" CLAUDE.md):
      `POST /api/teacher-qr` (role guru dicek `reqctx.Role` langsung, BUKAN
      permission RBAC — pegawai TIDAK bisa) → token 24 char TTL 60 detik,
      cleanup lazy (token kedaluwarsa MILIK USER ITU dihapus saat generate
      baru); **interface publik** `Service.ConsumeToken(ctx, schoolID,
      rawToken) (teacherUserID int64, err error)` (dipakai
      exitpermit/latearrival lewat consumer-side interface, signature SUDAH
      primitif — TIDAK butuh adapter) — atomik via `UPDATE ... WHERE
      consumed_at IS NULL AND expires_at > now RETURNING user_id` (row lock
      implisit Postgres, dua consume bersamaan HANYA SATU berhasil); sukses
      publish realtime `teacherqr` `{}` ke user_id pemilik (frontend guru
      auto-regenerate QR); gagal → `410 qr_expired` "QR tidak berlaku. Minta
      guru menampilkan QR baru." (pesan PERSIS sesuai instruksi tugas)
    - `internal/exitpermit/` (modul baru): state machine
      `pending_duty_teacher → pending_class_teacher → pending_bk →
      pending_leadership → issued → exited` | reject (admin) di tahap mana
      pun sebelum exited → `rejected` | cancel (pemilik) selama pending_* →
      `canceled`. Tahap 1 = flag `late_arrival_duty` (DIBAGI dgn
      latearrival — "piket" satu konsep dipakai 2 alur, sesuai docs tugas
      literal). Tahap 2 = **BUKAN flag**, murni jadwal: `ScheduleGateway.
      TeacherMatchesClassNow` (guru dgn slot kelas siswa SEDANG berjalan,
      atau BILA TIDAK ADA, slot PALING AWAL berikutnya hari itu) DAN
      `teacherUserID != duty_by` (beda orang, eksplisit, dicek SEBELUM
      panggil schedule gateway supaya pesan error jelas). Tahap 3 = flag
      `exit_bk_approval`. Tahap 4 = flag `exit_leadership_approval` →
      set `gate_token` (24 char) + `gate_expires_at` (
      `ScheduleGateway.GateExpiryToday` = akhir period TERAKHIR hari itu,
      fallback `at+6 jam` bila sekolah belum punya period). `POST
      /api/exit-permits` (SISWA saja utk dirinya sendiri via `MyStudentID` —
      BEDA dari studentleave yg juga terima orang tua, sesuai spek tugas
      literal "(siswa)"), maks 1 permit AKTIF (pending_*/issued) per siswa
      → 409; `POST .../{id}/scan {token}` (SISWA PEMILIK, consume via
      teacherqr lalu validasi tahap, SEMUA transaksional lewat `UPDATE ...
      WHERE status = $tahap_lama` execrows race guard) → response permit +
      `stage_advanced_to`; `POST .../{id}/cancel` (pemilik, pending saja);
      `POST .../{id}/reject {comment}` (perm `student:manage`, admin, tahap
      mana pun sebelum exited — lihat "Keputusan dilaporkan" di bawah);
      `GET ?scope=mine|active|all&date=` (mine: siswa/ortu; active: permit
      HARI INI non-final, perm `student:read`; all: perm `student:manage`);
      `POST /api/exit-permits/gate-scan {gate_token}` (flag `exit_security`,
      validasi issued+belum-expired → `410 gate_expired` "Izin sudah
      kedaluwarsa." PERSIS sesuai instruksi, atau `409` bila sudah
      exited/belum issued) → `{student,reason,issued_at,gate_expires_at,
      exited_at}`; `GET .../gate-history?date=` (flag `exit_security` ATAU
      perm `student:read`, dua gerbang OR)
    - `internal/latearrival/` (modul baru): SATU endpoint `POST
      /api/late-arrivals/scan {token}` menjalankan SELURUH state machine
      (docs tugas): belum ada record hari itu (dicek via rentang tanggal
      LOKAL sekolah) → token pemegang flag `late_arrival_duty` → CREATE
      LANGSUNG `pending_leadership` (duty_by/duty_at terisi saat baris
      dibuat — TIDAK ADA baris `pending_duty_teacher` yang benar-benar
      tersimpan, lihat model.go), `late_count` = COUNT seluruh record siswa
      TA AKTIF (lintas hari, BUKAN per-hari) + 1, `action` dari
      `lateArrivalAction` MURNI (ke-2&5→`call_parent`, ke-3&6→`send_home`,
      lainnya `none`) → notif ortu SEGERA; record `pending_leadership` →
      flag `late_arrival_leadership` → `pending_class_teacher`; →
      `ScheduleGateway.TeacherTeachesClassToday` (guru py SLOT kelas siswa
      hari itu, TANPA syarat jam berjalan/berikutnya — beda dari
      exitpermit) → `completed`. **Maks 1 record/hari** (keputusan tugas,
      DITAMBAHKAN eksplisit): record `completed` hari itu memblokir scan
      baru (409) — bukan CHECK/index DB, ditegakkan service layer via
      rentang tanggal lokal. `GET ?scope=mine|today|all&month=` (mine:
      siswa/ortu; today: perm `student:read`; all: perm `student:manage`
      ATAU role `kepala_sekolah`); `GET .../summary?month=` (admin/kepsek,
      per siswa count bulan berjalan)
    - Interface publik BARU `schedule.Service`: `ClassSlotNowOrNext` (slot
      kelas sedang berjalan/berikutnya hari itu — dipakai exitpermit tahap
      2), `TeachesClassToday` (ADA slot kelas hari itu, tanpa syarat jam —
      dipakai latearrival tahap akhir), `LastPeriodEndToday` (akhir period
      terakhir hari itu waktu lokal sekolah — dipakai exitpermit
      gate_expires_at). **KEPUTUSAN desain**: KETIGANYA TIDAK dipenuhi
      langsung oleh exitpermit/latearrival — SlotView.Teacher.ID adalah
      profil guru (FK teachers), bukan user_id hasil `teacherqr.ConsumeToken`
      — dijembatani adapter BARU `cmd/server/b2adapter.go` (`b2ScheduleGateway`,
      pola SAMA `scheduleadapter.go`) yang MENGGABUNGKAN
      `schedule.Service` + `student.Service.MyTeacherID` (method YANG SUDAH
      ADA sejak fase 5, TIDAK ada method baru di modul student) untuk
      memetakan teacher profil ID ↔ user_id
    - Notifikasi baru (`internal/notification/model.go` + `templates.go`):
      `exitpermit.issued` (→ siswa+ortu, push+in_app, "tunjukkan QR di
      gerbang"), `exitpermit.exited` (→ ortu, push+in_app, sebut jam WIB),
      `latearrival.recorded` (→ ortu, push+in_app, sebut hitungan + label
      Indonesia aksi BILA ADA — `{{if .action}}` di template, `action:""`
      utk `ActionNone` supaya kalimat aksi disembunyikan sepenuhnya, BUKAN
      menampilkan "none"); didaftarkan `docs/08-notification.md`
    - Realtime: event `teacherqr` `{}` → user_id pemilik token (consume);
      `exitpermit` `{permit_id}` → pemilik+roles admin_sekolah/kepala_sekolah
      (submit/scan/cancel/reject/gate-scan); `latearrival` `{}` → roles
      admin_sekolah/kepala_sekolah+pemilik+guru yg scan (SEMUA via
      `SetRealtime`/`PublishTo`, pola sama modul lain — TIDAK diverifikasi
      lewat probe WebSocket live sesi ini, hanya compile+service test hijau,
      di luar cakupan WAJIB e2e curl tugas)
    - `sqlc.yaml` ditambah 3 blok (`teacherqrdb`/`exitpermitdb`/
      `latearrivaldb`) — sqlc CLI TERSEDIA & DIPAKAI (`go run
      github.com/sqlc-dev/sqlc/cmd/sqlc@latest generate` dari host,
      BERHASIL, sama seperti Gelombang B1) — seluruh kode `*db` package
      GENERATED
    - Bootstrap idempoten (`cmd/bootstrap/ensureDemoDuties`): Guru Piket &
      Pimpinan (BELUM punya assignee sejak B1) di-assign ke Sari (lihat
      "Keputusan dilaporkan" di bawah)
    - **Keputusan dilaporkan (deviasi reject, docs tugas eksplisit meminta
      dilaporkan)**: rantai QR SION TIDAK punya "tolak" formal per tahap
      (approver hanya bisa menyetujui via scan — tidak ada UI tolak per
      tahap di SION). Diputuskan: siswa boleh CANCEL selama pending (pola
      sama studentleave) DAN admin_sekolah (perm `student:manage`) diberi
      `POST .../reject` administratif di tahap mana pun sebelum exited,
      utk kasus keliru/darurat — TANPA meniru UI approve/reject per tahap
      yang tidak punya presedan SION. Diuji e2e (admin reject berhasil,
      siswa reject 403).
    - **Keputusan dilaporkan (demo duty Gelombang B2)**: Guru Piket &
      Pimpinan (kosong sejak Gelombang B1) di-assign ke **Sari** (BUKAN
      Rendi) — Sari jadi pemegang SEMUA flag approval (Guru BK+Piket+
      Pimpinan = `leave_issuance`+`exit_bk_approval`+`late_arrival_duty`+
      `exit_leadership_approval`+`late_arrival_leadership`), Rendi TETAP
      hanya Wali Kelas + pengajar terjadwal. Alasan: tahap 2 rantai keluar
      ("guru pengajar jam berjalan") murni dari JADWAL, bukan flag — demo
      menjadwalkan Rendi mengajar XII RPL 1 (`ensureDemoTodaySlots`
      existing), jadi dengan SEMUA approval lain di tangan Sari, aturan
      "tahap 1 ≠ tahap 2" SELALU bisa diuji bersih (Sari≠Rendi) tanpa
      bergantung jam berapa e2e dijalankan — sesuai saran eksplisit tugas
      "(mis. Sari = Piket+BK+Pimpinan, Rendi = pengajar)".
    - **Catatan periode uji (TIDAK permanen)**: sesi ini dijalankan jam
      15:xx WIB — SEMUA 9 period demo (07:00–14:00) sudah lewat, jadi
      `ClassSlotNowOrNext` tidak nemu slot "sekarang/berikutnya" apa pun utk
      diuji live. Ditambahkan SEMENTARA period ke-10 (15:00–23:59) + 1 slot
      XII RPL 1/Rendi via API admin biasa (`PUT /api/periods`, `POST
      /api/schedule/slots`) supaya tahap 2 exit-permit bisa diuji live
      dengan guru yang BENAR (bukan cuma unit test) — **DIHAPUS/DIREVERT**
      setelah verifikasi (`DELETE` slot, `PUT /api/periods` kembali ke 9
      semula, dikonfirmasi `GET /api/schedule/today?class_id=1` kembali ke 2
      slot asli). Jalur "tidak ada slot sama sekali" (`hasSlot=false`) &
      gate-expiry murni waktu (410) HANYA diuji unit test (`clock.Fixed`),
      sesuai presedan Fase 6 ("cukup, sesuai instruksi tugas") — di luar itu
      SEMUA jalur lain (termasuk tahap 2 dgn guru benar/salah/sama-orang)
      terverifikasi LIVE.
    - Akun tambahan sesi ini: siswa `budi`/`budi12345` (aktivasi kode
      undangan NIS 22103, tertaut guardian `ortu.budi` — dipakai verifikasi
      notifikasi ortu exit-permit & late-arrival, siswa demo NIS 22101 TETAP
      tanpa ortu sejak B1)
- ✅ Gelombang C: Penilaian (modul baru `grading`, toggle per sekolah) ✅ backend selesai (Fase 14)
  Backend terverifikasi end-to-end di Docker dev (`demo.localhost`): admin
  login → `GET /api/grading/status` (`enabled:true`, seed sudah aktif) →
  rendi login → `GET /api/grading/components?class_id=1&subject_id=1` → 3
  komponen seed (TP1 w30/Sumatif w50/Praktik w20, kktp 75 semua),
  `total_weight:100` → `POST` komponen baru "Tugas" weight 20 kktp 70 → 201,
  `total_weight` jadi 120 (normalisasi, BUKAN /100 — boleh) → `PUT
  .../components/4/grades` nilai siswa 1 (90) & siswa 2 (85) → `GET
  /api/grading/recap?class_id=1&subject_id=1` → final DIVERIFIKASI MANUAL:
  siswa 1 (semua 4 komponen terisi) = (80×30+85×50+78×20+90×20)/120 =
  **83.4167** → label "B" (bulat half-up 83); siswa 3/Budi (3 komponen
  terisi, Tugas kosong) = (60×30+65×50+55×20)/100 = **61.5** → label "C",
  `below_kktp:[1,2,3]` (semua di bawah kktp 75); siswa 2/Siti (TP1+Tugas+
  Sumatif, Praktik kosong) = (70×30+75×50+85×20)/100 = **75.5** → label "B",
  `below_kktp:[1]` (hanya TP1 70<75) — SEMUA angka cocok dengan hasil API →
  sari coba `PATCH /api/grading/components/1` (Basis Data XII RPL 1,
  bukan miliknya) → 403 (object-level `schedule.TeachesClassSubject`
  terbukti benar) → `PUT /api/grading/publication` publish → siswa
  (`siswa`/`siswa12345`, student_id 1) `GET /api/my-grades` → muncul 1 mapel
  (Basis Data) final 83.4167 label B + 4 komponen lengkap → ortu.budi (password
  direset sekali pakai via `POST /api/admin/users/{id}/reset-password`,
  endpoint RESMI host platform — beda dari catatan Fase 9/14A/B1 yang butuh
  script sekali pakai) `GET /api/my-grades?student_id=3` (anaknya Budi) →
  200, final 61.5 label C vs `?student_id=1` (BUKAN anaknya) → 403 → stars:
  rendi `POST /api/grading/stars` siswa 1 delta+1 visibility=private → siswa
  `GET /api/my-stars` → `total:2` (HANYA 2 bintang seed visibility=student
  yang tampil, bintang private BARU TIDAK ikut — keputusan "total & items
  sama-sama hanya visible" terbukti benar) → admin `GET
  /api/grading/stars?student_id=1` (raw, admin/guru lihat SEMUA termasuk
  private, 3 baris) & `?class_id=1` (total per siswa) → rendi `DELETE
  .../stars/3` (miliknya sendiri) → 200 → `GET /api/grading/export` → 200
  xlsx (6590 bytes, `Microsoft Excel 2007+` valid) → admin `PUT
  /api/settings/grading {enabled:false}` → `GET /api/grading/components` →
  404 `grading_disabled` (SEMUA endpoint lain digerbang, `GET
  /api/grading/status` TETAP 200 `enabled:false`) → `PUT` kembali
  `{enabled:true}` → components bisa diakses lagi. `go build/vet/test ./...`
  hijau; test service (fake repo/gateway): normalisasi bobot ≠100, siswa
  sebagian komponen terisi, siswa tanpa nilai sama sekali (final null), label
  ranges (batas persis 74/75/84/85 + pembulatan half-up termasuk kasus .5),
  validasi ranges settings (celah/tumpang-tindih/tidak mulai 0/tidak
  berakhir 100 ditolak), guard disabled → 404 grading_disabled, object-level
  guru (kelas-mapel bukan miliknya 403, dengan slot diizinkan, admin bebas
  bypass), publish → notif target TEPAT (siswa ber-akun + SEMUA ortu, siswa
  tanpa akun di-skip; unpublish TIDAK mengirim notif), stars visibility
  (siswa TIDAK lihat private, total HANYA dari visible), delta=0 ditolak,
  guru butuh `TeachesClass` (kelas, mapel apa pun) utk bintang.
  - ✅ Migrasi `00017_grading.sql`: `assessment_components` (class_id/
    subject_id FK, type CHECK tp/sumatif/praktik/lainnya, weight>0, kktp
    0..100), `student_grades` (score numeric(5,2) NOT NULL — baris HANYA ada
    bila nilai terisi, UNIQUE component_id+student_id, FK component_id ON
    DELETE CASCADE), `grade_publications` (PK school+TA+class+subject),
    `classroom_star_events` (delta CHECK !=0, visibility CHECK
    private/student)
  - ✅ **Keputusan desain**: kolom `student_grades.score` (numeric) SELALU
    di-cast `::float8` di setiap query `internal/grading/queries.sql` supaya
    sqlc menghasilkan Go `float64` murni — TIDAK ADA kode di repo ini yang
    menangani `pgtype.Numeric` (presisi 2 desimal cukup direpresentasikan
    float64 pada skala nilai 0..100)
  - ✅ Settings module `grading` (`{enabled bool, ranges:[{min,max,label}]}`,
    default `{enabled:false, ranges:[{0,74,C},{75,84,B},{85,100,A}]}`,
    validasi ranges menutup 0..100 persis tanpa celah/tumpang-tindih & urut)
    terdaftar `tenant.NewModuleSettings` — mutasi lewat endpoint GENERIK
    yang sudah ada `GET/PUT /api/settings/grading` (auth semua role baca,
    `settings:manage` tulis), TIDAK ada endpoint khusus baru untuk settings
    itu sendiri
  - ✅ `internal/grading/`: `GET /api/grading/status` (auth SEMUA role, TANPA
    requirePerm & TANPA guard disabled — dipakai klien mengecek nyala/
    tidaknya modul); SELURUH endpoint lain digerbang `requirePerm(grading:manage)`
    (admin_sekolah + guru) DAN `Service.requireEnabled` (404
    `{code:"grading_disabled"}` bila settings.enabled=false, dicek PALING
    AWAL tiap method service — keputusan dilaporkan: kepala_sekolah TIDAK
    diberi akses baca pada gelombang ini, bisa ditambah nanti); CRUD
    komponen (`GET/POST/PATCH/DELETE /api/grading/components[/{id}]`,
    DELETE cascade nilai via FK, publikasi TIDAK mem-freeze mutasi komponen/
    nilai — docs/12: "nilai bisa diedit lalu publikasi tetap"); nilai
    (`GET/PUT .../components/{id}/grades` bulk upsert transaksional per-baris,
    `score:null` = hapus baris, validasi siswa harus anggota rombel & skor
    0..100); rekap (`GET /api/grading/recap` — final = normalisasi HANYA
    komponen siswa itu TERISI, label dari `settings.Ranges` dibulatkan
    half-up, `below_kktp` HANYA dari komponen berskor); publikasi (`PUT
    /api/grading/publication` upsert/delete row + notif `grading.published`
    ke siswa ber-akun & SEMUA ortu kelas + realtime); `GET /api/my-grades`
    (siswa sendiri via `student.MyStudentID`, ortu via `?student_id=` +
    `CanViewStudent`, HANYA subjek yang published, kelas = enrollment TA
    aktif); bintang (`POST/DELETE /api/grading/stars[/{id}]` DELETE
    pembuat/admin, `GET ?student_id=|class_id=` raw admin/guru lihat SEMUA
    termasuk private, `GET /api/my-stars` HANYA visibility=student utk items
    DAN total — keputusan dilaporkan sesuai instruksi tugas); export xlsx
    (`GET /api/grading/export`, excelize, header menyertakan bobot per
    komponen, pola dua-lapis sama `internal/discipline/export.go`)
  - ✅ Permission baru `grading:manage` (admin_sekolah + guru,
    `internal/identity/rbac.go` + tabel `docs/02-identity.md`) — object-level
    guru DIPERSEMPIT di service (bukan cuma gerbang RBAC kasar)
  - ✅ Interface publik baru `schedule.Service`: `TeachesClassSubject` (guru
    punya slot (class_id,subject_id) TA aktif — dipakai komponen/nilai/
    publikasi) & `TeachesClass` (guru punya slot di kelas itu, MAPEL APA
    PUN, lebih longgar — dipakai bintang kelas sesuai instruksi tugas
    "guru punya slot di KELAS siswa (mapel apa pun) -> boleh") — KEDUANYA
    primitif (`schoolID, teacherUserID, classID[, subjectID] -> bool`),
    dipenuhi `*schedule.Service` LANGSUNG oleh `grading.ScheduleGateway`
    TANPA adapter (beda dari exitpermit/latearrival yang butuh
    `b2ScheduleGateway` — grading tidak butuh data kaya SlotView, cukup bool)
  - ✅ `grading.SettingsGateway` (baca RAW json `school_settings` module
    "grading") dipenuhi `*tenant.Repository` LANGSUNG (method `GetSetting`
    primitif `(ctx,schoolID,module string)->([]byte,bool,error)`, sudah ada
    sejak Fase 1) — **keputusan desain dilaporkan**: grading TIDAK bisa
    memenuhi parameter bertipe `tenant.Settings` (interface lintas modul)
    lewat consumer-side interface karena aturan "hanya primitif" (CLAUDE.md),
    jadi `Service.loadSettings` mereplikasi logika default-bila-belum-ada
    `tenant.SettingsService.Get` secara lokal di `internal/grading/service.go`
  - ✅ Notifikasi baru `grading.published` (→ siswa kelas ber-akun + SEMUA
    orang tua kelas, webpush, "Nilai {mapel} telah dipublikasikan") —
    didaftarkan `internal/notification/model.go` + `templates.go`
  - ✅ Realtime event baru `grading` `{class_id}` — **keputusan desain
    dilaporkan**: broadcast RINGAN by-role (admin_sekolah/kepala_sekolah/
    guru) SAJA, BUKAN menghimpun user_id siswa+ortu kelas satu per satu
    (docs tugas eksplisit menyarankan "cukup broadcast sekolah — ringan") —
    dipancarkan saat `PutGrades` & `PutPublication`, TIDAK saat CRUD
    komponen (scope literal tugas "publish/nilai berubah")
  - ✅ `sqlc.yaml` ditambah blok `gradingdb` — sqlc CLI TERSEDIA & DIPAKAI
    (`go run github.com/sqlc-dev/sqlc/cmd/sqlc@latest generate` dari host,
    BERHASIL, sama seperti Gelombang B1/B2) — seluruh kode `*db` package
    GENERATED
  - ✅ Bootstrap idempoten (`cmd/bootstrap/ensureDemoGrading`): aktifkan
    toggle grading (TIDAK menimpa bila sudah enabled — pola sama
    `ensureDemoAttendanceSettings`), 3 komponen XII RPL 1 x Basis Data (idempoten
    by nama), nilai 3 siswa (NIS 22101 LENGKAP 80/85/78 sesuai instruksi
    tugas; NIS 22102 SEBAGIAN tanpa Praktik untuk mendemokan normalisasi;
    NIS 22103/Budi lengkap tapi SEMUA di bawah kktp untuk mendemokan
    below_kktp — `UpsertGrade` idempoten by component+student), publikasikan
    kelas-mapel itu, 2 bintang visibility=student siswa NIS 22101 (idempoten:
    dilewati bila siswa itu SUDAH punya bintang sama sekali — TIDAK ada
    UNIQUE constraint di `classroom_star_events`, cek manual menegakkannya)
  - ✅ **Keputusan dilaporkan (kepsek read)**: docs tugas eksplisit meminta
    "JANGAN rumit: grading:manage saja + kepsek TIDAK (bisa nyusul);
    laporkan" — diikuti persis, kepala_sekolah TIDAK diberi akses baca modul
    grading pada gelombang ini
  - ✅ **Keputusan dilaporkan (stars total)**: docs tugas bertanya "total
    tetap semua?" lalu memutuskan sendiri "total & items sama-sama hanya
    visible" — diikuti persis di `Service.MyStars` (BEDA dari `ListStars`
    raw admin/guru yang tetap menampilkan SEMUA termasuk private)
  - ✅ UI frontend Gelombang C sudah dikerjakan agent frontend (build hijau). Konfigurasi rapor LANJUTAN (pemetaan TP, nilai manual/sebelumnya, analisis mendalam) = deviasi sadar dari SION, dicatat di Ide tertunda (rapor formal penuh)
- ✅ Gelombang D (FINAL, backend): Konseling BK, guru pengganti, period day
  overrides, kalender presensi siswa, admin impersonate user, template surat
  — terverifikasi end-to-end di Docker dev (`localhost:8210`, `demo.localhost`):
  sari (Guru BK) `POST /api/counselings` siswa Budi Santoso (NIS 22103) +
  evidence PNG → 201 → `GET /api/counselings?student_id=3` (1 baris) +
  `GET .../1/report/html` (200, kop NouSchool + identitas + 3 seksi) +
  `GET .../1/evidence` (200, `image/png`, `Content-Disposition` benar) →
  rendi (guru biasa, wali kelas — TANPA flag `leave_issuance`) `GET
  /api/counselings` → 403 (privat BK terbukti) → rendi `POST /api/substitutions`
  slot Senin miliknya (id 1, Basis Data XII RPL 1) ke Sari tanggal
  2026-08-17 → 201 pending → sari `GET ?scope=for-me` (1 baris) + `GET
  /api/notifications` (`substitution.requested` masuk) → sari `POST
  .../1/accept` → 200 accepted → rendi `GET /api/notifications`
  (`substitution.decided` "...telah diterima oleh Sari Wulandari" masuk) →
  admin `PUT /api/periods/overrides` hari Jumat (day_of_week=5, 4 jam
  pendek) → **bug ditemukan & diperbaiki saat e2e**: `period_day_overrides.label`
  NOT NULL DEFAULT '' tapi repository memakai `textOrNil("")` (menghasilkan
  NULL) → `internal/schedule/repository.go` diperbaiki (Valid:true selalu,
  bukan textOrNil, utk kolom override) → retry sukses 200 → `GET
  /api/periods/overrides?day=5` balik 4 periode persis (unit test
  `TestCurrentPeriod_DayOverride_FridayShorterThanDefault` membuktikan
  CurrentPeriod day-aware Jumat vs default, tidak diuji live krn hari
  berjalan sesi ini Minggu) → admin `GET
  /api/students/3/attendance/calendar?month=2026-08` → 200, 31 hari, tgl 16
  berisi `status:"sakit"` (2 sesi digabung, note dari data fase 9 sebelumnya),
  `counts.sakit:1` — kalender day-merge & bulan-penuh terbukti → admin `POST
  /api/users/4/impersonate` (rendi) → 200, cookie berganti langsung → `GET
  /api/me` → `role:"guru"` + `impersonated_by:{"name":"Admin Demo"}` → `POST
  /api/auth/impersonation/stop` → 200 role admin_sekolah → `GET /api/me`
  → kembali admin TANPA `impersonated_by` → dicoba juga `POST
  /api/users/12/impersonate` (akun `display`) → 422 ditolak jelas (target
  terlarang) → `PUT /api/settings/letters`
  `{sp_footer_note,leave_footer_note}` → 200 → `GET
  /api/discipline/letters/1/pdf` (surat SP1 lama, diterbitkan SEBELUM
  Gelombang D ada) → 200, PDF valid, ukuran naik 1899→2170 byte (footer
  benar-benar dirender tanpa perlu menerbitkan ulang surat) → `GET
  /api/student-leave/1/letter/pdf` (surat izin lama) → 200 PDF valid juga
  (footer leave ikut dirender). Seluruh audit_log baru diverifikasi lewat
  psql: `counseling.create`, `substitution.request`, `substitution.accepted`,
  `schedule.period_overrides_replace`, `admin.user_impersonate_started`,
  `admin.user_impersonate_stopped`. `go build/vet/test ./...` (host DAN
  kontainer `sekolah-api-1`) hijau — host Windows dev TIDAK bisa mengeksekusi
  binary `go test` hasil compile sendiri (Application Control Policy
  memblokir exe di temp folder, ditemukan sesi ini) sehingga `go test`
  dijalankan via `docker exec sekolah-api-1 go test ./...` (build/vet tetap
  bisa langsung di host); `go run .../sqlc@latest generate` dari host BERHASIL
  (sqlc CLI tersedia di lingkungan ini, dikonfirmasi ulang — beda dari
  catatan lama Gelombang A yang bilang tidak tersedia).
  - Migrasi baru: `00018_counseling.sql` (`counselings`), `00019_substitution.sql`
    (`teacher_substitution_requests` + partial unique aktif
    `(schedule_slot_id,date) WHERE status IN ('pending','accepted')`),
    `00020_period_overrides.sql` (`period_day_overrides`, UNIQUE
    `(school_id,day_of_week,number)`), `00021_user_impersonation.sql`
    (`sessions.impersonator_user_id bigint NULL REFERENCES users(id)`)
  - `internal/counseling/` (modul baru, sqlc package `counselingdb`
    DIGENERATE dari `queries.sql`): `GET /api/counselings?student_id=&page=`
    (20/halaman), `POST /api/counselings` (multipart, evidence opsional
    pdf/jpg/png ≤5MB pola sama `internal/leave`), `PATCH/DELETE
    /api/counselings/{id}` (pembuat ATAU admin_sekolah — guru BK LAIN yang
    bukan pembuat 403, dibuktikan test & bukan sekadar longgar "sesama BK"),
    `GET .../evidence`, `GET .../report/html`. Otorisasi baca/kelola: flag
    duty `leave_issuance` (Guru BK) ATAU role `admin_sekolah` = kelola;
    `kepala_sekolah` = baca saja; siswa/ortu TANPA akses sama sekali (privat
    BK, TIDAK ada jalur object-level seperti modul discipline) — consumer-side
    interface `DutyGateway.UserHasFlag` (dipenuhi `duty.Service`)
  - `internal/substitution/` (modul baru, sqlc package `substitutiondb`):
    `POST /api/substitutions` (guru pemilik slot ATAU admin_sekolah,
    tanggal ≥ hari ini lokal sekolah, tanggal harus jatuh pada
    `day_of_week` slot, substitute WAJIB guru aktif sekolah ini, tidak boleh
    diri sendiri), `GET ?scope=mine|for-me|all&date=` (`all` butuh
    `schedule:manage`), `POST .../{id}/accept|reject` (HANYA pengganti yang
    diminta), `POST .../{id}/cancel` (HANYA pengaju, selama masih pending —
    race guard transaksional `UPDATE ... WHERE status=$from` pola sama
    `internal/exitpermit`). Validasi kepemilikan slot & keanggotaan guru
    lewat JOIN LANGSUNG ke `schedule_slots`/`teachers`/`memberships`
    (read-only, pola sama `internal/discipline` — bukan consumer-side
    interface Go baru). Interface publik `Service.SubstituteName`/
    `IsSubstituteToday` dikonsumsi `internal/teaching` lewat consumer-side
    interface `SubstitutionLookup` (opsional, setter `SetSubstitutions`,
    nil-safe): (a) `teaching.Scan` — scanner BUKAN pemilik slot TAPI
    pengganti accepted utk slot yg SEDANG berjalan di ruangan itu →
    diizinkan, journal dapat flag baru `FlagSubstitute="substitute"`,
    `teacher_id` journal = profil pengganti sendiri (bukan pemilik asli);
    (b) `teaching.Status` — `TeacherRef.Name` diganti `"{pengganti}
    (pengganti)"` bila ada substitusi accepted hari itu (ID tetap pemilik
    asli, keputusan dilaporkan: hanya nama yang berubah, sesuai literal
    tugas "teacher_name tampil"). Realtime: `Accept` memancarkan event
    `"schedule"` (event YANG SAMA dipakai modul schedule sendiri, bukan
    event baru — klien yang sudah dengar "schedule" auto-refetch). Notifikasi
    baru `substitution.requested` (→ pengganti) & `substitution.decided`
    (→ pengaju, berisi label keputusan diterima/ditolak/dibatalkan)
  - `internal/schedule/`: `GET/PUT /api/periods/overrides?day=` (PUT
    perm `schedule:manage`, `[]` = hapus override hari itu). **Loader
    day-aware SATU TITIK**: `Service.periodsForDay(schoolID,dayOfWeek)` —
    override bila sekolah punya baris utk hari itu, else periods default —
    dipakai `currentPeriod` (jadi `CurrentPeriod`, `SlotsToday.is_now`, dan
    `SlotNow` ikut day-aware otomatis) DAN `LastPeriodEndToday` (dipakai
    `exitpermit`). Validasi bentuk override (`parsePeriodItems`, diekstrak
    dari `ReplacePeriods` supaya SATU aturan dipakai dua endpoint) SAMA
    dengan periods default (nomor unik+berurutan dari 1, start<end, tanpa
    overlap) TAPI SENGAJA TANPA cek "period_in_use" 409 (keputusan
    dilaporkan: override hanya mengganti WAKTU utk nomor yang sama, TIDAK
    menghapus nomor jam ke- dari keberadaan struktural seperti
    `ReplacePeriods` bisa lakukan — slot manapun yang mereferensikan nomor
    itu tidak pernah yatim)
  - `internal/attendance/`: `GET
    /api/students/{id}/attendance/calendar?month=YYYY-MM` (object-level
    SAMA `StudentHistory` — `attendance:report` ATAU siswa
    sendiri/orang tua anaknya), query baru `StudentCalendarRecords` (semua
    record daily+subject sebulan + `session.type`). Fungsi murni
    `resolveDayStatus`: sesi DAILY MENANG MUTLAK bila ada; tanpa daily →
    status TERBURUK di antara record subject hari itu
    (alpa>izin>sakit>terlambat>hadir, note dari record berstatus itu).
    Response SELALU berisi SELURUH hari dalam bulan (termasuk yang tanpa
    record sama sekali, `status`/`note` null, `session_count:0`) —
    `counts` dihitung dari status FINAL per hari (bukan per record mentah)
  - `internal/identity/`: Fase 14 Gelombang D "Impersonate USER" —
    **modul terpisah** `impersonation_user.go` (BEDA dari `impersonation.go`
    fase 13 "masuk sebagai sekolah" super admin, sentinel role
    `admin_sekolah:impersonating`): `POST /api/users/{id}/impersonate`
    (permission baru `user:impersonate`, hanya `admin_sekolah`) — target
    WAJIB member AKTIF sekolah ini via `PickActiveRole` (role SAMA seperti
    Login), DITOLAK bila target admin_sekolah lain, super admin
    (`users.is_super_admin`), role `display`, atau diri sendiri. Sesi baru
    disimpan dengan **role ASLI target** (bukan sentinel — RBAC otomatis
    benar tanpa translasi apa pun), TTL **1 jam PERSIS**,
    `sessions.impersonator_user_id` = admin. Cookie diganti LANGSUNG (admin
    & target di host tenant yang sama, beda dari token sekali-pakai lintas
    tab fase 13). `POST /api/auth/impersonation/stop` — HANYA sesi dengan
    `impersonator_user_id` terisi yang lolos (pesan generik SATU macam bila
    bukan), menghapus sesi impersonasi lalu menerbitkan sesi BARU utk admin
    dengan TTL NORMAL (`sessionTTLForRole`, sliding renewal berlaku lagi).
    **Keputusan desain dilaporkan**: docs tugas menyarankan "reuse mekanisme
    sentinel/renew-window 0" — DIIMPLEMENTASIKAN LEWAT KOLOM BARU
    `impersonator_user_id`, BUKAN sentinel role, karena target session role
    bisa salah satu dari 5 kemungkinan (beda dari fase 13 yang selalu jadi
    admin_sekolah) — `RequireAuth` (middleware.go) mengecek kolom ini utk
    (a) melewati sliding renewal SAMA SEKALI & (b) tidak butuh translasi
    role apa pun. Pola fungsi MURNI (`impersonateUser`/`stopImpersonation`,
    interface kecil `impersonateUserRepo`/`stopImpersonationRepo`) + method
    `Service` tipis SAMA PERSIS dengan pola `impersonation.go` fase 13.
    `GET /api/me`: field baru `impersonated_by:{name}` (dari
    `reqctx.ImpersonatorUserID`, konteks baru diisi `RequireAuth`) — HANYA
    terisi saat sesi impersonasi USER
  - Settings module baru `letters` (`internal/tenant/letters.go`,
    `LettersSettings{sp_footer_note,leave_footer_note}`, default `""`,
    validasi maks 1000 karakter) — dibaca `internal/discipline` &
    `internal/studentleave` lewat consumer-side interface `SettingsGateway`
    (RAW json, pola sama `internal/grading.SettingsGateway`), disuntik
    setter opsional `SetSettingsGateway` (nil-safe, supaya call site test
    lama tidak berubah) — `BuildLetterPDF`/`BuildLeavePDF` dapat parameter
    baru `footerNote`, ditambahkan di bagian bawah PDF (garis pemisah +
    teks italic) BILA NON-KOSONG, dibaca LIVE saat render (BUKAN bagian
    snapshot) supaya admin bisa ubah catatan tanpa menerbitkan ulang surat
    lama — dibuktikan e2e (footer muncul di surat SP1 yang sudah lama ada)
  - Test baru: `internal/counseling` (otorisasi BK-flag vs guru biasa 403,
    admin OK, kepsek read-only, siswa/ortu 403 total, pembuat vs BK lain
    utk update/delete), `internal/substitution` (state machine
    pending→accepted/rejected/canceled, unique aktif per slot+tanggal 409,
    validasi pemilik slot/hari/guru-aktif, scope=all butuh
    `schedule:manage`), `internal/teaching` (scan pengganti accepted
    diizinkan+flag `substitute`, bukan-pengganti tetap `needs_manual`,
    status menampilkan "(pengganti)" dengan ID slot tetap pemilik asli),
    `internal/schedule` (`TestCurrentPeriod_DayOverride_FridayShorterThanDefault`
    Jumat vs default day-aware, validasi override sama pola `ReplacePeriods`,
    PUT `[]` menghapus override), `internal/attendance`
    (`resolveDayStatus` semua kombinasi prioritas terburuk, bulan kosong
    31 hari null, merge multi-record per hari, object-level ortu),
    `internal/identity` (target admin/super admin/display/bukan-member/diri
    sendiri semua ditolak, TTL 1 jam PERSIS, role DI DB adalah role asli
    BUKAN sentinel, stop mengembalikan admin dgn TTL normal, stop dua kali
    gagal, audit start+stop terpanggil tepat sekali dgn actor yang benar),
    `internal/discipline`+`internal/studentleave` (footer note terbaca dari
    fake SettingsGateway, `""` saat gateway nil/module belum tersimpan,
    PDF dengan footer LEBIH BESAR dari tanpa footer — dibuktikan byte count,
    bukan cuma "tidak error")
  - `sqlc.yaml` ditambah blok `counselingdb`/`substitutiondb`; blok
    `scheduledb`/`attendancedb`/`identitydb` existing menangkap query baru
    otomatis (queries ditambahkan ke `queries.sql` module yang sudah ada)
  - **Ide tertunda ditambahkan sadar (DILEWATI Gelombang D ini)**: deadline
    koreksi absensi & single-device login — lihat "Ide tertunda" di bawah

## Fase 15 — Penutupan sisa gap SION ✅ (diminta user 16 Agu 2026; gap 7 = stack, keputusan desain, tidak dikerjakan)
- ✅ Gap 1: Rapor lanjutan (pemetaan TP, nilai manual/sebelumnya, analisis, export rapor per kelas)
  - Migrasi `00023_report_config.sql`: `report_tp_mappings` (component_id FK UNIQUE) +
    `report_manual_scores` (kind previous/manual, UNIQUE per tahun-kelas-mapel-siswa-kind)
  - `internal/grading/report.go` (service: GetTPMappings/PutTPMappings replace-penuh,
    GetManualScores/PutManualScores, ReportAnalysis) + `report_repository.go` (data access) +
    `report_export.go` (xlsx 2 sheet: "Rapor Kelas" per mapel+rata-rata, "TP" mapping)
  - Resolusi nilai rapor: manual MENANG atas computed final; previous HANYA info
  - Endpoint: `GET/PUT /api/grading/report/tp-mappings`, `GET/PUT /api/grading/report/manual-scores`,
    `GET /api/grading/report/analysis`, `GET /api/grading/report/export?class_id=`
  - e2e terverifikasi: rendi PUT tp-mapping TP1 → GET balik cocok → PUT manual (siswa 1 manual 90)
    + previous (siswa 3, 75) → analysis (resolved_source manual:1, avg/min/max wajar) →
    export xlsx valid (kolom Basis Data siswa 1 = "90.00 (A)", sheet TP terisi)
- ✅ Gap 2: Editor matrix permission per role per sekolah (override, tanpa custom role baru)
  - Migrasi `00022_role_perms_gap.sql`: `school_role_permissions (school_id, role, permission,
    allowed, PRIMARY KEY(school_id,role,permission))` — PENGECUALIAN per sekolah dari peta
    `rolePermissions` statis (rbac.go), BUKAN tabel role/permission baru
  - `internal/identity/permoverride.go` (baru): `GET/PUT /api/role-permissions` (perm
    `settings:manage`) — 6 role editable (admin_sekolah/kepala_sekolah/guru/siswa/orang_tua/
    display — BUKAN pegawai/super_admin) × 24 permission kanonik + label Indonesia; PUT body
    `{"overrides":{"role":{"perm":true|false|null}}}` (null = hapus, balik ke default); validasi
    MENOLAK menyentuh role `admin_sekolah` (mencegah admin mengunci diri) DAN permission
    `settings:manage` (siapa pun) — all-or-nothing per request, `ReplaceRolePermissionOverrides`
    transaksional
  - **Enforcement**: `RequirePerm` (middleware.go) — host TENANT saja — cek override dulu lewat
    `Service.effectivePermission` (cache in-memory per sekolah TTL 60dtk, pola sama
    `tenant.HostResolver`, invalidate SETELAH PUT sukses), fallback `HasPermission` statis.
    **Keterbatasan disengaja didokumentasikan**: `HasPermission(role,perm)` paket-level tetap baca
    peta statis TANPA context — dipakai beberapa modul sbg "object-level shortcut" langsung (lewat
    consumer-side interface `identitySvc.HasPermission`) TIDAK ikut kena override; gerbang UTAMA
    tiap route (`RequirePerm`) SELALU kena override
  - Test: `permoverride_test.go` (`effectivePermissionFrom` murni: default-allow→deny &
    default-deny→allow, fallback; `validateRolePermissionChanges`: admin_sekolah/settings:manage
    ditolak upsert & hapus; get/put fake repo: upsert+delete null, all-or-nothing saat gagal;
    cache TTL+invalidate)
  - e2e: admin cabut `discipline:record` dari guru → rendi POST discipline/records 403 (sebelumnya
    422 lolos RBAC) → hapus override → 422 lagi (cache invalidate LANGSUNG, tanpa tunggu TTL) →
    arah sebaliknya: beri siswa `student:read` → GET /api/students 403→200 → larangan admin_sekolah
    & settings:manage keduanya 422 jelas
- ✅ Gap 3: Reject per tahap oleh approver di dispensasi keluar
  - `internal/exitpermit/service.go`: `RejectRequest` sekarang menerima admin (student:manage,
    perilaku lama) ATAU approver SAH tahap berjalan (`requireRejectAuthority` — validasi IDENTIK
    Scan tapi TANPA token: piket=flag late_arrival_duty, guru kelas=jadwal jam berjalan/berikutnya
    & beda orang dari tahap 1, BK/pimpinan=flag masing-masing); komentar WAJIB (validasi baru)
  - Query baru `GetStudentClassID`/`GetStudentUserID` (query langsung tabel siswa, tanpa
    mengimpor modul student) dipakai validasi tahap 2 & notifikasi siswa
  - Notifikasi baru `exitpermit.rejected` (`internal/notification/model.go`+`templates.go`) →
    siswa + ortu; realtime `exitpermit` tetap publish (pola existing)
  - e2e terverifikasi: siswa ajukan izin → sari (piket, tahap 1) reject dengan komentar (200,
    status rejected, notif in-app siswa masuk) → rendi (guru non-piket) coba reject permit baru
    (403) → reject permit sudah final (422) → reject tanpa komentar (422)
- ✅ Gap 4: Single-device login (toggle keamanan per sekolah)
  - `internal/tenant/security.go` (baru): `SecuritySettings{single_device bool}` default `false`,
    terdaftar `tenant.NewModuleSettings` module `"security"` — `PUT /api/settings/security` BIASA
    (perm `settings:manage`, BUKAN superadmin-only)
  - `internal/identity/service.go`: `SecuritySettingsGateway` (consumer-side interface, dipenuhi
    `*tenant.Repository` langsung lewat `GetSetting`, pola sama `grading.SettingsGateway`,
    disuntik `SetSecuritySettingsGateway(tenantRepo)` main.go) + `singleDeviceEnabled` (fungsi
    murni, dites fake gateway). `Login` (HOST TENANT saja) — setelah sesi baru dibuat, bila aktif
    → `repo.DeleteOtherSessionsByUserSchool(user, school, keepTokenHash)`. **Keputusan desain**:
    di-key `token_hash` sesi BARU (bukan session ID — `CreateSession` tidak mengembalikan baris
    yang dibuat, dan mengubah signature-nya berdampak ke banyak pemanggil lain di modul ini;
    `token_hash` sudah unik & diketahui SEBELUM insert, setara secara semantik dgn "keep session id")
  - Test: `security_test.go` (`singleDeviceEnabled`: gateway nil/belum tersimpan/true/false/JSON
    malformed, fake repo)
  - e2e: aktifkan `single_device` → login rendi 2x → `GET /api/me` cookie sesi PERTAMA 401, sesi
    KEDUA tetap 200 → nonaktifkan lagi → login 2x → KEDUA sesi tetap hidup (toggle dua arah terbukti)
- ✅ Gap 5: Akses baca modul nilai untuk kepala sekolah
  - `internal/grading/model.go`: `PermGradingRead = "grading:read"` (literal, konstanta identity
    didaftarkan agent paralel di rbac.go)
  - `Service.requireReadAccess` — endpoint READ (components GET, grades GET, recap GET,
    report/analysis GET) menerima grading:manage ATAU grading:read; pemegang grading:read SAJA
    (kepsek) melewati object-level guru (baca SEMUA kelas-mapel), TANPA mutasi (PUT/POST/DELETE
    tetap grading:manage-only, termasuk stars & report/tp-mappings & report/manual-scores &
    report/export yang SENGAJA TIDAK diberi akses grading:read)
  - `routes.go`: 4 endpoint READ itu diganti middleware dari `requirePerm(grading:manage)` jadi
    `requireAuth` saja — permission check dipindah ke service supaya kepsek tidak diblok di mux
  - e2e terverifikasi: kepsek GET recap BDT XII RPL 1 (200, baca kelas yang bukan miliknya) &
    POST components (403)
  - **Catatan (agen identity)**: `docs/02-identity.md` tabel permission BELUM diupdate di sesi ini
    (instruksi tugas: "docs/02 diupdate orchestrator") — baris baru yang perlu ditambahkan:
    `grading:read` dengan kolom kepsek ✅ saja, semua kolom lain kosong
- ✅ Gap 6: Nonaktifkan/aktifkan user + presence online (WS hub)
  - `internal/identity/memberstatus.go` (baru): `PATCH /api/members/{userId}/status` (perm
    `student:manage`, sama gerbang CRUD siswa/pegawai) `{role, status:'active'|'inactive'}` —
    target SATU baris membership (userId,school,role — user bisa >1 role). Larangan: diri sendiri,
    role target `admin_sekolah`, target `is_super_admin`. Saat `inactive` →
    `repo.DeleteSessionsByUserSchool(user,school)` (SEMUA sesi user itu DI SEKOLAH INI SAJA — beda
    dari `DeleteSessionsByUser` lintas sekolah yang dipakai `AdminResetPassword`). Audit
    `admin.member_status`. **Enforcement login OTOMATIS, TIDAK ada kode tambahan**: `Login` sudah
    memanggil `ListActiveMemberships` yang SQL-nya SUDAH memfilter `status='active'` sejak Fase 1 —
    diverifikasi e2e (bukan diasumsikan)
  - `internal/realtime/hub.go`: `Hub.OnlineUsers(schoolID) []OnlineUser{UserID,Role}` (tipe
    bernama, bukan struct anonim) — dedup per `user_id` dari koneksi WS aktif.
    `internal/realtime/presence.go` (baru): `GET /api/presence` (perm `teaching:monitor` —
    ditempatkan DI MODUL REALTIME karena state koneksi tinggal di Hub; identity hanya dibutuhkan
    utk nama user lewat consumer-side interface `realtime.IdentityGateway.UserName`, dipenuhi
    `*identity.Service` langsung, disuntik `realtimeHandler.SetIdentityGateway(identitySvc)`
    main.go) → `{total, by_role:{role:count}, users:[{user_id,name,role}]}`.
    `identity.Service.UserName` (gateway.go, baru) membungkus `GetUserBasic`
  - `realtime.RegisterRoutes` diperluas terima `requirePerm` (dipakai KHUSUS gerbang
    `/api/presence`; `/api/ws` tetap tanpa requirePerm sesuai Fase 12)
  - Test: `memberstatus_test.go` (fake repo: tolak diri sendiri/role admin_sekolah/target super
    admin/status tak dikenal/membership tak ditemukan, inactive hapus sesi HANYA di sekolah ini,
    active tidak hapus sesi), `internal/realtime/hub_test.go` `TestOnlineUsersDedup` (user 2
    koneksi dedup jadi 1, sekolah lain tidak ikut, sekolah kosong slice kosong)
  - e2e: admin PATCH siswa (NIS 22101/Ahmad Fauzi, user_id 10) → inactive → login siswa → 401
    `invalid_credentials` → cookie sesi lama siswa juga ikut 401 → aktifkan lagi → login siswa
    → 200 lagi → larangan diri sendiri & role admin_sekolah keduanya 422 jelas → kepsek `GET
    /api/presence` → 200 `{total,by_role,users}` shape benar (1 koneksi WS admin nyata terdeteksi
    saat verifikasi) → guru `GET /api/presence` → 403 (`teaching:monitor` bukan milik guru)

## Fase 16 — Perombakan UI admin & super admin (feedback user 16 Agu 2026: "masih banyak kosong, sidebar ikut scroll, terlalu banyak whitespace, menu super admin harus lengkap") ⬜
- ⬜ AppShell: sidebar & top bar STICKY (h-dvh, sidebar fixed/sticky, konten yang scroll) — bug nyata
- ⬜ Hapus batas lebar konten di layar admin/super admin (pakai lebar penuh dengan padding; docs/10 §5 direvisi: 640px hanya utk form/detail personal)
- ⬜ Super admin: nav LENGKAP bergrup — Beranda; Sekolah (daftar, tambah/onboarding, minat); Langganan (plans, pendapatan, invoice menunggu); Operasional (outbox, pengumuman platform, audit); Akun (profil) — semua fitur P1-P6 punya menu, bukan hanya kartu
- ⬜ Admin sekolah: nav sidebar desktop LENGKAP bergrup (Akademik: siswa/rombel/guru/pegawai/mapel/jadwal/jam/ruangan/tugas; Kegiatan: absensi/izin/izin siswa/kedisiplinan/nilai/konseling/pengganti/monitoring/pengumuman; Sistem: pengaturan/hak akses/tagihan) — mobile tetap 5 tab
- ⬜ Dashboard admin sekolah desktop: isi nyata (StatTile hari ini, daftar perlu tindakan, DataTable terbaru) — bukan tumpukan kartu link
- ⬜ Terapkan DataTable ke semua daftar admin desktop (guru, pegawai, rombel, mapel, rekap, outbox, pendapatan, sekolah)

## Ide tertunda (JANGAN dikerjakan tanpa keputusan user)
- Rapor formal penuh (pemetaan TP, nilai manual/sebelumnya, analisis — sisa konfigurasi lanjutan SION); Surat izin siswa dari ortu → status absen; kuota cuti guru; custom role/permission di DB; opt-out notifikasi per user; RLS Postgres; PKL/magang SMK; SPP/pembayaran siswa; rapor.
- Deadline koreksi absensi (batas waktu guru boleh mengoreksi record lama sebelum "terkunci permanen" — beda dari `edit_window_hours` yang sudah ada, ini lebih ke kebijakan administratif jangka panjang) & single-device login (satu akun hanya boleh punya satu sesi aktif, cabut sesi lama saat login baru) — disebut eksplisit di docs tugas Fase 14 Gelombang D sebagai "DILEWATI sadar", butuh keputusan user (dampak UX & multi-perangkat cukup besar, terutama single-device utk akun yang dipakai bergantian keluarga/wali).
