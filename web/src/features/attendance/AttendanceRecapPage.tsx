import { useState } from 'react';
import { Navigate } from 'react-router-dom';
import { FileBarChart } from 'lucide-react';
import { ListRow } from '../../components/ui/ListRow';
import { Skeleton } from '../../components/ui/Skeleton';
import { EmptyState } from '../../components/ui/EmptyState';
import { ErrorState } from '../../components/ui/ErrorState';
import { Tag } from '../../components/ui/Tag';
import { Input } from '../../components/ui/Field';
import { todayISODate } from '../../lib/date';
import { useMe } from '../auth/api';
import { useAttendanceSummary } from './api';
import type { AttendanceClassSummary } from '../../lib/types';

function formatCounts(row: AttendanceClassSummary): string {
  const parts: string[] = [];
  if (row.hadir) parts.push(`${row.hadir}H`);
  if (row.terlambat) parts.push(`${row.terlambat}T`);
  if (row.izin) parts.push(`${row.izin}I`);
  if (row.sakit) parts.push(`${row.sakit}S`);
  if (row.alpa) parts.push(`${row.alpa}A`);
  return parts.length > 0 ? parts.join(' ') : '-';
}

function SessionStatusTag({ status }: { status: AttendanceClassSummary['session_status'] }) {
  if (status === 'finalized') return <Tag variant="done">Selesai</Tag>;
  if (status === 'open') return <Tag variant="now">Sedang diisi</Tag>;
  return <Tag variant="neutral">Belum diabsen</Tag>;
}

export function AttendanceRecapPage() {
  const { data: me } = useMe();
  const [date, setDate] = useState(todayISODate());
  const canView = me?.role === 'admin_sekolah' || me?.role === 'kepala_sekolah';
  const { data: rows, isLoading, isError, refetch } = useAttendanceSummary(date, canView);

  if (me && !canView) {
    return <Navigate to="/" replace />;
  }

  return (
    <div className="mx-auto flex max-w-[640px] flex-col gap-6 px-5 py-6">
      <div>
        <p className="text-[11px] font-semibold uppercase tracking-[0.1em] text-muted">Absensi</p>
        <h1 className="text-[21px] font-semibold text-ink">Rekap Harian</h1>
      </div>

      <div className="max-w-[220px]">
        <Input type="date" value={date} onChange={(e) => setDate(e.target.value)} aria-label="Pilih tanggal" />
      </div>

      {isLoading ? (
        <div className="flex flex-col gap-2">
          <Skeleton className="h-14 w-full" />
          <Skeleton className="h-14 w-full" />
          <Skeleton className="h-14 w-full" />
        </div>
      ) : isError ? (
        <ErrorState message="Gagal memuat rekap absensi." onRetry={() => refetch()} />
      ) : rows && rows.length === 0 ? (
        <EmptyState icon={FileBarChart} message="Belum ada sesi absensi pada tanggal ini." />
      ) : (
        <div>
          {rows?.map((row) => (
            <ListRow
              key={row.class_id}
              title={row.class_name}
              subtitle={`total ${row.total}`}
              trailing={
                <div className="flex items-center gap-3">
                  <span className="num text-[13px] font-semibold text-ink">{formatCounts(row)}</span>
                  <SessionStatusTag status={row.session_status} />
                </div>
              }
            />
          ))}
        </div>
      )}
    </div>
  );
}
