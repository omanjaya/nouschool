import { useEffect, useState } from 'react';
import { Navigate } from 'react-router-dom';
import { ListRow } from '../../components/ui/ListRow';
import { StatusChip } from '../../components/ui/StatusChip';
import { StatTile } from '../../components/ui/StatTile';
import { SegmentedControl, type SegmentedOption } from '../../components/ui/SegmentedControl';
import { Skeleton } from '../../components/ui/Skeleton';
import { EmptyState } from '../../components/ui/EmptyState';
import { ErrorState } from '../../components/ui/ErrorState';
import { Field, Select } from '../../components/ui/Field';
import { formatDate, isoDateDaysAgo, todayISODate } from '../../lib/date';
import { useMe } from '../auth/api';
import { useChildren, useStudentAttendanceHistory } from './api';

type RangeOption = '30' | '180';

const RANGE_OPTIONS: SegmentedOption<RangeOption>[] = [
  { value: '30', label: '30 Hari' },
  { value: '180', label: 'Semester' },
];

function AttendanceHistoryBody({ studentId }: { studentId: string }) {
  const [range, setRange] = useState<RangeOption>('30');
  const to = todayISODate();
  const from = isoDateDaysAgo(Number(range), to);
  const { data, isLoading, isError, refetch } = useStudentAttendanceHistory(studentId, from, to);

  return (
    <div className="flex flex-col gap-5">
      <SegmentedControl options={RANGE_OPTIONS} value={range} onChange={setRange} />

      {isLoading ? (
        <div className="flex flex-col gap-2">
          <Skeleton className="h-20 w-full" />
          <Skeleton className="h-14 w-full" />
        </div>
      ) : isError ? (
        <ErrorState message="Gagal memuat riwayat kehadiran." onRetry={() => refetch()} />
      ) : data ? (
        <>
          <div className="grid grid-cols-3 gap-x-2 gap-y-4">
            <StatTile label="Hadir" value={data.counts.hadir} />
            <StatTile label="Terlambat" value={data.counts.terlambat} />
            <StatTile label="Izin" value={data.counts.izin} />
            <StatTile label="Sakit" value={data.counts.sakit} />
            <StatTile label="Alpa" value={data.counts.alpa} variant={data.counts.alpa > 0 ? 'danger' : 'default'} />
          </div>

          {data.items.length === 0 ? (
            <EmptyState message="Belum ada catatan kehadiran." />
          ) : (
            <div>
              {data.items.map((item, idx) => (
                <ListRow
                  key={`${item.date}-${idx}`}
                  title={formatDate(item.date)}
                  subtitle={item.note ?? undefined}
                  trailing={<StatusChip status={item.status} size="md" />}
                />
              ))}
            </div>
          )}
        </>
      ) : null}
    </div>
  );
}

function SiswaHistory({ studentId }: { studentId?: string | null }) {
  return (
    <div className="mx-auto flex max-w-[640px] flex-col gap-6 px-5 py-6">
      <div>
        <p className="text-[11px] font-semibold uppercase tracking-[0.1em] text-muted">Kehadiran</p>
        <h1 className="text-[21px] font-semibold text-ink">Riwayat Kehadiran</h1>
      </div>
      {studentId ? (
        <AttendanceHistoryBody studentId={studentId} />
      ) : (
        <ErrorState message="Data siswa untuk akun ini tidak ditemukan." />
      )}
    </div>
  );
}

function OrangTuaHistory() {
  const { data: children, isLoading, isError, refetch } = useChildren();
  const [selectedId, setSelectedId] = useState<string | undefined>(undefined);

  useEffect(() => {
    if (children && children.length > 0 && !selectedId) {
      setSelectedId(children[0].student_id);
    }
  }, [children, selectedId]);

  return (
    <div className="mx-auto flex max-w-[640px] flex-col gap-6 px-5 py-6">
      <div>
        <p className="text-[11px] font-semibold uppercase tracking-[0.1em] text-muted">Kehadiran</p>
        <h1 className="text-[21px] font-semibold text-ink">Kehadiran Anak</h1>
      </div>

      {isLoading ? (
        <Skeleton className="h-20 w-full" />
      ) : isError ? (
        <ErrorState message="Gagal memuat data anak." onRetry={() => refetch()} />
      ) : !children || children.length === 0 ? (
        <EmptyState message="Belum ada anak yang terhubung ke akun ini." />
      ) : (
        <>
          {children.length > 1 && (
            <Field label="Pilih anak">
              <Select value={selectedId} onChange={(e) => setSelectedId(e.target.value)}>
                {children.map((c) => (
                  <option key={c.student_id} value={c.student_id}>
                    {c.name} — {c.class_name}
                  </option>
                ))}
              </Select>
            </Field>
          )}
          {selectedId && <AttendanceHistoryBody studentId={selectedId} />}
        </>
      )}
    </div>
  );
}

/** /kehadiran — riwayat kehadiran untuk siswa (dirinya) & orang tua (pilih anak). */
export function AttendanceHistoryPage() {
  const { data: me } = useMe();

  if (!me) return null;
  if (me.role === 'siswa') return <SiswaHistory studentId={me.student_id} />;
  if (me.role === 'orang_tua') return <OrangTuaHistory />;
  return <Navigate to="/" replace />;
}
