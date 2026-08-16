import { useMemo, useState } from 'react';
import { ChevronLeft, ChevronRight } from 'lucide-react';
import { Dialog } from '../../components/ui/Dialog';
import { StatTile } from '../../components/ui/StatTile';
import { Skeleton } from '../../components/ui/Skeleton';
import { ErrorState } from '../../components/ui/ErrorState';
import { formatDate } from '../../lib/date';
import { useStudentAttendanceCalendar } from './api';
import type { AttendanceStatus, StudentAttendanceCalendarDay } from '../../lib/types';

const MONTHS_FULL_ID = [
  'Januari', 'Februari', 'Maret', 'April', 'Mei', 'Juni',
  'Juli', 'Agustus', 'September', 'Oktober', 'November', 'Desember',
];

const WEEKDAY_HEADERS = ['Sen', 'Sel', 'Rab', 'Kam', 'Jum', 'Sab', 'Min'];

const STATUS_DOT_CLASS: Record<AttendanceStatus, string> = {
  hadir: 'bg-st-hadir',
  terlambat: 'bg-st-terlambat',
  izin: 'bg-st-izin',
  sakit: 'bg-st-sakit',
  alpa: 'bg-st-alpa',
};

const STATUS_LABEL: Record<AttendanceStatus, string> = {
  hadir: 'Hadir',
  terlambat: 'Terlambat',
  izin: 'Izin',
  sakit: 'Sakit',
  alpa: 'Alpa',
};

function pad2(n: number): string {
  return String(n).padStart(2, '0');
}

function toISODate(d: Date): string {
  return `${d.getFullYear()}-${pad2(d.getMonth() + 1)}-${pad2(d.getDate())}`;
}

function currentISOMonth(): string {
  const d = new Date();
  return `${d.getFullYear()}-${pad2(d.getMonth() + 1)}`;
}

function shiftMonth(month: string, delta: number): string {
  const [y, m] = month.split('-').map(Number);
  const d = new Date(y, m - 1 + delta, 1);
  return `${d.getFullYear()}-${pad2(d.getMonth() + 1)}`;
}

function monthLabel(month: string): string {
  const [y, m] = month.split('-').map(Number);
  return `${MONTHS_FULL_ID[m - 1] ?? month} ${y}`;
}

interface GridCell {
  date: string;
  dayNum: number;
  inMonth: boolean;
}

/** Grid Senin-Minggu untuk satu bulan, padding hari luar bulan (muted, non-interaktif). */
function buildGrid(month: string): GridCell[] {
  const [y, m] = month.split('-').map(Number);
  const daysInMonth = new Date(y, m, 0).getDate();
  const firstWeekday = (new Date(y, m - 1, 1).getDay() + 6) % 7; // 0=Senin..6=Minggu
  const prevMonthDays = new Date(y, m - 1, 0).getDate();

  const cells: GridCell[] = [];
  for (let i = firstWeekday - 1; i >= 0; i--) {
    const dayNum = prevMonthDays - i;
    cells.push({ date: toISODate(new Date(y, m - 2, dayNum)), dayNum, inMonth: false });
  }
  for (let dayNum = 1; dayNum <= daysInMonth; dayNum++) {
    cells.push({ date: toISODate(new Date(y, m - 1, dayNum)), dayNum, inMonth: true });
  }
  let nextDayNum = 1;
  while (cells.length % 7 !== 0) {
    cells.push({ date: toISODate(new Date(y, m, nextDayNum)), dayNum: nextDayNum, inMonth: false });
    nextDayNum += 1;
  }
  return cells;
}

/**
 * Kalender presensi bulanan (Fase 14 Gelombang D) — grid CSS murni dari token
 * warna status (docs/10 `--st-*`), tanpa library kalender eksternal. Tap sel
 * (hanya hari dalam bulan yang punya data) membuka Dialog detail.
 */
