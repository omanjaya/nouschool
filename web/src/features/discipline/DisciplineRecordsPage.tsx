import { useState } from 'react';
import { ShieldAlert, Trash2 } from 'lucide-react';
import { ListRow } from '../../components/ui/ListRow';
import { Tag } from '../../components/ui/Tag';
import { Button } from '../../components/ui/Button';
import { Dialog } from '../../components/ui/Dialog';
import { Skeleton } from '../../components/ui/Skeleton';
import { EmptyState } from '../../components/ui/EmptyState';
import { ErrorState } from '../../components/ui/ErrorState';
import { SegmentedControl, type SegmentedOption } from '../../components/ui/SegmentedControl';
import { Select } from '../../components/ui/Field';
import { useToast } from '../../components/ui/Toast';
import { ApiError } from '../../lib/api';
import { formatDate, isoDateDaysAgo, todayISODate } from '../../lib/date';
import { useMe } from '../auth/api';
import { useClasses } from '../classes/api';
import { useDeleteDisciplineRecord, useDisciplineRecords } from './api';
import type { DisciplineRecord } from '../../lib/types';

type RangeOption = '7' | '30';

const RANGE_OPTIONS: SegmentedOption<RangeOption>[] = [
  { value: '7', label: '7 Hari' },
  { value: '30', label: '30 Hari' },
];

/** Tab "Catatan" (default) — daftar pelanggaran tercatat, filter rombel + rentang tanggal. */
export function DisciplineRecordsPage() {
  const { data: me } = useMe();
  const { showToast } = useToast();
  const { data: classes } = useClasses();
  const [classId, setClassId] = useState('');
  const [range, setRange] = useState<RangeOption>('7');
  const [deleting, setDeleting] = useState<DisciplineRecord | undefined>();

  const to = todayISODate();
  const from = isoDateDaysAgo(Number(range), to);

  const { data, isLoading, isError, refetch, hasNextPage, fetchNextPage, isFetchingNextPage } = useDisciplineRecords({
    classId: classId || undefined,
    from,
    to,
  });
  const deleteRecord = useDeleteDisciplineRecord();

  const isAdmin = me?.role === 'admin_sekolah';
  const items = data?.pages.flatMap((p) => p.items) ?? [];

  function handleDelete() {
    if (!deleting) return;
    deleteRecord.mutate(deleting.id, {
      onSuccess: () => {
        setDeleting(undefined);
        showToast('Catatan pelanggaran dihapus.');
      },
      onError: (err) => {
        showToast(err instanceof ApiError ? err.message : 'Gagal menghapus catatan.', 'error');
      },
    });
  }

  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <SegmentedControl options={RANGE_OPTIONS} value={range} onChange={setRange} />
        <div className="sm:w-56">
          <Select value={classId} onChange={(e) => setClassId(e.target.value)} aria-label="Filter rombel">
            <option value="">Semua rombel</option>
            {classes?.map((c) => (
              <option key={c.id} value={c.id}>
                {c.name}
              </option>
            ))}
          </Select>
        </div>
      </div>

      {isLoading ? (
        <div className="flex flex-col gap-2">
          <Skeleton className="h-16 w-full" />
          <Skeleton className="h-16 w-full" />
          <Skeleton className="h-16 w-full" />
        </div>
      ) : isError ? (
        <ErrorState message="Gagal memuat catatan pelanggaran." onRetry={() => refetch()} />
      ) : items.length === 0 ? (
        <EmptyState icon={ShieldAlert} message="Belum ada catatan pelanggaran pada rentang ini." />
      ) : (
        <div className="flex flex-col gap-3">
          <div>
            {items.map((r) => (
              <ListRow
                key={r.id}
                className="min-h-[56px]"
                title={r.student.name}
                subtitle={
                  <span className="flex flex-col gap-0.5">
                    <span className="truncate">
                      {r.violation.name} · {r.student.class_name} · {r.noted_by_name} · {formatDate(r.occurred_on)}
                    </span>
                    {r.note && <span className="truncate">{r.note}</span>}
                  </span>
                }
                trailing={
                  <div className="flex items-center gap-1.5">
                    {/* Tag "+N poin" dipakai token `danger` (sama hex dgn --st-alpa, docs/10) — Tag belum punya varian st-alpa tersendiri. */}
                    <Tag variant="danger">+{r.violation.points} poin</Tag>
                    {isAdmin && (
                      <button
                        type="button"
                        onClick={() => setDeleting(r)}
                        aria-label={`Hapus catatan ${r.student.name}`}
                        className="flex h-9 w-9 items-center justify-center rounded-lg text-muted transition-colors duration-150 hover:bg-surface-2 hover:text-danger"
                      >
                        <Trash2 size={16} strokeWidth={2} aria-hidden="true" />
                      </button>
                    )}
                  </div>
                }
              />
            ))}
          </div>

          {hasNextPage && (
            <Button
              variant="secondary"
              onClick={() => fetchNextPage()}
              loading={isFetchingNextPage}
              className="self-center"
            >
              Muat lebih banyak
            </Button>
          )}
        </div>
      )}

      <Dialog open={deleting !== undefined} onClose={() => setDeleting(undefined)} title="Hapus catatan pelanggaran?">
        <p className="text-[14px] text-ink">
          Catatan &quot;{deleting?.violation.name}&quot; untuk {deleting?.student.name} akan dihapus permanen.
        </p>
        {deleteRecord.isError && (
          <p className="mt-3 text-[12px] text-danger">
            {deleteRecord.error instanceof ApiError ? deleteRecord.error.message : 'Gagal menghapus catatan.'}
          </p>
        )}
        <div className="mt-4 flex justify-end gap-2">
          <Button type="button" variant="secondary" onClick={() => setDeleting(undefined)}>
            Batal
          </Button>
          <Button type="button" variant="danger" loading={deleteRecord.isPending} onClick={handleDelete}>
            Hapus
          </Button>
        </div>
      </Dialog>
    </div>
  );
}
