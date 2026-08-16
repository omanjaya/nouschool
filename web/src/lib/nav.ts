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
  DoorOpen,
  LayoutDashboard,
  PlusCircle,
  Inbox,
  CreditCard,
  TrendingUp,
  Megaphone,
  GraduationCap,
  Layers,
  UserCog,
  Briefcase,
  BookOpen,
  Clock,
  Building2,
  ClipboardList,
  BarChart3,
  CalendarClock,
  ShieldAlert,
  Award,
  MessageCircle,
  RefreshCw,
  Tv,
  Lock,
  QrCode,
  ScanLine,
  FileText,
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

/** Grup nav sidebar desktop (Fase 16, docs/10-design-system.md §5) — `title` kosong untuk grup pertama (tanpa eyebrow, biasanya cuma "Beranda"). */
export interface NavGroup {
  title?: string;
  items: NavItemDef[];
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

/**
 * Nav pegawai (Fase 14 Gelombang B2, docs/12-sion-parity.md Gelombang B) —
 * staf non-guru (mis. security gerbang, `internal/employee`). Sebelumnya
 * `getNavItems` jatuh ke `GURU_NAV` untuk peran ini (belum punya nav
 * khusus); sekarang diberi nav sendiri karena satu-satunya tugas produknya
 * (scan gate izin keluar) tidak relevan dengan nav guru (Absensi/Izin guru).
 */
const PEGAWAI_NAV: NavItemDef[] = [
  { to: '/', label: 'Beranda', icon: House },
  { to: '/gerbang', label: 'Gerbang', icon: DoorOpen },
  { to: '/profil', label: 'Profil', icon: User },
];

/** Nav per peran (bottom tab bar mobile, ≤5 item) — lihat docs/10-design-system.md #5 untuk daftar lengkap per role. */
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
  if (me.role === 'pegawai') return PEGAWAI_NAV;

  return GURU_NAV;
}

/**
 * Nav sidebar desktop (Fase 16) — grup lengkap per peran, dirender oleh
 * `AppShell` untuk `lg:` (>=1024px). Berbeda dari `getNavItems` (bottom tab
 * mobile, tetap ≤5 item) — di sini semua fitur yang selama ini cuma kartu di
 * beranda ikut dimasukkan supaya sidebar tidak kosong/setengah jadi.
 */
