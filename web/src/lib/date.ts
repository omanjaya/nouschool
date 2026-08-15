const MONTHS_ID = [
  'Jan', 'Feb', 'Mar', 'Apr', 'Mei', 'Jun', 'Jul', 'Agu', 'Sep', 'Okt', 'Nov', 'Des',
];

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
