import type { ReactNode } from 'react';

interface ListRowProps {
  leading?: ReactNode;
  title: ReactNode;
  subtitle?: ReactNode;
  trailing?: ReactNode;
  onClick?: () => void;
  /** Kelas tambahan, mis. `min-h-[52px]` untuk layar dengan target sentuh lebih besar. */
  className?: string;
  /**
   * Override kelas pembungkus subtitle — default `block truncate` (1 baris).
   * Dipakai varian yang butuh lebih dari satu baris (mis. body notifikasi
   * 2 baris + waktu relatif, lihat features/notifications).
   */
  subtitleClassName?: string;
}

/** Baris list hairline (bukan kartu per item). Tinggi minimal 48px. */
export function ListRow({
  leading,
  title,
  subtitle,
  trailing,
  onClick,
  className = '',
  subtitleClassName = 'block truncate text-[12px] text-muted',
}: ListRowProps) {
  const content = (
    <>
      {leading && <span className="flex shrink-0 items-center text-muted">{leading}</span>}
      <span className="min-w-0 flex-1">
        <span className="block truncate text-[14px] text-ink">{title}</span>
        {subtitle && <span className={subtitleClassName}>{subtitle}</span>}
      </span>
      {trailing && <span className="flex shrink-0 items-center">{trailing}</span>}
    </>
  );

  if (onClick) {
    return (
      <button
        type="button"
        onClick={onClick}
        className={`flex min-h-12 w-full items-center gap-3 border-b border-line py-3 text-left transition-colors duration-150 hover:bg-surface-2 ${className}`}
      >
        {content}
      </button>
    );
  }

  return (
    <div className={`flex min-h-12 w-full items-center gap-3 border-b border-line py-3 ${className}`}>
      {content}
    </div>
  );
}
