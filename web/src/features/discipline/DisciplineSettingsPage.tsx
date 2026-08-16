import { useEffect, useState } from 'react';
import { Navigate, useNavigate } from 'react-router-dom';
import { Plus, ShieldAlert, Trash2 } from 'lucide-react';
import { AppBar } from '../../components/ui/AppBar';
import { Card } from '../../components/ui/Card';
import { ListRow } from '../../components/ui/ListRow';
import { Tag } from '../../components/ui/Tag';
import { Button } from '../../components/ui/Button';
import { Dialog } from '../../components/ui/Dialog';
import { Field, Input } from '../../components/ui/Field';
import { Skeleton } from '../../components/ui/Skeleton';
import { EmptyState } from '../../components/ui/EmptyState';
import { ErrorState } from '../../components/ui/ErrorState';
import { useToast } from '../../components/ui/Toast';
import { ApiError } from '../../lib/api';
import { useMe } from '../auth/api';
import {
  useDeleteDisciplineType,
  useDisciplineSpSettings,
  useDisciplineTypes,
  useUpdateDisciplineSpSettings,
  useUpdateDisciplineType,
} from './api';
import { DisciplineTypeFormDialog } from './DisciplineTypeFormDialog';
import type { DisciplineSpSettings, DisciplineType } from '../../lib/types';
import { PAGE_WIDE } from '../../components/ui/page';

/** /kedisiplinan/pengaturan — master jenis pelanggaran + ambang Surat Peringatan (admin only). */
export function DisciplineSettingsPage() {
  const { data: me, isLoading } = useMe();
  const navigate = useNavigate();

  if (isLoading) {
    return (
      <div className={`${PAGE_WIDE} flex flex-col gap-4`}>
        <Skeleton className="h-6 w-40" />
        <Skeleton className="h-52 w-full" />
      </div>
    );
  }

  if (!me || me.role !== 'admin_sekolah') {
    return <Navigate to="/" replace />;
  }

  return (
    <div className="flex min-h-dvh flex-col">
      <AppBar title="Pengaturan Kedisiplinan" onBack={() => navigate('/kedisiplinan')} />
      <div className={`${PAGE_WIDE} flex flex-col gap-8`}>
        <DisciplineTypesSection />
        <div className="border-t border-line pt-6">
          <SpSettingsSection />
        </div>
      </div>
    </div>
  );
}

