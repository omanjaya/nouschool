import { Link } from 'react-router-dom';
import { Building2, ChevronLeft, ShieldAlert } from 'lucide-react';
import { Skeleton } from '../../components/ui/Skeleton';
import { EmptyState } from '../../components/ui/EmptyState';
import { ErrorState } from '../../components/ui/ErrorState';
import { formatRelativeTime } from '../../lib/date';
import { useMe } from '../auth/api';
import { useInterestLeads } from './api';
import { PAGE_WIDE } from '../../components/ui/page';

/** /admin/minat — daftar minat sekolah dari form landing page (super admin). */
export function InterestLeadsPage() {
  const { data: me } = useMe();
  const { data: leads, isLoading, isError, refetch } = useInterestLeads();

  if (me && !me.is_super_admin) {
    return (
      <div className={PAGE_WIDE}>
        <EmptyState icon={ShieldAlert} message="Anda tidak memiliki akses ke halaman ini." />
      </div>
    );
  }

  return (
    <div className={`${PAGE_WIDE} flex flex-col gap-6`}>
      <div>
        <Link
          to="/admin/sekolah"
          className="mb-3 inline-flex items-center gap-1 text-[12px] font-medium text-muted hover:text-ink"
        >
          <ChevronLeft size={16} strokeWidth={2} aria-hidden="true" />
          Sekolah
        </Link>
        <p className="text-[11px] font-semibold uppercase tracking-[0.1em] text-muted">Platform</p>
        <h1 className="text-[21px] font-semibold text-ink">Minat Sekolah</h1>
      </div>

      {isLoading ? (
        <div className="flex flex-col gap-2">
          <Skeleton className="h-16 w-full" />
          <Skeleton className="h-16 w-full" />
          <Skeleton className="h-16 w-full" />
        </div>
      ) : isError ? (
        <ErrorState message="Gagal memuat daftar minat sekolah." onRetry={() => refetch()} />
      ) : !leads || leads.length === 0 ? (
        <EmptyState icon={Building2} message="Belum ada sekolah yang mendaftar minat lewat landing page." />
      ) : (
        <div>
          {leads.map((lead, index) => (
            <div key={lead.id ?? index} className="flex flex-col gap-1 border-b border-line py-3">
              <div className="flex items-start justify-between gap-3">
                <p className="text-[14px] font-medium text-ink">{lead.school_name}</p>
                <span className="shrink-0 text-[12px] text-muted">{formatRelativeTime(lead.created_at)}</span>
              </div>
              <p className="text-[12px] text-muted">
                {lead.contact_name} · {lead.phone}
                {lead.email ? ` · ${lead.email}` : ''}
              </p>
              {lead.note && <p className="text-[13px] text-ink">{lead.note}</p>}
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