export function AttendanceCalendar({ studentId }: { studentId: string }) {
  const [month, setMonth] = useState(currentISOMonth());
  const [selectedDay, setSelectedDay] = useState<StudentAttendanceCalendarDay | null>(null);
  const { data, isLoading, isError, refetch } = useStudentAttendanceCalendar(studentId, month);

  const dayByDate = useMemo(() => {
    const map = new Map<string, StudentAttendanceCalendarDay>();
    data?.days.forEach((d) => map.set(d.date, d));
    return map;
  }, [data]);

  const grid = useMemo(() => buildGrid(month), [month]);

  return (
    <div className="flex flex-col gap-5">
      <div className="flex items-center justify-between gap-3">
        <button
          type="button"
          onClick={() => setMonth((m) => shiftMonth(m, -1))}
          aria-label="Bulan sebelumnya"
          className="flex h-9 w-9 items-center justify-center rounded-lg text-muted transition-colors duration-150 hover:bg-surface-2 hover:text-ink"
        >
          <ChevronLeft size={18} strokeWidth={2} aria-hidden="true" />
        </button>
        <p className="text-[14px] font-semibold text-ink">{monthLabel(month)}</p>
        <button
          type="button"
          onClick={() => setMonth((m) => shiftMonth(m, 1))}
          aria-label="Bulan berikutnya"
          className="flex h-9 w-9 items-center justify-center rounded-lg text-muted transition-colors duration-150 hover:bg-surface-2 hover:text-ink"
        >
          <ChevronRight size={18} strokeWidth={2} aria-hidden="true" />
        </button>
      </div>

      {isLoading ? (
        <Skeleton className="h-72 w-full" />
      ) : isError ? (
        <ErrorState message="Gagal memuat kalender kehadiran." onRetry={() => refetch()} />
      ) : (
        <>
          <div className="grid grid-cols-7 gap-1">
            {WEEKDAY_HEADERS.map((h) => (
              <div key={h} className="py-1 text-center text-[11px] font-semibold uppercase text-muted">
                {h}
              </div>
            ))}
            {grid.map((cell) => {
              const day = cell.inMonth ? dayByDate.get(cell.date) : undefined;
              const isInteractive = cell.inMonth;
              return (
                <button
                  key={cell.date}
                  type="button"
                  disabled={!isInteractive}
                  onClick={() => day && setSelectedDay(day)}
                  className={`flex aspect-square flex-col items-center justify-center gap-1 rounded-lg text-[13px] transition-colors duration-150 ${
                    cell.inMonth ? 'text-ink hover:bg-surface-2' : 'text-muted/50'
                  } ${!day ? 'cursor-default' : ''}`}
                >
                  <span className="num">{cell.dayNum}</span>
                  {cell.inMonth && day?.status && (
                    <span
                      aria-hidden="true"
                      className={`h-1.5 w-1.5 rounded-full ${STATUS_DOT_CLASS[day.status]}`}
                    />
                  )}
                </button>
              );
            })}
          </div>

          {data && (
            <div className="grid grid-cols-5 gap-x-1 gap-y-3 border-t border-line pt-4">
              <StatTile label="Hadir" value={data.counts.hadir} />
              <StatTile label="Terlmbt" value={data.counts.terlambat} />
              <StatTile label="Izin" value={data.counts.izin} />
              <StatTile label="Sakit" value={data.counts.sakit} />
              <StatTile label="Alpa" value={data.counts.alpa} variant={data.counts.alpa > 0 ? 'danger' : 'default'} />
            </div>
          )}
        </>
      )}

      <Dialog
        open={selectedDay !== null}
        onClose={() => setSelectedDay(null)}
        title={selectedDay ? formatDate(selectedDay.date) : ''}
      >
        {selectedDay && (
          <div className="flex flex-col gap-3">
            <div className="flex items-center justify-between border-b border-line pb-2.5">
              <span className="text-[12px] text-muted">Status</span>
              <span className="text-[14px] font-medium text-ink">
                {selectedDay.status ? STATUS_LABEL[selectedDay.status] : 'Tidak ada data'}
              </span>
            </div>
            <div className="flex items-center justify-between border-b border-line pb-2.5">
              <span className="text-[12px] text-muted">Jumlah sesi</span>
              <span className="num text-[14px] text-ink">{selectedDay.session_count}</span>
            </div>
            <div>
              <span className="text-[12px] text-muted">Catatan</span>
              <p className="mt-1 text-[14px] text-ink">{selectedDay.note ?? '-'}</p>
            </div>
          </div>
        )}
      </Dialog>
    </div>
  );
}
