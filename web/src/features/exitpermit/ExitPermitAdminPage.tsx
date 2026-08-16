import { useState } from 'react';
import { LogOut } from 'lucide-react';
import { Button } from '../../components/ui/Button';
import { Dialog } from '../../components/ui/Dialog';
import { Field, Input, Textarea } from '../../components/ui/Field';
import { ListRow } from '../../components/ui/ListRow';
import { Skeleton } from '../../components/ui/Skeleton';
import { EmptyState } from '../../components/ui/EmptyState';
import { ErrorState } from '../../components/ui/ErrorState';
import { Tag } from '../../components/ui/Tag';
import { useToast } from '../../components/ui/Toast';
import { ApiError } from '../../lib/api';
import { formatDateTime, todayISODate } from '../../lib/date';
import { useMe } from '../auth/api';
import { useExitPermits, useRejectExitPermit, type ExitPermitScope } from './api';
import {
  EXIT_PERMIT_STAGE_LABEL,
  EXIT_PERMIT_STAGE_ORDER,
  EXIT_PERMIT_STATUS_LABEL,
  EXIT_PERMIT_STATUS_TAG_VARIANT,
  exitPermitStageDecision,
} from './format';
import type { ExitPermit } from '../../lib/types';

/** Ringkasan 4 tahap sebagai titik kecil (bukan stepper penuh — "trail ringkas" per baris daftar admin). */
function StageTrailDots({ permit }: { permit: ExitPermit }) {
  return (
    <span className="inline-flex items-center gap-1">
      {EXIT_PERMIT_STAGE_ORDER.map((field) => {
        const decision = exitPermitStageDecision(permit, field);
        const colorClass = decision === 'approved' ? 'bg-st-hadir' : decision === 'rejected' ? 'bg-st-alpa' : 'bg-line';
        return (
          <span
            key={field}
            title={EXIT_PERMIT_STAGE_LABEL[field]}
            aria-label={`${EXIT_PERMIT_STAGE_LABEL[field]}: ${decision === 'approved' ? 'selesai' : decision === 'rejected' ? 'ditolak' : 'belum'}`}
            className={`h-1.5 w-1.5 rounded-full ${colorClass}`}
          />
        );
      })}
    </span>
  );
}

/**
 * Tab "Dispensasi Keluar" di `/izin-siswa/dispensasi` (`StudentLeaveAdminLayout`)
 * — pengawasan dispensasi keluar sekolah pada satu tanggal. Admin/kepsek
 * melihat SELURUH sekolah (`scope=all`, butuh `student:manage`); guru melihat
 * yang aktif hari itu (`scope=active`, butuh `student:read`, guru SUDAH
 * punya) — GAP 3 Fase 15: guru piket/pengajar/BK/pimpinan yang sedang
 * menjadi approver tahap berjalan juga perlu bisa menolak dari sini, bukan
 * cuma lewat scan QR mereka sendiri.
 *
 * Tombol "Tolak" SELALU tampil untuk item pending (bukan admin-only lagi) —
 * otorisasi sebenarnya diputuskan backend per tahap; kalau aktor BUKAN
 * approver sah tahap berjalan, `POST .../reject` membalas 403 dan
 * ditampilkan sebagai Toast, bukan disembunyikan di UI (guru tidak selalu
 * tahu di muka apakah dialah approver tahap ini tanpa mencoba).
 * Komentar alasan kini WAJIB diisi (docs tugas GAP 3) — docs/12-sion-parity.md
 * Gelombang B alur 2.
 */
