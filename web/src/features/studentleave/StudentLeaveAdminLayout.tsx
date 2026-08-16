import { Navigate, Outlet } from 'react-router-dom';
import { Tabs, type TabItem } from '../../components/ui/Tabs';
import { useMe } from '../auth/api';

const ADMIN_TABS: TabItem[] = [
  { to: '/izin-siswa', label: 'Surat Izin', end: true },
  { to: '/izin-siswa/dispensasi', label: 'Dispensasi Keluar' },
  { to: '/izin-siswa/terlambat', label: 'Terlambat', end: true },
];

/**
 * Layout `/izin-siswa` (pola sama `DisciplinePage`) — header + Tabs "Surat
 * Izin" (izin terencana, Fase 14 Gelombang B1) · "Dispensasi Keluar" ·
 * "Terlambat" (keduanya Gelombang B2), konten tiap tab lewat `<Outlet/>`.
 * Tab kedua & ketiga HANYA untuk admin/kepsek yang mengawasi seluruh sekolah
 * (`scope=all`) — guru hanya melihat tab tunggal "Surat Izin" (antrian review
 * miliknya sendiri, `scope=queue`), jadi Tabs disembunyikan untuk peran ini.
 */
export function StudentLeaveAdminLayout() {
  const { data: me } = useMe();

  if (me && me.role !== 'guru' && me.role !== 'admin_sekolah' && me.role !== 'kepala_sekolah') {
    return <Navigate to="/" replace />;
  }

  const isStaffAdmin = me?.role === 'admin_sekolah' || me?.role === 'kepala_sekolah';

  return (
    <div className="mx-auto flex max-w-[640px] flex-col gap-5 px-5 pt-6 lg:max-w-[1000px]">
      <div>
        <p className="text-[11px] font-semibold uppercase tracking-[0.1em] text-muted">Izin</p>
        <h1 className="text-[21px] font-semibold text-ink">Izin Siswa</h1>
      </div>

      {isStaffAdmin && <Tabs items={ADMIN_TABS} />}

      <div className="pb-6">
        <Outlet />
      </div>
    </div>
  );
}
