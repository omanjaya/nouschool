# 02 — Identity: User, Membership, RBAC, Session

Modul: `internal/identity/`

## Skema

```sql
users (
  id            bigserial PRIMARY KEY,
  email         text UNIQUE,               -- boleh NULL untuk siswa (login via username)
  username      text,                      -- untuk siswa: NIS@slug atau username lokal
  password_hash text NOT NULL,             -- argon2id
  name          text NOT NULL,
  phone         text,
  created_at    timestamptz NOT NULL DEFAULT now()
)

memberships (
  id         bigserial PRIMARY KEY,
  user_id    bigint NOT NULL REFERENCES users,
  school_id  bigint NOT NULL REFERENCES schools,
  role       text NOT NULL,   -- admin_sekolah|kepala_sekolah|guru|siswa|orang_tua|display|pegawai
  status     text NOT NULL DEFAULT 'active',  -- active|inactive
  UNIQUE (user_id, school_id, role)
)

sessions (
  id          bigserial PRIMARY KEY,
  user_id     bigint NOT NULL REFERENCES users,
  school_id   bigint NOT NULL,             -- session terikat tenant tempat login
  token_hash  bytea NOT NULL UNIQUE,       -- SHA-256 dari token acak 32 byte
  role        text NOT NULL,               -- role aktif session ini
  expires_at  timestamptz NOT NULL,
  created_at  timestamptz NOT NULL DEFAULT now(),
  ip          inet, user_agent text
)

invitations (
  id         bigserial PRIMARY KEY,
  school_id  bigint NOT NULL,
  code       text UNIQUE NOT NULL,         -- kode aktivasi
  role       text NOT NULL,                -- siswa|orang_tua|guru
  target_id  bigint,                       -- student_id utk siswa/ortu, NULL utk guru batch
  expires_at timestamptz NOT NULL,
  used_at    timestamptz
)

audit_log (
  id        bigserial PRIMARY KEY,
  school_id bigint, user_id bigint,
  action    text NOT NULL,                 -- 'attendance.update', 'settings.save', ...
  entity    text NOT NULL, entity_id bigint,
  old_value jsonb, new_value jsonb,
  at        timestamptz NOT NULL DEFAULT now()
)
```

- `super_admin`: flag terpisah `users.is_super_admin boolean` (bukan membership — dia lintas sekolah).
- Login selalu terjadi DI domain sekolah → session terikat `school_id` domain itu. User yang punya membership di 2 sekolah login terpisah di masing-masing domain (isolasi cookie per domain, by design).
- Jika user punya >1 role di sekolah yang sama (jarang, mis. guru + orang_tua): pilih role saat login / switcher; session menyimpan role aktif.

## Autentikasi

- Password: argon2id (time=1, memory=64MB, threads=4 — parameter di config).
- Cookie: `HttpOnly; Secure; SameSite=Lax`, umur 30 hari (sliding), role `display` 1 tahun.
- Rate limit login: 5 gagal / 15 menit per akun, + limit per IP. Lockout progresif, bukan permanen.
- CSRF: SameSite=Lax + custom header check (`X-Requested-With`) untuk mutasi; token CSRF hanya jika ada form non-fetch.
- Aktivasi siswa/orang tua: sekolah generate kode undangan (per siswa: satu kode siswa, satu kode wali) → user buka `/{aktivasi}`, isi kode + buat password → membership terbentuk. Tidak ada open registration.
- Reset password: via email jika ada; siswa tanpa email di-reset oleh admin sekolah (generate password sementara).

## RBAC

- Middleware `requireAuth` → load session → user_id, role, school_id ke context.
- Middleware `requirePerm(perm)` → cek `rolePermissions[role]` (map statis di kode, bukan DB — permission granular tapi assignment role→permission cukup hardcode dulu; pindah ke DB kalau ada kebutuhan custom role).
- **Object-level check tetap di service** (bukan middleware): orang tua → hanya anak yang terhubung via `guardians`; siswa → hanya dirinya; guru → hanya kelas/jadwal miliknya (kecuali punya perm lebih luas).

## Daftar permission kanonik

