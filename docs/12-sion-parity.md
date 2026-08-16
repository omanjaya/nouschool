# 12 — Paritas Fitur SION (acuan per-sekolah dari user)

Acuan: repo `arimartana/sion` (rebuild "Pecalang", Go + Next.js). User meminta fitur PER SEKOLAH NouSchool menyamai fitur SION **yang terimplementasi** (bukan seluruh PRD-nya — modul perpustakaan/koperasi/LMS/SPP/SNPMB dkk di PRD SION belum dibangun di sana juga). Inventarisasi lengkap: hasil sweep 178 route + 42 migrasi + 39 halaman.

## Sudah SETARA atau LEBIH di NouSchool (tidak perlu apa-apa)

Auth & session, dashboard per role, master data (TA/kelas/mapel/jam/ruangan), data siswa+guru+import Excel, jadwal + builder + deteksi bentrok, presensi per sesi (kita malah 3 metode: manual/QR kartu/self check-in GPS), jurnal mengajar (kita: via scan QR ruangan), laporan & export presensi, pengumuman, notifikasi in-app + Web Push (kita + WhatsApp & email), branding, monitoring realtime (kita: WS + TV board), impersonation, dan semua keunggulan SaaS kita (multi-tenant, billing, custom domain, Dapodik).

## GAP — ada di SION, belum di NouSchool (urutan usulan build)

### Gelombang A — Kedisiplinan / Pelanggaran (modul baru `discipline`) ✅ backend selesai (Fase 14)
- Master jenis pelanggaran + poin (per sekolah)
- Catat pelanggaran siswa (opsional tertaut sesi presensi; unik per sesi+siswa+jenis), catatan
- Ambang SP1/SP2/SP3 per tahun ajaran (validasi sp1<sp2<sp3)
- **Surat Peringatan otomatis**: snapshot immutable + nomor surat unik per (TA, siswa, level); render HTML/PDF; unik per level
- Rekap & export pelanggaran; siswa/ortu lihat poin sendiri

### Gelombang B — Izin Siswa (modul baru `studentleave`) + fondasi otorisasi
Prasyarat arsitektur (adopsi pola SION):
- **Tugas tambahan + capability flags** (`duties`) ✅ Gelombang B1 (backend): guru/pegawai diberi tugas (Wali Kelas, Guru BK, Guru Piket, Pimpinan, Security) per TA; tiap tugas membawa flags (`leave_homeroom_review`, `leave_issuance`, `exit_bk_approval`, `exit_leadership_approval`, `exit_security`, `late_arrival_duty`, `late_arrival_leadership`, `all_attendance_reports`) — otorisasi alur izin membaca flags, bukan role
- **Role `pegawai`** ✅ Gelombang B1 (backend, staff non-guru, mis. security) + profilnya (`internal/employee`)
- **QR token guru** (kebalikan QR kita, Gelombang B2 ⬜): guru tampilkan QR berumur pendek sekali-pakai; siswa scan untuk approval
Tiga alur:
1. **Izin terencana** ✅ Gelombang B1 (backend, `internal/studentleave`): siswa ajukan (+dokumen) → wali kelas → BK terbitkan surat bernomor → verifikasi surat publik (`GET /api/public/leave-verify`)
2. **Izin dispensasi keluar** ⬜ Gelombang B2: rantai QR 4 tahap (piket → guru pengajar jam berjalan & bukan orang yang sama → BK → pimpinan) → gate token kedaluwarsa otomatis di akhir jam izin → scan gate oleh security → `exited`; row-lock transaksional
3. **Izin terlambat** ⬜ Gelombang B2: siswa scan QR → piket → pimpinan → guru kelas → selesai; aksi otomatis by hitungan: telat ke-2 & ke-5 = panggil ortu, ke-3 & ke-6 = pulangkan

### Gelombang C — Penilaian (modul baru `grading`, toggle per sekolah)
- Komponen penilaian per kelas+mapel (tp/sumatif/praktik/lainnya, bobot, KKTP)
- Input nilai; **nilai akhir = Σ(score×weight terisi) / Σ(weight terisi)** (normalisasi)
- Publikasi nilai per kelas-mapel; siswa/ortu lihat nilai terpublikasi
- Bintang kelas (beri/kurangi + catatan + visibility)
- Konfigurasi rapor: rentang nilai/pembulatan, pemetaan TP, nilai manual/sebelumnya, export & analisis
- Toggle `grading_enabled` via school_settings (menu ikut hilang)

### Gelombang D — Pelengkap
- Konseling BK (`counseling`): sesi (tujuan karir, masalah, rencana tindak lanjut, foto bukti) + report HTML
- Guru pengganti (substitution request → accept/reject; monitoring/TV menganggap pengganti sebagai pengajar)
- Period day overrides (jam khusus per hari, mis. Jumat)
- Kalender presensi siswa (UI kalender bulanan + detail per hari)
- Impersonate USER oleh admin sekolah (admin → masuk sebagai guru/siswa; reuse infra impersonation, audit)
- General settings tambahan: template surat (izin/SP), deadline koreksi, single-device login (opsional)

## TIDAK diadopsi (keputusan)
- JWT (kita tetap cookie session), MySQL (tetap Postgres), Next.js (tetap Vite PWA), Tabler (tetap design system Rapor)
- Role/permission editor dinamis penuh — duty-capability flags (Gelombang B) menutup kebutuhan yang sama dengan lebih terarah; editor role penuh tetap di "ide tertunda"
- Modul PRD SION yang di sana pun belum dibangun (perpustakaan, koperasi/POS, LMS, SPP, SNPMB, guru wali, diagnostic, supervisi, chat, Telegram) — masuk ide tertunda

## Aturan implementasi
Semua mengikuti CLAUDE.md (modul terpisah, consumer-side interface, school_id dari context, audit, design system Rapor, state machine eksplisit dengan test). Fitur baru menulis event realtime & notifikasi yang relevan (mis. SP terbit → notif ortu; exit permit tahap berikut → notif approver).
