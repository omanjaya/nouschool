import { useState } from 'react';
import { Navigate, useNavigate } from 'react-router-dom';
import { CalendarDays, ChevronRight, FileBarChart, Plus } from 'lucide-react';
import { ListRow } from '../../components/ui/ListRow';
import { Skeleton } from '../../components/ui/Skeleton';
import { EmptyState } from '../../components/ui/EmptyState';
import { ErrorState } from '../../components/ui/ErrorState';
import { Tag } from '../../components/ui/Tag';
import { Button } from '../../components/ui/Button';
import { SegmentedControl, type SegmentedOption } from '../../components/ui/SegmentedControl';
import { formatDateRange } from '../../lib/date';
import { useMe } from '../auth/api';
import { useLeaveApprovals, useLeaveRequests } from './api';
import { LeaveRequestFormDialog } from './LeaveRequestFormDialog';
import { LEAVE_STATUS_LABEL, LEAVE_STATUS_TAG_VARIANT } from './format';
import type { LeaveRequestStatus } from '../../lib/types';

type StatusFilter = '' | LeaveRequestStatus;

const STATUS_OPTIONS: SegmentedOption<'all' | 'pending' | 'approved' | 'rejected'>[] = [
  { value: 'all', label: 'Semua' },
  { value: 'pending', label: 'Menunggu' },
  { value: 'approved', label: 'Disetujui' },
  { value: 'rejected', label: 'Ditolak' },
];

/** /izin — pengajuan izin guru: daftar milik sendiri + antrian persetujuan bila berlaku. */
export function LeavePage() {
  const { data: me } = useMe();
  const navigate = useNavigate();
  const [filter, setFilter] = useState<'all' | 'pending' | 'approved' | 'rejected'>('all');
  const [formOpen, setFormOpen] = useState(false);

  const statusParam: StatusFilter = filter === 'all' ? '' : filter;
  const { data: items, isLoading, isError, refetch } = useLeaveRequests('mine', statusParam || undefined);
  const { data: approvals } = useLeaveApprovals();

  if (me && (me.role === 'siswa' || me.role === 'orang_tua')) {
    return <Navigate to="/" replace />;
  }

  const isApproverRole = me?.role === 'kepala_sekolah' || me?.role === 'admin_sekolah';
  const showApprovalsSection = isApproverRole || (approvals && approvals.length > 0);
  const canViewRecap = me?.role === 'kepala_sekolah' || me?.role === 'admin_sekolah';

  return (
    <div className="mx-auto flex max-w-[640px] flex-col gap-6 px-5 py-6">
      <div className="flex items-start justify-between gap-3">
        <div>
          <p className="text-[11px] font-semibold uppercase tracking-[0.1em] text-muted">Izin</p>
          <h1 className="text-[21px] font-semibold text-ink">Pengajuan Izin</h1>
        </div>
        {canViewRecap && (
          <Button variant="secondary" onClick={() => navigate('/izin/rekap')}>
            <FileBarChart size={16} strokeWidth={2} aria-hidden="true" />
            Rekap
          </Button>
        )}
      </div>

      <Button onClick={() => setFormOpen(true)} className="self-start">
        <Plus size={16} strokeWidth={2} aria-hidden="true" />
        Ajukan Izin
      </Button>

      {showApprovalsSection && (
        <div>
          <p className="mb-1 text-[11px] font-semibold uppercase tracking-[0.1em] text-muted">Persetujuan</p>
          <ListRow
            title="Antrian Persetujuan"
            subtitle={
              approvals && approvals.length > 0
                ? `${approvals.length} pengajuan menunggu keputusan Anda`
                : 'Tidak ada pengajuan menunggu saat ini'
            }
            trailing={<ChevronRight size={18} strokeWidth={2} className="text-muted" aria-hidden="true" />}
            onClick={() => navigate('/izin/persetujuan')}
          />
        </div>
      )}

      <div className="flex flex-col gap-3">
        <p className="text-[11px] font-semibold uppercase tracking-[0.1em] text-muted">Pengajuan Saya</p>
        <SegmentedControl options={STATUS_OPTIONS} value={filter} onChange={setFilter} />

        {isLoading ? (
          <div className="flex flex-col gap-2">
            <Skeleton className="h-16 w-full" />
            <Skeleton className="h-16 w-full" />
          </div>
        ) : isError ? (
          <ErrorState message="Gagal memuat pengajuan izin." onRetry={() => refetch()} />
        ) : !items || items.length === 0 ? (
          <EmptyState icon={CalendarDays} message="Belum ada pengajuan izin." />
        ) : (
          <div>
            {items.map((item) => (
              <ListRow
                key={item.id}
                className="min-h-[56px]"
                title={item.type_label}
                subtitle={
                  <span className="flex flex-col gap-0.5">
                    <span className="num">
                      {formatDateRange(item.date_start, item.date_end)} · {item.days} hari
                    </span>
                    <span className="truncate">{item.reason}</span>
                  </span>
                }
                trailing={<Tag variant={LEAVE_STATUS_TAG_VARIANT[item.status]}>{LEAVE_STATUS_LABEL[item.status]}</Tag>}
                onClick={() => navigate(`/izin/${item.id}`)}
              />
            ))}
          </div>
        )}
      </div>

      <LeaveRequestFormDialog open={formOpen} onClose={() => setFormOpen(false)} onSubmitted={() => setFormOpen(false)} />
    </div>
  );
}
