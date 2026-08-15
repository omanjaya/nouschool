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

/** Tanggal 1 di bulan berjalan (perangkat), "YYYY-MM-DD" — dipakai rentang "Bulan ini". */
export function startOfMonthISODate(fromIsoDate: string = todayISODate()): string {
  const match = /^(\d{4})-(\d{2})-(\d{2})/.exec(fromIsoDate);
  if (!match) return fromIsoDate;
  const [, year, month] = match;
  return `${year}-${month}-01`;
}

/**
 * Rentang tanggal ringkas untuk satu baris — "18–20 Agu 2026" (rentang beda hari,
 * bulan/tahun sama), "28 Agu–2 Sep 2026" (bulan beda), atau tanggal tunggal
 * kalau start === end (docs/10-design-system.md #8).
 */
export function formatDateRange(startIso: string, endIso: string): string {
  if (startIso === endIso) return formatDate(startIso);
  const start = /^(\d{4})-(\d{2})-(\d{2})/.exec(startIso);
  const end = /^(\d{4})-(\d{2})-(\d{2})/.exec(endIso);
  if (!start || !end) return `${formatDate(startIso)} – ${formatDate(endIso)}`;
  const [, sy, sm, sd] = start;
  const [, ey, em] = end;
  if (sy === ey && sm === em) {
    return `${Number(sd)}–${formatDate(endIso)}`;
  }
  return `${formatDate(startIso)} – ${formatDate(endIso)}`;
}

/**
 * Format tanggal+jam — "18 Agu 2026 · 08.30" (jam 24-jam, docs/10 #8). `isoDateTime`
 * adalah timestamptz dari backend (mis. "2026-08-18T08:30:00Z"); ditampilkan di
 * jam lokal perangkat.
 */
export function formatDateTime(isoDateTime: string): string {
  const d = new Date(isoDateTime);
  if (Number.isNaN(d.getTime())) return isoDateTime;
  const hh = String(d.getHours()).padStart(2, '0');
  const mm = String(d.getMinutes()).padStart(2, '0');
  return `${formatDate(toISODate(d))} · ${hh}.${mm}`;
}

/**
 * Ambil jam saja dari timestamp — "08.30" (jam 24-jam, docs/10 #8). Dipakai baris
 * ringkas (jurnal mengajar, monitoring status guru) yang hanya perlu jam tanpa tanggal.
 */
export function formatTimeOfDay(isoDateTime: string): string {
  const d = new Date(isoDateTime);
  if (Number.isNaN(d.getTime())) return '-';
  const hh = String(d.getHours()).padStart(2, '0');
  const mm = String(d.getMinutes()).padStart(2, '0');
  return `${hh}.${mm}`;
}

/** Bulan berjalan (perangkat) dikodekan "YYYY-MM" — dipakai filter jurnal "Bulan Ini". */
export function currentISOMonth(): string {
  return todayISODate().slice(0, 7);
}

/**
 * Waktu relatif ringkas untuk inbox notifikasi (docs/08) — "Baru saja", "5 mnt lalu",
 * "2 jam lalu", "Kemarin", lalu jatuh balik ke tanggal biasa ("18 Agu 2026") untuk
 * yang lebih lama dari kemarin.
 */
export function formatRelativeTime(isoDateTime: string): string {
  const d = new Date(isoDateTime);
  if (Number.isNaN(d.getTime())) return isoDateTime;

  const now = new Date();
  const diffSec = Math.floor((now.getTime() - d.getTime()) / 1000);

  if (diffSec < 60) return 'Baru saja';
  const diffMin = Math.floor(diffSec / 60);
  if (diffMin < 60) return `${diffMin} mnt lalu`;
  const diffHour = Math.floor(diffMin / 60);

  const startOfToday = new Date(now.getFullYear(), now.getMonth(), now.getDate());
  const startOfDate = new Date(d.getFullYear(), d.getMonth(), d.getDate());
  const dayDiff = Math.round((startOfToday.getTime() - startOfDate.getTime()) / 86_400_000);

  if (dayDiff <= 0) return `${diffHour} jam lalu`;
  if (dayDiff === 1) return 'Kemarin';
  return formatDate(toISODate(d));
}
