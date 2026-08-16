import { useEffect, useState } from 'react';
import { Navigate } from 'react-router-dom';
import { Star } from 'lucide-react';
import { StatTile } from '../../components/ui/StatTile';
import { ListRow } from '../../components/ui/ListRow';
import { Field, Select } from '../../components/ui/Field';
import { Skeleton } from '../../components/ui/Skeleton';
import { EmptyState } from '../../components/ui/EmptyState';
import { ErrorState } from '../../components/ui/ErrorState';
import { formatRelativeTime } from '../../lib/date';
import { useMe } from '../auth/api';
import { useGradingChildren, useGradingStatus, useMyStars } from './api';

function MyStarsBody({ studentId }: { studentId?: string }) {
  const { data, isLoading, isError, refetch } = useMyStars(studentId);

  if (isLoading) {
    return (
      <div className="flex flex-col gap-3">
        <Skeleton className="h-20 w-full" />
        <Skeleton className="h-14 w-full" />
      </div>
    );
  }

  if (isError || !data) {
    return <ErrorState message="Gagal memuat bintang kelas." onRetry={() => refetch()} />;
  }

  return (
    <div className="flex flex-col gap-6">
      <StatTile label="Total Bintang" value={data.total} />

      {data.items.length === 0 ? (
        <EmptyState icon={Star} message="Belum ada bintang tercatat. Terus tunjukkan performa terbaikmu!" />
      ) : (
        <div>
          {data.items.map((item, i) => (
            <ListRow
              key={i}
              title={item.note || (item.delta > 0 ? 'Bintang diberikan' : 'Bintang dikurangi')}
              subtitle={`${item.given_by_name} · ${formatRelativeTime(item.created_at)}`}
              trailing={
                <span className={`num text-[16px] font-bold ${item.delta > 0 ? 'text-st-hadir' : 'text-danger'}`}>
                  {item.delta > 0 ? `+${item.delta}` : item.delta}
                </span>
              }
            />
          ))}
        </div>
      )}
    </div>
  );
}

/** /bintang-saya — bintang kelas siswa (dirinya) & orang tua (pilih anak), pola sama `MyDisciplinePage`. */
export function MyStarsPage() {
  const { data: me } = useMe();
  const { data: status, isLoading: statusLoading, isError: statusError, refetch: refetchStatus } = useGradingStatus();
  const { data: children, isLoading: childrenLoading, isError: childrenError, refetch: refetchChildren } = useGradingChildren(
    me?.role === 'orang_tua',
  );
  const [selectedChild, setSelectedChild] = useState<string | undefined>();

  useEffect(() => {
    if (children && children.length > 0 && !selectedChild) setSelectedChild(children[0].student_id);
  }, [children, selectedChild]);

  if (!me) return null;
  if (me.role !== 'siswa' && me.role !== 'orang_tua') return <Navigate to="/" replace />;

  return (
    <div className="mx-auto flex max-w-[640px] flex-col gap-6 px-5 py-6">
      <div>
        <p className="text-[11px] font-semibold uppercase tracking-[0.1em] text-muted">Penilaian</p>
        <h1 className="text-[21px] font-semibold text-ink">{me.role === 'siswa' ? 'Bintang Kelas' : 'Bintang Anak'}</h1>
      </div>

      {statusLoading ? (
        <Skeleton className="h-40 w-full" />
      ) : statusError ? (
        <ErrorState message="Gagal memuat status modul penilaian." onRetry={() => refetchStatus()} />
      ) : !status?.enabled ? (
        <EmptyState icon={Star} message="Modul penilaian tidak aktif." />
      ) : me.role === 'siswa' ? (
        <MyStarsBody />
      ) : childrenLoading ? (
        <Skeleton className="h-20 w-full" />
      ) : childrenError ? (
        <ErrorState message="Gagal memuat data anak." onRetry={() => refetchChildren()} />
      ) : !children || children.length === 0 ? (
        <EmptyState message="Belum ada anak yang terhubung ke akun ini." />
      ) : (
        <>
          {children.length > 1 && (
            <Field label="Pilih anak">
              <Select value={selectedChild} onChange={(e) => setSelectedChild(e.target.value)}>
                {children.map((c) => (
                  <option key={c.student_id} value={c.student_id}>
                    {c.name} — {c.class_name}
                  </option>
                ))}
              </Select>
            </Field>
          )}
          {selectedChild && <MyStarsBody studentId={selectedChild} />}
        </>
      )}
    </div>
  );
}
