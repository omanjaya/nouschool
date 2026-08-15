import { useState } from 'react';
import { Navigate, useNavigate } from 'react-router-dom';
import { FileBarChart } from 'lucide-react';
import { AppBar } from '../../components/ui/AppBar';
import { ListRow } from '../../components/ui/ListRow';
import { SegmentedControl, type SegmentedOption } from '../../components/ui/SegmentedControl';
import { Skeleton } from '../../components/ui/Skeleton';
import { EmptyState } from '../../components/ui/EmptyState';
import { ErrorState } from '../../components/ui/ErrorState';
import { isoDateDaysAgo, todayISODate } from '../../lib/date';
import { useMe } from '../auth/api';
import { useTeachingCompliance } from './api';
import type { TeachingComplianceRow } from '../../lib/types';

type RangeOption = '7' | '30';

const RANGE_OPTIONS: SegmentedOption<RangeOption>[] = [
  { value: '7', label: '7 Hari' },
  { value: '30', label: '30 Hari' },
];

function pctColorClass(pct: number): string {
  if (pct >= 90) return 'text-st-hadir';
  if (pct >= 70) return 'text-st-terlambat';
  return 'text-st-alpa';
}

function pctBarClass(pct: number): string {
  if (pct >= 90) return 'bg-st-hadir';
  if (pct >= 70) return 'bg-st-terlambat';
  return 'bg-st-alpa';
}

function ComplianceRow({ row }: { row: TeachingComplianceRow }) {
  const pct = Math.round(row.pct);
  return (
    <ListRow
      className="min-h-[64px] py-3"
      title={row.teacher.name}
      subtitle={
        <span className="flex flex-col gap-0.5">
          <span className="truncate">
            {row.taught}/{row.scheduled} slot terlaksana · {row.unscheduled} di luar jadwal
          </span>
          <span className="truncate">Materi terisi {Math.round(row.material_pct)}%</span>
        </span>
      }
      trailing={
        <div className="flex w-24 flex-col items-end gap-1.5">
          <span className={`num text-[21px] font-bold ${pctColorClass(pct)}`}>{pct}%</span>
          <div className="h-1 w-full overflow-hidden rounded-full bg-line">
            <div className={`h-full rounded-full ${pctBarClass(pct)}`} style={{ width: `${Math.min(100, Math.max(0, pct))}%` }} />
          </div>
        </div>
      }
    />
  );
}

/** /monitoring/rekap — ketertiban mengajar: % slot jadwal terlaksana per guru (kepsek/admin, docs/06). */
export function ComplianceRecapPage() {
  const { data: me } = useMe();
  const navigate = useNavigate();
  const [range, setRange] = useState<RangeOption>('7');
  const canView = me?.role === 'kepala_sekolah' || me?.role === 'admin_sekolah';

  const to = todayISODate();
  const from = isoDateDaysAgo(Number(range) - 1, to);

  const { data: rows, isLoading, isError, refetch } = useTeachingCompliance(from, to, canView);

  if (me && !canView) {
    return <Navigate to="/monitoring" replace />;
  }

  return (
    <div className="flex min-h-dvh flex-col">
      <AppBar title="Rekap Ketertiban" subtitle="Mengajar" onBack={() => navigate('/monitoring')} />

      <div className="mx-auto flex w-full max-w-[640px] flex-1 flex-col gap-5 px-5 py-5 lg:max-w-[1000px]">
        <SegmentedControl options={RANGE_OPTIONS} value={range} onChange={setRange} />

        {isLoading ? (
          <div className="flex flex-col gap-2">
            <Skeleton className="h-16 w-full" />
            <Skeleton className="h-16 w-full" />
            <Skeleton className="h-16 w-full" />
          </div>
        ) : isError ? (
          <ErrorState message="Gagal memuat rekap ketertiban mengajar." onRetry={() => refetch()} />
        ) : !rows || rows.length === 0 ? (
          <EmptyState icon={FileBarChart} message="Belum ada data ketertiban mengajar pada rentang ini." />
        ) : (
          <div>
            {rows.map((row) => (
              <ComplianceRow key={row.teacher.id} row={row} />
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
