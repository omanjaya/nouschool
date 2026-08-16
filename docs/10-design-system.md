# 10 — Design System "Rapor" (FINAL — jangan diubah tanpa persetujuan user)

Arah visual dipilih user dari 6 sample (artifact "Arah Visual NouSchool"): **Sample 1 — Rapor, institusional minimal**. Karakter: putih bersih, pemisah hairline tipis, tipografi tenang, aksen dipakai sangat hemat. Rasa dokumen resmi sekolah yang dirapikan — tegas, terpercaya, cepat.

**Larangan keras (anti AI-slop, keputusan user):** tidak ada emoji di UI (icon Lucide saja), tidak ada gradien (apalagi ungu-biru), tidak ada glassmorphism/blur/glow, tidak ada shadow dekoratif, tidak ada kartu ber-border yang membungkus segalanya — pemisah utama adalah hairline. Tidak menambah font eksternal.

## 1. Token warna

Semua warna HANYA lewat CSS variables di `web/src/styles/tokens.css` — komponen tidak pernah hardcode hex. Dark mode belum dibuat, tapi karena semua lewat token, nanti cukup satu set token baru.

```css
:root {
  /* Netral (bias biru-tinta, bukan abu murni) */
  --bg:        #FFFFFF;
  --surface:   #FFFFFF;
  --surface-2: #F5F7FA;   /* latar sekunder: header tabel, hover */
  --ink:       #17233A;   /* teks utama — navy tinta, bukan hitam murni */
  --muted:     #64708A;   /* teks sekunder */
  --line:      #E5E8EE;   /* hairline — SATU-satunya warna border pemisah */

  /* Aksen = warna brand sekolah (dari school_settings.branding), default hijau tua */
  --primary:      #0E6B4E;
  --primary-ink:  #FFFFFF;   /* teks di atas primary; divalidasi kontras saat sekolah set warna */
  --primary-soft: #E7F2ED;   /* tint latar: tag aktif, highlight — digenerate dari --primary */

  /* Semantik status (TETAP — tidak ikut brand sekolah, supaya makna konsisten lintas sekolah) */
  --st-hadir:     #0E6B4E;  --st-hadir-line:     #BFDCCF;
  --st-terlambat: #A3620D;  --st-terlambat-line: #E4CFA8;
  --st-sakit:     #A3620D;  --st-sakit-line:     #E4CFA8;
  --st-izin:      #34567F;  --st-izin-line:      #C1D0E2;
  --st-alpa:      #B3382A;  --st-alpa-line:      #E3BCB5;
  --danger:       #B3382A;
  --danger-soft:  #F9ECEA;
}
```

Aturan pakai aksen: `--primary` hanya untuk (1) tombol aksi utama — maks. satu per layar, (2) indikator tab/nav aktif, (3) eyebrow/angka yang benar-benar perlu ditonjolkan. Selebihnya halaman adalah putih + tinta + hairline.

Warna brand per sekolah: `--primary` di-inject dari branding; `--primary-soft` dan `--primary-ink` dihitung (tint + cek kontras WCAG AA; jika kontras teks putih < 4.5:1, pakai `--ink`). Validasi ini di util `web/src/lib/color.ts`, dipakai juga saat admin memilih warna.

## 2. Tipografi

- Stack: `"Segoe UI", system-ui, -apple-system, Roboto, Arial, sans-serif`. **Tanpa webfont** — kecepatan adalah fitur.
- Skala (px): 11 (eyebrow/caption) · 12 (sekunder) · 14 (body — default) · 16 (subjudul) · 18 (judul kartu) · 21 (judul halaman) · 28 (angka besar dashboard). Jangan membuat ukuran di luar skala.
- Bobot: 400 body · 600 penekanan/judul · 700 hanya angka penting & chip. Tidak memakai 800+.
- Eyebrow/label seksi: 11px, uppercase, `letter-spacing: 0.1em`, warna `--muted` (atau `--primary` jika berfungsi sebagai penanda aktif).
- Semua angka berjajar (jam, NIS, rekap, tabel): `font-variant-numeric: tabular-nums` (class util `num`).

## 3. Bentuk, jarak, elevasi

- Radius: **8px** untuk semua permukaan & tombol; 999px hanya untuk chip/pill. Tidak ada radius lain.
- Spacing: skala 4 — 4/8/12/16/20/24/32. Padding layar mobile: 20–22px. Gap antarseksi: 24px.
- Border: hairline `1px solid var(--line)`. TIDAK ada shadow elevasi; pengecualian tunggal: overlay (dialog/sheet/dropdown) boleh `0 8px 24px rgba(23,35,58,0.12)`.
- Pemisah list: `border-bottom` hairline pada baris, bukan kartu per item.
- Touch target minimal 44×44px; baris list minimal tinggi 48px.

## 4. Icon

- **Lucide** (`lucide-react`), satu-satunya sumber icon. **Emoji dilarang di seluruh UI, notifikasi, dan dokumen yang dilihat pengguna.**
- Ukuran: 20px default, 16px inline/di dalam tombol, 24px hanya empty state. `stroke-width: 2`.
- Warna icon mengikuti warna teks di sebelahnya (`currentColor`) — tidak pernah warna sendiri.

