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

## Fase 12 — Realtime WebSocket 🚧 (diminta user 16 Agu 2026, backend ✅)
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

## Ide tertunda (JANGAN dikerjakan tanpa keputusan user)
- Surat izin siswa dari ortu → status absen; kuota cuti guru; custom role/permission di DB; opt-out notifikasi per user; RLS Postgres; PKL/magang SMK; SPP/pembayaran siswa; rapor.