export function getSidebarNav(me: Me): NavGroup[] {
  const isPlatformAdmin = me.is_super_admin && !me.school;

  if (isPlatformAdmin) {
    return [
      { items: [{ to: '/admin', label: 'Beranda', icon: LayoutDashboard, end: true }] },
      {
        title: 'Sekolah',
        items: [
          { to: '/admin/sekolah', label: 'Daftar Sekolah', icon: School },
          { to: '/admin/sekolah/new', label: 'Tambah Sekolah', icon: PlusCircle },
          { to: '/admin/minat', label: 'Minat Sekolah', icon: Inbox },
        ],
      },
      {
        title: 'Langganan',
        items: [
          { to: '/admin/plans', label: 'Plan & Harga', icon: CreditCard },
          { to: '/admin/pendapatan', label: 'Pendapatan', icon: TrendingUp },
        ],
      },
      {
        title: 'Operasional',
        items: [
          { to: '/admin/outbox', label: 'Outbox', icon: MailWarning },
          { to: '/admin/pengumuman', label: 'Pengumuman Platform', icon: Megaphone },
        ],
      },
      { title: 'Akun', items: [{ to: '/profil', label: 'Profil', icon: User }] },
    ];
  }

  if (me.role === 'admin_sekolah') {
    return [
      { items: [{ to: '/', label: 'Beranda', icon: House }] },
      {
        title: 'Akademik',
        items: [
          { to: '/data/siswa', label: 'Siswa', icon: GraduationCap },
          { to: '/data/rombel', label: 'Rombel', icon: Layers },
          { to: '/data/guru', label: 'Guru', icon: UserCog },
          { to: '/data/pegawai', label: 'Pegawai', icon: Briefcase },
          { to: '/data/mapel', label: 'Mapel', icon: BookOpen },
          { to: '/data/jadwal', label: 'Jadwal', icon: CalendarDays },
          { to: '/data/jam', label: 'Jam Pelajaran', icon: Clock },
          { to: '/data/ruangan', label: 'Ruangan', icon: Building2 },
          { to: '/data/tugas', label: 'Tugas Tambahan', icon: ClipboardList },
        ],
      },
      {
        title: 'Kegiatan',
        items: [
          { to: '/absensi', label: 'Absensi', icon: ClipboardCheck },
          { to: '/absensi/rekap', label: 'Rekap Absensi', icon: BarChart3 },
          { to: '/izin', label: 'Izin Guru', icon: CalendarClock },
          { to: '/izin-siswa', label: 'Izin Siswa', icon: FileText },
          { to: '/kedisiplinan', label: 'Kedisiplinan', icon: ShieldAlert },
          { to: '/nilai', label: 'Nilai', icon: Award },
          { to: '/konseling', label: 'Konseling', icon: MessageCircle },
          { to: '/pengganti', label: 'Guru Pengganti', icon: RefreshCw },
          { to: '/monitoring', label: 'Monitoring', icon: Tv },
          { to: '/pengumuman', label: 'Pengumuman', icon: Megaphone },
        ],
      },
      {
        title: 'Sistem',
        items: [
          { to: '/pengaturan', label: 'Pengaturan', icon: Settings },
          { to: '/pengaturan/hak-akses', label: 'Hak Akses', icon: Lock },
          { to: '/tagihan', label: 'Tagihan', icon: CreditCard },
          { to: '/notifikasi', label: 'Notifikasi', icon: Bell },
          { to: '/profil', label: 'Profil', icon: User },
        ],
      },
    ];
  }

  if (me.role === 'kepala_sekolah') {
    return [
      { items: [{ to: '/', label: 'Beranda', icon: House }] },
      {
        title: 'Pantau',
        items: [
          { to: '/monitoring', label: 'Monitoring', icon: Tv },
          { to: '/monitoring/rekap', label: 'Rekap Ketertiban', icon: BarChart3 },
          { to: '/absensi/rekap', label: 'Rekap Absensi', icon: ClipboardCheck },
          { to: '/kedisiplinan', label: 'Kedisiplinan', icon: ShieldAlert },
          { to: '/nilai', label: 'Nilai', icon: Award },
          { to: '/konseling', label: 'Konseling', icon: MessageCircle },
        ],
      },
      {
        title: 'Persetujuan',
        items: [
          { to: '/izin', label: 'Izin Guru', icon: CalendarClock },
          { to: '/izin-siswa', label: 'Izin Siswa', icon: FileText },
        ],
      },
      {
        title: 'Info',
        items: [
          { to: '/pengumuman', label: 'Pengumuman', icon: Megaphone },
          { to: '/tagihan', label: 'Tagihan', icon: CreditCard },
          { to: '/notifikasi', label: 'Notifikasi', icon: Bell },
          { to: '/profil', label: 'Profil', icon: User },
        ],
      },
    ];
  }

  if (me.role === 'siswa') {
    return [
      { items: SISWA_NAV },
      {
        title: 'Menu',
        items: [
          { to: '/izin-saya', label: 'Izin Saya', icon: FileText },
          { to: '/izin-keluar', label: 'Izin Keluar', icon: DoorOpen },
          { to: '/terlambat', label: 'Terlambat', icon: Clock },
          { to: '/nilai-saya', label: 'Nilai Saya', icon: Award },
          { to: '/bintang-saya', label: 'Bintang', icon: GraduationCap },
          { to: '/pelanggaran-saya', label: 'Pelanggaran Saya', icon: ShieldAlert },
          { to: '/checkin', label: 'Check-in', icon: ScanLine },
        ],
      },
    ];
  }

  if (me.role === 'orang_tua') {
    return [
      { items: ORANG_TUA_NAV },
      {
        title: 'Menu',
        items: [
          { to: '/izin-saya', label: 'Izin Anak', icon: FileText },
          { to: '/nilai-saya', label: 'Nilai Anak', icon: Award },
          { to: '/pelanggaran-saya', label: 'Pelanggaran', icon: ShieldAlert },
        ],
      },
    ];
  }

  if (me.role === 'pegawai') {
    return [{ items: PEGAWAI_NAV }];
  }

  // Guru (default) — bottom nav + kartu beranda yang selama ini bukan nav.
  return [
    { items: GURU_NAV },
    {
      title: 'Menu',
      items: [
        { to: '/jadwal', label: 'Jadwal', icon: CalendarDays },
        { to: '/jurnal', label: 'Jurnal', icon: BookOpen },
        { to: '/scan', label: 'Scan QR', icon: ScanLine },
        { to: '/qr-saya', label: 'QR Saya', icon: QrCode },
        { to: '/kedisiplinan', label: 'Kedisiplinan', icon: ShieldAlert },
        { to: '/nilai', label: 'Nilai', icon: Award },
        { to: '/izin-siswa', label: 'Izin Siswa', icon: FileText },
        { to: '/konseling', label: 'Konseling', icon: MessageCircle },
        { to: '/pengganti', label: 'Guru Pengganti', icon: RefreshCw },
      ],
    },
  ];
}
