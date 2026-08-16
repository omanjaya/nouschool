import { useEffect, useState, type FormEvent } from 'react';
import { Navigate } from 'react-router-dom';
import { LogOut, QrCode } from 'lucide-react';
import { Card } from '../../components/ui/Card';
import { Button } from '../../components/ui/Button';
import { Dialog } from '../../components/ui/Dialog';
import { Field, Select, Textarea } from '../../components/ui/Field';
import { ListRow } from '../../components/ui/ListRow';
import { Skeleton } from '../../components/ui/Skeleton';
import { EmptyState } from '../../components/ui/EmptyState';
import { ErrorState } from '../../components/ui/ErrorState';
import { Tag } from '../../components/ui/Tag';
import { useToast } from '../../components/ui/Toast';
import { QrImage } from '../../lib/qrcode';
import { ApiError } from '../../lib/api';
import { formatDateTime } from '../../lib/date';
import { useMe } from '../auth/api';
import { TeacherTokenScanSheet } from '../teacherqr/TeacherTokenScanSheet';
import {
  EXIT_PERMIT_GATE_QR_PREFIX,
  useCancelExitPermit,
  useCreateExitPermit,
  useExitPermitChildren,
  useExitPermits,
  useScanExitPermit,
} from './api';
import { EXIT_PERMIT_STATUS_LABEL, EXIT_PERMIT_STATUS_TAG_VARIANT } from './format';
import { ExitPermitStepper } from './ExitPermitStepper';
import type { ExitPermit } from '../../lib/types';

const TERMINAL_STATUSES = ['exited', 'rejected', 'canceled'];

/** Countdown ringkas mm:ss ke `gate_expires_at` — murni tampilan, tidak memicu aksi otomatis apa pun saat habis. */
function GateCountdown({ expiresAt }: { expiresAt: string }) {
  const [remainingMs, setRemainingMs] = useState(0);

  useEffect(() => {
    const target = new Date(expiresAt).getTime();
    function tick() {
      setRemainingMs(Math.max(0, target - Date.now()));
    }
    tick();
    const id = window.setInterval(tick, 1000);
    return () => window.clearInterval(id);
  }, [expiresAt]);

  if (remainingMs <= 0) return <p className="text-[13px] text-danger">Kode gerbang sudah kedaluwarsa.</p>;

  const totalSec = Math.floor(remainingMs / 1000);
  const mm = String(Math.floor(totalSec / 60)).padStart(2, '0');
  const ss = String(totalSec % 60).padStart(2, '0');
  return <p className="num text-[13px] text-muted">Berlaku {mm}:{ss} lagi</p>;
}

function CreatePermitForm() {
  const { showToast } = useToast();
  const create = useCreateExitPermit();
  const [reason, setReason] = useState('');
  const [error, setError] = useState<string | null>(null);

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    if (!reason.trim()) {
      setError('Alasan wajib diisi.');
      return;
    }
    setError(null);
    try {
      await create.mutateAsync({ reason: reason.trim() });
      setReason('');
      showToast('Pengajuan dispensasi keluar terkirim.');
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Gagal mengirim pengajuan.');
    }
  }

  return (
    <Card className="flex flex-col gap-4">
      <div>
        <p className="text-[14px] font-semibold text-ink">Ajukan Dispensasi Keluar</p>
        <p className="text-[12px] text-muted">
          Alasan akan ditinjau piket, guru pengajar jam berjalan, BK, lalu pimpinan secara berurutan.
        </p>
      </div>
      <form onSubmit={handleSubmit} className="flex flex-col gap-3">
        <Field label="Alasan" htmlFor="exit-permit-reason">
          <Textarea
            id="exit-permit-reason"
            value={reason}
            onChange={(e) => setReason(e.target.value)}
            placeholder="Jelaskan alasan keluar…"
            rows={3}
            required
          />
        </Field>
        {error && <p className="text-[12px] text-danger">{error}</p>}
        <Button type="submit" loading={create.isPending} className="self-start">
          Ajukan
        </Button>
      </form>
    </Card>
  );
}

