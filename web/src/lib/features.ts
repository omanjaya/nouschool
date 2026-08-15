import type { Me } from './types';

/**
 * Cek apakah kunci fitur aktif di langganan sekolah (docs/09-billing.md
 * "Feature gating") — dipakai untuk menyembunyikan elemen UI (UX saja,
 * server tetap menegakkan lewat `requireFeature`). `me` boleh `undefined`
 * (query `/api/me` belum selesai) — dianggap tidak aktif sampai terbukti.
 */
export function hasFeature(me: Me | null | undefined, key: string): boolean {
  return Boolean(me?.features?.includes(key));
}
