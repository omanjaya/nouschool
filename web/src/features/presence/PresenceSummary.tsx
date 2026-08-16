import { useState } from 'react';
import { ChevronDown, ChevronUp, Circle } from 'lucide-react';
import { Skeleton } from '../../components/ui/Skeleton';
import { Tag } from '../../components/ui/Tag';
import { ListRow } from '../../components/ui/ListRow';
import { ROLE_LABEL } from '../../lib/roles';
import { usePresence } from './api';

/**
 * Seksi kecil "Sedang Online" (Fase 15 GAP 6b) — dipasang di TOP
 * `MonitoringPage` (satu-satunya pemakai, sudah digating admin/kepsek di
 * sana). Gagal diam-diam kalau error (bukan seksi utama halaman, jangan
 * menutupi konten monitoring dengan ErrorState) — cukup disembunyikan.
 */
export function PresenceSummary() {
  const { data, isLoading } = usePresence(true);
  const [expanded, setExpanded] = useState(false);

  if (isLoading) {
    return <Skeleton className="h-16 w-full" />;
  }

  if (!data) return null;

  const roleEntries = Object.entries(data.by_role).filter(([, count]) => count > 0);

  return (
    <div className="rounded-lg border border-line bg-surface-2 px-4 py-3">
      <button
        type="button"
        onClick={() => setExpanded((v) => !v)}
        className="flex w-full items-center justify-between gap-3 text-left"
        aria-expanded={expanded}
      >
        <div className="flex flex-wrap items-center gap-2">
          <span className="inline-flex items-center gap-1.5 text-[13px] font-semibold text-ink">
            <Circle size={8} strokeWidth={0} className="fill-st-hadir text-st-hadir" aria-hidden="true" />
            Sedang Online: {data.total}
          </span>
          {roleEntries.length > 0 && (
            <span className="flex flex-wrap items-center gap-1.5">
              {roleEntries.map(([role, count]) => (
                <Tag key={role} variant="neutral">
                  {ROLE_LABEL[role] ?? role} {count}
                </Tag>
              ))}
            </span>
          )}
        </div>
        {data.users.length > 0 &&
          (expanded ? (
            <ChevronUp size={16} strokeWidth={2} className="shrink-0 text-muted" aria-hidden="true" />
          ) : (
            <ChevronDown size={16} strokeWidth={2} className="shrink-0 text-muted" aria-hidden="true" />
          ))}
      </button>

      {expanded && data.users.length > 0 && (
        <div className="mt-2 border-t border-line pt-1">
          {data.users.map((u) => (
            <ListRow key={u.user_id} title={u.name} subtitle={ROLE_LABEL[u.role] ?? u.role} className="min-h-10 py-2" />
          ))}
        </div>
      )}
    </div>
  );
}
