import type { DisciplineLetterLevel } from '../../lib/types';

/**
 * Varian `Tag` per level Surat Peringatan — "warna bertingkat warning→danger"
 * (spesifikasi Fase 14 Gelombang A). Design system (docs/10) hanya punya dua
 * token semantik yang relevan di sini (`--st-terlambat` "warning" & `--danger`
 * "danger") — TIDAK ada token oranye/merah terpisah untuk tiga tingkat, jadi
 * ASUMSI pemetaan: SP1 = warning (peringatan awal), SP2 & SP3 = danger (sudah
 * dua kali diperingatkan, tingkat keseriusan sama-sama tinggi).
 */
export const SP_LEVEL_TAG_VARIANT: Record<DisciplineLetterLevel, 'warning' | 'danger'> = {
  1: 'warning',
  2: 'danger',
  3: 'danger',
};

export function spLevelLabel(level: DisciplineLetterLevel | number): string {
  return `SP${level}`;
}

/**
 * Warna angka total poin di Rekap — ambang dari `GET /api/discipline/sp-settings`
 * (spesifikasi: "<sp1 muted, ≥sp1 warning, ≥sp3 danger"). Kelas Tailwind sama
 * persis dengan `StatTile`'s `VALUE_COLOR_CLASS` supaya konsisten meski dipakai
 * di luar `StatTile` (trailing `ListRow`, bukan kartu ringkasan).
 */
export function totalPointsColorClass(totalPoints: number, sp1: number, sp3: number): string {
  if (totalPoints >= sp3) return 'text-danger';
  if (totalPoints >= sp1) return 'text-st-terlambat';
  return 'text-muted';
}
