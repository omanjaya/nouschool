import type { ReactNode } from 'react';
import { TrendingDown, TrendingUp } from 'lucide-react';

type StatTileVariant = 'default' | 'danger' | 'success' | 'info' | 'warning' | 'muted';

const VALUE_COLOR_CLASS: Record<StatTileVariant, string> = {
  default: 'text-ink',
  danger: 'text-danger',
  /** Angka "baik" (mis. jumlah guru sedang mengajar) — token --st-hadir. */
  success: 'text-st-hadir',
  /** Angka netral-informatif (mis. jumlah guru izin) — token --st-izin. */
  info: 'text-st-izin',
  /** Angka "perlu perhatian tapi belum darurat" (mis. sekolah masa grace) — token --st-terlambat. */
  warning: 'text-st-terlambat',
  /** Angka yang belum relevan/tidak perlu ditonjolkan (mis. "belum mulai"). */
  muted: 'text-muted',
};

export interface StatTileTrend {
  /** Angka delta yang ditampilkan, mis. `12` → dirender "+12%" (naik) / "-12%" (turun). */
  value: number;
  direction: 'up' | 'down';
  /** Satuan di belakang angka delta — default "%". */
  unit?: string;
  /** Konteks pembanding, mis. "vs 7 hari lalu". */
  label?: string;
}

interface StatTileProps {
  label: string;
  value: ReactNode;
  variant?: StatTileVariant;
  /**
   * Delta opsional (naik/turun) dari `@shadcnblocks/stats1` — warna & ikon
   * semantik: naik = `--st-hadir` (TrendingUp), turun = `--danger` (TrendingDown).
   * Caller yang menentukan arah mana yang "baik" untuk metrik itu, komponen
   * ini hanya merender apa yang dikirim (tidak pernah mengarang angka).
   */
  trend?: StatTileTrend;
  /** Catatan kecil di bawah angka, mis. "dari 30 hari terakhir". */
  hint?: string;
}

/**
 * Angka besar 28px tabular + label eyebrow — dipakai di ringkasan dashboard/import.
 * `value` bisa berupa string panjang (mis. "Rp 10.000.000") — `whitespace-nowrap`
 * mencegah pecah baris di tengah angka, dan ukuran turun ke 22px kalau string-nya
 * panjang supaya tetap muat di kolom sempit tanpa wrap janggal.
 */
export function StatTile({ label, value, variant = 'default', trend, hint }: StatTileProps) {
  const isLongValue = typeof value === 'string' && value.length > 9;
  const sizeClass = isLongValue ? 'text-[22px]' : 'text-[28px]';

  return (
    <div className="flex min-w-0 flex-col gap-1">
      <span className={`num block truncate whitespace-nowrap font-semibold ${sizeClass} ${VALUE_COLOR_CLASS[variant]}`}>
        {value}
      </span>
      <span className="text-[11px] font-semibold uppercase tracking-[0.1em] text-muted">{label}</span>
      {trend && (
        <span className="flex items-center gap-1 text-[12px]">
          <span
            className={`num flex items-center gap-0.5 font-semibold ${
              trend.direction === 'up' ? 'text-st-hadir' : 'text-danger'
            }`}
          >
            {trend.direction === 'up' ? (
              <TrendingUp size={14} strokeWidth={2} aria-hidden="true" />
            ) : (
              <TrendingDown size={14} strokeWidth={2} aria-hidden="true" />
            )}
            {trend.direction === 'up' ? '+' : '-'}
            {Math.abs(trend.value)}
            {trend.unit ?? '%'}
          </span>
          {trend.label && <span className="truncate text-muted">{trend.label}</span>}
        </span>
      )}
      {hint && <span className="truncate text-[12px] text-muted">{hint}</span>}
    </div>
  );
}
