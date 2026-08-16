import { useState } from 'react';
import { Plus, Trash2, UsersRound } from 'lucide-react';
import { ListRow } from '../../components/ui/ListRow';
import { Tag } from '../../components/ui/Tag';
import { Button } from '../../components/ui/Button';
import { Dialog } from '../../components/ui/Dialog';
import { Skeleton } from '../../components/ui/Skeleton';
import { EmptyState } from '../../components/ui/EmptyState';
import { ErrorState } from '../../components/ui/ErrorState';
import { useToast } from '../../components/ui/Toast';
import { ApiError } from '../../lib/api';
import { useDeleteDuty, useDuties, useUpdateDuty } from './api';
import { DutyFormDialog } from './DutyFormDialog';
import { ManageDutyAssigneesDialog } from './ManageDutyAssigneesDialog';
import type { Duty } from '../../lib/types';

const FOR_ROLE_LABEL: Record<Duty['for_role'], string> = {
  guru: 'Guru',
  pegawai: 'Pegawai',
};

/** Tab "Tugas" di /data — tugas tambahan (Wali Kelas, Guru BK, dst) + kapabilitas + petugas per TA aktif. */
export function DutiesListPage() {
  const { data: duties, isLoading, isError, refetch } = useDuties();
  const { showToast } = useToast();
  const [formOpen, setFormOpen] = useState(false);
  const [editing, setEditing] = useState<Duty | undefined>();
  const [managing, setManaging] = useState<Duty | undefined>();
  const [deleting, setDeleting] = useState<Duty | undefined>();
  const [conflictMessage, setConflictMessage] = useState<string | null>(null);
  const deleteDuty = useDeleteDuty();
  const deactivateDuty = useUpdateDuty(deleting?.id ?? '');

  function openCreate() {
    setEditing(undefined);
    setFormOpen(true);
  }

  function openEdit(d: Duty) {
    setEditing(d);
    setFormOpen(true);
  }

  function openDelete(d: Duty) {
    setDeleting(d);
    setConflictMessage(null);
    deleteDuty.reset();
  }

  function closeDelete() {
    setDeleting(undefined);
    setConflictMessage(null);
  }

  function handleDelete() {
    if (!deleting) return;
    deleteDuty.mutate(deleting.id, {
      onSuccess: () => {
        showToast('Tugas tambahan dihapus.');
        closeDelete();
      },
      onError: (err) => {
        if (err instanceof ApiError && err.status === 409) {
          setConflictMessage(err.message);
        } else {
          showToast(err instanceof ApiError ? err.message : 'Gagal menghapus tugas tambahan.', 'error');
        }
      },
    });
  }

  function handleDeactivate() {
    if (!deleting) return;
    deactivateDuty.mutate(
      { name: deleting.name, for_role: deleting.for_role, flags: deleting.flags, active: false },
      {
        onSuccess: () => {
          showToast('Tugas tambahan dinonaktifkan.');
          closeDelete();
        },
        onError: (err) => {
          showToast(err instanceof ApiError ? err.message : 'Gagal menonaktifkan tugas tambahan.', 'error');
        },
      },
    );
  }

  return (
    <div className="flex flex-col gap-6">
      <div className="flex flex-wrap items-center justify-end gap-2">
        <Button onClick={openCreate}>
          <Plus size={16} strokeWidth={2} aria-hidden="true" />
          Tambah Tugas
        </Button>
      </div>

      {isLoading ? (
        <div className="flex flex-col gap-2">
          <Skeleton className="h-12 w-full" />
          <Skeleton className="h-12 w-full" />
        </div>
      ) : isError ? (
        <ErrorState message="Gagal memuat daftar tugas tambahan." onRetry={() => refetch()} />
      ) : !duties || duties.length === 0 ? (
        <EmptyState
          icon={UsersRound}
          message="Belum ada tugas tambahan. Tambahkan mis. Wali Kelas atau Guru BK."
          action={
            <Button variant="secondary" onClick={openCreate}>
              Tambah Tugas
            </Button>
          }
        />
      ) : (
        <div>
          {duties.map((d) => (
            <ListRow
              key={d.id}
              title={d.name}
              subtitle={`${FOR_ROLE_LABEL[d.for_role]} · ${d.flags.length} kapabilitas · ${d.assignee_count} petugas`}
              onClick={() => openEdit(d)}
              trailing={
                <div className="flex items-center gap-1.5">
                  {!d.active && <Tag variant="done">Nonaktif</Tag>}
                  <Button
                    variant="secondary"
                    onClick={(e) => {
                      e.stopPropagation();
                      setManaging(d);
                    }}
                  >
                    <UsersRound size={16} strokeWidth={2} aria-hidden="true" />
                    Kelola Petugas
                  </Button>
                  <button
                    type="button"
                    onClick={(e) => {
                      e.stopPropagation();
                      openDelete(d);
                    }}
                    aria-label={`Hapus ${d.name}`}
                    className="flex h-9 w-9 items-center justify-center rounded-lg text-muted transition-colors duration-150 hover:bg-surface-2 hover:text-danger"
                  >
                    <Trash2 size={16} strokeWidth={2} aria-hidden="true" />
                  </button>
                </div>
              }
            />
          ))}
        </div>
      )}

      <DutyFormDialog open={formOpen} onClose={() => setFormOpen(false)} duty={editing} />
      <ManageDutyAssigneesDialog open={managing !== undefined} onClose={() => setManaging(undefined)} duty={managing} />

      <Dialog open={deleting !== undefined} onClose={closeDelete} title="Hapus tugas tambahan?">
        {conflictMessage ? (
          <div className="flex flex-col gap-3">
            <p className="text-[14px] text-ink">{conflictMessage}</p>
            <p className="text-[12px] text-muted">
              Tugas ini masih punya petugas aktif — nonaktifkan saja supaya tidak dipilih lagi, tanpa melepas petugas
              yang sudah dipasang.
            </p>
          </div>
        ) : (
          <p className="text-[14px] text-ink">
            &quot;{deleting?.name}&quot; akan dihapus permanen dari daftar tugas tambahan.
          </p>
        )}
        <div className="mt-4 flex justify-end gap-2">
          <Button type="button" variant="secondary" onClick={closeDelete}>
            Batal
          </Button>
          {conflictMessage ? (
            <Button type="button" loading={deactivateDuty.isPending} onClick={handleDeactivate}>
              Nonaktifkan
            </Button>
          ) : (
            <Button type="button" variant="danger" loading={deleteDuty.isPending} onClick={handleDelete}>
              Hapus
            </Button>
          )}
        </div>
      </Dialog>
    </div>
  );
}
