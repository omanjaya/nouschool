import { useState } from 'react';
import { Link } from 'react-router-dom';
import { BarChart3, ChevronLeft } from 'lucide-react';
import { Field, Input } from '../../components/ui/Field';
import { ListRow } from '../../components/ui/ListRow';
import { Skeleton } from '../../components/ui/Skeleton';
import { EmptyState } from '../../components/ui/EmptyState';
import { ErrorState } from '../../components/ui/ErrorState';
import { currentISOMonth, formatDateTime } from '../../lib/date';
import { useLateArrivalSummary } from './api';
import { PAGE_WIDE } from '../../components/ui/page';

/** /izin-siswa/terlambat/rekap — rekap jumlah keterlambatan per siswa dalam satu bulan (`GET /api/late-arrivals/summary?month=`). */
export function LateArrivalRecapPage() {
  const [month, setMonth] = useState(currentISOMonth());
  const { data: rows, isLoading, isError, refetch } = useLateArrivalSummary(month);

  return (
    <div className={`${PAGE_WIDE} flex flex-col gap-6`}>
      <div className="flex items-center gap-3">
        <Link
          to="/izin-siswa/terlambat"
          aria-label="Kembali"
          className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg text-muted transition-colors duration-150 hover:bg-surface-2 hover:text-ink"
        >
          <ChevronLeft size={20} strokeWidth={2} aria-hidden="true" />
        </Link>
        <div>
          <p className="text-[11px] font-semibold uppercase tracking-[0.1em] text-muted">Terlambat</p>
          <h1 className="text-[21px] font-semibold text-ink">Rekap per Siswa</h1>
        </div>
      </div>

      <Field label="Bulan" htmlFor="late-arrival-recap-month" className="max-w-[180px]">
        <Input id="late-arrival-recap-month" type="month" value={month} onChange={(e) => setMonth(e.target.value)} />
      </Field>

      {isLoading ? (
        <div className="flex flex-col gap-2">
          <Skeleton className="h-14 w-full" />
          <Skeleton className="h-14 w-full" />
        </div>
      ) : isError ? (
        <ErrorState message="Gagal memuat rekap keterlambatan." onRetry={() => refetch()} />
      ) : !rows || rows.length === 0 ? (
        <EmptyState icon={BarChart3} message="Belum ada keterlambatan tercatat bulan ini." />
      ) : (
        <div>
          {rows.map((row) => (
            <ListRow
              key={row.student.id}
              className="min-h-[56px]"
              title={row.student.name}
              subtitle={
                <span className="num">
                  {row.student.nis} · {row.student.class_name}
                  {row.last_at ? ` · terakhir ${formatDateTime(row.last_at)}` : ''}
                </span>
              }
              trailing={<span className="num text-[14px] font-semibold text-ink">{row.count}×</span>}
            />
          ))}
        </div>
      )}
    </div>
  );
}