function ActivePermitCard({ permit, readOnly }: { permit: ExitPermit; readOnly: boolean }) {
  const { showToast } = useToast();
  const scan = useScanExitPermit();
  const cancelMutation = useCancelExitPermit();
  const [scanOpen, setScanOpen] = useState(false);
  const [scanError, setScanError] = useState<string | null>(null);
  const [cancelConfirmOpen, setCancelConfirmOpen] = useState(false);

  const isPending = permit.status.startsWith('pending_');
  const isIssued = permit.status === 'issued';
  const isExited = permit.status === 'exited';

  function handleScanSubmit(rawValue: string) {
    setScanError(null);
    scan.mutate(
      { id: permit.id, token: rawValue },
      {
        onSuccess: (result) => {
          setScanOpen(false);
          showToast(`Tahap dikonfirmasi — status: ${EXIT_PERMIT_STATUS_LABEL[result.stage_advanced_to]}.`);
        },
        onError: (err) => {
          setScanError(err instanceof ApiError ? err.message : 'Gagal memindai QR. Coba lagi.');
        },
      },
    );
  }

  function handleCancelConfirm() {
    cancelMutation.mutate(permit.id, {
      onSuccess: () => {
        showToast('Dispensasi keluar dibatalkan.');
        setCancelConfirmOpen(false);
      },
      onError: (err) => {
        showToast(err instanceof ApiError ? err.message : 'Gagal membatalkan.', 'error');
        setCancelConfirmOpen(false);
      },
    });
  }

  return (
    <Card className="flex flex-col gap-4">
      <div>
        <div className="flex items-start justify-between gap-3">
          <p className="text-[14px] font-semibold text-ink">Dispensasi Keluar</p>
          <Tag variant={EXIT_PERMIT_STATUS_TAG_VARIANT[permit.status]}>{EXIT_PERMIT_STATUS_LABEL[permit.status]}</Tag>
        </div>
        <p className="mt-1 text-[13px] text-muted">{permit.reason}</p>
      </div>

      <ExitPermitStepper permit={permit} />

      {isIssued && permit.gate_token && (
        <div className="flex flex-col items-center gap-3 rounded-lg border border-line p-5 text-center">
          <QrImage value={`${EXIT_PERMIT_GATE_QR_PREFIX}${permit.gate_token}`} sizePx={220} />
          <p className="text-[13px] font-medium text-ink">Tunjukkan ke petugas gerbang</p>
          {permit.gate_expires_at && <GateCountdown expiresAt={permit.gate_expires_at} />}
        </div>
      )}

      {isExited && (
        <div className="rounded-lg bg-surface-2 p-4 text-center">
          <p className="text-[13px] text-ink">
            Keluar tercatat{permit.exited_at ? ` ${formatDateTime(permit.exited_at)}` : ''}.
          </p>
        </div>
      )}

      {!readOnly && isPending && (
        <div className="flex flex-wrap gap-2">
          <Button onClick={() => setScanOpen(true)}>
            <QrCode size={16} strokeWidth={2} aria-hidden="true" />
            Scan QR Guru
          </Button>
          <Button variant="secondary" onClick={() => setCancelConfirmOpen(true)}>
            Batalkan
          </Button>
        </div>
      )}

      <TeacherTokenScanSheet
        open={scanOpen}
        onClose={() => setScanOpen(false)}
        onSubmit={handleScanSubmit}
        submitting={scan.isPending}
        errorMessage={scanError}
        instruction="Arahkan kamera ke QR guru untuk tahap berikutnya"
      />

      <Dialog open={cancelConfirmOpen} onClose={() => setCancelConfirmOpen(false)} title="Batalkan dispensasi?">
        <p className="text-[14px] text-ink">Pengajuan dispensasi keluar ini akan dibatalkan.</p>
        <div className="mt-4 flex justify-end gap-2">
          <Button variant="secondary" onClick={() => setCancelConfirmOpen(false)}>
            Batal
          </Button>
          <Button variant="danger" loading={cancelMutation.isPending} onClick={handleCancelConfirm}>
            Batalkan
          </Button>
        </div>
      </Dialog>
    </Card>
  );
}

function HistoryList({ items }: { items: ExitPermit[] }) {
  if (items.length === 0) return <EmptyState icon={LogOut} message="Belum ada riwayat dispensasi keluar." />;
  return (
    <div>
      {items.map((item) => (
        <ListRow
          key={item.id}
          className="min-h-[56px]"
          title={item.reason}
          subtitle={<span className="num">{formatDateTime(item.created_at)}</span>}
          trailing={<Tag variant={EXIT_PERMIT_STATUS_TAG_VARIANT[item.status]}>{EXIT_PERMIT_STATUS_LABEL[item.status]}</Tag>}
        />
      ))}
    </div>
  );
}

