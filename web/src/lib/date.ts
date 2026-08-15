const MONTHS_ID = [
  'Jan', 'Feb', 'Mar', 'Apr', 'Mei', 'Jun', 'Jul', 'Agu', 'Sep', 'Okt', 'Nov', 'Des',
];

const DAYS_ID = ['Minggu', 'Senin', 'Selasa', 'Rabu', 'Kamis', 'Jumat', 'Sabtu'];

/**
 * Format tanggal "YYYY-MM-DD..." → "18 Agu 2026" (docs/10-design-system.md #8).
 * Diparsing sebagai string, bukan lewat `new Date()`, supaya tidak bergeser
 * sehari akibat konversi zona waktu lokal browser.
 */
export function formatDate(isoDate: string): string {
  const match = /^(\d{4})-(\d{2})-(\d{2})/.exec(isoDate);
  if (!match) return isoDate;
  const [, year, month, day] = match;
  const monthLabel = MONTHS_ID[Number(month) - 1];
  if (!monthLabel) return isoDate;
  return `${Number(day)} ${monthLabel} ${year}`;
}

/**
 * Format tanggal dengan nama hari — "Selasa, 18 Agu 2026" (docs/10 #8).
 * Dipakai untuk header layar absensi harian.
 */
export function formatDateWithDay(isoDate: string): string {
  const match = /^(\d{4})-(\d{2})-(\d{2})/.exec(isoDate);
  if (!match) return isoDate;
  const [, year, month, day] = match;
  const monthLabel = MONTHS_ID[Number(month) - 1];
  if (!monthLabel) return isoDate;
  const dayLabel = DAYS_ID[new Date(Number(year), Number(month) - 1, Number(day)).getDay()];
  return `${dayLabel}, ${Number(day)} ${monthLabel} ${year}`;
}

/** Tanggal hari ini di perangkat, dikodekan "YYYY-MM-DD" (bukan UTC). */
export function todayISODate(): string {
  return toISODate(new Date());
}

function toISODate(d: Date): string {
  const y = d.getFullYear();
  const m = String(d.getMonth() + 1).padStart(2, '0');
  const day = String(d.getDate()).padStart(2, '0');
  return `${y}-${m}-${day}`;
}

/** `isoDate` (default hari ini) mundur `days` hari — dipakai rentang riwayat kehadiran. */
export function isoDateDaysAgo(days: number, fromIsoDate: string = todayISODate()): string {
  const match = /^(\d{4})-(\d{2})-(\d{2})/.exec(fromIsoDate);
  if (!match) return fromIsoDate;
  const [, year, month, day] = match;
  const d = new Date(Number(year), Number(month) - 1, Number(day));
  d.setDate(d.getDate() - days);
  return toISODate(d);
}
