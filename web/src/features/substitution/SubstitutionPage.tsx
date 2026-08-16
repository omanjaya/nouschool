import { useState } from 'react';
import { Navigate, Outlet } from 'react-router-dom';
import { Plus } from 'lucide-react';
import { Button } from '../../components/ui/Button';
import { Tabs, type TabItem } from '../../components/ui/Tabs';
import { useMe } from '../auth/api';
import { RequestSubstitutionDialog } from './RequestSubstitutionDialog';
import { PAGE_WIDE } from '../../components/ui/page';

/**
 * Layout `/pengganti` (pola sama `DisciplinePage`/`StudentLeaveAdminLayout`)
 * — header + tombol "Ajukan" + Tabs "Pengajuan Saya" · "Untuk Saya" ·
 * "Semua" (admin only), konten tiap tab lewat `<Outlet/>`.
 */
export function SubstitutionPage() {
  const { data: me } = useMe();
  const [requestOpen, setRequestOpen] = useState(false);

  if (me && me.role !== 'guru' && me.role !== 'admin_sekolah') {
    return <Navigate to="/" replace />;
  }

  const isAdmin = me?.role === 'admin_sekolah';
  const canRequest = me?.role === 'guru';

  const tabs: TabItem[] = [
    ...(canRequest
      ? [
          { to: '/pengganti', label: 'Pengajuan Saya', end: true },
          { to: '/pengganti/untuk-saya', label: 'Untuk Saya' },
        ]
      : []),
    ...(isAdmin ? [{ to: '/pengganti/semua', label: 'Semua' }] : []),
  ];

  return (
    <div className={`${PAGE_WIDE} flex flex-col gap-5`}>
      <div className="flex items-start justify-between gap-3">
        <div>
          <p className="text-[11px] font-semibold uppercase tracking-[0.1em] text-muted">Jadwal</p>
          <h1 className="text-[21px] font-semibold text-ink">Guru Pengganti</h1>
        </div>
        {canRequest && (
          <Button onClick={() => setRequestOpen(true)} className="shrink-0">
            <Plus size={16} strokeWidth={2} aria-hidden="true" />
            Ajukan
          </Button>
        )}
      </div>

      <Tabs items={tabs} />

      <div className="pb-6">
        <Outlet />
      </div>

      <RequestSubstitutionDialog open={requestOpen} onClose={() => setRequestOpen(false)} />
    </div>
  );
}
