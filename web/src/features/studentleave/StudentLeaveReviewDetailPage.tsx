import { useState } from 'react';
import { useLocation, useNavigate, useParams } from 'react-router-dom';
import { AppBar } from '../../components/ui/AppBar';
import { Button } from '../../components/ui/Button';
import { Dialog } from '../../components/ui/Dialog';
import { Textarea } from '../../components/ui/Field';
import { Skeleton } from '../../components/ui/Skeleton';
import { ErrorState } from '../../components/ui/ErrorState';
import { useToast } from '../../components/ui/Toast';
import { ApiError } from '../../lib/api';
import { useIssueStudentLeaveRequest, useReviewStudentLeaveRequest, useStudentLeaveRequests, type StudentLeaveScope } from './api';
import { StudentLeaveDetailBody } from './StudentLeaveDetailBody';

/**
 * /izin-siswa/:id — detail satu pengajuan + aksi Setujui/Tolak. Endpoint
 * dipilih dari status saat ini: `pending_homeroom` → POST .../review (tahap
 * wali kelas), `pending_bk` → POST .../issue (tahap BK, menerbitkan surat
 * bila disetujui). Backend yang menegakkan siapa berhak memutuskan lewat
 * duty-capability flags — frontend hanya menampilkan tombol berdasar status.
 */
export function StudentLeaveReviewDetailPage() {
  const { id } = useParams<{ id: string }>();
  const location = useLocation();
  const navigate = useNavigate();
  const { showToast } = useToast();

  const scope: StudentLeaveScope = (location.state as { scope?: StudentLeaveScope } | null)?.scope ?? 'queue';
  const { data: items, isLoading, isError, refetch } = useStudentLeaveRequests(scope);
  const review = useReviewStudentLeaveRequest();
  const issue = useIssueStudentLeaveRequest();

  const [rejectOpen, setRejectOpen] = useState(false);
  const [rejectComment, setRejectComment] = useState('');
  const [rejectError, setRejectError] = useState<string | null>(null);

  const item = items?.find((i) => i.id === id);
  const decide = item?.status === 'pending_bk' ? issue : review;
  const isPending = decide.isPending;

  async function handleApprove() {
    if (!id) return;
    try {
      await decide.mutateAsync({ id, input: { decision: 'approve' } });
      showToast('Pengajuan izin disetujui.');
      navigate('/izin-siswa');
    } catch (err) {
      showToast(err instanceof ApiError ? err.message : 'Gagal menyimpan keputusan.', 'error');
    }
  }

  async function handleReject() {
    if (!id) return;
    if (!rejectComment.trim()) {
      setRejectError('Komentar wajib diisi saat menolak pengajuan.');
      return;
    }
    try {
      await decide.mutateAsync({ id, input: { decision: 'reject', comment: rejectComment.trim() } });
      showToast('Pengajuan izin ditolak.');
      setRejectOpen(false);
      navigate('/izin-siswa');
    } catch (err) {
      showToast(err instanceof ApiError ? err.message : 'Gagal menyimpan keputusan.', 'error');
    }
  }

  const canDecide = item?.status === 'pending_homeroom' || item?.status === 'pending_bk';

  return (
    <div className="flex min-h-dvh flex-col">
      <AppBar title="Detail Izin Siswa" onBack={() => navigate('/izin-siswa')} />

      <div className="mx-auto w-full max-w-[640px] flex-1 px-5 py-5">
        {isLoading ? (
          <div className="flex flex-col gap-3">
            <Skeleton className="h-24 w-full" />
            <Skeleton className="h-14 w-full" />
          </div>
        ) : isError ? (
          <ErrorState message="Gagal memuat detail pengajuan." onRetry={() => refetch()} />
        ) : !item ? (
          <ErrorState message="Pengajuan ini tidak ditemukan." />
        ) : (
          <div className="flex flex-col gap-6">
            <StudentLeaveDetailBody request={item} showStudent />

            {canDecide && (
              <div className="flex gap-2">
                <Button className="flex-1" onClick={handleApprove} loading={isPending}>
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
                  disabled={isPending}
                >
                  Tolak
                </Button>
              </div>
            )}
          </div>
        )}
      </div>

      <Dialog open={rejectOpen} onClose={() => setRejectOpen(false)} title="Tolak pengajuan izin">
        <div className="flex flex-col gap-3">
          <p className="text-[14px] text-ink">Jelaskan alasan penolakan — akan dikirim ke siswa/orang tua yang mengajukan.</p>
          <Textarea
            value={rejectComment}
            onChange={(e) => setRejectComment(e.target.value)}
            placeholder="Mis. dokumen kurang lengkap…"
            rows={3}
            autoFocus
          />
          {rejectError && <p className="text-[12px] text-danger">{rejectError}</p>}
          <div className="mt-1 flex justify-end gap-2">
            <Button variant="secondary" onClick={() => setRejectOpen(false)}>
              Batal
            </Button>
            <Button variant="danger" onClick={handleReject} loading={isPending}>
              Tolak Pengajuan
            </Button>
          </div>
        </div>
      </Dialog>
    </div>
  );
}
