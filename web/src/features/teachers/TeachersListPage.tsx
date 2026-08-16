import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { FileSpreadsheet, LogIn, Pencil, Plus, Upload, UserCheck, UserRound, UserX } from 'lucide-react';
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
import { importTemplateUrl } from '../import/api';
import { useMe } from '../auth/api';
import { ImpersonateUserDialog } from '../impersonateuser/ImpersonateUserDialog';
import { useSetTeacherStatus, useTeachers } from './api';
import { TeacherFormDialog } from './TeacherFormDialog';
import type { Teacher } from '../../lib/types';

export function TeachersListPage() {
  const { data: me } = useMe();
  const { data: teachers, isLoading, isError, refetch } = useTeachers();
  const [dialogOpen, setDialogOpen] = useState(false);
  const [editing, setEditing] = useState<Teacher | undefined>();
  const [impersonating, setImpersonating] = useState<Teacher | undefined>();
  const [statusTarget, setStatusTarget] = useState<Teacher | undefined>();
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

  /** Kolom desktop (docs/10 §5) — mobile tetap `ListRow` di bawah. */
  const columns: DataTableColumn<Teacher>[] = [
    { key: 'name', header: 'Nama', sortable: true, sortValue: (t) => t.name, cell: (t) => <span className="font-medium text-ink">{t.name}</span> },
    { key: 'email', header: 'Email', sortable: true, sortValue: (t) => t.email ?? '', cell: (t) => t.email ?? <span className="text-muted">—</span> },
    { key: 'nip', header: 'NIP', sortable: true, sortValue: (t) => t.nip ?? '', cell: (t) => t.nip ?? <span className="text-muted">—</span> },
    {
      key: 'status',
      header: 'Status Akun',
      cell: (t) =>
        t.membership_status === 'inactive' ? <Tag variant="danger">Nonaktif</Tag> : <Tag variant="success">Aktif</Tag>,
    },
  ];

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
        <>
          {/* Mobile: ListRow (docs/10 §5). Desktop: DataTable — sama data, tampilan admin. */}
          <div className="lg:hidden">
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
                    {me?.role === 'admin_sekolah' && (
                      <button
                        type="button"
                        onClick={() => setStatusTarget(t)}
                        aria-label={t.membership_status === 'inactive' ? `Aktifkan ${t.name}` : `Nonaktifkan ${t.name}`}
                        className="flex h-9 w-9 items-center justify-center rounded-lg text-muted transition-colors duration-150 hover:bg-surface-2 hover:text-ink"
                      >
                        {t.membership_status === 'inactive' ? (
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
              data={teachers ?? []}
              keyField={(t) => t.id}
              emptyIcon={UserRound}
              emptyMessage="Belum ada guru."
              actions={(t) => [
                { label: 'Ubah', icon: Pencil, onClick: () => openEdit(t) },
                ...(canImpersonate ? [{ label: 'Masuk Sebagai', icon: LogIn, onClick: () => setImpersonating(t) }] : []),
                ...(me?.role === 'admin_sekolah'
                  ? [
                      {
                        label: t.membership_status === 'inactive' ? 'Aktifkan' : 'Nonaktifkan',
                        icon: t.membership_status === 'inactive' ? UserCheck : UserX,
                        onClick: () => setStatusTarget(t),
                        variant: t.membership_status === 'inactive' ? ('default' as const) : ('danger' as const),
                      },
                    ]
                  : []),
              ]}
            />
          </div>
        </>
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
      <DeactivateTeacherDialog teacher={statusTarget} onClose={() => setStatusTarget(undefined)} />
    </div>
  );
}

/**
 * Dialog konfirmasi nonaktifkan/aktifkan akun guru (Fase 15 GAP 6a). Ikon
 * trigger & judul dialog mengikuti `membership_status` kalau backend
 * mengirimnya; kalau tidak ada (undefined), diperlakukan sebagai "aktif"
 * (default aman: aksi utama yang ditawarkan adalah menonaktifkan).
 */
function DeactivateTeacherDialog({ teacher, onClose }: { teacher: Teacher | undefined; onClose: () => void }) {
  const isInactive = teacher?.membership_status === 'inactive';
  const setStatus = useSetTeacherStatus(teacher?.user_id ?? '');
  const { showToast } = useToast();

  function handleConfirm() {
    if (!teacher) return;
    setStatus.mutate(isInactive ? 'active' : 'inactive', {
      onSuccess: () => {
        showToast(isInactive ? 'Akun guru diaktifkan kembali.' : 'Akun guru dinonaktifkan.');
        onClose();
      },
      onError: (err) => showToast(err instanceof ApiError ? err.message : 'Gagal mengubah status akun.', 'error'),
    });
  }

  return (
    <Dialog
      open={teacher !== undefined}
      onClose={onClose}
      title={isInactive ? 'Aktifkan akun guru?' : 'Nonaktifkan akun guru?'}
    >
      <p className="text-[14px] text-ink">
        {isInactive ? (
          <>
            Akun <span className="font-medium">{teacher?.name}</span> bisa login kembali seperti biasa.
          </>
        ) : (
          <>
            Akun <span className="font-medium">{teacher?.name}</span> tidak bisa login sampai diaktifkan lagi;
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
