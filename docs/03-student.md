# 03 — Student: Siswa, Rombel, Enrollment, Wali, Import

Modul: `internal/student/`

## Skema

```sql
students (
  id         bigserial PRIMARY KEY,
  school_id  bigint NOT NULL,
  nis        text NOT NULL,        -- nomor induk sekolah
  nisn       text,                 -- nomor induk nasional (nullable, dipakai matching Dapodik)
  name       text NOT NULL,
  gender     text, birth_date date,
  user_id    bigint REFERENCES users,  -- NULL sampai siswa mengaktifkan akun
  status     text NOT NULL DEFAULT 'active',  -- active|graduated|moved|dropped
  UNIQUE (school_id, nis)
)

classes (            -- rombel, terikat tahun ajaran
  id               bigserial PRIMARY KEY,
  school_id        bigint NOT NULL,
  academic_year_id bigint NOT NULL,
  name             text NOT NULL,          -- "XII RPL 1"
  grade            text NOT NULL,          -- X|XI|XII|XIII
  major            text,                   -- jurusan SMK/peminatan (nullable)
  homeroom_teacher_id bigint,              -- wali kelas (membership guru)
  UNIQUE (school_id, academic_year_id, name)
)

enrollments (        -- siswa X ada di rombel Y pada tahun ajaran Z
  id         bigserial PRIMARY KEY,
  school_id  bigint NOT NULL,
  student_id bigint NOT NULL REFERENCES students,
  class_id   bigint NOT NULL REFERENCES classes,
  UNIQUE (student_id, class_id)
)
-- absensi/rapor SELALU lewat enrollment, bukan langsung student → riwayat antar tahun aman

guardians (
  id          bigserial PRIMARY KEY,
  school_id   bigint NOT NULL,
  user_id     bigint NOT NULL REFERENCES users,   -- akun orang tua
  student_id  bigint NOT NULL REFERENCES students,
  relation    text NOT NULL,   -- ayah|ibu|wali
  UNIQUE (user_id, student_id)
)

teachers ( -- profil kepegawaian guru; akun/role-nya tetap di memberships
  id         bigserial PRIMARY KEY,
  school_id  bigint NOT NULL,
  user_id    bigint NOT NULL REFERENCES users,
  nip        text,             -- NIP/NUPTK
  UNIQUE (school_id, user_id)
)

subjects (
  id        bigserial PRIMARY KEY,
  school_id bigint NOT NULL,
  code      text NOT NULL, name text NOT NULL,
  UNIQUE (school_id, code)
)
```

Naik kelas / tahun ajaran baru = buat classes baru + enrollments baru. Data lama tidak disentuh.

## Import data awal

Dua jalur, satu pipeline internal (`ImportRows -> validate -> preview -> commit`):

### 1. Template Excel/CSV NouSchool
- Template disediakan untuk: siswa (+rombel), guru, mapel, jadwal (lihat 04).
- Alur UX: upload → **preview & validasi di layar** (baris error ditandai: NIS dobel, rombel tak dikenal, dsb) → user konfirmasi → commit transaksional (all-or-nothing per file).
- Parsing Excel: lib `excelize`. CSV juga diterima.
- Import ulang file yang sama harus idempotent: match by `(school_id, nis)` → update, bukan duplikat.

### 2. File export Dapodik
- Tidak ada API resmi; sekolah export dari aplikasi Dapodik (umumnya Excel/CSV daftar peserta didik & PTK).
- Buat parser adaptor `dapodik.go` yang memetakan kolom-kolom umum export Dapodik → `ImportRows` yang sama. Matching by NISN bila ada, fallback NIS.
- Format export Dapodik bisa berbeda antar versi — parser harus toleran (mapping kolom by header name, bukan posisi) dan selalu lewat layar preview sebelum commit.

## Undangan akun (kaitan 02)

- Setelah import, admin bisa generate kode undangan massal: per siswa 2 kode (siswa & wali), export ke Excel/PDF untuk dibagikan wali kelas.
- Guru: undangan via email jika ada, atau kode.

## Object-level rules (ditegakkan di service)

- Orang tua hanya membaca data siswa yang terhubung di `guardians`.
- Siswa hanya dirinya (`students.user_id = ctx.userID`).
- Wali kelas mendapat akses rekap kelas perwaliannya.
