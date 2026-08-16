import { useState } from 'react';
import { Navigate, Outlet, useNavigate } from 'react-router-dom';
import { Plus, Settings } from 'lucide-react';
import { Button } from '../../components/ui/Button';
import { Tabs, type TabItem } from '../../components/ui/Tabs';
import { useMe } from '../auth/api';
import { RecordViolationDialog } from './RecordViolationDialog';

const TABS: TabItem[] = [
  { to: '/kedisiplinan', label: 'Catatan', end: true },
  { to: '/kedisiplinan/rekap', label: 'Rekap' },
  { to: '/kedisiplinan/surat', label: 'Surat Peringatan' },
];

/**
 * Layout `/kedisiplinan` — hanya admin/kepsek/guru (docs/12-sion-parity.md
 * Gelombang A). Header + tombol "Catat Pelanggaran" (guru & admin) + icon
 * Pengaturan (admin only) + tab Catatan/Rekap/Surat Peringatan, konten
 * masing-masing tab lewat `<Outlet/>`.
 */
export function DisciplinePage() {
  const { data: me } = useMe();
  const navigate = useNavigate();
  const [recordOpen, setRecordOpen] = useState(false);

  if (me && me.role !== 'admin_sekolah' && me.role !== 'kepala_sekolah' && me.role !== 'guru') {
    return <Navigate to="/" replace />;
  }

  const canRecord = me?.role === 'admin_sekolah' || me?.role === 'guru';
  const isAdmin = me?.role === 'admin_sekolah';

  return (
    <div className="mx-auto flex max-w-[640px] flex-col gap-5 px-5 pt-6 lg:max-w-[1000px]">
      <div className="flex items-start justify-between gap-3">
        <div>
          <p className="text-[11px] font-semibold uppercase tracking-[0.1em] text-muted">Kedisiplinan</p>
          <h1 className="text-[21px] font-semibold text-ink">Kedisiplinan</h1>
        </div>
        {isAdmin && (
          <button
            type="button"
            onClick={() => navigate('/kedisiplinan/pengaturan')}
            aria-label="Pengaturan Kedisiplinan"
            className="flex h-11 w-11 shrink-0 items-center justify-center rounded-lg border border-line text-muted transition-colors duration-150 hover:bg-surface-2 hover:text-ink"
          >
            <Settings size={20} strokeWidth={2} aria-hidden="true" />
          </button>
        )}
      </div>

      {canRecord && (
        <Button onClick={() => setRecordOpen(true)} className="self-start">
          <Plus size={16} strokeWidth={2} aria-hidden="true" />
          Catat Pelanggaran
        </Button>
      )}

      <Tabs items={TABS} />

      <div className="pb-6">
        <Outlet />
      </div>

      <RecordViolationDialog open={recordOpen} onClose={() => setRecordOpen(false)} />
    </div>
  );
}
