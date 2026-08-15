import { House, ClipboardCheck, CalendarDays, Bell, User, School, Settings, Users, type LucideIcon } from 'lucide-react';
import type { Me } from './types';

export interface NavItemDef {
  to: string;
  label: string;
  icon: LucideIcon;
}

/** Nav guru (default) — dipakai juga untuk peran yang belum punya nav khusus di Fase 1. */
const GURU_NAV: NavItemDef[] = [
  { to: '/', label: 'Beranda', icon: House },
  { to: '/absensi', label: 'Absensi', icon: ClipboardCheck },
  { to: '/izin', label: 'Izin', icon: CalendarDays },
  { to: '/notifikasi', label: 'Notifikasi', icon: Bell },
  { to: '/profil', label: 'Profil', icon: User },
];

/** Nav per peran — lihat docs/10-design-system.md #5 untuk daftar lengkap per role. */
export function getNavItems(me: Me): NavItemDef[] {
  const isPlatformAdmin = me.is_super_admin && !me.school;

  if (isPlatformAdmin) {
    return [
      { to: '/admin', label: 'Sekolah', icon: School },
      { to: '/profil', label: 'Profil', icon: User },
    ];
  }

  if (me.role === 'admin_sekolah') {
    return [
      { to: '/', label: 'Beranda', icon: House },
      { to: '/data', label: 'Data', icon: Users },
      { to: '/pengaturan', label: 'Pengaturan', icon: Settings },
      { to: '/profil', label: 'Profil', icon: User },
    ];
  }

  return GURU_NAV;
}
