import { useState } from 'react';
import { Navigate, useNavigate } from 'react-router-dom';
import { CalendarClock } from 'lucide-react';
import { ListRow } from '../../components/ui/ListRow';
import { Skeleton } from '../../components/ui/Skeleton';
import { EmptyState } from '../../components/ui/EmptyState';
import { ErrorState } from '../../components/ui/ErrorState';
import { Tag } from '../../components/ui/Tag';
import { SegmentedControl, type SegmentedOption } from '../../components/ui/SegmentedControl';
import { Select } from '../../components/ui/Field';
import { formatDateRange } from '../../lib/date';
import { useMe } from '../auth/api';
import { useStudentLeaveRequests, type StudentLeaveScope } from './api';
import { STUDENT_LEAVE_STATUS_LABEL, STUDENT_LEAVE_STATUS_TAG_VARIANT, STUDENT_LEAVE_TYPE_LABEL } from './format';
import type { StudentLeaveStatus } from '../../lib/types';

type Tab = 'queue' | 'all';

const TAB_OPTIONS: SegmentedOption<Tab>[] = [
  { value: 'queue', label: 'Menunggu Saya' },
  { value: 'all', label: 'Semua' },
];

const STATUS_OPTIONS: { value: '' | StudentLeaveStatus; label: string }[] = [
  { value: '', label: 'Semua status' },
  { value: 'pending_homeroom', label: STUDENT_LEAVE_STATUS_LABEL.pending_homeroom },
  { value: 'pending_bk', label: STUDENT_LEAVE_STATUS_LABEL.pending_bk },
  { value: 'issued', label: STUDENT_LEAVE_STATUS_LABEL.issued },
  { value: 'rejected', label: STUDENT_LEAVE_STATUS_LABEL.rejected },
  { value: 'canceled', label: STUDENT_LEAVE_STATUS_LABEL.canceled },
];

/**
 * /izin-siswa — antrian review izin siswa untuk guru. Wali kelas & guru BK
 * adalah guru biasa (bukan role terpisah) — otorisasi lewat duty-capability
 * flags di backend; frontend cukup memanggil `scope=queue` (200, mungkin
 * kosong bila guru ini tidak memegang tugas terkait). Tab "Semua" (scope=all
 * + filter status) khusus admin_sekolah untuk pengawasan.
 */
export function StudentLeaveQueuePage() {
  const { data: me } = useMe();
  const navigate = useNavigate();
  const isAdmin = me?.role === 'admin_sekolah';
  const [tab, setTab] = useState<Tab>('queue');
  const [status, setStatus] = useState<'' | StudentLeaveStatus>('');

  const scope: StudentLeaveScope = tab === 'all' && isAdmin ? 'all' : 'queue';
  const { data: items, isLoading, isError, refetch } = useStudentLeaveRequests(
    scope,
    scope === 'all' ? status || undefined : undefined,
  );

  if (me && me.role !== 'guru' && me.role !== 'admin_sekolah') {
    return <Navigate to="/" replace />;
  }

  return (
    <div className="mx-auto flex max-w-[640px] flex-col gap-6 px-5 py-6">
      <div>
        <p className="text-[11px] font-semibold uppercase tracking-[0.1em] text-muted">Izin</p>
        <h1 className="text-[21px] font-semibold text-ink">Izin Siswa</h1>
      </div>

      {isAdmin && <SegmentedControl options={TAB_OPTIONS} value={tab} onChange={setTab} />}

      {scope === 'all' && (
        <Select value={status} onChange={(e) => setStatus(e.target.value as '' | StudentLeaveStatus)} aria-label="Filter status">
          {STATUS_OPTIONS.map((opt) => (
            <option key={opt.value} value={opt.value}>
              {opt.label}
            </option>
          ))}
        </Select>
      )}

      {isLoading ? (
        <div className="flex flex-col gap-2">
          <Skeleton className="h-16 w-full" />
          <Skeleton className="h-16 w-full" />
        </div>
      ) : isError ? (
        <ErrorState message="Gagal memuat antrian izin siswa." onRetry={() => refetch()} />
      ) : !items || items.length === 0 ? (
        <EmptyState
          icon={CalendarClock}
          message={
            scope === 'queue' ? 'Tidak ada pengajuan izin yang menunggu Anda.' : 'Belum ada pengajuan izin siswa.'
          }
        />
      ) : (
        <div>
          {items.map((item) => (
            <ListRow
              key={item.id}
              className="min-h-[56px]"
              title={item.student.name}
              subtitle={
                <span className="flex flex-col gap-0.5">
                  <span className="num">
                    {STUDENT_LEAVE_TYPE_LABEL[item.type]} · {formatDateRange(item.date_start, item.date_end)} ·{' '}
                    {item.student.class_name}
                  </span>
                  <span className="truncate">{item.reason}</span>
                </span>
              }
              trailing={<Tag variant={STUDENT_LEAVE_STATUS_TAG_VARIANT[item.status]}>{STUDENT_LEAVE_STATUS_LABEL[item.status]}</Tag>}
              onClick={() => navigate(`/izin-siswa/${item.id}`, { state: { scope } })}
            />
          ))}
        </div>
      )}
    </div>
  );
}