function DisciplineTypesSection() {
  const { data: types, isLoading, isError, refetch } = useDisciplineTypes();
  const { showToast } = useToast();
  const [formOpen, setFormOpen] = useState(false);
  const [editing, setEditing] = useState<DisciplineType | undefined>();
  const [deleting, setDeleting] = useState<DisciplineType | undefined>();
  const [conflictMessage, setConflictMessage] = useState<string | null>(null);
  const deleteType = useDeleteDisciplineType();
  const deactivateType = useUpdateDisciplineType(deleting?.id ?? '');

  function openCreate() {
    setEditing(undefined);
    setFormOpen(true);
  }

  function openEdit(t: DisciplineType) {
    setEditing(t);
    setFormOpen(true);
  }

  function openDelete(t: DisciplineType) {
    setDeleting(t);
    setConflictMessage(null);
    deleteType.reset();
  }

  function closeDelete() {
    setDeleting(undefined);
    setConflictMessage(null);
  }

  function handleDelete() {
    if (!deleting) return;
    deleteType.mutate(deleting.id, {
      onSuccess: () => {
        showToast('Jenis pelanggaran dihapus.');
        closeDelete();
      },
      onError: (err) => {
        if (err instanceof ApiError && err.status === 409) {
          setConflictMessage(err.message);
        } else {
          showToast(err instanceof ApiError ? err.message : 'Gagal menghapus jenis pelanggaran.', 'error');
        }
      },
    });
  }

  function handleDeactivate() {
    if (!deleting) return;
    deactivateType.mutate(
      { name: deleting.name, points: deleting.points, category: deleting.category, active: false },
      {
        onSuccess: () => {
          showToast('Jenis pelanggaran dinonaktifkan.');
          closeDelete();
        },
        onError: (err) => {
          showToast(err instanceof ApiError ? err.message : 'Gagal menonaktifkan jenis pelanggaran.', 'error');
        },
      },
    );
  }

  return (
    <div className="flex flex-col gap-4">
      <div className="flex items-center justify-between gap-3">
        <div>
          <p className="text-[11px] font-semibold uppercase tracking-[0.1em] text-muted">Master Data</p>
          <h2 className="text-[18px] font-semibold text-ink">Jenis Pelanggaran</h2>
        </div>
        <Button onClick={openCreate}>
          <Plus size={16} strokeWidth={2} aria-hidden="true" />
          Tambah
        </Button>
      </div>

      {isLoading ? (
        <div className="flex flex-col gap-2">
          <Skeleton className="h-12 w-full" />
          <Skeleton className="h-12 w-full" />
        </div>
      ) : isError ? (
        <ErrorState message="Gagal memuat jenis pelanggaran." onRetry={() => refetch()} />
      ) : !types || types.length === 0 ? (
        <EmptyState
          icon={ShieldAlert}
          message="Belum ada jenis pelanggaran."
          action={
            <Button variant="secondary" onClick={openCreate}>
              Tambah Jenis Pelanggaran
            </Button>
          }
        />
      ) : (
        <div>
          {types.map((t) => (
            <ListRow
              key={t.id}
              title={t.name}
              subtitle={`+${t.points} poin`}
              onClick={() => openEdit(t)}
              trailing={
                <div className="flex items-center gap-1.5">
                  <Tag variant="neutral">{t.category}</Tag>
                  {!t.active && <Tag variant="done">Nonaktif</Tag>}
                  <button
                    type="button"
                    onClick={(e) => {
                      e.stopPropagation();
                      openDelete(t);
                    }}
                    aria-label={`Hapus ${t.name}`}
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

      <DisciplineTypeFormDialog open={formOpen} onClose={() => setFormOpen(false)} type={editing} />

      <Dialog open={deleting !== undefined} onClose={closeDelete} title="Hapus jenis pelanggaran?">
        {conflictMessage ? (
          <div className="flex flex-col gap-3">
            <p className="text-[14px] text-ink">{conflictMessage}</p>
            <p className="text-[12px] text-muted">
              Jenis ini masih dipakai di catatan pelanggaran yang ada — nonaktifkan saja supaya tidak dipilih lagi
              untuk catatan baru, tanpa mengubah riwayat yang sudah tercatat.
            </p>
          </div>
        ) : (
          <p className="text-[14px] text-ink">
            &quot;{deleting?.name}&quot; akan dihapus permanen dari daftar jenis pelanggaran.
          </p>
        )}
        <div className="mt-4 flex justify-end gap-2">
          <Button type="button" variant="secondary" onClick={closeDelete}>
            Batal
          </Button>
          {conflictMessage ? (
            <Button type="button" loading={deactivateType.isPending} onClick={handleDeactivate}>
              Nonaktifkan
            </Button>
          ) : (
            <Button type="button" variant="danger" loading={deleteType.isPending} onClick={handleDelete}>
              Hapus
            </Button>
          )}
        </div>
      </Dialog>
    </div>
  );
}

function SpSettingsSection() {
  const { data, isLoading, isError, refetch } = useDisciplineSpSettings();
  const update = useUpdateDisciplineSpSettings();
  const { showToast } = useToast();
  const [form, setForm] = useState<{ sp1: string; sp2: string; sp3: string }>({ sp1: '', sp2: '', sp3: '' });
  const [formError, setFormError] = useState<string | null>(null);

  useEffect(() => {
    if (data) setForm({ sp1: String(data.sp1), sp2: String(data.sp2), sp3: String(data.sp3) });
  }, [data]);

  function handleSave() {
    setFormError(null);
    const sp1 = Number(form.sp1);
    const sp2 = Number(form.sp2);
    const sp3 = Number(form.sp3);

    if (![sp1, sp2, sp3].every((n) => Number.isFinite(n) && n > 0)) {
      setFormError('Ambang SP1/SP2/SP3 harus berupa angka poin lebih dari 0.');
      return;
    }
    if (!(sp1 < sp2 && sp2 < sp3)) {
      setFormError('Urutan ambang harus SP1 < SP2 < SP3.');
      return;
    }

    const payload: DisciplineSpSettings = { sp1, sp2, sp3 };
    update.mutate(payload, {
      onSuccess: () => showToast('Ambang Surat Peringatan disimpan.'),
      onError: (err) => setFormError(err instanceof ApiError ? err.message : 'Gagal menyimpan ambang Surat Peringatan.'),
    });
  }

  if (isLoading) {
    return <Skeleton className="h-40 w-full" />;
  }

  if (isError || !data) {
    return <ErrorState message="Gagal memuat ambang Surat Peringatan." onRetry={() => refetch()} />;
  }

  return (
    <div className="flex flex-col gap-4">
      <div>
        <p className="text-[11px] font-semibold uppercase tracking-[0.1em] text-muted">Tahun Ajaran Aktif</p>
        <h2 className="text-[18px] font-semibold text-ink">Ambang Surat Peringatan</h2>
        <p className="mt-1 text-[12px] text-muted">
          Total poin siswa mencapai ambang ini akan otomatis menerbitkan Surat Peringatan.
        </p>
      </div>

      <Card className="flex flex-col gap-4">
        <div className="flex flex-wrap gap-3">
          <Field label="SP1 (poin)" htmlFor="sp1" className="min-w-[100px] flex-1">
            <Input
              id="sp1"
              type="number"
              min={1}
              value={form.sp1}
              onChange={(e) => setForm((f) => ({ ...f, sp1: e.target.value }))}
            />
          </Field>
          <Field label="SP2 (poin)" htmlFor="sp2" className="min-w-[100px] flex-1">
            <Input
              id="sp2"
              type="number"
              min={1}
              value={form.sp2}
              onChange={(e) => setForm((f) => ({ ...f, sp2: e.target.value }))}
            />
          </Field>
          <Field label="SP3 (poin)" htmlFor="sp3" className="min-w-[100px] flex-1">
            <Input
              id="sp3"
              type="number"
              min={1}
              value={form.sp3}
              onChange={(e) => setForm((f) => ({ ...f, sp3: e.target.value }))}
            />
          </Field>
        </div>

        {formError && <p className="text-[12px] text-danger">{formError}</p>}

        <Button onClick={handleSave} loading={update.isPending} className="self-start">
          Simpan
        </Button>
      </Card>
    </div>
  );
}
