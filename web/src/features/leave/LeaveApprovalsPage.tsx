import { useNavigate } from 'react-router-dom';
import { ClipboardCheck } from 'lucide-react';
import { AppBar } from '../../components/ui/AppBar';
import { ListRow } from '../../components/ui/ListRow';
import { Skeleton } from '../../components/ui/Skeleton';
import { EmptyState } from '../../components/ui/EmptyState';
import { ErrorState } from '../../components/ui/ErrorState';
import { formatDateRange } from '../../lib/date';
import { useLeaveApprovals } from './api';

/** /izin/persetujuan — antrian pengajuan izin menunggu keputusan user ini (kepsek/admin/approver spesifik). */
export function LeaveApprovalsPage() {
  const navigate = useNavigate();
  const { data: items, isLoading, isError, refetch } = useLeaveApprovals();

  return (
    <div className="flex min-h-dvh flex-col">
      <AppBar title="Antrian Persetujuan" subtitle="Izin" onBack={() => navigate('/izin')} />

      <div className="mx-auto w-full max-w-[640px] flex-1 px-5 py-5">
        {isLoading ? (
          <div className="flex flex-col gap-2">
            <Skeleton className="h-16 w-full" />
            <Skeleton className="h-16 w-full" />
          </div>
        ) : isError ? (
          <ErrorState message="Gagal memuat antrian persetujuan." onRetry={() => refetch()} />
        ) : !items || items.length === 0 ? (
          <EmptyState icon={ClipboardCheck} message="Tidak ada pengajuan izin yang menunggu persetujuan Anda." />
        ) : (
          <div>
            {items.map((item) => (
              <ListRow
                key={item.step_id}
                className="min-h-[56px]"
                title={item.request.teacher.name}
                subtitle={
                  <span className="flex flex-col gap-0.5">
                    <span className="num">
                      {item.request.type_label} · {formatDateRange(item.request.date_start, item.request.date_end)}
                    </span>
                    <span className="truncate">{item.request.reason}</span>
                  </span>
                }
                onClick={() => navigate(`/izin/persetujuan/${item.step_id}`)}
              />
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
