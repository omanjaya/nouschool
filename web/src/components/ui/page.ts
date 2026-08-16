/**
 * Lebar wrapper konten halaman (docs/10-design-system.md §5, Fase 16).
 *
 * - `PAGE_WIDE`: layar daftar/dashboard/rekap/tabel/pengaturan admin & super
 *   admin — mengisi layar penuh (tanpa `mx-auto max-w-*`) supaya tidak ada
 *   whitespace besar di kiri-kanan pada layar lebar.
 * - `PAGE_NARROW`: layar form/detail personal (login, aktivasi, /izin-saya,
 *   /nilai-saya, /profil, /checkin, detail siswa, form-form) — tetap
 *   dibatasi ±720px supaya enak dibaca.
 */
export const PAGE_WIDE = 'w-full px-6 lg:px-8 py-6';
export const PAGE_NARROW = 'mx-auto w-full max-w-[720px] px-5 py-6';
