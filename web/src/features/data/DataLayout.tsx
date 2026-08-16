import { Outlet } from 'react-router-dom';
import { ShieldAlert } from 'lucide-react';
import { Skeleton } from '../../components/ui/Skeleton';
import { EmptyState } from '../../components/ui/EmptyState';
import { Tabs } from '../../components/ui/Tabs';
import { useMe } from '../auth/api';

const TABS = [
  { to: '/data/siswa', label: 'Siswa' },
  { to: '/data/rombel', label: 'Rombel' },
  { to: '/data/guru', label: 'Guru' },
  { to: '/data/pegawai', label: 'Pegawai' },
  { to: '/data/mapel', label: 'Mapel' },
  { to: '/data/jadwal', label: 'Jadwal' },
  { to: '/data/jam', label: 'Jam' },
  { to: '/data/ruangan', label: 'Ruangan' },
  { to: '/data/tugas', label: 'Tugas' },
];

/** Hub /data — tab Siswa/Rombel/Guru/Mapel, hanya untuk admin_sekolah. */
export function DataLayout() {
  const { data: me, isLoading } = useMe();

  if (isLoading) {
    return (
      <div className="mx-auto flex max-w-[640px] flex-col gap-4 px-5 py-6">
        <Skeleton className="h-6 w-40" />
        <Skeleton className="h-24 w-full" />
      </div>
    );
  }

  if (!me || me.role !== 'admin_sekolah') {
    return (
      <div className="mx-auto max-w-[640px] px-5 py-6">
        <EmptyState icon={ShieldAlert} message="Anda tidak memiliki akses ke halaman ini." />
      </div>
    );
  }

  return (
    <div className="mx-auto max-w-[640px] px-5 pt-6 lg:max-w-[900px]">
      <div className="mb-4">
        <p className="text-[11px] font-semibold uppercase tracking-[0.1em] text-muted">Sekolah</p>
        <h1 className="text-[21px] font-semibold text-ink">Data</h1>
      </div>
      <Tabs items={TABS} />
      <div className="py-6">
        <Outlet />
      </div>
    </div>
  );
}
