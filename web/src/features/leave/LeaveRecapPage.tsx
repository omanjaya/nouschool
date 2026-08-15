import { useState } from 'react';
import { Navigate, useNavigate } from 'react-router-dom';
import { Download, FileBarChart } from 'lucide-react';
import { AppBar } from '../../components/ui/AppBar';
import { ListRow } from '../../components/ui/ListRow';
import { SegmentedControl, type SegmentedOption } from '../../components/ui/SegmentedControl';
import { Skeleton } from '../../components/ui/Skeleton';
import { EmptyState } from '../../components/ui/EmptyState';
import { ErrorState } from '../../components/ui/ErrorState';
import { isoDateDaysAgo, startOfMonthISODate, todayISODate } from '../../lib/date';
import { useMe } from '../auth/api';
import { useLeaveSummary, useLeaveTypes } from './api';
import type { LeaveSummaryRow } from '../../lib/types';

type RangeOption = 'month' | 'semester';

const RANGE_OPTIONS: SegmentedOption<RangeOption>[] = [
  { value: 'month', label: 'Bulan ini' },
  { value: 'semester', label: 'Semester' },
];

function formatPerType(row: LeaveSummaryRow, typeLabels: Map<string, string>): string {
  const parts = Object.entries(row.per_type)
    .filter(([, days]) => days > 0)
    .map(([key, days]) => `${typeLabels.get(key) ?? key} ${days}`);
  return parts.length > 0 ? parts.join(' · ') : 'Tidak ada izin';
}

/** /izin/rekap — rekap izin per guru dalam rentang (kepsek/admin). */
export function LeaveRecapPage() {
  const { data: me } = useMe();
  const navigate = useNavigate();
  const [range, setRange] = useState<RangeOption>('month');
  const canView = me?.role === 'kepala_sekolah' || me?.role === 'admin_sekolah';

  const to = todayISODate();
  const from = range === 'month' ? startOfMonthISODate(to) : isoDateDaysAgo(180, to);

  const { data: rows, isLoading, isError, refetch } = useLeaveSummary(from, to, canView);
  const { data: types } = useLeaveTypes();
  const typeLabels = new Map((types ?? []).map((t) => [t.key, t.label]));
  const exportHref = `/api/leave/export?from=${from}&to=${to}`;

  if (me && !canView) {
    return <Navigate to="/izin" replace />;
  }

  return (
    <div className="flex min-h-dvh flex-col">
      <AppBar title="Rekap Izin" subtitle="Izin" onBack={() => navigate('/izin')} />

      <div className="mx-auto flex w-full max-w-[640px] flex-1 flex-col gap-5 px-5 py-5">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <SegmentedControl options={RANGE_OPTIONS} value={range} onChange={setRange} />
          <a
            href={exportHref}
            className="inline-flex min-h-11 items-center gap-2 rounded-lg border border-line px-4 text-[14px] font-medium text-ink transition-colors duration-150 hover:bg-surface-2"
          >
            <Download size={16} strokeWidth={2} aria-hidden="true" />
            Unduh Excel
          </a>
        </div>

        {isLoading ? (
          <div className="flex flex-col gap-2">
            <Skeleton className="h-14 w-full" />
            <Skeleton className="h-14 w-full" />
            <Skeleton className="h-14 w-full" />
          </div>
        ) : isError ? (
          <ErrorState message="Gagal memuat rekap izin." onRetry={() => refetch()} />
        ) : !rows || rows.length === 0 ? (
          <EmptyState icon={FileBarChart} message="Belum ada izin tercatat pada rentang ini." />
        ) : (
          <div>
            {rows.map((row) => (
              <ListRow
                key={row.teacher_id}
                title={row.name}
                subtitle={formatPerType(row, typeLabels)}
                trailing={<span className="num text-[16px] font-semibold text-ink">{row.total_days}</span>}
              />
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
