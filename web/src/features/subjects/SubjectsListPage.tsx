import { useState } from 'react';
import { BookOpen, Plus } from 'lucide-react';
import { ListRow } from '../../components/ui/ListRow';
import { DataTable, type DataTableColumn } from '../../components/ui/DataTable';
import { Skeleton } from '../../components/ui/Skeleton';
import { EmptyState } from '../../components/ui/EmptyState';
import { ErrorState } from '../../components/ui/ErrorState';
import { Button } from '../../components/ui/Button';
import { useSubjects } from './api';
import { SubjectFormDialog } from './SubjectFormDialog';
import type { Subject } from '../../lib/types';

export function SubjectsListPage() {
  const { data: subjects, isLoading, isError, refetch } = useSubjects();
  const [dialogOpen, setDialogOpen] = useState(false);
  const [editing, setEditing] = useState<Subject | undefined>();

  function openEdit(s: Subject) {
    setEditing(s);
    setDialogOpen(true);
  }

  function openCreate() {
    setEditing(undefined);
    setDialogOpen(true);
  }

  /** Kolom desktop (docs/10 §5) — mobile tetap `ListRow` di bawah. */
  const columns: DataTableColumn<Subject>[] = [
    { key: 'name', header: 'Nama', sortable: true, sortValue: (s) => s.name, cell: (s) => <span className="font-medium text-ink">{s.name}</span> },
    { key: 'code', header: 'Kode', sortable: true, sortValue: (s) => s.code, cell: (s) => s.code },
  ];

  return (
    <div className="flex flex-col gap-6">
      <div className="flex items-center justify-end">
        <Button onClick={openCreate}>
          <Plus size={16} strokeWidth={2} aria-hidden="true" />
          Tambah Mapel
        </Button>
      </div>

      {isLoading ? (
        <div className="flex flex-col gap-2">
          <Skeleton className="h-12 w-full" />
          <Skeleton className="h-12 w-full" />
          <Skeleton className="h-12 w-full" />
        </div>
      ) : isError ? (
        <ErrorState message="Gagal memuat daftar mata pelajaran." onRetry={() => refetch()} />
      ) : subjects && subjects.length === 0 ? (
        <EmptyState
          icon={BookOpen}
          message="Belum ada mata pelajaran."
          action={
            <Button variant="secondary" onClick={openCreate}>
              Tambah Mapel
            </Button>
          }
        />
      ) : (
        <>
          <div className="lg:hidden">
            {subjects?.map((s) => (
              <ListRow key={s.id} title={s.name} subtitle={s.code} onClick={() => openEdit(s)} />
            ))}
          </div>
          <div className="hidden lg:block">
            <DataTable
              columns={columns}
              data={subjects ?? []}
              keyField={(s) => s.id}
              onRowClick={(s) => openEdit(s)}
              emptyIcon={BookOpen}
              emptyMessage="Belum ada mata pelajaran."
            />
          </div>
        </>
      )}

      <SubjectFormDialog open={dialogOpen} onClose={() => setDialogOpen(false)} subject={editing} />
    </div>
  );
}
