import { useState } from 'react';
import { Navigate } from 'react-router-dom';
import { Repeat } from 'lucide-react';
import { useMe } from '../auth/api';
import { ListRow } from '../../components/ui/ListRow';
import { Tag } from '../../components/ui/Tag';
import { Button } from '../../components/ui/Button';
import { Dialog } from '../../components/ui/Dialog';
import { Skeleton } from '../../components/ui/Skeleton';
import { EmptyState } from '../../components/ui/EmptyState';
import { ErrorState } from '../../components/ui/ErrorState';
import { useToast } from '../../components/ui/Toast';
import { ApiError } from '../../lib/api';
import { formatDate } from '../../lib/date';
import { DAY_LABELS } from '../schedule/format';
import { useAcceptSubstitution, useCancelSubstitution, useRejectSubstitution, useSubstitutions, type SubstitutionScope } from './api';
import type { SubstitutionRequest, SubstitutionStatus } from '../../lib/types';

const STATUS_LABEL: Record<SubstitutionStatus, string> = {
  pending: 'Menunggu',
  accepted: 'Diterima',
  rejected: 'Ditolak',
  canceled: 'Dibatalkan',
};

const STATUS_VARIANT: Record<SubstitutionStatus, 'warning' | 'success' | 'danger' | 'neutral'> = {
  pending: 'warning',
  accepted: 'success',
  rejected: 'danger',
  canceled: 'neutral',
};

function slotLabel(slot: SubstitutionRequest['slot']): string {
  const periodLabel = slot.period_start === slot.period_end ? `${slot.period_start}` : `${slot.period_start}-${slot.period_end}`;
  return `${DAY_LABELS[slot.day_of_week]} jam ${periodLabel} · ${slot.subject_name} · ${slot.class_name}`;
}

/**
 * Daftar pengajuan guru pengganti untuk satu `scope` — dipakai ketiga tab
 * `/pengganti` (mine/for-me/all, lihat `SubstitutionPage`). Aksi berbeda per
 * scope: `for-me` pending → Terima/Tolak (pengganti yang diajukan), `mine`
 * pending → Batalkan (pengaju).
 */
