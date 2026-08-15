import { Inbox, type LucideIcon } from 'lucide-react';
import type { ReactNode } from 'react';

interface EmptyStateProps {
  icon?: LucideIcon;
  message: string;
  action?: ReactNode;
}

/** WAJIB dipakai untuk setiap list yang bisa kosong — kalimat kontekstual, bukan generik. */
export function EmptyState({ icon: Icon = Inbox, message, action }: EmptyStateProps) {
  return (
    <div className="flex flex-col items-center gap-3 px-6 py-10 text-center">
      <Icon size={24} strokeWidth={2} className="text-muted" aria-hidden="true" />
      <p className="text-[14px] text-muted">{message}</p>
      {action}
    </div>
  );
}
