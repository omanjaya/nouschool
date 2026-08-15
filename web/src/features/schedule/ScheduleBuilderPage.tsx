import { useMemo, useState } from 'react';
import { FileSpreadsheet, Plus, Upload } from 'lucide-react';
import { useNavigate } from 'react-router-dom';
import { SegmentedControl } from '../../components/ui/SegmentedControl';
import { Select } from '../../components/ui/Field';
import { Button } from '../../components/ui/Button';
import { Skeleton } from '../../components/ui/Skeleton';
import { EmptyState } from '../../components/ui/EmptyState';
import { ErrorState } from '../../components/ui/ErrorState';
import { importTemplateUrl } from '../import/api';
import { useClasses } from '../classes/api';
import { useTeachers } from '../teachers/api';
import { DAY_OPTIONS, formatClock } from './format';
import { usePeriods, useScheduleSlots } from './api';
import { SlotFormDialog } from './SlotFormDialog';
import { CopyScheduleDialog } from './CopyScheduleDialog';
import type { DayOfWeek, Period, ScheduleSlot } from '../../lib/types';

type Mode = 'kelas' | 'guru';

interface CellTarget {
  slot?: ScheduleSlot;
  day: DayOfWeek;
  periodNumber: number;
}

/** /data/jadwal — builder grid jadwal (admin): mode Per Kelas (editable) & Per Guru (read-only). */
export function ScheduleBuilderPage() {
  const [mode, setMode] = useState<Mode>('kelas');
  const [classId, setClassId] = useState('');
  const [teacherId, setTeacherId] = useState('');
  const [cellTarget, setCellTarget] = useState<CellTarget | null>(null);
  const [copyOpen, setCopyOpen] = useState(false);
  const navigate = useNavigate();

  const { data: classes, isLoading: classesLoading } = useClasses();
  const { data: teachers, isLoading: teachersLoading } = useTeachers();
  const { data: periods, isLoading: periodsLoading, isError: periodsError, refetch: refetchPeriods } = usePeriods();

  const selectedId = mode === 'kelas' ? classId : teacherId;
  const filter = mode === 'kelas' ? { classId } : { teacherId };
  const {
    data: slots,
    isLoading: slotsLoading,
    isError: slotsError,
    refetch: refetchSlots,
  } = useScheduleSlots(filter, Boolean(selectedId));

  const selectedClassName = classes?.find((c) => c.id === classId)?.name ?? '';

  const sortedPeriods = useMemo(() => (periods ?? []).slice().sort((a, b) => a.number - b.number), [periods]);

  const periodPosition = useMemo(() => {
    const map = new Map<number, number>();
    sortedPeriods.forEach((p, idx) => map.set(p.number, idx + 1));
    return map;
  }, [sortedPeriods]);

  const { slotStart, covered } = useMemo(() => {
    const start = new Map<string, ScheduleSlot>();
    const cov = new Set<string>();
    for (const slot of slots ?? []) {
      start.set(`${slot.day_of_week}-${slot.period_start}`, slot);
      for (let p = slot.period_start + 1; p <= slot.period_end; p++) {
        cov.add(`${slot.day_of_week}-${p}`);
      }
    }
    return { slotStart: start, covered: cov };
  }, [slots]);

  function handleModeChange(next: Mode) {
    setMode(next);
  }

  function openCell(day: DayOfWeek, periodNumber: number, slot?: ScheduleSlot) {
    if (mode !== 'kelas' || !classId) return;
    setCellTarget({ day, periodNumber, slot });
  }

  const showEmptyPicker = !selectedId;

  return (
    <div className="flex flex-col gap-6">
      <div className="flex flex-wrap items-center gap-3">
        <SegmentedControl
          options={[
            { value: 'kelas', label: 'Per Kelas' },
            { value: 'guru', label: 'Per Guru' },
          ]}
          value={mode}
          onChange={handleModeChange}
        />

        {mode === 'kelas' ? (
          <Select
            aria-label="Pilih kelas"
            value={classId}
            onChange={(e) => setClassId(e.target.value)}
            className="w-auto min-w-[180px]"
            disabled={classesLoading}
          >
            <option value="">Pilih kelas</option>
            {classes?.map((c) => (
              <option key={c.id} value={c.id}>
                {c.name}
              </option>
            ))}
          </Select>
        ) : (
          <Select
            aria-label="Pilih guru"
            value={teacherId}
            onChange={(e) => setTeacherId(e.target.value)}
            className="w-auto min-w-[180px]"
            disabled={teachersLoading}
          >
            <option value="">Pilih guru</option>
            {teachers?.map((t) => (
              <option key={t.id} value={t.id}>
                {t.name}
              </option>
            ))}
          </Select>
        )}

        <div className="ml-auto flex flex-wrap items-center gap-2">
          {mode === 'kelas' && classId && (
            <Button variant="secondary" onClick={() => setCopyOpen(true)}>
              Salin dari kelas...
            </Button>
          )}
          <Button variant="secondary" onClick={() => navigate('/data/jadwal/import')}>
            <Upload size={16} strokeWidth={2} aria-hidden="true" />
            Import
          </Button>
        </div>
      </div>

      <a
        href={importTemplateUrl('schedule')}
        className="inline-flex w-fit items-center gap-1.5 text-[12px] font-medium text-primary hover:opacity-80"
      >
        <FileSpreadsheet size={16} strokeWidth={2} aria-hidden="true" />
        Unduh template jadwal
      </a>

      {periodsLoading ? (
        <Skeleton className="h-64 w-full" />
      ) : periodsError ? (
        <ErrorState message="Gagal memuat jam pelajaran." onRetry={() => refetchPeriods()} />
      ) : sortedPeriods.length === 0 ? (
        <EmptyState message="Belum ada jam pelajaran. Atur di tab Jam terlebih dahulu." />
      ) : showEmptyPicker ? (
        <EmptyState message={mode === 'kelas' ? 'Pilih kelas untuk melihat/mengatur jadwalnya.' : 'Pilih guru untuk melihat beban mengajarnya.'} />
      ) : slotsLoading ? (
        <Skeleton className="h-64 w-full" />
      ) : slotsError ? (
        <ErrorState message="Gagal memuat jadwal." onRetry={() => refetchSlots()} />
      ) : (
        <div className="overflow-x-auto">
          <div
            className="grid min-w-[760px] gap-px overflow-hidden rounded-lg border border-line bg-line"
            style={{
              gridTemplateColumns: `76px repeat(6, minmax(150px, 1fr))`,
              gridTemplateRows: `auto repeat(${sortedPeriods.length}, minmax(64px, auto))`,
            }}
          >
            <div className="bg-surface-2 px-2 py-2 text-[11px] font-semibold uppercase tracking-[0.1em] text-muted">
              Jam
            </div>
            {DAY_OPTIONS.map((d) => (
              <div
                key={d.value}
                className="bg-surface-2 px-2 py-2 text-center text-[12px] font-semibold text-ink"
              >
                {d.label}
              </div>
            ))}

            {sortedPeriods.map((period, rowIdx) => (
              <PeriodRow
                key={period.id}
                period={period}
                rowIndex={rowIdx + 1}
                mode={mode}
                slotStart={slotStart}
                covered={covered}
                periodPosition={periodPosition}
                onCellClick={openCell}
              />
            ))}
          </div>
        </div>
      )}

      {mode === 'kelas' && classId && sortedPeriods.length > 0 && !slotsLoading && !slotsError && (
        <SlotFormDialog
          open={cellTarget !== null}
          onClose={() => setCellTarget(null)}
          classId={classId}
          className={selectedClassName}
          periods={sortedPeriods}
          slot={cellTarget?.slot}
          defaultDay={cellTarget?.day}
          defaultPeriodNumber={cellTarget?.periodNumber}
        />
      )}

      {mode === 'kelas' && classId && (
        <CopyScheduleDialog
          open={copyOpen}
          onClose={() => setCopyOpen(false)}
          targetClassId={classId}
          targetClassName={selectedClassName}
        />
      )}
    </div>
  );
}

