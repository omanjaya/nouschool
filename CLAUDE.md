# NouSchool

Multi-tenant SaaS untuk sekolah (target awal SMA/SMK/MA): absensi siswa, monitoring guru mengajar (jurnal + dashboard TV), izin guru, dashboard kepala sekolah. Setiap sekolah punya subdomain default (`{slug}.nouschool.id`) atau custom domain sendiri.

**Sebelum mengerjakan fitur apa pun, baca design doc modulnya di `docs/`** — semua keputusan desain sudah diputuskan dan dicatat di sana. Jangan mendesain ulang dari nol. Kalau sebuah keputusan terasa salah, tanyakan ke user dulu, jangan diam-diam menyimpang.

## Alur kerja (wajib tiap sesi)

1. **Centang ROADMAP**: setiap item selesai → update `docs/ROADMAP.md` (⬜ → ✅, atau 🚧 saat mulai). Ini satu-satunya sumber kebenaran progres.
2. **Commit git per unit kerja**: selesai satu unit logis (satu item roadmap / satu perbaikan) → langsung `git add` + `git commit` dengan pesan jelas berbahasa Indonesia (mis. `fase-1: middleware resolusi tenant`). Jangan menumpuk banyak pekerjaan tanpa commit.
3. **Pembagian model**: sesi utama/orchestrator memakai **Fable**; setiap subagent paralel (Agent tool, workflow, Explore) jalankan dengan **model `sonnet`** — kecuali user minta lain.

## Stack (sudah final, jangan diganti)

- **Backend**: Go (stdlib `net/http` Go 1.22+ routing; boleh chi bila perlu), satu binary, modular monolith
- **DB**: PostgreSQL via `pgx` + `sqlc` (SQL asli, type-safe). **Tanpa ORM. Tanpa GORM.**
- **Migrasi**: goose, file SQL di `migrations/`, hanya bertambah — jangan pernah mengedit migrasi yang sudah dijalankan
- **Frontend**: React + Vite + TypeScript PWA di `web/` — TanStack Query, Tailwind, shadcn/ui, vite-plugin-pwa. Tanpa Redux, tanpa component library berat (MUI/AntD)
- **Reverse proxy**: Caddy dengan On-Demand TLS untuk custom domain per sekolah
- **Session**: cookie-based (HttpOnly, Secure, SameSite=Lax), token di-hash di DB. **Bukan JWT di localStorage**

## Arsitektur & aturan modul

Struktur: `cmd/server/main.go` (wiring saja) → `internal/<modul>/` → `internal/platform/` (shared).

Modul: `tenant`, `identity`, `student`, `schedule`, `attendance`, `leave`, `teaching` (jurnal + monitoring), `notification`, `billing`, `dashboard`.

Setiap modul berisi file seragam: `routes.go`, `handler.go`, `service.go`, `repository.go`, `model.go`, (opsional `settings.go`).

1. **Arah dependency satu arah**: handler → service → repository. Handler tidak memegang SQL; repository tidak tahu HTTP.
2. **Antar-modul lewat interface kecil yang dideklarasikan di sisi PEMAKAI** (consumer-side interface), di-inject lewat constructor di `main.go`. Jangan import repository modul lain secara langsung.
3. **Wiring eksplisit di `main.go`** — tanpa framework dependency injection, tanpa global state, tanpa `init()` yang menyembunyikan alur.
4. Konfigurasi per sekolah memakai **satu pola**: tabel `school_settings (school_id, module, settings jsonb)` + struct typed per modul dengan `DefaultSettings()` dan validasi saat save. Jangan bikin mekanisme setting baru.
5. Fitur berbayar dicek lewat feature gate `requireFeature("...")` dari modul billing — jangan hardcode cek tier.

## Aturan multi-tenant (PALING KRITIS — pelanggaran = data bocor antar sekolah)

- `school_id` **selalu** diambil dari request context (hasil resolusi Host header oleh middleware tenant). **Tidak pernah** dari body, query param, atau path.
- **Setiap** query pada tabel tenant-scoped **wajib** memfilter `school_id`. Setiap query sqlc baru harus dicek hal ini sebelum dianggap selesai.
- Unique constraint pada tabel tenant-scoped harus menyertakan `school_id` (mis. NIS unik per sekolah, bukan global).
- Object-level check di service layer: orang tua hanya boleh mengakses anaknya sendiri (via `guardians`), guru hanya kelas/jadwalnya. Permission middleware saja tidak cukup.

## Aturan security