export function SubstitutionList({ scope }: { scope: SubstitutionScope }) {
  const { showToast } = useToast();
  const { data: items, isLoading, isError, refetch } = useSubstitutions(scope);
  const accept = useAcceptSubstitution();
  const reject = useRejectSubstitution();
  const cancel = useCancelSubstitution();

  const [rejecting, setRejecting] = useState<SubstitutionRequest | undefined>();
  const [canceling, setCanceling] = useState<SubstitutionRequest | undefined>();

  function handleAccept(item: SubstitutionRequest) {
    accept.mutate(item.id, {
      onSuccess: () => showToast('Pengajuan guru pengganti diterima.'),
      onError: (err) => showToast(err instanceof ApiError ? err.message : 'Gagal menerima pengajuan.', 'error'),
    });
  }

  function handleReject() {
    if (!rejecting) return;
    reject.mutate(rejecting.id, {
      onSuccess: () => {
        setRejecting(undefined);
        showToast('Pengajuan guru pengganti ditolak.');
      },
      onError: (err) => showToast(err instanceof ApiError ? err.message : 'Gagal menolak pengajuan.', 'error'),
    });
  }

  function handleCancel() {
    if (!canceling) return;
    cancel.mutate(canceling.id, {
      onSuccess: () => {
        setCanceling(undefined);
        showToast('Pengajuan guru pengganti dibatalkan.');
      },
      onError: (err) => showToast(err instanceof ApiError ? err.message : 'Gagal membatalkan pengajuan.', 'error'),
    });
  }

  if (isLoading) {
    return (
      <div className="flex flex-col gap-2">
        <Skeleton className="h-16 w-full" />
        <Skeleton className="h-16 w-full" />
      </div>
    );
  }

  if (isError) {
    return <ErrorState message="Gagal memuat pengajuan guru pengganti." onRetry={() => refetch()} />;
  }

  if (!items || items.length === 0) {
    const message =
      scope === 'mine'
        ? 'Belum ada pengajuan guru pengganti dari Anda.'
        : scope === 'for-me'
          ? 'Belum ada pengajuan yang meminta Anda sebagai pengganti.'
          : 'Belum ada pengajuan guru pengganti.';
    return <EmptyState icon={Repeat} message={message} />;
  }

  return (
    <>
      <div>
        {items.map((item) => (
          <ListRow
            key={item.id}
            className="min-h-[64px] items-start"
            title={slotLabel(item.slot)}
            subtitle={
              <span className="flex flex-col gap-0.5">
                <span className="truncate">{formatDate(item.date)}</span>
                <span className="truncate">
                  {scope === 'mine' ? `Pengganti: ${item.substitute_name}` : `Diajukan oleh ${item.requested_by_name}`}
                </span>
              </span>
            }
            subtitleClassName="block text-[12px] text-muted"
            trailing={
              <div className="flex flex-col items-end gap-1.5">
                <Tag variant={STATUS_VARIANT[item.status]}>{STATUS_LABEL[item.status]}</Tag>
                {scope === 'for-me' && item.status === 'pending' && (
                  <div className="flex gap-1.5">
                    <Button onClick={() => handleAccept(item)} loading={accept.isPending}>
                      Terima
                    </Button>
                    <Button variant="danger" onClick={() => setRejecting(item)}>
                      Tolak
                    </Button>
                  </div>
                )}
                {scope === 'mine' && item.status === 'pending' && (
                  <Button variant="secondary" onClick={() => setCanceling(item)}>
                    Batalkan
                  </Button>
                )}
              </div>
            }
          />
        ))}
      </div>

      <Dialog open={rejecting !== undefined} onClose={() => setRejecting(undefined)} title="Tolak pengajuan?">
        <p className="text-[14px] text-ink">
          Anda akan menolak menjadi pengganti untuk {rejecting ? slotLabel(rejecting.slot) : ''} pada{' '}
          {rejecting ? formatDate(rejecting.date) : ''}.
        </p>
        <div className="mt-4 flex justify-end gap-2">
          <Button variant="secondary" onClick={() => setRejecting(undefined)}>
            Batal
          </Button>
          <Button variant="danger" loading={reject.isPending} onClick={handleReject}>
            Tolak
          </Button>
        </div>
      </Dialog>

      <Dialog open={canceling !== undefined} onClose={() => setCanceling(undefined)} title="Batalkan pengajuan?">
        <p className="text-[14px] text-ink">
          Pengajuan guru pengganti untuk {canceling ? slotLabel(canceling.slot) : ''} pada{' '}
          {canceling ? formatDate(canceling.date) : ''} akan dibatalkan.
        </p>
        <div className="mt-4 flex justify-end gap-2">
          <Button variant="secondary" onClick={() => setCanceling(undefined)}>
            Tutup
          </Button>
          <Button variant="danger" loading={cancel.isPending} onClick={handleCancel}>
            Batalkan Pengajuan
          </Button>
        </div>
      </Dialog>
    </>
  );
}

/**
 * Rute index `/pengganti` — guru melihat "Pengajuan Saya" langsung; admin
 * (tidak mengajar, tanpa tab "Pengajuan Saya"/"Untuk Saya") dialihkan ke tab
 * "Semua" supaya index tidak pernah menampilkan list scope `mine` yang selalu
 * kosong untuk peran ini.
 */
export function SubstitutionIndexPage() {
  const { data: me } = useMe();
  if (me?.role === 'admin_sekolah') return <Navigate to="/pengganti/semua" replace />;
  return <SubstitutionList scope="mine" />;
}

export function SubstitutionMinePage() {
  return <SubstitutionList scope="mine" />;
}

export function SubstitutionForMePage() {
  return <SubstitutionList scope="for-me" />;
}

export function SubstitutionAllPage() {
  return <SubstitutionList scope="all" />;
}
