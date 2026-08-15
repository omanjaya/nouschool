import type { AttendanceStatus } from '../../lib/types';

type StatusChipSize = 'sm' | 'md';

interface StatusConfig {
  letter: string;
  label: string;
  colorClass: string;
}

const STATUS_CONFIG: Record<AttendanceStatus, StatusConfig> = {
  hadir: { letter: 'H', label: 'Hadir', colorClass: 'border-st-hadir-line text-st-hadir' },
  terlambat: { letter: 'T', label: 'Terlambat', colorClass: 'border-st-terlambat-line text-st-terlambat' },
  izin: { letter: 'I', label: 'Izin', colorClass: 'border-st-izin-line text-st-izin' },
  sakit: { letter: 'S', label: 'Sakit', colorClass: 'border-st-sakit-line text-st-sakit' },
  alpa: { letter: 'A', label: 'Alpa', colorClass: 'border-st-alpa-line text-st-alpa' },
};

interface StatusChipProps {
  status: AttendanceStatus;
  /** sm = huruf tunggal (mis. ringkasan), md = label penuh (mis. baris siswa). */
  size?: StatusChipSize;
  /** Angka pendamping (tabular) — dipakai bar ringkasan absensi. */
  count?: number;
  /** Kalau diisi, chip jadi tombol (dipakai untuk siklus status tap-to-cycle). */
  onClick?: () => void;
  className?: string;
}

/**
 * Status kehadiran: outline + warna semantik token `--st-*`. Ukuran `sm`
 * (huruf H/T/I/S/A) & `md` (label penuh). Saat interaktif (`onClick`), target
 * sentuh dinaikkan ke minimal 44px sesuai docs/10-design-system.md #3.
 */
export function StatusChip({ status, size = 'md', count, onClick, className = '' }: StatusChipProps) {
  const cfg = STATUS_CONFIG[status];
  const isInteractive = Boolean(onClick);
  const label = size === 'sm' ? cfg.letter : cfg.label;

  const sizeClass =
    size === 'sm'
      ? 'h-6 min-w-6 gap-1 px-1.5 text-[11px]'
      : isInteractive
        ? 'min-h-11 gap-1.5 px-4 text-[14px]'
        : 'h-7 gap-1 px-3 text-[13px]';

  const base = `inline-flex items-center justify-center rounded-full border font-semibold ${cfg.colorClass} ${sizeClass}`;

  const content = (
    <>
      <span>{label}</span>
      {count !== undefined && <span className="num">{count}</span>}
    </>
  );

  if (onClick) {
    return (
      <button
        type="button"
        onClick={onClick}
        aria-label={`Status kehadiran: ${cfg.label}. Ketuk untuk ubah.`}
        className={`${base} transition-colors duration-150 hover:bg-surface-2 ${className}`}
      >
        {content}
      </button>
    );
  }

  return (
    <span className={`${base} ${className}`} aria-label={`Status kehadiran: ${cfg.label}`}>
      {content}
    </span>
  );
}
