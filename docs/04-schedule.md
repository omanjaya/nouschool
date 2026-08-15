# 04 — Schedule: Jadwal Pelajaran

Modul: `internal/schedule/`. Prasyarat untuk: absensi mode per-mapel (05), jurnal mengajar & TV (06).

## Skema

```sql
periods (             -- definisi "jam ke-" per sekolah
  id         bigserial PRIMARY KEY,
  school_id  bigint NOT NULL,
  number     int NOT NULL,          -- jam ke-1, ke-2, ...
  starts_at  time NOT NULL,         -- 07:00 (waktu lokal sekolah)
  ends_at    time NOT NULL,
  label      text,                  -- NULL utk jam pelajaran; "Istirahat"/"Upacara" utk non-KBM
  UNIQUE (school_id, number)
)

rooms (               -- ruang kelas fisik — dipakai QR per kelas (06)
  id         bigserial PRIMARY KEY,
  school_id  bigint NOT NULL,
  name       text NOT NULL,         -- "R. 12", "Lab TKJ 1"
  qr_token   text UNIQUE NOT NULL,  -- token acak untuk QR yang ditempel di ruangan
  UNIQUE (school_id, name)
)

schedule_slots (      -- inti jadwal: kelas × mapel × guru × hari × jam
  id               bigserial PRIMARY KEY,
  school_id        bigint NOT NULL,
  academic_year_id bigint NOT NULL,
  class_id         bigint NOT NULL REFERENCES classes,
  subject_id       bigint NOT NULL REFERENCES subjects,
  teacher_id       bigint NOT NULL REFERENCES teachers,
  room_id          bigint REFERENCES rooms,   -- nullable (olahraga/lapangan)
  day_of_week      int NOT NULL,              -- 1=Senin ... 6=Sabtu
  period_start     int NOT NULL,              -- jam ke- mulai
  period_end       int NOT NULL               -- jam ke- selesai (blok 2 JP: start=3,end=4)
)
```

Jadwal terikat `academic_year_id` — ganti semester/tahun = set slot baru, riwayat aman. (Kalau sekolah butuh jadwal beda per semester, tambah kolom `semester` nanti — jangan bangun sekarang.)

## Deteksi bentrok (aturan service, berlaku untuk builder DAN import)

Pada `(day_of_week, rentang period)` yang beririsan, tolak jika:
1. **Guru dobel**: teacher_id sama di 2 slot beririsan.
2. **Kelas dobel**: class_id sama di 2 slot beririsan.
3. **Ruang dobel**: room_id sama (non-NULL) di 2 slot beririsan.

Validasi di service dalam satu transaksi; error menyebutkan slot mana yang bentrok dengan apa.

## Builder UI

- Grid per kelas: kolom = hari, baris = jam ke-; klik sel → pilih mapel + guru + ruang; blok multi-JP dengan drag/merge.
- Tampilan alternatif per guru (untuk cek beban mengajar & bentrok visual).
- Bentrok dicek real-time saat menyimpan slot (bukan hanya saat submit semua).
- Copy jadwal: duplikat dari kelas lain / tahun ajaran sebelumnya sebagai titik awal.

## Import Excel

- Template: satu sheet per kelas ATAU format panjang (baris = slot: kelas, hari, jam mulai, jam selesai, kode mapel, kode/NIP guru, ruang).
- Pipeline sama dengan import siswa: upload → preview + daftar bentrok/referensi tak dikenal → konfirmasi → commit transaksional.
- Mapel/guru/ruang di-match by kode; yang tak dikenal ditawarkan dibuat otomatis (dengan konfirmasi) atau diperbaiki manual.

## Query kunci (dipakai modul lain)

- `SlotNow(schoolID, teacherID, at)` — slot guru pada waktu `at` (untuk scan QR kelas, 06).
- `SlotsToday(schoolID, classID|teacherID)` — untuk layar guru/siswa & TV.
- `CurrentPeriod(schoolID, at)` — jam ke berapa sekarang + countdown pergantian (TV).
Semua perhitungan "sekarang" memakai timezone sekolah via `platform/clock`.
