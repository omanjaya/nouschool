import { useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { AppBar } from '../../components/ui/AppBar';
import { Button } from '../../components/ui/Button';
import { Dialog } from '../../components/ui/Dialog';
import { Skeleton } from '../../components/ui/Skeleton';
import { ErrorState } from '../../components/ui/ErrorState';
import { useToast } from '../../components/ui/Toast';
import { ApiError } from '../../lib/api';
import { useCancelLeaveRequest, useLeaveRequests } from './api';
import { LeaveRequestDetailBody } from './LeaveRequestDetailBody';

/** /izin/:id — detail satu pengajuan izin milik guru sendiri (+ aksi batalkan bila masih pending). */
export function LeaveDetailPage() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const { showToast } = useToast();
  const { data: items, isLoading, isError, refetch } = useLeaveRequests('mine');
  const cancelRequest = useCancelLeaveRequest();
  const [confirmCancelOpen, setConfirmCancelOpen] = useState(false);

  const request = items?.find((r) => r.id === id);

  async function handleCancel() {
    if (!id) return;
    try {
      await cancelRequest.mutateAsync(id);
      setConfirmCancelOpen(false);
      showToast('Pengajuan izin dibatalkan.');
    } catch (err) {
      setConfirmCancelOpen(false);
      showToast(err instanceof ApiError ? err.message : 'Gagal membatalkan pengajuan.', 'error');
    }
  }

  return (
    <div className="flex min-h-dvh flex-col">
      <AppBar title="Detail Izin" onBack={() => navigate('/izin')} />

      <div className="mx-auto w-full max-w-[640px] flex-1 px-5 py-5">
        {isLoading ? (
          <div className="flex flex-col gap-3">
            <Skeleton className="h-24 w-full" />
            <Skeleton className="h-14 w-full" />
            <Skeleton className="h-14 w-full" />
          </div>
        ) : isError ? (
          <ErrorState message="Gagal memuat detail pengajuan izin." onRetry={() => refetch()} />
        ) : !request ? (
          <ErrorState message="Pengajuan izin tidak ditemukan." />
        ) : (
          <div className="flex flex-col gap-6">
            <LeaveRequestDetailBody request={request} />

            {request.status === 'pending' && (
              <Button variant="danger" onClick={() => setConfirmCancelOpen(true)} className="self-start">
                Batalkan Pengajuan
              </Button>
            )}
          </div>
        )}
      </div>

      <Dialog open={confirmCancelOpen} onClose={() => setConfirmCancelOpen(false)} title="Batalkan pengajuan izin?">
        <p className="text-[14px] text-ink">Pengajuan yang dibatalkan tidak bisa diaktifkan kembali.</p>
        <div className="mt-4 flex justify-end gap-2">
          <Button variant="secondary" onClick={() => setConfirmCancelOpen(false)}>
            Batal
          </Button>
          <Button variant="danger" onClick={handleCancel} loading={cancelRequest.isPending}>
            Batalkan Pengajuan
          </Button>
        </div>
      </Dialog>
    </div>
  );
}
