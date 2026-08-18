# 13 — Rencana Fokus UI (Fase 17): dari "berfungsi" ke "premium"

Status per 18 Agu 2026: SEMUA fitur selesai (Fase 0–16). Feedback user setelah Fase 16: UI admin/super admin masih terasa "jelek, kosong, whitespace berlebih" walau shell/nav/DataTable sudah dirombak. Fase 17 = **fokus UI murni**, tanpa fitur baru. Design system Rapor (docs/10) TETAP dikunci — yang diperbaiki adalah **kerapatan informasi, hierarki visual, dan polish**, bukan mengganti gaya.

## Cara kerja fase ini (aturan main)
1. **User adalah auditor visual utama.** Fable orkestrasi + verifikasi build; Sonnet menulis kode. Fable TIDAK menyatakan "selesai" untuk sprint UI — user yang menyatakan setelah cek langsung.
2. Kerja **per sprint kecil (1 layar/area per agent)**, commit per sprint, supaya user bisa cek bertahap dan feedback tidak menumpuk.
3. Setiap sprint DIMULAI dengan agent membaca `docs/10-design-system.md` + `web/src/components/ui/*` + halaman target, dan DIAKHIRI dengan `npm run build` + lint hijau + grep anti-slop (`gray-|zinc-|slate-|bg-gradient|shadow-` selain overlay).
4. Bahan block: `npx shadcn@latest add @shadcnblocks/<n>` (key di `web/.env.local`; CLI menulis ke folder `web/@/` — pindahkan ke `src/`, adaptasi token, hapus `web/@/`; lihat `web/src/components/blocks/README.md`). Block hanya BAHAN — hasil akhir harus Rapor.
5. Feedback user dicatat sebagai item ⬜ baru di seksi "Backlog feedback" bawah, lalu dieksekusi sprint berikutnya.

## Diagnosis akar (kenapa masih terasa kosong)
- **Kerapatan rendah**: kartu/list satu-per-baris dengan padding besar; desktop admin butuh **density tinggi** (baris 40–44px, font 13px di tabel, padding kartu 16px, jarak antar seksi 20px).
- **Hierarki datar**: semua elemen setara — tidak ada "hal terpenting" per layar. Perlu: 1 area utama dominan + area sekunder lebih kecil/redup.
- **Halaman berisi 1 hal**: banyak halaman hanya list. Perlu **panel samping/atas** (ringkasan, filter, aksi) mengisi lebar.
- **Konsistensi header halaman**: eyebrow+judul+aksi berbeda-beda per halaman → buat `PageHeader` bersama (judul, deskripsi 1 kalimat, breadcrumb sudah di top bar, slot aksi kanan, slot tab).
- **Empty state & angka nol** mendominasi di data demo — perlu data demo lebih kaya (bootstrap: lebih banyak siswa/absensi/izin/nilai) supaya UI terlihat "hidup" saat dicek.

## Sprint (urutan — tiap sprint = 1 agent Sonnet, commit terpisah)

### S1 — Fondasi density & PageHeader (menyentuh komponen bersama)
- Token spacing/density: kelas util `density-compact` untuk area admin desktop (ListRow 44px, DataTable row 40px, font 13px, Card padding 16px).
- Komponen bersama baru: `PageHeader` (judul 20px, deskripsi muted, slot aksi, slot tab/filter bar sticky di bawah top bar), `Toolbar` (search + filter + aksi dalam satu baris hairline), `SectionCard` (judul seksi 13px + aksi kanan + body — pengganti Card polos utk dashboard), `KpiRow` (StatTile grid auto-fit dengan pembatas hairline vertikal, bukan kartu-kartu terpisah).
- Terapkan `PageHeader` + `Toolbar` ke SEMUA halaman admin/super admin (grep judul `text-[21px]`).
- **Data demo lebih kaya** (bootstrap `-demo`): 4 rombel × 25 siswa, absensi 14 hari terakhir terisi (variasi status), 6 guru + jadwal penuh, 8 izin guru (campur status), 20 catatan pelanggaran, nilai 3 mapel, 5 pengumuman, notifikasi — supaya setiap layar punya isi saat diaudit.

