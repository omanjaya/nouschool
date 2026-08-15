import type { ReactNode } from 'react';
import { ChevronLeft } from 'lucide-react';

interface AppBarProps {
  title: string;
  subtitle?: string;
  onBack?: () => void;
  action?: ReactNode;
  className?: string;
}

/** App bar halaman-dalam: tombol back + judul 17px + subjudul 12px muted. Tanpa app bar berwarna. */
export function AppBar({ title, subtitle, onBack, action, className = '' }: AppBarProps) {
  return (
    <header className={`flex items-center gap-3 border-b border-line bg-surface px-4 py-3 ${className}`}>
      {onBack && (
        <button
          type="button"
          onClick={onBack}
          aria-label="Kembali"
          className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg text-muted transition-colors duration-150 hover:bg-surface-2 hover:text-ink"
        >
          <ChevronLeft size={20} strokeWidth={2} aria-hidden="true" />
        </button>
      )}
      <div className="min-w-0 flex-1">
        <h1 className="truncate text-[17px] font-semibold text-ink">{title}</h1>
        {subtitle && <p className="truncate text-[12px] text-muted">{subtitle}</p>}
      </div>
      {action}
    </header>
  );
}
