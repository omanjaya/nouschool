import { useMemo, useState } from 'react';
import { CalendarDays } from 'lucide-react';
import { ListRow } from '../../components/ui/ListRow';
import { Skeleton } from '../../components/ui/Skeleton';
import { EmptyState } from '../../components/ui/EmptyState';
import { ErrorState } from '../../components/ui/ErrorState';
import { Tag } from '../../components/ui/Tag';
import { SegmentedControl } from '../../components/ui/SegmentedControl';
import { useMe } from '../auth/api';
import { DAY_OPTIONS, formatClock, todayDayOfWeek } from './format';
import { usePeriods, useScheduleSlots, useScheduleToday } from './api';
import type { DayOfWeek, Period, ScheduleSlot, ScheduleTodaySlot } from '../../lib/types';

function periodTimeRange(periods: Period[] | undefined, start: number, end: number): string {
  if (!periods) return '';
  const startPeriod = periods.find((p) => p.number === start);
  const endPeriod = periods.find((p) => p.number === end);
  if (!startPeriod || !endPeriod) return '';
  return `${formatClock(startPeriod.starts_at)}–${formatClock(endPeriod.ends_at)}`;
}

/**
 * /jadwal — jadwal pribadi guru/siswa. Guru: default "hari ini" + toggle Senin-Sabtu.
 * Siswa: sama, berdasarkan rombelnya.
 *
 * ASUMSI: dipanggil TANPA class_id/teacher_id — backend infer dari sesi (lihat
 * catatan di features/schedule/api.ts `useScheduleToday`). Kalau backend menolak,
 * layar menampilkan ErrorState + retry (bukan crash).
 */
export function SchedulePage() {
  const { data: me } = useMe();
  const today = todayDayOfWeek();
  const [selectedDay, setSelectedDay] = useState<DayOfWeek>(today ?? 1);

  const { data: periods } = usePeriods();
  const isToday = selectedDay === today;

  const todayQuery = useScheduleToday({}, isToday);
  const weekQuery = useScheduleSlots({}, !isToday);

  const isSupportedRole = me?.role === 'guru' || me?.role === 'siswa';

  const daySlots: (ScheduleSlot | ScheduleTodaySlot)[] = useMemo(() => {
    if (isToday) return todayQuery.data ?? [];
    return (weekQuery.data ?? []).filter((s) => s.day_of_week === selectedDay);
  }, [isToday, todayQuery.data, weekQuery.data, selectedDay]);

  const sortedSlots = useMemo(
    () => daySlots.slice().sort((a, b) => a.period_start - b.period_start),
    [daySlots],
  );

  const isLoading = isToday ? todayQuery.isLoading : weekQuery.isLoading;
  const isError = isToday ? todayQuery.isError : weekQuery.isError;
  const refetch = isToday ? todayQuery.refetch : weekQuery.refetch;

  if (!me) return null;

  if (!isSupportedRole) {
    return (
      <div className="mx-auto max-w-[640px] px-5 py-6">
        <EmptyState icon={CalendarDays} message="Jadwal pribadi tidak tersedia untuk peran ini." />
      </div>
    );
  }

  return (
    <div className="mx-auto flex max-w-[640px] flex-col gap-6 px-5 py-6">
      <div>
        <p className="text-[11px] font-semibold uppercase tracking-[0.1em] text-muted">Jadwal</p>
        <h1 className="text-[21px] font-semibold text-ink">
          {me.role === 'guru' ? 'Jadwal Mengajar' : 'Jadwal Pelajaran'}
        </h1>
      </div>

      <SegmentedControl
        options={DAY_OPTIONS.map((d) => ({ value: String(d.value), label: d.label }))}
        value={String(selectedDay)}
        onChange={(v) => setSelectedDay(Number(v) as DayOfWeek)}
        className="w-full overflow-x-auto"
      />

      {isLoading ? (
        <div className="flex flex-col gap-2">
          <Skeleton className="h-14 w-full" />
          <Skeleton className="h-14 w-full" />
          <Skeleton className="h-14 w-full" />
        </div>
      ) : isError ? (
        <ErrorState message="Gagal memuat jadwal. Coba lagi." onRetry={() => refetch()} />
      ) : sortedSlots.length === 0 ? (
        <EmptyState icon={CalendarDays} message="Tidak ada jadwal pada hari ini." />
      ) : (
        <div>
          {sortedSlots.map((slot) => {
            const range = periodTimeRange(periods, slot.period_start, slot.period_end);
            const periodLabel =
              slot.period_start === slot.period_end ? `Jam ke-${slot.period_start}` : `Jam ke-${slot.period_start}–${slot.period_end}`;
            const isNow = isToday && Boolean((slot as ScheduleTodaySlot).is_now);
            const subtitleParts = [
              slot.subject.name,
              me.role === 'guru' ? slot.class.name : slot.teacher.name,
              slot.room?.name,
            ].filter(Boolean);

            return (
              <ListRow
                key={slot.id}
                title={`${periodLabel}${range ? ` · ${range}` : ''}`}
                subtitle={subtitleParts.join(' · ')}
                trailing={isNow ? <Tag variant="now">Sekarang</Tag> : undefined}
              />
            );
          })}
        </div>
      )}
    </div>
  );
}
