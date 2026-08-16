import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { BarChart3, Clock } from 'lucide-react';
import { Button } from '../../components/ui/Button';
import { Field, Input } from '../../components/ui/Field';
import { ListRow } from '../../components/ui/ListRow';
import { Skeleton } from '../../components/ui/Skeleton';
import { EmptyState } from '../../components/ui/EmptyState';
import { ErrorState } from '../../components/ui/ErrorState';
import { Tag } from '../../components/ui/Tag';
import { currentISOMonth, formatDateTime } from '../../lib/date';
import { useLateArrivals } from './api';
import { LATE_ARRIVAL_ACTION_LABEL, LATE_ARRIVAL_STATUS_LABEL, LATE_ARRIVAL_STATUS_TAG_VARIANT } from './format';

/**
 * Tab "Terlambat" di `/izin-siswa/terlambat` (`StudentLeaveAdminLayout`,
 * admin/kepsek) — seluruh keterlambatan sekolah bulan berjalan (`scope=all&month=`),
 * plus tautan ke rekap per siswa (docs/12-sion-parity.md Gelombang B alur 3).
 */
export function LateArrivalAdminPage() {
  const navigate = useNavigate();
  const [month, setMonth] = useState(currentISOMonth());
  const { data: items, isLoading, isError, refetch } = useLateArrivals('all', month);

  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-wrap items-end justify-between gap-3">
        <Field label="Bulan" htmlFor="late-arrival-admin-month" className="max-w-[180px]">
          <Input id="late-arrival-admin-month" type="month" value={month} onChange={(e) => setMonth(e.target.value)} />
        </Field>
        <Button variant="secondary" onClick={() => navigate('/izin-siswa/terlambat/rekap')}>
          <BarChart3 size={16} strokeWidth={2} aria-hidden="true" />
          Rekap per Siswa
        </Button>
      </div>

      {isLoading ? (
        <div className="flex flex-col gap-2">
          <Skeleton className="h-16 w-full" />
          <Skeleton className="h-16 w-full" />
        </div>
      ) : isError ? (
        <ErrorState message="Gagal memuat data keterlambatan." onRetry={() => refetch()} />
      ) : !items || items.length === 0 ? (
        <EmptyState icon={Clock} message="Belum ada keterlambatan tercatat bulan ini." />
      ) : (
        <div>
          {items.map((item) => (
            <ListRow
              key={item.id}
              className="min-h-[56px]"
              title={item.student?.name ?? `Keterlambatan ke-${item.late_count}`}
              subtitle={
                <span className="flex flex-col gap-0.5">
                  <span className="num">
                    {item.student ? `${item.student.nis} · ${item.student.class_name} · ` : ''}
                    {formatDateTime(item.arrived_at)}
                  </span>
                  <span>
                    {LATE_ARRIVAL_ACTION_LABEL[item.action]}
                    {item.action !== 'none' ? ` · ke-${item.late_count}` : ''}
                  </span>
                </span>
              }
              trailing={<Tag variant={LATE_ARRIVAL_STATUS_TAG_VARIANT[item.status]}>{LATE_ARRIVAL_STATUS_LABEL[item.status]}</Tag>}
            />
          ))}
        </div>
      )}
    </div>
  );
}