function ExitPermitBody({ items, isLoading, isError, onRetry, readOnly, showCreateForm }: {
  items: ExitPermit[] | undefined;
  isLoading: boolean;
  isError: boolean;
  onRetry: () => void;
  readOnly: boolean;
  showCreateForm: boolean;
}) {
  if (isLoading) {
    return (
      <div className="flex flex-col gap-3">
        <Skeleton className="h-48 w-full" />
      </div>
    );
  }
  if (isError) {
    return <ErrorState message="Gagal memuat dispensasi keluar." onRetry={onRetry} />;
  }

  const active = items?.find((i) => !TERMINAL_STATUSES.includes(i.status)) ?? null;
  const history = items?.filter((i) => i.id !== active?.id) ?? [];

  return (
    <>
      {active ? <ActivePermitCard permit={active} readOnly={readOnly} /> : showCreateForm ? <CreatePermitForm /> : (
        <EmptyState icon={LogOut} message="Tidak ada dispensasi keluar yang sedang berjalan." />
      )}

      <div className="flex flex-col gap-3">
        <p className="text-[11px] font-semibold uppercase tracking-[0.1em] text-muted">Riwayat</p>
        <HistoryList items={history} />
      </div>
    </>
  );
}

function SiswaExitPermit() {
  const { data: items, isLoading, isError, refetch } = useExitPermits('mine');

  return (
    <div className="mx-auto flex max-w-[640px] flex-col gap-6 px-5 py-6">
      <div>
        <p className="text-[11px] font-semibold uppercase tracking-[0.1em] text-muted">Izin Keluar</p>
        <h1 className="text-[21px] font-semibold text-ink">Dispensasi Keluar</h1>
      </div>
      <ExitPermitBody
        items={items}
        isLoading={isLoading}
        isError={isError}
        onRetry={() => refetch()}
        readOnly={false}
        showCreateForm
      />
    </div>
  );
}

function OrangTuaExitPermit() {
  const { data: children, isLoading: childrenLoading, isError: childrenError, refetch: refetchChildren } =
    useExitPermitChildren();
  const [selectedId, setSelectedId] = useState<string | undefined>(undefined);
  // ASUMSI (sama pola `StudentLeaveQueuePage`'s `OrangTuaMyLeave`): `scope=mine`
  // untuk akun orang tua mengembalikan gabungan dispensasi SEMUA anaknya
  // (tiap baris sudah punya `student` ref) — filter per anak dilakukan di
  // client lewat Select anak, karena kontrak GET tidak punya parameter `student_id`.
  const { data: items, isLoading: itemsLoading, isError: itemsError, refetch: refetchItems } = useExitPermits(
    'mine',
    undefined,
    Boolean(children && children.length > 0),
  );

  useEffect(() => {
    if (children && children.length > 0 && !selectedId) {
      setSelectedId(children[0].student_id);
    }
  }, [children, selectedId]);

  const filteredItems = items?.filter((i) => i.student.id === selectedId);

  return (
    <div className="mx-auto flex max-w-[640px] flex-col gap-6 px-5 py-6">
      <div>
        <p className="text-[11px] font-semibold uppercase tracking-[0.1em] text-muted">Izin Keluar</p>
        <h1 className="text-[21px] font-semibold text-ink">Dispensasi Keluar Anak</h1>
      </div>

      {childrenLoading ? (
        <Skeleton className="h-20 w-full" />
      ) : childrenError ? (
        <ErrorState message="Gagal memuat data anak." onRetry={() => refetchChildren()} />
      ) : !children || children.length === 0 ? (
        <EmptyState message="Belum ada anak yang terhubung ke akun ini." />
      ) : (
        <>
          {children.length > 1 && (
            <Field label="Pilih anak">
              <Select value={selectedId} onChange={(e) => setSelectedId(e.target.value)}>
                {children.map((c) => (
                  <option key={c.student_id} value={c.student_id}>
                    {c.name} — {c.class_name}
                  </option>
                ))}
              </Select>
            </Field>
          )}

          <ExitPermitBody
            items={filteredItems}
            isLoading={itemsLoading}
            isError={itemsError}
            onRetry={() => refetchItems()}
            readOnly
            showCreateForm={false}
          />
        </>
      )}
    </div>
  );
}

/** /izin-keluar — dispensasi keluar rantai QR: siswa (ajukan + scan tiap tahap) & orang tua (lihat status anak, read-only). */
export function MyExitPermitPage() {
  const { data: me } = useMe();

  if (!me) return null;
  if (me.role === 'siswa') return <SiswaExitPermit />;
  if (me.role === 'orang_tua') return <OrangTuaExitPermit />;
  return <Navigate to="/" replace />;
}
