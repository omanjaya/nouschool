import type { ReactNode } from 'react';

interface StatTileProps {
  label: string;
  value: ReactNode;
  variant?: 'default' | 'danger';
}

/** Angka besar 28px tabular + label eyebrow — dipakai di ringkasan dashboard/import. */
export function StatTile({ label, value, variant = 'default' }: StatTileProps) {
  return (
    <div className="flex flex-col gap-1">
      <span className={`num text-[28px] font-semibold ${variant === 'danger' ? 'text-danger' : 'text-ink'}`}>
        {value}
      </span>
      <span className="text-[11px] font-semibold uppercase tracking-[0.1em] text-muted">{label}</span>
    </div>
  );
}