### S2 — Dashboard super admin (`/admin`) 
- Layout 12 kolom: KpiRow atas (6 tile hairline); kiri 8 kolom: DataTable "Perlu Perhatian" + "Aktivitas Sekolah" (dengan sparkline kecil login 7 hari bila mudah — SVG inline, token); kanan 4 kolom: panel "Pendapatan bulan ini" (angka + mini bar 12 bulan SVG), "Leads terbaru", "Outbox status".
- Kartu sekolah di `/admin/sekolah`: DataTable + panel ringkas kanan saat baris dipilih (preview: status, langganan, TA aktif, tombol Masuk sebagai) — pola master-detail.
- `/admin/sekolah/:id` (detail sekolah — SANGAT panjang sekarang): ubah ke **Tabs** (Ringkasan · Onboarding · Anggota · Langganan · Notifikasi · Audit · Statistik) — bukan 8 seksi bertumpuk.

### S3 — Dashboard admin sekolah (`/`) & kepsek
- Layout 12 kolom serupa: KpiRow; kiri: "Absensi hari ini" DataTable + "Perlu Tindakan"; kanan: "Guru mengajar sekarang" (live), "Pengumuman", "Izin menunggu".
- Kepsek: sama tapi menonjolkan monitoring & rekap.
- Beranda guru/siswa/ortu: kartu ringkas 2 kolom TETAP tapi dipadatkan + "hari ini" (jadwal/absen berikutnya) di atas.

### S4 — Halaman data (Data › Siswa/Guru/Rombel/…) 
- Master-detail desktop: DataTable kiri + **drawer/panel detail kanan** saat baris diklik (bukan pindah halaman) untuk siswa & guru; aksi (edit, reset password, nonaktifkan, masuk-sebagai, kartu QR) di panel.
- Toolbar seragam (search, filter rombel/status, tombol import/tambah).
- Halaman detail siswa penuh: Tabs (Profil · Kehadiran (kalender) · Kedisiplinan · Nilai · Izin).

### S5 — Halaman operasional (Absensi/Izin/Kedisiplinan/Nilai/Monitoring/Pengaturan)
- `/absensi/sesi/:id`: layar guru dipertahankan (sudah dioptimalkan jempol) — hanya polish.
- `/absensi/rekap`, `/kedisiplinan`, `/izin*`: PageHeader + Toolbar + DataTable + panel ringkas kanan (KPI kecil rentang terpilih).
- `/monitoring`: grid guru × jam sebagai **heatmap tabel** (status warna semantik) + panel presence.
- `/pengaturan`: anchor nav kiri sudah ada → rapikan tiap seksi jadi `SectionCard`, form 2 kolom di desktop.
- `/nilai`: tabel input nilai bergaya spreadsheet (sel rapat, sticky kolom nama).

### S6 — Landing & login/aktivasi polish
- Landing: rapikan spacing block, tambah seksi "Untuk siapa" (admin/guru/ortu) & FAQ (block accordion shadcnblocks → adaptasi), screenshot produk asli (ambil dari app setelah S2–S3 rapi).
- Login/aktivasi/impersonate/verifikasi surat: satu template `AuthLayout` (kiri brand+ilustrasi token, kanan form) di desktop.

### S7 — Detail kecil lintas app
- Konsistensi ikon ukuran & label tombol, empty state kontekstual per layar, skeleton menyerupai layout final, transisi 150ms hover/fokus, focus ring, tabel: kolom angka rata kanan `num`, header sort indikator, zebra OFF tetap.
- Cek responsif 3 breakpoint (375 / 1024 / 1440) untuk 10 halaman utama.

## Definisi "cukup bagus" (acuan audit user)
Layar admin desktop tanpa scroll sudah menampilkan ≥3 jenis informasi (KPI, tabel, panel), tidak ada area kosong > 25% lebar tanpa alasan, header & toolbar konsisten di semua halaman, dan tidak ada elemen yang melanggar docs/10.

## Backlog feedback user (tambahkan di sini)
- (kosong — isi saat user memberi feedback per sprint)
