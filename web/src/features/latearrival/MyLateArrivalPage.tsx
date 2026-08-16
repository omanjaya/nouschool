import { useState } from 'react';
import { Navigate } from 'react-router-dom';
import { AlertTriangle, Clock, QrCode } from 'lucide-react';
import { Card } from '../../components/ui/Card';
import { Button } from '../../components/ui/Button';
import { ListRow } from '../../components/ui/ListRow';
import { Skeleton } from '../../components/ui/Skeleton';
import { EmptyState } from '../../components/ui/EmptyState';
import { ErrorState } from '../../components/ui/ErrorState';
import { Tag } from '../../components/ui/Tag';
import { useToast } from '../../components/ui/Toast';
import { ApiError } from '../../lib/api';
import { currentISOMonth, formatDateTime, formatTimeOfDay } from '../../lib/date';
import { useMe } from '../auth/api';
import { TeacherTokenScanSheet } from '../teacherqr/TeacherTokenScanSheet';
import { useLateArrivals, useScanLateArrival } from './api';
import { LATE_ARRIVAL_STATUS_LABEL, LATE_ARRIVAL_STATUS_TAG_VARIANT } from './format';
import { LateArrivalStepper } from './LateArrivalStepper';
import type { LateArrivalAction, LateArrivalRecord } from '../../lib/types';

/** Banner menonjol saat aksi otomatis (hitungan keterlambatan ke-2/5 = hubungi ortu, ke-3/6 = pulangkan — docs/12-sion-parity.md Gelombang B alur 3). */
function ActionBanner({ action, count }: { action: LateArrivalAction; count: number }) {
  if (action === 'none') return null;
  const isDanger = action === 'send_home';
  return (
    <div
      className={`flex items-start gap-3 rounded-lg p-4 ${isDanger ? 'bg-danger-soft' : 'bg-st-terlambat-line'}`}
      role="status"
    >
      <AlertTriangle
        size={20}
        strokeWidth={2}
        className={isDanger ? 'shrink-0 text-danger' : 'shrink-0 text-st-terlambat'}
        aria-hidden="true"
      />
      <p className={`text-[14px] font-medium ${isDanger ? 'text-danger' : 'text-st-terlambat'}`}>
        {isDanger
          ? `Sesuai ketentuan, kamu dipulangkan — keterlambatan ke-${count}.`
          : `Orang tua akan dihubungi — keterlambatan ke-${count}.`}
      </p>
    </div>
  );
}

function ActiveRecordCard({ record }: { record: LateArrivalRecord }) {
  const { showToast } = useToast();
  const scan = useScanLateArrival();
  const [scanOpen, setScanOpen] = useState(false);
  const [scanError, setScanError] = useState<string | null>(null);

  const isPending = record.status !== 'completed';

  function handleScanSubmit(rawValue: string) {
    setScanError(null);
    scan.mutate(rawValue, {
      onSuccess: (result) => {
        setScanOpen(false);
        showToast(`Tahap dikonfirmasi — status: ${LATE_ARRIVAL_STATUS_LABEL[result.record.status]}.`);
      },
      onError: (err) => {
        setScanError(err instanceof ApiError ? err.message : 'Gagal memindai QR. Coba lagi.');
      },
    });
  }

  return (
    <Card className="flex flex-col gap-4">
      <div className="flex items-start justify-between gap-3">
        <div>
          <p className="text-[14px] font-semibold text-ink">Lapor Terlambat</p>
          <p className="text-[12px] text-muted">
            Datang {formatTimeOfDay(record.arrived_at)} · keterlambatan ke-{record.late_count}
          </p>
        </div>
        <Tag variant={LATE_ARRIVAL_STATUS_TAG_VARIANT[record.status]}>{LATE_ARRIVAL_STATUS_LABEL[record.status]}</Tag>
      </div>

      <ActionBanner action={record.action} count={record.late_count} />

      <LateArrivalStepper record={record} />

      {isPending && (
        <Button onClick={() => setScanOpen(true)} className="self-start">
          <QrCode size={16} strokeWidth={2} aria-hidden="true" />
          Scan QR Guru
        </Button>
      )}

      <TeacherTokenScanSheet
        open={scanOpen}
        onClose={() => setScanOpen(false)}
        onSubmit={handleScanSubmit}
        submitting={scan.isPending}
        errorMessage={scanError}
        instruction="Arahkan kamera ke QR guru untuk tahap berikutnya"
      />
    </Card>
  );
}