- Password: **argon2id**. Session token: random 32 byte, simpan hash-nya.
- RBAC: role adalah atribut **membership** (`memberships: user_id, school_id, role`), bukan atribut user. Satu orang bisa guru di sekolah A dan orang tua di sekolah B.
- Otorisasi per route lewat middleware `requirePerm("modul:aksi")` yang dideklarasikan di `routes.go` — jangan cek role dengan `if role == "guru"` di dalam handler/service.
- Registrasi siswa/orang tua hanya via kode undangan dari sekolah — tidak ada open registration.
- Semua mutasi penting (absensi, izin, setting, billing) ditulis ke `audit_log` (school_id, user_id, action, entity, entity_id, old/new, at).
- SQL selalu parameterized (otomatis via sqlc) — jangan pernah merangkai SQL dengan fmt.Sprintf.
- Rate limit login per IP + per akun. Endpoint internal (mis. `/internal/check-domain` untuk Caddy) hanya bind ke localhost.
- Secrets hanya via env var; `.env.example` sebagai dokumentasi, tidak pernah commit nilai asli.

## Konvensi kode

- Error domain terpusat di `platform/httpx` (`ErrNotFound`, `ErrForbidden`, `ErrValidation(...)`); service mengembalikan error itu; satu tempat menerjemahkan ke HTTP status + JSON `{"error": {"code", "message"}}`.
- Validasi input di handler (bentuk/format), aturan bisnis di service.
- Waktu: simpan `timestamptz` (UTC) di DB; tampilkan sesuai `schools.timezone` (WIB/WITA/WIT per sekolah). Untuk logika "hari absensi" gunakan tanggal lokal sekolah. Ambil waktu lewat `platform/clock` (injectable) agar bisa dites.
- API JSON: path `/api/...`, snake_case untuk field JSON, bahasa Indonesia untuk pesan error yang dilihat user.
- Test: minimal service-level test untuk aturan bisnis (jendela edit absen, approval chain, object-level access, isolasi tenant).
- Frontend: satu folder per fitur di `web/src/features/`, data fetching hanya lewat TanStack Query, komponen mobile-first.

## Aturan UI (design system "Rapor" — FINAL, lihat docs/10-design-system.md)

- Arah visual sudah dipilih user dan DIKUNCI: institusional minimal — putih, hairline `--line`, radius 8px, tanpa shadow, aksen `--primary` sangat hemat (maks 1 tombol primary per layar). Jangan menggeser gaya ini.
- **Dilarang: emoji di UI/notifikasi/output pengguna, gradien, glassmorphism, shadow dekoratif, warna hardcode.** Icon hanya **Lucide** (`lucide-react`, currentColor).
- Semua warna & ukuran lewat token di `web/src/styles/tokens.css` + kelas semantik Tailwind (`text-ink`, `border-line`) — palet default Tailwind (gray-*, indigo-*) dilarang.
- Pola UI berulang wajib pakai shared components di `web/src/components/ui/` (Button, ListRow, StatusChip, EmptyState, dst — daftar di docs/10). Varian baru ditambahkan di komponen bersama, bukan one-off di fitur.
- Setiap layar wajib punya state loading (Skeleton), kosong (EmptyState), error (ErrorState + retry), dan sukses (Toast). Happy-path-only = belum selesai.
- Responsive: satu codebase — `AppShell` menampilkan bottom tab bar di mobile dan sidebar di desktop (breakpoint 1024px); list ListRow di mobile boleh menjadi DataTable di desktop untuk layar admin.

## Perintah

(diisi setelah scaffold — lihat Makefile: `make dev`, `make migrate`, `make sqlc`, `make build`)

## Peta dokumen

| Dok | Isi |
|---|---|
| `docs/00-overview.md` | Visi, arsitektur, keputusan final (decision log) |
| `docs/01-tenant.md` | Sekolah, domain & Caddy, tahun ajaran, settings, branding/PWA per tenant |
| `docs/02-identity.md` | User, membership, RBAC, daftar permission, session, undangan |
| `docs/03-student.md` | Siswa, rombel, enrollment, wali, import Excel/Dapodik |
| `docs/04-schedule.md` | Jadwal pelajaran: import + builder, deteksi bentrok |
| `docs/05-attendance.md` | Absensi siswa: sesi/record, mode daily & per-mapel, 3 metode input |
| `docs/06-teaching.md` | Jurnal mengajar, QR per kelas, dashboard TV, akun display |
| `docs/07-leave.md` | Izin guru: approval engine konfigurable (snapshot steps) |
| `docs/08-notification.md` | Notifikasi pluggable: in-app, web push, WhatsApp, email |
| `docs/09-billing.md` | Langganan tahunan: tier + bracket siswa, invoice, transfer manual + gateway, grace period |
| `docs/10-design-system.md` | Design system "Rapor" FINAL: token, tipografi, shared components, aturan icon/no-emoji, copywriting |
| `docs/11-superadmin.md` | Sistem super admin: fitur existing, impersonation, rencana P1-P6 |
| `docs/12-sion-parity.md` | Acuan paritas fitur SION per-sekolah: gap analysis + gelombang A-D |
| `docs/ROADMAP.md` | Urutan build & status per fitur — **update setiap selesai mengerjakan sesuatu** |
