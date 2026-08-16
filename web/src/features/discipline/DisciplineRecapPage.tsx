import { useState } from 'react';
import { Download, FileBarChart } from 'lucide-react';
import { ListRow } from '../../components/ui/ListRow';
import { Skeleton } from '../../components/ui/Skeleton';
import { EmptyState } from '../../components/ui/EmptyState';
import { ErrorState } from '../../components/ui/ErrorState';
import { SegmentedControl, type SegmentedOption } from '../../components/ui/SegmentedControl';
import { Select } from '../../components/ui/Field';
import { isoDateDaysAgo, todayISODate } from '../../lib/date';
import { useClasses } from '../classes/api';
import { disciplineExportUrl, useDisciplineSpSettings, useDisciplineSummary } from './api';
import { totalPointsColorClass } from './format';

type RangeOption = '7' | '30';

const RANGE_OPTIONS: SegmentedOption<RangeOption>[] = [
  { value: '7', label: '7 Hari' },
  { value: '30', label: '30 Hari' },
];

/** Tab "Rekap" — total poin per siswa dalam rentang, warna angka mengikuti ambang SP1/SP3. */
export function DisciplineRecapPage() {
  const { data: classes } = useClasses();
  const [classId, setClassId] = useState('');
  const [range, setRange] = useState<RangeOption>('30');

  const to = todayISODate();
  const from = isoDateDaysAgo(Number(range), to);

  const { data: rows, isLoading, isError, refetch } = useDisciplineSummary({ classId: classId || undefined, from, to });
  const { data: spSettings } = useDisciplineSpSettings();
  const exportHref = disciplineExportUrl({ classId: classId || undefined, from, to });

  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <SegmentedControl options={RANGE_OPTIONS} value={range} onChange={setRange} />
        <div className="sm:w-56">
          <Select value={classId} onChange={(e) => setClassId(e.target.value)} aria-label="Filter rombel">
            <option value="">Semua rombel</option>
            {classes?.map((c) => (
              <option key={c.id} value={c.id}>
                {c.name}
              </option>
            ))}
          </Select>
        </div>
      </div>

      {isLoading ? (
        <div className="flex flex-col gap-2">
          <Skeleton className="h-16 w-full" />
          <Skeleton className="h-16 w-full" />
        </div>
      ) : isError ? (
        <ErrorState message="Gagal memuat rekap pelanggaran." onRetry={() => refetch()} />
      ) : !rows || rows.length === 0 ? (
        <EmptyState icon={FileBarChart} message="Belum ada pelanggaran tercatat pada rentang ini." />
      ) : (
        <div>
          {rows.map((row) => (
            <ListRow
              key={row.student.id}
              title={row.student.name}
              subtitle={`${row.student.nis} · ${row.student.class_name} · ${row.record_count} catatan`}
              trailing={
                <span
                  className={`num text-[21px] font-bold ${
                    spSettings ? totalPointsColorClass(row.total_points, spSettings.sp1, spSettings.sp3) : 'text-ink'
                  }`}
                >
                  {row.total_points}
                </span>
              }
            />
          ))}
        </div>
      )}

      <div className="flex justify-start border-t border-line pt-4">
        <a
          href={exportHref}
          className="inline-flex min-h-11 items-center gap-2 rounded-lg border border-line px-4 text-[14px] font-medium text-ink transition-colors duration-150 hover:bg-surface-2"
        >
          <Download size={16} strokeWidth={2} aria-hidden="true" />
          Unduh Excel
        </a>
      </div>
    </div>
  );
}
