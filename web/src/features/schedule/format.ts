import type { DayOfWeek } from '../../lib/types';

/** 1=Senin ... 6=Sabtu (docs/04-schedule.md). */
export const DAY_LABELS: Record<DayOfWeek, string> = {
  1: 'Senin',
  2: 'Selasa',
  3: 'Rabu',
  4: 'Kamis',
  5: 'Jumat',
  6: 'Sabtu',
};

export const DAY_OPTIONS: { value: DayOfWeek; label: string }[] = [1, 2, 3, 4, 5, 6].map((d) => ({
  value: d as DayOfWeek,
  label: DAY_LABELS[d as DayOfWeek],
}));

/**
 * Hari ini sebagai `day_of_week` (1=Senin..6=Sabtu), atau `null` kalau Minggu
 * (tidak ada jadwal KBM). `Date#getDay()` sudah 1=Senin..6=Sabtu, 0=Minggu —
 * jadi tidak perlu remap selain menolak 0.
 */
export function todayDayOfWeek(): DayOfWeek | null {
  const jsDay = new Date().getDay();
  return jsDay === 0 ? null : (jsDay as DayOfWeek);
}

/** Jam "07:00" → menit sejak 00:00, untuk perbandingan/urutan. */
export function timeToMinutes(time: string): number {
  const match = /^(\d{1,2}):(\d{2})/.exec(time);
  if (!match) return 0;
  return Number(match[1]) * 60 + Number(match[2]);
}

/** Tambah menit ke jam "HH:MM", dibungkus 24 jam. */
export function addMinutes(time: string, minutes: number): string {
  const total = (timeToMinutes(time) + minutes + 24 * 60) % (24 * 60);
  const hh = String(Math.floor(total / 60)).padStart(2, '0');
  const mm = String(total % 60).padStart(2, '0');
  return `${hh}:${mm}`;
}

/** "07:00" → "07.00" (docs/10 #8: jam 24-jam pakai titik, bukan titik dua). */
export function formatClock(time: string): string {
  return time.replace(':', '.').slice(0, 5);
}
