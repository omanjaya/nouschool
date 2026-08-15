import type { ReactNode } from 'react';

type TagVariant = 'now' | 'done' | 'neutral' | 'danger' | 'success' | 'warning' | 'info';

const VARIANT_CLASS: Record<TagVariant, string> = {
  now: 'bg-primary-soft text-primary',
  done: 'bg-surface-2 text-muted',
  neutral: 'bg-surface-2 text-ink',
  danger: 'bg-danger-soft text-danger',
  /** Status semantik "baik" (mis. izin disetujui, guru sedang mengajar) — token --st-hadir. */
  success: 'bg-st-hadir-line text-st-hadir',
  /** Status semantik "menunggu/perlu perhatian" (mis. izin pending) — token --st-terlambat. */
  warning: 'bg-st-terlambat-line text-st-terlambat',
  /** Status semantik netral-informatif (mis. guru berstatus izin) — token --st-izin. */
  info: 'bg-st-izin-line text-st-izin',
};

interface TagProps {
  variant?: TagVariant;
  children: ReactNode;
}

/** Label kecil pill: `now` (primary-soft), `done` (surface-2), netral. */
export function Tag({ variant = 'neutral', children }: TagProps) {
  return (
    <span className={`inline-flex items-center rounded-full px-2.5 py-1 text-[12px] font-medium ${VARIANT_CLASS[variant]}`}>
      {children}
    </span>
  );
}
