import type { ReactNode } from 'react';

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

interface StatTileProps {
  label: string;
  value: ReactNode;
  variant?: StatTileVariant;
}

/**
 * Angka besar 28px tabular + label eyebrow — dipakai di ringkasan dashboard/import.
 * `value` bisa berupa string panjang (mis. "Rp 10.000.000") — `whitespace-nowrap`
 * mencegah pecah baris di tengah angka, dan ukuran turun ke 22px kalau string-nya
 * panjang supaya tetap muat di kolom sempit tanpa wrap janggal.
 */
export function StatTile({ label, value, variant = 'default' }: StatTileProps) {
  const isLongValue = typeof value === 'string' && value.length > 9;
  const sizeClass = isLongValue ? 'text-[22px]' : 'text-[28px]';

  return (
    <div className="flex min-w-0 flex-col gap-1">
      <span className={`num block truncate whitespace-nowrap font-semibold ${sizeClass} ${VALUE_COLOR_CLASS[variant]}`}>
        {value}
      </span>
      <span className="text-[11px] font-semibold uppercase tracking-[0.1em] text-muted">{label}</span>
    </div>
  );
}
