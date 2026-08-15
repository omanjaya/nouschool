import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { FileSpreadsheet, Plus, Upload, UserRound } from 'lucide-react';
import { ListRow } from '../../components/ui/ListRow';
import { Skeleton } from '../../components/ui/Skeleton';
import { EmptyState } from '../../components/ui/EmptyState';
import { ErrorState } from '../../components/ui/ErrorState';
import { Button } from '../../components/ui/Button';
import { importTemplateUrl } from '../import/api';
import { useTeachers } from './api';
import { TeacherFormDialog } from './TeacherFormDialog';
import type { Teacher } from '../../lib/types';

export function TeachersListPage() {
  const { data: teachers, isLoading, isError, refetch } = useTeachers();
  const [dialogOpen, setDialogOpen] = useState(false);
  const [editing, setEditing] = useState<Teacher | undefined>();
  const navigate = useNavigate();

  function openEdit(t: Teacher) {
    setEditing(t);
    setDialogOpen(true);
  }

  function openCreate() {
    setEditing(undefined);
    setDialogOpen(true);
  }

  return (
    <div className="flex flex-col gap-6">
      <div className="flex flex-wrap items-center justify-end gap-2">
        <Button variant="secondary" onClick={() => navigate('/data/guru/import')}>
          <Upload size={16} strokeWidth={2} aria-hidden="true" />
          Import
        </Button>
        <Button onClick={openCreate}>
          <Plus size={16} strokeWidth={2} aria-hidden="true" />
          Tambah Guru
        </Button>
      </div>

      <a
        href={importTemplateUrl('teachers')}
        className="inline-flex w-fit items-center gap-1.5 text-[12px] font-medium text-primary hover:opacity-80"
      >
        <FileSpreadsheet size={16} strokeWidth={2} aria-hidden="true" />
        Unduh template Excel
      </a>

      {isLoading ? (
        <div className="flex flex-col gap-2">
          <Skeleton className="h-12 w-full" />
          <Skeleton className="h-12 w-full" />
          <Skeleton className="h-12 w-full" />
        </div>
      ) : isError ? (
        <ErrorState message="Gagal memuat daftar guru." onRetry={() => refetch()} />
      ) : teachers && teachers.length === 0 ? (
        <EmptyState
          icon={UserRound}
          message="Belum ada guru. Tambahkan manual atau import dari Excel."
          action={
            <Button variant="secondary" onClick={openCreate}>
              Tambah Guru
            </Button>
          }
        />
      ) : (
        <div>
          {teachers?.map((t) => (
            <ListRow
              key={t.id}
              title={t.name}
              subtitle={[t.nip, t.email].filter(Boolean).join(' · ') || undefined}
              onClick={() => openEdit(t)}
            />
          ))}
        </div>
      )}

      <TeacherFormDialog open={dialogOpen} onClose={() => setDialogOpen(false)} teacher={editing} />
    </div>
  );
}
