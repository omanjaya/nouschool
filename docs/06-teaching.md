# 06 — Teaching: Jurnal Mengajar, QR per Kelas, Dashboard TV

Modul: `internal/teaching/`. Prasyarat: schedule (04). Catatan keputusan: TIDAK ada absensi jam datang/pulang guru — monitoring murni "guru sedang mengajar di mana".

## Konsep inti: scan QR ruangan = satu aksi, tiga hasil

Setiap ruang kelas punya QR fisik tercetak (dari `rooms.qr_token`, lihat 04). Saat guru masuk kelas:

1. Guru scan QR ruangan dari app (kamera) → `POST /api/teaching/scan {room_token}`.
2. Server: resolve room → cari `SlotNow(schoolID, teacherID, now)`:
   - **Slot ketemu & ruang cocok** → buat `teaching_journals` entry (status `ongoing`).
   - **Slot ketemu tapi ruang beda** → tetap catat, flag `room_mismatch` (pindah ruangan itu normal; TV menampilkan ruang aktual).
   - **Tidak ada slot** (guru pengganti/insidental) → app minta pilih kelas+mapel manual → journal dengan flag `unscheduled`.
3. Sekaligus tawarkan **buka sesi absensi siswa** untuk slot itu (mode per_subject) — satu scan: jurnal terisi + sesi absen siap. Ini insentif alami supaya guru rajin scan.
4. Selesai mengajar: entry otomatis `done` saat periode slot berakhir (tidak perlu scan keluar); guru boleh menutup manual lebih awal.
5. Guru melengkapi jurnal (materi yang diajarkan, catatan) — boleh diisi belakangan hari itu.

```sql
teaching_journals (
  id               bigserial PRIMARY KEY,
  school_id        bigint NOT NULL,
  teacher_id       bigint NOT NULL,
  schedule_slot_id bigint REFERENCES schedule_slots,  -- NULL jika unscheduled
  class_id         bigint NOT NULL,
  subject_id       bigint,
  room_id          bigint,                 -- ruang AKTUAL (dari QR yang discan)
  date             date NOT NULL,
  started_at       timestamptz NOT NULL,   -- waktu scan
  ended_at         timestamptz,
  material         text,                   -- materi yang diajarkan
  note             text,
  flags            text[],                 -- {room_mismatch, unscheduled}
  UNIQUE (schedule_slot_id, date)
)
```

## Status mengajar (derivasi, bukan tabel)

Untuk setiap slot jadwal pada jam berjalan:
- **Mengajar** (hijau): ada journal `started_at` dalam slot ini.
- **Belum masuk kelas** (merah): slot berjalan > X menit (default 10, configurable) tanpa journal.
- **Izin** (kuning): guru punya leave request approved hari ini (baca dari modul leave via interface).
- **Belum mulai** (abu): slot belum mencapai jam mulai.

## Dashboard TV ruang guru

Akses: **akun display** — membership role `display`, session panjang (1 th), permission read-only (`teaching:monitor`, `schedule:read`). Login sekali di TV/mini-PC, buka `/{tv}` fullscreen.

Konten (semua dipilih user, satu layar dengan rotasi/panel):
1. **Status guru mengajar** (panel utama): grid guru × jam hari ini, warna sesuai status di atas; sorot merah untuk slot berjalan tanpa journal.
2. **Jam & pergantian**: jam pelajaran ke berapa sekarang + countdown pergantian (`CurrentPeriod`).
3. **Pengumuman**: teks berjalan/panel dari tabel `announcements (school_id, title, body, starts_at, ends_at, created_by)` — dikelola admin/kepsek (`announcement:manage`).
4. **Rekap absensi siswa hari ini**: hadir/izin/sakit/alpa per kelas (baca dari attendance via interface).

Teknis TV:
- Data refresh via polling (interval 15–30 dtk) — cukup; JANGAN bangun WebSocket dulu.
- Layout untuk 1920×1080, font besar, auto-reload harian (guard kalau app di-deploy ulang).
- Endpoint TV mengembalikan payload gabungan satu kali fetch (`GET /api/tv/board`) supaya ringan.

## Dashboard kepala sekolah (subset modul `dashboard`)

- Panel yang sama dengan TV versi interaktif + drill-down: klik guru → riwayat jurnal & kepatuhan scan; klik kelas → rekap absen.
- Rekap mingguan/bulanan: % slot terlaksana (journal vs jadwal) per guru — metrik "ketertiban mengajar".

## Anti-abuse ringan

- QR ruangan = token acak; bisa di-regenerate per ruangan (kartu QR dicetak ulang) bila bocor.
- Scan hanya valid dari akun role guru sekolah ybs; scan di luar jam slot ±toleransi masuk `unscheduled`.
- Guru scan dari rumah? Terekam `started_at` + tidak ada mitigasi GPS di v1 (kepsek melihat pola; jangan over-engineer).
