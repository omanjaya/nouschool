# 05 — Attendance: Absensi Siswa

Modul: `internal/attendance/`. Fitur bernilai tertinggi — dibangun dalam 2 tahap: mode `daily` dulu (tanpa jadwal), mode `per_subject` setelah modul schedule.

## Settings (module `attendance` di school_settings)

```go
type Settings struct {
    Mode            Mode     `json:"mode"`             // "daily" | "per_subject"
    Methods         []Method `json:"methods"`          // subset: manual|qr_card|self_checkin
    SelfCheckin     *SelfCheckinRule `json:"self_checkin,omitempty"`
    EditWindowHours int      `json:"edit_window_hours"` // default 24; setelahnya hanya admin
    LateAfterMin    int      `json:"late_after_min"`    // menit toleransi telat (self check-in)
}
type SelfCheckinRule struct {
    Lat, Lng   float64 `json:"lat","lng"`   // titik sekolah
    RadiusM    int     `json:"radius_m"`    // mis. 150
    OpenFrom   string  `json:"open_from"`   // "06:00" waktu lokal
    CloseAt    string  `json:"close_at"`    // "07:30"
}
// Default: {Mode: daily, Methods: [manual], EditWindowHours: 24}
```

## Skema

```sql
attendance_sessions (   -- satu "peristiwa absen" untuk satu rombel
  id               bigserial PRIMARY KEY,
  school_id        bigint NOT NULL,
  academic_year_id bigint NOT NULL,
  class_id         bigint NOT NULL,
  date             date NOT NULL,            -- tanggal LOKAL sekolah
  type             text NOT NULL,            -- 'daily' | 'subject'
  schedule_slot_id bigint REFERENCES schedule_slots,  -- NULL utk daily
  opened_by        bigint NOT NULL,          -- teacher yang membuka
  status           text NOT NULL DEFAULT 'open',      -- open|finalized
  created_at       timestamptz NOT NULL DEFAULT now(),
  UNIQUE (class_id, date) WHERE type = 'daily',            -- partial unique
  UNIQUE (schedule_slot_id, date) WHERE type = 'subject'
)

attendance_records (
  id         bigserial PRIMARY KEY,
  school_id  bigint NOT NULL,
  session_id bigint NOT NULL REFERENCES attendance_sessions,
  student_id bigint NOT NULL,
  status     text NOT NULL,      -- hadir|terlambat|izin|sakit|alpa
  method     text NOT NULL,      -- manual|qr_card|self_checkin
  marked_by  bigint NOT NULL,    -- user penginput (guru; siswa jika self_checkin)
  marked_at  timestamptz NOT NULL DEFAULT now(),
  meta       jsonb,              -- self_checkin: {lat,lng,accuracy,device}; qr_card: {scanner_device}
  note       text,
  UNIQUE (session_id, student_id)
)
```

Prinsip: **tiga metode input = tiga jalur menuju record yang sama.** Rekap & laporan hanya membaca sessions+records — tidak peduli metode.

## Alur per metode

### Manual (fondasi, selalu aktif)
1. Guru buka kelas → app buat/ambil sesi (daily: hari ini; per_subject: dari slot jadwalnya — atau otomatis via scan QR ruangan, lihat 06).
2. Daftar siswa dari enrollment rombel, default semua `hadir` → guru ubah yang tidak hadir → simpan sekali (bulk upsert).
3. UI mobile-first: satu layar, tap siklus status, simpan satu tombol.

### QR kartu siswa (`qr_card`)
- Kartu per siswa berisi QR = token acak (BUKAN NIS polos — token di tabel `student_qr_tokens (student_id, token UNIQUE, revoked_at)`, bisa dicabut kalau kartu hilang).
- Guru buka mode scan di sesi → kamera HP scan berturut-turut → tiap scan = upsert record `hadir` (atau `terlambat` sesuai waktu) → sisanya di-set manual/alpa saat sesi ditutup.
- Generator kartu: PDF grid kartu per kelas (nama, NIS, QR) untuk dicetak sekolah.

### Self check-in siswa (`self_checkin`)
- Untuk absen datang ke sekolah (sesi daily) — bukan per kelas.
- Siswa login → tombol check-in aktif pada jendela waktu → kirim koordinat GPS.
- Server validasi: dalam radius, dalam jendela waktu, belum check-in. Simpan bukti di `meta` (lat/lng/accuracy/UA).
- **Bukan anti-curang** (fake GPS mudah): tampilkan anomali ke wali kelas (banyak siswa koordinat identik, accuracy aneh) sebagai alat bantu; keputusan akhir tetap guru — guru bisa override record.

## Aturan bisnis

- Edit record hanya dalam `EditWindowHours` sejak sesi dibuat; setelah itu hanya `admin_sekolah` (dan tercatat di audit_log dengan old/new value).
- Sesi `finalized` mengunci input non-admin.
- Semua perubahan status setelah input pertama → audit_log.
- Izin guru yang disetujui TIDAK otomatis mengubah absensi siswa (beda domain). Tapi surat izin SISWA (sakit/izin dari ortu) fase nanti bisa jadi input status — catat sebagai ide, jangan bangun sekarang.

## Laporan (dipakai modul dashboard)

- Rekap harian per kelas (untuk TV & wali kelas), rekap per siswa per rentang (untuk ortu/siswa: `attendance:read_own`), rekap bulanan per kelas/sekolah (export Excel), persentase kehadiran per mapel (mode per_subject).
- Query rekap = agregasi SQL langsung (bukan loop di Go); tambahkan index `(school_id, date)`, `(student_id, session_id)`.