## 5. Layout & navigasi

- **Mobile-first.** Konten maks 640px di tengah untuk layar form/list; dashboard admin/kepsek desktop boleh 1120px.
- **Bottom tab bar** di mobile (5 item maks), item aktif: warna `--primary` + underline 2px. Per role:
  - Guru: Beranda · Absensi · Izin · Notifikasi · Profil
  - Siswa: Beranda · Kehadiran · Jadwal · Notifikasi · Profil
  - Orang tua: Beranda · Anak · Notifikasi · Profil
  - Admin/Kepsek di mobile: Beranda · Data · Laporan · Notifikasi · Profil
- Desktop (≥1024px): tab bar berubah jadi **sidebar kiri** dengan item yang sama + item tambahan admin. Satu komponen `AppShell` menangani keduanya.
- App bar halaman-dalam: tombol back (chevron-left) + judul 17px + subjudul 12px muted. Tanpa app bar berwarna.
- TV dashboard (06): layout terpisah `/tv`, token sama, ukuran font digandakan — bukan design system lain.

## 6. Shared components (`web/src/components/ui/`)

Fitur DILARANG membuat styling sendiri untuk pola di bawah — pakai komponen bersama. Butuh varian baru → tambahkan di komponen bersama, bukan one-off di fitur.

| Komponen | Kontrak singkat |
|---|---|
| `AppShell` | Bottom nav mobile / sidebar desktop, per role, badge notifikasi |
| `AppBar` | back?, title, subtitle?, action? |
| `Card` | permukaan ber-hairline radius 8; varian `plain` (tanpa border, untuk grouping) |
| `Button` | `primary` (isi --primary) · `secondary` (outline hairline) · `ghost` · `danger`; state loading (spinner) & disabled |
| `StatusChip` | status kehadiran: outline + warna semantik; ukuran `sm` (huruf H/S/I/T/A) & `md` (label penuh) |
| `Tag` | label kecil pill: `now` (primary-soft), `done` (surface-2), netral |
| `ListRow` | baris list hairline: leading?, title, subtitle?, trailing; tinggi ≥48px |
| `StatTile` | angka besar 28px tabular + label eyebrow — dashboard |
| `Field` / `Input` / `Select` / `Textarea` / `DateInput` | label 12px, border hairline, fokus ring `--primary`, pesan error `--danger` di bawah |
| `EmptyState` | icon 24 + kalimat + aksi opsional — WAJIB untuk setiap list yang bisa kosong |
| `ErrorState` | pesan apa yang salah + tombol coba lagi |
| `Skeleton` | placeholder loading list/kartu — bukan spinner fullscreen |
| `Toast` | konfirmasi aksi ("Absensi tersimpan") — kalimat aktif, tanpa emoji |
| `Dialog` / `Sheet` | konfirmasi & form pendek; sheet dari bawah di mobile |
| `DataTable` | tabel admin desktop: header surface-2 sticky, zebra off, hairline, kolom angka rata kanan tabular; sort klik header (client-side, ikon ArrowUpDown), aksi baris via dropdown `MoreHorizontal`, scroll-x |
| `SegmentedControl` | filter pendek (mis. rentang tanggal) |

## 7. State wajib (ciri utama non-AI-slop)

Setiap layar/list HARUS mendesain: **loading** (Skeleton), **kosong** (EmptyState dengan kalimat kontekstual, mis. "Belum ada pengajuan izin bulan ini"), **error** (ErrorState + retry), **sukses** (Toast). PR/fitur yang hanya menangani happy path dianggap belum selesai.

## 8. Copywriting UI

- Bahasa Indonesia baku-santai, kalimat aktif. Tombol menyebut aksinya: "Simpan Absensi", bukan "OK"/"Submit".
- Error menjelaskan apa yang salah + cara memperbaiki: "Kode undangan sudah dipakai. Minta kode baru ke admin sekolah." — tanpa minta maaf berlebihan, tanpa istilah teknis.
- Tanggal: "Selasa, 18 Agu 2026"; jam 24-jam "08.30". Istilah domain konsisten: Hadir/Terlambat/Izin/Sakit/Alpa, Rombel, Jam ke-, Tahun Ajaran.

## 9. Implementasi Tailwind

- Token dipetakan ke Tailwind theme via CSS variables (`colors: { ink: "var(--ink)", ... }`) di `tailwind.config` — kelas semantik (`text-ink`, `border-line`, `bg-primary`), BUKAN palet default Tailwind (`gray-500`, `indigo-600` dilarang).
- shadcn/ui boleh dipakai sebagai basis komponen, tapi WAJIB di-restyle ke token Rapor (radius 8, hairline, tanpa shadow) — bukan tampilan default shadcn.
- `prefers-reduced-motion` dihormati; animasi hanya transisi opacity/transform ≤200ms, tanpa bounce.