interface PeriodRowProps {
  period: Period;
  rowIndex: number;
  mode: Mode;
  slotStart: Map<string, ScheduleSlot>;
  covered: Set<string>;
  periodPosition: Map<number, number>;
  onCellClick: (day: DayOfWeek, periodNumber: number, slot?: ScheduleSlot) => void;
}

function PeriodRow({ period, rowIndex, mode, slotStart, covered, periodPosition, onCellClick }: PeriodRowProps) {
  const gridRow = rowIndex + 1;
  const timeLabel = `${formatClock(period.starts_at)}–${formatClock(period.ends_at)}`;

  if (period.label) {
    return (
      <>
        <div
          className="flex flex-col justify-center bg-surface-2 px-2 py-2 text-[11px] text-muted"
          style={{ gridRow, gridColumn: 1 }}
        >
          <span className="num">Ke-{period.number}</span>
          <span className="num">{timeLabel}</span>
        </div>
        <div
          className="flex items-center justify-center bg-surface-2 px-2 py-2 text-[12px] font-medium text-muted"
          style={{ gridRow, gridColumn: '2 / span 6' }}
        >
          {period.label}
        </div>
      </>
    );
  }

  return (
    <>
      <div
        className="flex flex-col justify-center bg-surface px-2 py-2 text-[11px] text-muted"
        style={{ gridRow, gridColumn: 1 }}
      >
        <span className="num">Ke-{period.number}</span>
        <span className="num">{timeLabel}</span>
      </div>
      {DAY_OPTIONS.map((d) => {
        const key = `${d.value}-${period.number}`;
        if (covered.has(key)) return null;

        const slot = slotStart.get(key);
        if (slot) {
          const startPos = periodPosition.get(slot.period_start) ?? rowIndex;
          const endPos = periodPosition.get(slot.period_end) ?? startPos;
          const span = Math.max(1, endPos - startPos + 1);
          return (
            <button
              key={d.value}
              type="button"
              onClick={() => onCellClick(d.value, period.number, slot)}
              disabled={mode !== 'kelas'}
              className={`flex flex-col items-start gap-0.5 bg-surface px-2 py-2 text-left transition-colors duration-150 ${
                mode === 'kelas' ? 'hover:bg-primary-soft' : 'cursor-default'
              }`}
              style={{ gridRow: `${startPos + 1} / span ${span}`, gridColumn: d.value + 1 }}
            >
              {mode === 'kelas' ? (
                <>
                  <span className="text-[13px] font-semibold text-ink">{slot.subject.code}</span>
                  <span className="truncate text-[11px] text-muted">{slot.teacher.name}</span>
                  {slot.room && <span className="truncate text-[11px] text-muted">{slot.room.name}</span>}
                </>
              ) : (
                <>
                  <span className="text-[13px] font-semibold text-ink">{slot.class.name}</span>
                  <span className="truncate text-[11px] text-muted">{slot.subject.code}</span>
                  {slot.room && <span className="truncate text-[11px] text-muted">{slot.room.name}</span>}
                </>
              )}
            </button>
          );
        }

        return (
          <button
            key={d.value}
            type="button"
            onClick={() => onCellClick(d.value, period.number)}
            disabled={mode !== 'kelas'}
            className={`group flex items-center justify-center bg-surface px-2 py-2 text-muted transition-colors duration-150 ${
              mode === 'kelas' ? 'hover:bg-surface-2' : 'cursor-default'
            }`}
            style={{ gridRow, gridColumn: d.value + 1 }}
          >
            {mode === 'kelas' && (
              <Plus size={14} strokeWidth={2} aria-hidden="true" className="opacity-0 transition-opacity duration-150 group-hover:opacity-100" />
            )}
          </button>
        );
      })}
    </>
  );
}
