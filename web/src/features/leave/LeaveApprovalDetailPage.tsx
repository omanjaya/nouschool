import { useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { AppBar } from '../../components/ui/AppBar';
import { Button } from '../../components/ui/Button';
import { Dialog } from '../../components/ui/Dialog';
import { Textarea } from '../../components/ui/Field';
import { Skeleton } from '../../components/ui/Skeleton';
import { ErrorState } from '../../components/ui/ErrorState';
import { useToast } from '../../components/ui/Toast';
import { ApiError } from '../../lib/api';
import { useDecideLeaveApproval, useLeaveApprovals } from './api';
import { LeaveRequestDetailBody } from './LeaveRequestDetailBody';

/** /izin/persetujuan/:stepId — detail satu pengajuan dalam antrian + aksi Setujui/Tolak. */
export function LeaveApprovalDetailPage() {
  const { stepId } = useParams<{ stepId: string }>();
  const navigate = useNavigate();
  const { showToast } = useToast();
  const { data: items, isLoading, isError, refetch } = useLeaveApprovals();
  const decide = useDecideLeaveApproval();

  const [rejectOpen, setRejectOpen] = useState(false);
  const [rejectComment, setRejectComment] = useState('');
  const [rejectError, setRejectError] = useState<string | null>(null);

  const item = items?.find((i) => i.step_id === stepId);

  async function handleApprove() {
    if (!stepId) return;
    try {
      await decide.mutateAsync({ stepId, input: { decision: 'approved' } });
      showToast('Pengajuan izin disetujui.');
      navigate('/izin/persetujuan');
    } catch (err) {
      showToast(err instanceof ApiError ? err.message : 'Gagal menyimpan keputusan.', 'error');
    }
  }

  async function handleReject() {
    if (!stepId) return;
    if (!rejectComment.trim()) {
      setRejectError('Komentar wajib diisi saat menolak pengajuan.');
      return;
    }
    try {
      await decide.mutateAsync({ stepId, input: { decision: 'rejected', comment: rejectComment.trim() } });
      showToast('Pengajuan izin ditolak.');
      setRejectOpen(false);
      navigate('/izin/persetujuan');
    } catch (err) {
      showToast(err instanceof ApiError ? err.message : 'Gagal menyimpan keputusan.', 'error');
    }
  }

  return (
    <div className="flex min-h-dvh flex-col">
      <AppBar title="Detail Persetujuan" onBack={() => navigate('/izin/persetujuan')} />

      <div className="mx-auto w-full max-w-[640px] flex-1 px-5 py-5">
        {isLoading ? (
          <div className="flex flex-col gap-3">
            <Skeleton className="h-24 w-full" />
            <Skeleton className="h-14 w-full" />
          </div>
        ) : isError ? (
          <ErrorState message="Gagal memuat detail pengajuan." onRetry={() => refetch()} />
        ) : !item ? (
          <ErrorState message="Pengajuan ini tidak lagi menunggu keputusan Anda." />
        ) : (
          <div className="flex flex-col gap-6">
            <LeaveRequestDetailBody request={item.request} showTeacher />

            <div className="flex gap-2">
              <Button className="flex-1" onClick={handleApprove} loading={decide.isPending}>
                Setujui
              </Button>
              <Button
                variant="danger"
                className="flex-1"
                onClick={() => {
                  setRejectComment('');
                  setRejectError(null);
                  setRejectOpen(true);
                }}
                disabled={decide.isPending}
              >
                Tolak
              </Button>
            </div>
          </div>
        )}
      </div>

      <Dialog open={rejectOpen} onClose={() => setRejectOpen(false)} title="Tolak pengajuan izin">
        <div className="flex flex-col gap-3">
          <p className="text-[14px] text-ink">Jelaskan alasan penolakan — akan dikirim ke guru yang mengajukan.</p>
          <Textarea
            value={rejectComment}
            onChange={(e) => setRejectComment(e.target.value)}
            placeholder="Mis. bertabrakan dengan jadwal ujian…"
            rows={3}
            autoFocus
          />
          {rejectError && <p className="text-[12px] text-danger">{rejectError}</p>}
          <div className="mt-1 flex justify-end gap-2">
            <Button variant="secondary" onClick={() => setRejectOpen(false)}>
              Batal
            </Button>
            <Button variant="danger" onClick={handleReject} loading={decide.isPending}>
              Tolak Pengajuan
            </Button>
          </div>
        </div>
      </Dialog>
    </div>
  );
}
