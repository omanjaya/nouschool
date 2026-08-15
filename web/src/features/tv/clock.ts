/** Parse tanggal "YYYY-MM-DD" + jam dinding "HH:MM" jadi `Date` lokal perangkat. */
export function parseWallClock(date: string, time: string): Date | null {
  const dateMatch = /^(\d{4})-(\d{2})-(\d{2})/.exec(date);
  const timeMatch = /^(\d{1,2}):(\d{2})/.exec(time);
  if (!dateMatch || !timeMatch) return null;
  const [, y, mo, d] = dateMatch;
  const [, hh, mm] = timeMatch;
  return new Date(Number(y), Number(mo) - 1, Number(d), Number(hh), Number(mm), 0, 0);
}

/** "10.15.42" — jam 24-jam dengan detik, gaya pemisah titik konsisten docs/10 #8. */
export function formatClockWithSeconds(d: Date): string {
  const hh = String(d.getHours()).padStart(2, '0');
  const mm = String(d.getMinutes()).padStart(2, '0');
  const ss = String(d.getSeconds()).padStart(2, '0');
  return `${hh}.${mm}.${ss}`;
}

/** Hitung mundur ke `target` dari `from` — "12.05" (mm.ss) atau "1.12.05" (h.mm.ss) kalau ≥1 jam. `null` kalau lewat/tidak ada target. */
export function formatCountdown(from: Date, target: Date | null): string | null {
  if (!target) return null;
  const diffMs = target.getTime() - from.getTime();
  if (diffMs <= 0) return null;
  const totalSeconds = Math.floor(diffMs / 1000);
  const hours = Math.floor(totalSeconds / 3600);
  const minutes = Math.floor((totalSeconds % 3600) / 60);
  const seconds = totalSeconds % 60;
  const mm = String(minutes).padStart(2, '0');
  const ss = String(seconds).padStart(2, '0');
  return hours > 0 ? `${hours}.${mm}.${ss}` : `${mm}.${ss}`;
}
