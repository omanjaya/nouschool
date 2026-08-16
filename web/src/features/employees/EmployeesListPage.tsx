import { useState } from 'react';
import { LogIn, Pencil, Plus, UserCheck, UserCog, UserX } from 'lucide-react';
import { ListRow } from '../../components/ui/ListRow';
import { DataTable, type DataTableColumn } from '../../components/ui/DataTable';
import { Skeleton } from '../../components/ui/Skeleton';
import { EmptyState } from '../../components/ui/EmptyState';
import { ErrorState } from '../../components/ui/ErrorState';
import { Button } from '../../components/ui/Button';
import { Dialog } from '../../components/ui/Dialog';
import { Tag } from '../../components/ui/Tag';
import { useToast } from '../../components/ui/Toast';
import { ApiError } from '../../lib/api';
import { useMe } from '../auth/api';
import { ImpersonateUserDialog } from '../impersonateuser/ImpersonateUserDialog';
import { useEmployees, useSetEmployeeStatus } from './api';
import { EmployeeFormDialog } from './EmployeeFormDialog';
import type { Employee } from '../../lib/types';

/** Tab "Pegawai" di /data — profil pegawai non-guru (mis. security, tata usaha) yang bisa diberi tugas tambahan. */
export function EmployeesListPage() {
  const { data: me } = useMe();
  const { data: employees, isLoading, isError, refetch } = useEmployees();
  const [dialogOpen, setDialogOpen] = useState(false);
  const [editing, setEditing] = useState<Employee | undefined>();
  const [impersonating, setImpersonating] = useState<Employee | undefined>();
  const [statusTarget, setStatusTarget] = useState<Employee | undefined>();
  const canImpersonate = me?.role === 'admin_sekolah';

  function openEdit(e: Employee) {
    setEditing(e);
    setDialogOpen(true);
  }

  function openCreate() {
    setEditing(undefined);
    setDialogOpen(true);
  }

  /** Kolom desktop (docs/10 §5) — mobile tetap `ListRow` di bawah. */
  const columns: DataTableColumn<Employee>[] = [
    { key: 'name', header: 'Nama', sortable: true, sortValue: (e) => e.name, cell: (e) => <span className="font-medium text-ink">{e.name}</span> },
    {
      key: 'contact',
      header: 'Email/Username',
      sortable: true,
      sortValue: (e) => e.email ?? e.username ?? '',
      cell: (e) => e.email ?? e.username ?? <span className="text-muted">—</span>,
    },
    { key: 'nip', header: 'NIP', sortable: true, sortValue: (e) => e.nip ?? '', cell: (e) => e.nip ?? <span className="text-muted">—</span> },
    {
      key: 'status',
      header: 'Status Akun',
      cell: (e) =>
        e.membership_status === 'inactive' ? <Tag variant="danger">Nonaktif</Tag> : <Tag variant="success">Aktif</Tag>,
    },
  ];

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
        <>
          <div className="lg:hidden">
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
                    {me?.role === 'admin_sekolah' && (
                      <button
                        type="button"
                        onClick={() => setStatusTarget(e)}
                        aria-label={e.membership_status === 'inactive' ? `Aktifkan ${e.name}` : `Nonaktifkan ${e.name}`}
                        className="flex h-9 w-9 items-center justify-center rounded-lg text-muted transition-colors duration-150 hover:bg-surface-2 hover:text-ink"
                      >
                        {e.membership_status === 'inactive' ? (
                          <UserCheck size={16} strokeWidth={2} aria-hidden="true" />
                        ) : (
                          <UserX size={16} strokeWidth={2} aria-hidden="true" />
                        )}
                      </button>
                    )}
                  </div>
                }
              />
            ))}
          </div>
          <div className="hidden lg:block">
            <DataTable
              columns={columns}
              data={employees ?? []}
              keyField={(e) => e.id}
              emptyIcon={UserCog}
              emptyMessage="Belum ada pegawai."
              actions={(e) => [
                { label: 'Ubah', icon: Pencil, onClick: () => openEdit(e) },
                ...(canImpersonate ? [{ label: 'Masuk Sebagai', icon: LogIn, onClick: () => setImpersonating(e) }] : []),
                ...(me?.role === 'admin_sekolah'
                  ? [
                      {
                        label: e.membership_status === 'inactive' ? 'Aktifkan' : 'Nonaktifkan',
                        icon: e.membership_status === 'inactive' ? UserCheck : UserX,
                        onClick: () => setStatusTarget(e),
                        variant: e.membership_status === 'inactive' ? ('default' as const) : ('danger' as const),
                      },
                    ]
                  : []),
              ]}
            />
          </div>
        </>
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
      <DeactivateEmployeeDialog employee={statusTarget} onClose={() => setStatusTarget(undefined)} />
    </div>
  );
}

/** Dialog konfirmasi nonaktifkan/aktifkan akun pegawai (Fase 15 GAP 6a) — sama pola `DeactivateTeacherDialog`. */
function DeactivateEmployeeDialog({ employee, onClose }: { employee: Employee | undefined; onClose: () => void }) {
  const isInactive = employee?.membership_status === 'inactive';
  const setStatus = useSetEmployeeStatus(employee?.user_id ?? '');
  const { showToast } = useToast();

  function handleConfirm() {
    if (!employee) return;
    setStatus.mutate(isInactive ? 'active' : 'inactive', {
      onSuccess: () => {
        showToast(isInactive ? 'Akun pegawai diaktifkan kembali.' : 'Akun pegawai dinonaktifkan.');
        onClose();
      },
      onError: (err) => showToast(err instanceof ApiError ? err.message : 'Gagal mengubah status akun.', 'error'),
    });
  }

  return (
    <Dialog
      open={employee !== undefined}
      onClose={onClose}
      title={isInactive ? 'Aktifkan akun pegawai?' : 'Nonaktifkan akun pegawai?'}
    >
      <p className="text-[14px] text-ink">
        {isInactive ? (
          <>
            Akun <span className="font-medium">{employee?.name}</span> bisa login kembali seperti biasa.
          </>
        ) : (
          <>
            Akun <span className="font-medium">{employee?.name}</span> tidak bisa login sampai diaktifkan lagi;
            semua sesinya keluar.
          </>
        )}
      </p>
      {setStatus.isError && (
        <p className="mt-2 text-[12px] text-danger">
          {setStatus.error instanceof ApiError ? setStatus.error.message : 'Gagal mengubah status akun.'}
        </p>
      )}
      <div className="mt-4 flex justify-end gap-2">
        <Button type="button" variant="secondary" onClick={onClose}>
          Batal
        </Button>
        <Button
          type="button"
          variant={isInactive ? 'primary' : 'danger'}
          loading={setStatus.isPending}
          onClick={handleConfirm}
        >
          {isInactive ? 'Aktifkan' : 'Nonaktifkan'}
        </Button>
      </div>
    </Dialog>
  );
}