export function ExitPermitAdminPage() {
  const { data: me } = useMe();
  const { showToast } = useToast();
  const [date, setDate] = useState(todayISODate());

  const isStaffAdmin = me?.role === 'admin_sekolah' || me?.role === 'kepala_sekolah';
  const isGuru = me?.role === 'guru';
  const scope: ExitPermitScope = isStaffAdmin ? 'all' : 'active';
  const { data: items, isLoading, isError, refetch } = useExitPermits(scope, date, Boolean(isStaffAdmin || isGuru));

  const rejectMutation = useRejectExitPermit();
  const [rejectTarget, setRejectTarget] = useState<ExitPermit | null>(null);
  const [rejectComment, setRejectComment] = useState('');

  function openReject(permit: ExitPermit) {
    setRejectTarget(permit);
    setRejectComment('');
  }

  function handleRejectConfirm() {
    if (!rejectTarget) return;
    const comment = rejectComment.trim();
    if (!comment) return;
    rejectMutation.mutate(
      { id: rejectTarget.id, input: { comment } },
      {
        onSuccess: () => {
          showToast('Dispensasi keluar ditolak.');
          setRejectTarget(null);
        },
        onError: (err) => {
          const message =
            err instanceof ApiError && err.status === 403
              ? 'Anda bukan petugas tahap ini.'
              : err instanceof ApiError
                ? err.message
                : 'Gagal menolak dispensasi keluar.';
          showToast(message, 'error');
        },
      },
    );
  }

  return (
    <div className="flex flex-col gap-4">
      <Field label="Tanggal" htmlFor="exit-permit-admin-date" className="max-w-[200px]">
        <Input id="exit-permit-admin-date" type="date" value={date} onChange={(e) => setDate(e.target.value)} />
      </Field>

      {isLoading ? (
        <div className="flex flex-col gap-2">
          <Skeleton className="h-20 w-full" />
          <Skeleton className="h-20 w-full" />
        </div>
      ) : isError ? (
        <ErrorState message="Gagal memuat dispensasi keluar." onRetry={() => refetch()} />
      ) : !items || items.length === 0 ? (
        <EmptyState icon={LogOut} message="Belum ada dispensasi keluar pada tanggal ini." />
      ) : (
        <div>
          {items.map((item) => (
            <ListRow
              key={item.id}
              className="min-h-[64px] items-start py-3"
              title={item.student.name}
              subtitle={
                <span className="flex flex-col gap-1">
                  <span className="num">
                    {item.student.nis} · {item.student.class_name} · {formatDateTime(item.created_at)}
                  </span>
                  <span className="truncate">{item.reason}</span>
                  <StageTrailDots permit={item} />
                </span>
              }
              trailing={
                <div className="flex flex-col items-end gap-2">
                  <Tag variant={EXIT_PERMIT_STATUS_TAG_VARIANT[item.status]}>{EXIT_PERMIT_STATUS_LABEL[item.status]}</Tag>
                  {item.status.startsWith('pending_') && (
                    <Button variant="danger" onClick={() => openReject(item)}>
                      Tolak
                    </Button>
                  )}
                </div>
              }
            />
          ))}
        </div>
      )}

      <Dialog open={rejectTarget !== null} onClose={() => setRejectTarget(null)} title="Tolak dispensasi keluar?">
        <div className="flex flex-col gap-3">
          <p className="text-[14px] text-ink">Dispensasi keluar {rejectTarget?.student.name} akan ditolak.</p>
          <Field label="Alasan" htmlFor="exit-permit-reject-comment" hint="Wajib diisi — siswa akan melihat alasan ini.">
            <Textarea
              id="exit-permit-reject-comment"
              value={rejectComment}
              onChange={(e) => setRejectComment(e.target.value)}
              rows={2}
              placeholder="Jelaskan alasan penolakan…"
              required
            />
          </Field>
        </div>
        <div className="mt-4 flex justify-end gap-2">
          <Button variant="secondary" onClick={() => setRejectTarget(null)}>
            Batal
          </Button>
          <Button
            variant="danger"
            loading={rejectMutation.isPending}
            disabled={!rejectComment.trim()}
            onClick={handleRejectConfirm}
          >
            Tolak
          </Button>
        </div>
      </Dialog>
    </div>
  );
}