function NoActiveRecordCard() {
  const { showToast } = useToast();
  const scan = useScanLateArrival();
  const [scanOpen, setScanOpen] = useState(false);
  const [scanError, setScanError] = useState<string | null>(null);

  function handleScanSubmit(rawValue: string) {
    setScanError(null);
    scan.mutate(rawValue, {
      onSuccess: (result) => {
        setScanOpen(false);
        showToast(`Keterlambatan tercatat — ${LATE_ARRIVAL_STATUS_LABEL[result.record.status]}.`);
      },
      onError: (err) => {
        setScanError(err instanceof ApiError ? err.message : 'Gagal memindai QR. Coba lagi.');
      },
    });
  }

  return (
    <Card className="flex flex-col items-center gap-4 py-8 text-center">
      <Clock size={24} strokeWidth={2} className="text-muted" aria-hidden="true" />
      <div>
        <p className="text-[14px] font-semibold text-ink">Datang terlambat?</p>
        <p className="text-[12px] text-muted">Scan QR guru piket untuk lapor kedatangan.</p>
      </div>
      <Button onClick={() => setScanOpen(true)}>
        <QrCode size={16} strokeWidth={2} aria-hidden="true" />
        Scan QR Guru Piket
      </Button>

      <TeacherTokenScanSheet
        open={scanOpen}
        onClose={() => setScanOpen(false)}
        onSubmit={handleScanSubmit}
        submitting={scan.isPending}
        errorMessage={scanError}
        instruction="Arahkan kamera ke QR guru piket"
      />
    </Card>
  );
}

/**
 * /terlambat — siswa lapor keterlambatan lewat scan QR guru piket, rantai 3
 * tahap Piket → Pimpinan → Guru Kelas, aksi otomatis by hitungan (docs/12-sion-parity.md
 * Gelombang B alur 3). `scope=today` untuk kartu status aktif (ringan, tidak
 * perlu ambil seluruh riwayat); `scope=mine&month=` untuk riwayat + hitungan bulan ini.
 */
export function MyLateArrivalPage() {
  const { data: me } = useMe();
  const isSiswa = me?.role === 'siswa';
  const month = currentISOMonth();

  const { data: todayItems, isLoading: todayLoading, isError: todayError, refetch: refetchToday } = useLateArrivals(
    'today',
    undefined,
    isSiswa,
  );
  const { data: monthItems, isLoading: monthLoading, isError: monthError, refetch: refetchMonth } = useLateArrivals(
    'mine',
    month,
    isSiswa,
  );

  if (!me) return null;
  if (me.role !== 'siswa') return <Navigate to="/" replace />;

  const activeRecord = todayItems?.find((r) => r.status !== 'completed') ?? null;

  return (
    <div className="mx-auto flex max-w-[640px] flex-col gap-6 px-5 py-6">
      <div>
        <p className="text-[11px] font-semibold uppercase tracking-[0.1em] text-muted">Terlambat</p>
        <h1 className="text-[21px] font-semibold text-ink">Lapor Terlambat</h1>
      </div>

      {todayLoading ? (
        <Skeleton className="h-48 w-full" />
      ) : todayError ? (
        <ErrorState message="Gagal memuat status keterlambatan." onRetry={() => refetchToday()} />
      ) : activeRecord ? (
        <ActiveRecordCard record={activeRecord} />
      ) : (
        <NoActiveRecordCard />
      )}

      <div className="flex flex-col gap-3">
        <p className="text-[11px] font-semibold uppercase tracking-[0.1em] text-muted">
          Riwayat Bulan Ini{monthItems ? ` (${monthItems.length}×)` : ''}
        </p>
        {monthLoading ? (
          <Skeleton className="h-16 w-full" />
        ) : monthError ? (
          <ErrorState message="Gagal memuat riwayat." onRetry={() => refetchMonth()} />
        ) : !monthItems || monthItems.length === 0 ? (
          <EmptyState icon={Clock} message="Belum ada keterlambatan tercatat bulan ini." />
        ) : (
          <div>
            {monthItems.map((item) => (
              <ListRow
                key={item.id}
                className="min-h-[56px]"
                title={`Keterlambatan ke-${item.late_count}`}
                subtitle={<span className="num">{formatDateTime(item.arrived_at)}</span>}
                trailing={<Tag variant={LATE_ARRIVAL_STATUS_TAG_VARIANT[item.status]}>{LATE_ARRIVAL_STATUS_LABEL[item.status]}</Tag>}
              />
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
