/**
 * Label peran (dulu didefinisikan lokal di beberapa fitur, mis.
 * `features/admin/api.ts` & `features/profile/ProfilePage.tsx`) — dipindah
 * ke sini supaya bisa dipakai `AppShell` (komponen bersama, tanpa import
 * lintas fitur ke `features/admin`) sekaligus jadi satu sumber kebenaran.
 */
export const ROLE_LABEL: Record<string, string> = {
  admin_sekolah: 'Admin Sekolah',
  kepala_sekolah: 'Kepala Sekolah',
  guru: 'Guru',
  siswa: 'Siswa',
  orang_tua: 'Orang Tua',
  pegawai: 'Pegawai',
  display: 'Display',
};
