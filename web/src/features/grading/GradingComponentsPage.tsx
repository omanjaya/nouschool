import { useState } from 'react';
import { useOutletContext } from 'react-router-dom';
import { Info, Plus, Trash2 } from 'lucide-react';
import { Button } from '../../components/ui/Button';
import { ListRow } from '../../components/ui/ListRow';
import { Dialog } from '../../components/ui/Dialog';
import { Skeleton } from '../../components/ui/Skeleton';
import { EmptyState } from '../../components/ui/EmptyState';
import { ErrorState } from '../../components/ui/ErrorState';
import { useToast } from '../../components/ui/Toast';
import { ApiError } from '../../lib/api';
import { useDeleteGradingComponent, useGradingComponents } from './api';
import { GradingComponentFormDialog } from './GradingComponentFormDialog';
import { GRADING_TYPE_LABEL } from './format';
import type { GradingContext } from './GradingPage';
import type { GradingComponent } from '../../lib/types';

/** Tab "Komponen" `/nilai` — daftar komponen nilai kelas+mapel terpilih, CRUD + indikator total bobot. */
export function GradingComponentsPage() {
  const { classId, subjectId } = useOutletContext<GradingContext>();
  const { showToast } = useToast();
  const [formOpen, setFormOpen] = useState(false);
  const [editing, setEditing] = useState<GradingComponent | undefined>();
  const [deleting, setDeleting] = useState<GradingComponent | undefined>();

  const hasContext = Boolean(classId && subjectId);
  const { data, isLoading, isError, refetch } = useGradingComponents({ classId, subjectId }, hasContext);
  const deleteComponent = useDeleteGradingComponent();

  function openCreate() {
    setEditing(undefined);
    setFormOpen(true);
  }

  function openEdit(c: GradingComponent) {
    setEditing(c);
    setFormOpen(true);
  }

  function handleDelete() {
    if (!deleting) return;
    deleteComponent.mutate(deleting.id, {
      onSuccess: () => {
        showToast('Komponen nilai dihapus.');
        setDeleting(undefined);
      },
      onError: (err) => {
        showToast(err instanceof ApiError ? err.message : 'Gagal menghapus komponen nilai.', 'error');
      },
    });
  }

  if (!hasContext) {
    return <EmptyState message="Pilih kelas & mata pelajaran terlebih dahulu." />;
  }

  return (
    <div className="flex flex-col gap-4">
      <div className="flex justify-end">
        <Button onClick={openCreate}>
          <Plus size={16} strokeWidth={2} aria-hidden="true" />
          Tambah Komponen
        </Button>
      </div>

      {isLoading ? (
        <div className="flex flex-col gap-2">
          <Skeleton className="h-14 w-full" />
          <Skeleton className="h-14 w-full" />
        </div>
      ) : isError ? (
        <ErrorState message="Gagal memuat komponen nilai." onRetry={() => refetch()} />
      ) : !data || data.items.length === 0 ? (
        <EmptyState
          message="Belum ada komponen nilai untuk kelas & mapel ini."
          action={
            <Button variant="secondary" onClick={openCreate}>
              Tambah Komponen
            </Button>
          }
        />
      ) : (
        <>
          <div>
            {data.items.map((c) => (
              <ListRow
                key={c.id}
                title={c.name}
                onClick={() => openEdit(c)}
                subtitle={`${GRADING_TYPE_LABEL[c.type]} · Bobot ${c.weight} · KKTP ${c.kktp} · Terisi ${c.graded_count}/${c.student_count}`}
                trailing={
                  <button
                    type="button"
                    onClick={(e) => {
                      e.stopPropagation();
                      setDeleting(c);
                    }}
                    aria-label={`Hapus ${c.name}`}
                    className="flex h-9 w-9 items-center justify-center rounded-lg text-muted transition-colors duration-150 hover:bg-surface-2 hover:text-danger"
                  >
                    <Trash2 size={16} strokeWidth={2} aria-hidden="true" />
                  </button>
                }
              />
            ))}
          </div>

          <div className="flex items-start gap-2 rounded-lg border border-line bg-surface-2 px-3 py-2.5">
            <Info size={16} strokeWidth={2} className="mt-0.5 shrink-0 text-muted" aria-hidden="true" />
            <p className="text-[12px] text-muted">
              Total bobot <span className="num font-semibold text-ink">{data.total_weight}</span>
              {data.total_weight !== 100 &&
                ' — nilai akhir dinormalisasi otomatis karena bobot tidak berjumlah 100.'}
            </p>
          </div>
        </>
      )}

      <GradingComponentFormDialog
        open={formOpen}
        onClose={() => setFormOpen(false)}
        classId={classId}
        subjectId={subjectId}
        component={editing}
      />

      <Dialog open={deleting !== undefined} onClose={() => setDeleting(undefined)} title="Hapus komponen nilai?">
        <p className="text-[14px] text-ink">
          &quot;{deleting?.name}&quot; akan dihapus — nilai yang sudah diisi untuk komponen ini ikut terhapus.
        </p>
        <div className="mt-4 flex justify-end gap-2">
          <Button variant="secondary" onClick={() => setDeleting(undefined)}>
            Batal
          </Button>
          <Button variant="danger" loading={deleteComponent.isPending} onClick={handleDelete}>
            Hapus
          </Button>
        </div>
      </Dialog>
    </div>
  );
}
