import {
  House,
  ClipboardCheck,
  CalendarDays,
  CalendarCheck,
  Bell,
  User,
  School,
  Settings,
  Users,
  MailWarning,
  type LucideIcon,
} from 'lucide-react';
import type { Me } from './types';

export interface NavItemDef {
  to: string;
  label: string;
  icon: LucideIcon;
  /**
   * Cocokkan hanya path persis (bukan prefix) — dipakai untuk item beranda yang
   * BUKAN `/` (mis. `/admin` platform admin), supaya tidak ikut aktif saat
   * berada di sub-rute sibling lain (`/admin/sekolah`, `/admin/outbox`, dst).
   * Default (tidak diisi): `AppShell` memakai `to === '/'`.
   */
  end?: boolean;
}

/** Nav guru — juga dipakai untuk peran yang belum punya nav khusus (mis. kepala_sekolah). */
const GURU_NAV: NavItemDef[] = [
  { to: '/', label: 'Beranda', icon: House },
  { to: '/absensi', label: 'Absensi', icon: ClipboardCheck },
  { to: '/izin', label: 'Izin', icon: CalendarDays },
  { to: '/notifikasi', label: 'Notifikasi', icon: Bell },
  { to: '/profil', label: 'Profil', icon: User },
];

/** Nav siswa (docs/10-design-system.md #5). */
const SISWA_NAV: NavItemDef[] = [
  { to: '/', label: 'Beranda', icon: House },
  { to: '/kehadiran', label: 'Kehadiran', icon: CalendarCheck },
  { to: '/jadwal', label: 'Jadwal', icon: CalendarDays },
  { to: '/notifikasi', label: 'Notifikasi', icon: Bell },
  { to: '/profil', label: 'Profil', icon: User },
];

/** Nav orang tua — item "Anak" mengarah ke riwayat kehadiran anak (docs/10 #5). */
const ORANG_TUA_NAV: NavItemDef[] = [
  { to: '/', label: 'Beranda', icon: House },
  { to: '/kehadiran', label: 'Anak', icon: CalendarCheck },
  { to: '/notifikasi', label: 'Notifikasi', icon: Bell },
  { to: '/profil', label: 'Profil', icon: User },
];

/** Nav per peran — lihat docs/10-design-system.md #5 untuk daftar lengkap per role. */
export function getNavItems(me: Me): NavItemDef[] {
  const isPlatformAdmin = me.is_super_admin && !me.school;

  if (isPlatformAdmin) {
    return [
      { to: '/admin', label: 'Beranda', icon: House, end: true },
      { to: '/admin/sekolah', label: 'Sekolah', icon: School },
      { to: '/admin/outbox', label: 'Outbox', icon: MailWarning },
      { to: '/profil', label: 'Profil', icon: User },
    ];
  }

  if (me.role === 'admin_sekolah') {
    return [
      { to: '/', label: 'Beranda', icon: House },
      { to: '/data', label: 'Data', icon: Users },
      { to: '/pengaturan', label: 'Pengaturan', icon: Settings },
      { to: '/notifikasi', label: 'Notifikasi', icon: Bell },
      { to: '/profil', label: 'Profil', icon: User },
    ];
  }

  if (me.role === 'siswa') return SISWA_NAV;
  if (me.role === 'orang_tua') return ORANG_TUA_NAV;

  return GURU_NAV;
}
