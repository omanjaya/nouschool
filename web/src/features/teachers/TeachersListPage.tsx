import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { FileSpreadsheet, LogIn, Pencil, Plus, Upload, UserRound } from 'lucide-react';
import { ListRow } from '../../components/ui/ListRow';
import { Skeleton } from '../../components/ui/Skeleton';
import { EmptyState } from '../../components/ui/EmptyState';
import { ErrorState } from '../../components/ui/ErrorState';
import { Button } from '../../components/ui/Button';
import { importTemplateUrl } from '../import/api';
import { useMe } from '../auth/api';
import { ImpersonateUserDialog } from '../impersonateuser/ImpersonateUserDialog';
import { useTeachers } from './api';
import { TeacherFormDialog } from './TeacherFormDialog';
import type { Teacher } from '../../lib/types';

export function TeachersListPage() {
  const { data: me } = useMe();
  const { data: teachers, isLoading, isError, refetch } = useTeachers();
  const [dialogOpen, setDialogOpen] = useState(false);
  const [editing, setEditing] = useState<Teacher | undefined>();
  const [impersonating, setImpersonating] = useState<Teacher | undefined>();
  const navigate = useNavigate();
  const canImpersonate = me?.role === 'admin_sekolah';

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
              trailing={
                <div className="flex items-center gap-1">
                  <button
                    type="button"
                    onClick={() => openEdit(t)}
                    aria-label={`Ubah ${t.name}`}
                    className="flex h-9 w-9 items-center justify-center rounded-lg text-muted transition-colors duration-150 hover:bg-surface-2 hover:text-ink"
                  >
                    <Pencil size={16} strokeWidth={2} aria-hidden="true" />
                  </button>
                  {canImpersonate && (
                    <button
                      type="button"
                      onClick={() => setImpersonating(t)}
                      aria-label={`Masuk sebagai ${t.name}`}
                      className="flex h-9 w-9 items-center justify-center rounded-lg text-muted transition-colors duration-150 hover:bg-surface-2 hover:text-ink"
                    >
                      <LogIn size={16} strokeWidth={2} aria-hidden="true" />
                    </button>
                  )}
                </div>
              }
            />
          ))}
        </div>
      )}

      <TeacherFormDialog open={dialogOpen} onClose={() => setDialogOpen(false)} teacher={editing} />
      {impersonating && (
        <ImpersonateUserDialog
          open={impersonating !== undefined}
          onClose={() => setImpersonating(undefined)}
          userId={impersonating.user_id}
          userName={impersonating.name}
        />
      )}
    </div>
  );
}