| Permission | admin | kepsek | guru | siswa | ortu | display | pegawai |
|---|---|---|---|---|---|---|---|
| `student:manage` | ✅ | | | | | | |
| `student:read` | ✅ | ✅ | ✅ | | | | |
| `schedule:manage` | ✅ | | | | | | |
| `schedule:read` | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | |
| `attendance:write` | ✅ | | ✅ | | | | |
| `attendance:self_checkin` | | | | ✅ | | | |
| `attendance:read_own` | | | | ✅ | ✅ | | |
| `attendance:report` | ✅ | ✅ | ✅* | | | | |
| `teaching:journal_write` | | | ✅ | | | | |
| `teaching:monitor` | ✅ | ✅ | | | | ✅ | |
| `leave:request` | | | ✅ | | | | |
| `leave:approve` | | ✅** | ✅** | | | | |
| `leave:manage` | ✅ | | | | | | |
| `settings:manage` | ✅ | | | | | | |
| `billing:view` | ✅ | ✅ | | | | | |
| `announcement:manage` | ✅ | ✅ | | | | | |
| `dashboard:school` | ✅ | ✅ | | | | | |
| `discipline:manage` | ✅ | | | | | | |
| `discipline:record` | ✅ | | ✅ | | | | |
| `discipline:read` | ✅ | ✅ | ✅ | | | | |
| `duty:manage` | ✅ | | | | | | |
| `grading:manage` | ✅ | | ✅ | | | | |
| `user:impersonate` | ✅ | | | | | | |

\* guru: rekap kelas/jadwalnya sendiri (object-level). \** `leave:approve` efektifnya ditentukan approval chain (07) — permission hanya gerbang kasar. Siswa/orang tua TIDAK punya permission modul `discipline` — akses ke poin/surat miliknya sendiri lewat object-level (`student.CanViewStudent`, sama pola dengan `attendance:read_own`), lihat docs/12-sion-parity.md Gelombang A. `duty:manage` SENGAJA hanya admin_sekolah — kepsek TIDAK butuh kelola tugas tambahan (Fase 14 Gelombang B1). `grading:manage` (Fase 14 Gelombang C) dipegang admin_sekolah DAN guru — object-level guru dipersempit di service (`schedule.TeachesClassSubject`/`TeachesClass`, guru hanya kelas-mapel yang dia ajar TA aktif); kepala_sekolah SENGAJA TIDAK diberi akses baca pada gelombang ini (bisa ditambah permission read terpisah nanti); siswa/orang tua akses nilai/bintang milik sendiri TANPA permission lewat object-level (`GET /api/my-grades`/`GET /api/my-stars`, sama pola `attendance:read_own`).

Role **`pegawai`** (staff non-guru, mis. security/tata usaha, Fase 14 Gelombang B1 — `internal/employee`) SENGAJA **TANPA permission apa pun** di tabel ini — akses hanya lewat endpoint auth-only (`GET /api/me`, `/api/announcements?active=1`, `/api/notifications`) dan **capability flags** modul `internal/duty` (mis. `exit_security` Gelombang B2), bukan RBAC. `student:manage` juga dipakai gerbang `GET/POST/PATCH /api/employees` (BUKAN permission baru — keputusan sendiri, sama admin sekolah yang mengelola siswa yang mengelola pegawai) dan scope `all` `GET /api/student-leave` (izin siswa, `internal/studentleave`).

Permission baru = tambah konstanta + baris di map + baris di tabel dokumen ini.

## Tugas tambahan & capability flags (`internal/duty`, Fase 14 Gelombang B1)

Adopsi pola SION (docs/12-sion-parity.md Gelombang B): guru/pegawai diberi **tugas tambahan** (Wali Kelas, Guru BK, Guru Piket, Pimpinan, Security) **per tahun ajaran**; tiap tugas membawa satu atau lebih **flags**. Modul lain (mis. `studentleave`) menggerbang alur bisnis dengan mengecek flag seseorang (`duty.Service.UserHasFlag`/`UserIDsWithFlag`), **bukan** role/permission RBAC langsung — satu orang bisa punya beberapa tugas sekaligus (mis. guru yang juga Pimpinan).

Flags kanonik (`internal/duty/model.go`): `leave_homeroom_review`, `leave_issuance` (dipakai Gelombang B1); `exit_bk_approval`, `exit_leadership_approval`, `exit_security`, `late_arrival_duty`, `late_arrival_leadership`, `all_attendance_reports` (didefinisikan Gelombang B1, dipakai Gelombang B2 — belum ada endpoint yang menggerbang dengan flag-flag ini).
