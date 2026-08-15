import { useState } from 'react';
import { Navigate } from 'react-router-dom';
import { Download, FileBarChart } from 'lucide-react';
import { ListRow } from '../../components/ui/ListRow';
import { Skeleton } from '../../components/ui/Skeleton';
import { EmptyState } from '../../components/ui/EmptyState';
import { ErrorState } from '../../components/ui/ErrorState';
import { Tag } from '../../components/ui/Tag';
import { Input, Select } from '../../components/ui/Field';
import { currentISOMonth, todayISODate } from '../../lib/date';
import { useMe } from '../auth/api';
import { useClasses } from '../classes/api';
import { useAttendanceAnomalies, useAttendanceSummary } from './api';
import type { AttendanceAnomaly, AttendanceClassSummary } from '../../lib/types';

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

function AnomalyIssueTag({ issue }: { issue: AttendanceAnomaly['issue'] }) {
  if (issue === 'same_location') return <Tag variant="danger">Lokasi sama</Tag>;
  return <Tag variant="warning">Akurasi rendah</Tag>;
}

export function AttendanceRecapPage() {
  const { data: me } = useMe();
  const [date, setDate] = useState(todayISODate());
  const [exportMonth, setExportMonth] = useState(currentISOMonth());
  const [exportClassId, setExportClassId] = useState('');
  const canView = me?.role === 'admin_sekolah' || me?.role === 'kepala_sekolah';
  const { data: rows, isLoading, isError, refetch } = useAttendanceSummary(date, canView);
  const { data: classes } = useClasses();
  const exportHref = `/api/attendance/export?month=${exportMonth}${exportClassId ? `&class_id=${exportClassId}` : ''}`;
  const {
    data: anomalies,
    isLoading: anomaliesLoading,
    isError: anomaliesError,
    refetch: refetchAnomalies,
  } = useAttendanceAnomalies(date, canView);

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

      <div className="flex flex-col gap-3 border-t border-line pt-6">
        <p className="text-[11px] font-semibold uppercase tracking-[0.1em] text-muted">Export Rekap Bulanan</p>
        <div className="flex flex-col gap-3 sm:flex-row sm:items-end">
          <div className="max-w-[180px]">
            <Input
              type="month"
              value={exportMonth}
              onChange={(e) => setExportMonth(e.target.value)}
              aria-label="Pilih bulan export"
            />
          </div>
          <div className="sm:w-48">
            <Select
              value={exportClassId}
              onChange={(e) => setExportClassId(e.target.value)}
              aria-label="Filter rombel export"
            >
              <option value="">Semua rombel</option>
              {classes?.map((c) => (
                <option key={c.id} value={c.id}>
                  {c.name}
                </option>
              ))}
            </Select>
          </div>
          <a
            href={exportHref}
            className="inline-flex min-h-11 items-center gap-2 rounded-lg border border-line px-4 text-[14px] font-medium text-ink transition-colors duration-150 hover:bg-surface-2"
          >
            <Download size={16} strokeWidth={2} aria-hidden="true" />
            Unduh Excel
          </a>
        </div>
      </div>

      <div className="flex flex-col gap-3 border-t border-line pt-6">
        <p className="text-[11px] font-semibold uppercase tracking-[0.1em] text-muted">Anomali Check-in</p>
        {anomaliesLoading ? (
          <Skeleton className="h-14 w-full" />
        ) : anomaliesError ? (
          <ErrorState message="Gagal memuat anomali check-in." onRetry={() => refetchAnomalies()} />
        ) : anomalies && anomalies.length === 0 ? (
          <p className="text-[13px] text-muted">Tidak ada anomali.</p>
        ) : (
          <div>
            {anomalies?.map((a) => (
              <ListRow
                key={a.student_id}
                title={a.name}
                subtitle={`${a.class_name} · ${a.detail}`}
                trailing={<AnomalyIssueTag issue={a.issue} />}
              />
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
