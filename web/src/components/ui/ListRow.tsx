import type { ReactNode } from 'react';

interface ListRowProps {
  leading?: ReactNode;
  title: ReactNode;
  subtitle?: ReactNode;
  trailing?: ReactNode;
  onClick?: () => void;
}

/** Baris list hairline (bukan kartu per item). Tinggi minimal 48px. */
export function ListRow({ leading, title, subtitle, trailing, onClick }: ListRowProps) {
  const content = (
    <>
      {leading && <span className="flex shrink-0 items-center text-muted">{leading}</span>}
      <span className="min-w-0 flex-1">
        <span className="block truncate text-[14px] text-ink">{title}</span>
        {subtitle && <span className="block truncate text-[12px] text-muted">{subtitle}</span>}
      </span>
      {trailing && <span className="flex shrink-0 items-center">{trailing}</span>}
    </>
  );

  if (onClick) {
    return (
      <button
        type="button"
        onClick={onClick}
        className="flex min-h-12 w-full items-center gap-3 border-b border-line py-3 text-left transition-colors duration-150 hover:bg-surface-2"
      >
        {content}
      </button>
    );
  }

  return (
    <div className="flex min-h-12 w-full items-center gap-3 border-b border-line py-3">
      {content}
    </div>
  );
}
