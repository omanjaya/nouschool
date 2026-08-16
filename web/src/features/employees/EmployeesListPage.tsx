import { useState } from 'react';
import { LogIn, Pencil, Plus, UserCog } from 'lucide-react';
import { ListRow } from '../../components/ui/ListRow';
import { Skeleton } from '../../components/ui/Skeleton';
import { EmptyState } from '../../components/ui/EmptyState';
import { ErrorState } from '../../components/ui/ErrorState';
import { Button } from '../../components/ui/Button';
import { useMe } from '../auth/api';
import { ImpersonateUserDialog } from '../impersonateuser/ImpersonateUserDialog';
import { useEmployees } from './api';
import { EmployeeFormDialog } from './EmployeeFormDialog';
import type { Employee } from '../../lib/types';

/** Tab "Pegawai" di /data — profil pegawai non-guru (mis. security, tata usaha) yang bisa diberi tugas tambahan. */
export function EmployeesListPage() {
  const { data: me } = useMe();
  const { data: employees, isLoading, isError, refetch } = useEmployees();
  const [dialogOpen, setDialogOpen] = useState(false);
  const [editing, setEditing] = useState<Employee | undefined>();
  const [impersonating, setImpersonating] = useState<Employee | undefined>();
  const canImpersonate = me?.role === 'admin_sekolah';

  function openEdit(e: Employee) {
    setEditing(e);
    setDialogOpen(true);
  }

  function openCreate() {
    setEditing(undefined);
    setDialogOpen(true);
  }

  return (
    <div className="flex flex-col gap-6">
      <div className="flex flex-wrap items-center justify-end gap-2">
        <Button onClick={openCreate}>
          <Plus size={16} strokeWidth={2} aria-hidden="true" />
          Tambah Pegawai
        </Button>
      </div>

      {isLoading ? (
        <div className="flex flex-col gap-2">
          <Skeleton className="h-12 w-full" />
          <Skeleton className="h-12 w-full" />
          <Skeleton className="h-12 w-full" />
        </div>
      ) : isError ? (
        <ErrorState message="Gagal memuat daftar pegawai." onRetry={() => refetch()} />
      ) : employees && employees.length === 0 ? (
        <EmptyState
          icon={UserCog}
          message="Belum ada pegawai. Tambahkan profil pegawai non-guru di sini."
          action={
            <Button variant="secondary" onClick={openCreate}>
              Tambah Pegawai
            </Button>
          }
        />
      ) : (
        <div>
          {employees?.map((e) => (
            <ListRow
              key={e.id}
              title={e.name}
              subtitle={[e.nip, e.email ?? e.username].filter(Boolean).join(' · ') || undefined}
              trailing={
                <div className="flex items-center gap-1">
                  <button
                    type="button"
                    onClick={() => openEdit(e)}
                    aria-label={`Ubah ${e.name}`}
                    className="flex h-9 w-9 items-center justify-center rounded-lg text-muted transition-colors duration-150 hover:bg-surface-2 hover:text-ink"
                  >
                    <Pencil size={16} strokeWidth={2} aria-hidden="true" />
                  </button>
                  {canImpersonate && (
                    <button
                      type="button"
                      onClick={() => setImpersonating(e)}
                      aria-label={`Masuk sebagai ${e.name}`}
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

      <EmployeeFormDialog open={dialogOpen} onClose={() => setDialogOpen(false)} employee={editing} />
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
