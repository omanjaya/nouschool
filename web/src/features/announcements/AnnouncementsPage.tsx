import { useState } from 'react';
import { Navigate } from 'react-router-dom';
import { Megaphone, Plus, Trash2 } from 'lucide-react';
import { ListRow } from '../../components/ui/ListRow';
import { Skeleton } from '../../components/ui/Skeleton';
import { EmptyState } from '../../components/ui/EmptyState';
import { ErrorState } from '../../components/ui/ErrorState';
import { Button } from '../../components/ui/Button';
import { Dialog } from '../../components/ui/Dialog';
import { Tag } from '../../components/ui/Tag';
import { useToast } from '../../components/ui/Toast';
import { ApiError } from '../../lib/api';
import { formatDateRange, todayISODate } from '../../lib/date';
import { useMe } from '../auth/api';
import { useAnnouncements, useDeleteAnnouncement } from './api';
import { AnnouncementFormDialog } from './AnnouncementFormDialog';
import type { Announcement } from '../../lib/types';

function isActiveToday(a: Announcement): boolean {
  const today = todayISODate();
  return a.starts_at <= today && today <= a.ends_at;
}

/** /pengumuman — CRUD pengumuman ditampilkan di TV & dashboard (admin & kepsek, docs/06 #3). */
export function AnnouncementsPage() {
  const { data: me } = useMe();
  const canManage = me?.role === 'admin_sekolah' || me?.role === 'kepala_sekolah';
  const { data: announcements, isLoading, isError, refetch } = useAnnouncements(false, canManage);
  const [formOpen, setFormOpen] = useState(false);
  const [editing, setEditing] = useState<Announcement | undefined>();
  const [deleting, setDeleting] = useState<Announcement | undefined>();
  const deleteAnnouncement = useDeleteAnnouncement();
  const { showToast } = useToast();

  if (me && !canManage) {
    return <Navigate to="/" replace />;
  }

  function openCreate() {
    setEditing(undefined);
    setFormOpen(true);
  }

  function openEdit(a: Announcement) {
    setEditing(a);
    setFormOpen(true);
  }

  function handleDelete() {
    if (!deleting) return;
    deleteAnnouncement.mutate(deleting.id, {
      onSuccess: () => {
        showToast('Pengumuman dihapus.');
        setDeleting(undefined);
      },
    });
  }

  return (
    <div className="mx-auto flex max-w-[640px] flex-col gap-6 px-5 py-6 lg:max-w-[1000px]">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <p className="text-[11px] font-semibold uppercase tracking-[0.1em] text-muted">Mengajar</p>
          <h1 className="text-[21px] font-semibold text-ink">Pengumuman</h1>
          <p className="mt-1 text-[13px] text-muted">Tampil di dashboard TV & beranda selama rentang tanggal aktif.</p>
        </div>
        <Button onClick={openCreate}>
          <Plus size={16} strokeWidth={2} aria-hidden="true" />
          Tambah Pengumuman
        </Button>
      </div>

      {isLoading ? (
        <div className="flex flex-col gap-2">
          <Skeleton className="h-14 w-full" />
          <Skeleton className="h-14 w-full" />
          <Skeleton className="h-14 w-full" />
        </div>
      ) : isError ? (
        <ErrorState message="Gagal memuat pengumuman." onRetry={() => refetch()} />
      ) : !announcements || announcements.length === 0 ? (
        <EmptyState
          icon={Megaphone}
          message="Belum ada pengumuman."
          action={
            <Button variant="secondary" onClick={openCreate}>
              Tambah Pengumuman
            </Button>
          }
        />
      ) : (
        <div>
          {announcements.map((a) => (
            <ListRow
              key={a.id}
              className="min-h-[56px]"
              title={a.title}
              subtitle={formatDateRange(a.starts_at, a.ends_at)}
              onClick={() => openEdit(a)}
              trailing={
                <div className="flex items-center gap-2">
                  {isActiveToday(a) && <Tag variant="now">Aktif</Tag>}
                  <button
                    type="button"
                    onClick={(e) => {
                      e.stopPropagation();
                      setDeleting(a);
                    }}
                    aria-label={`Hapus ${a.title}`}
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

      <AnnouncementFormDialog
        open={formOpen}
        onClose={() => setFormOpen(false)}
        announcement={editing}
        onSaved={(message) => showToast(message)}
      />

      <Dialog open={deleting !== undefined} onClose={() => setDeleting(undefined)} title="Hapus pengumuman?">
        <p className="text-[14px] text-ink">
          Pengumuman &quot;{deleting?.title}&quot; akan dihapus dan tidak lagi tampil di TV maupun beranda.
        </p>
        {deleteAnnouncement.isError && (
          <p className="mt-3 text-[12px] text-danger">
            {deleteAnnouncement.error instanceof ApiError ? deleteAnnouncement.error.message : 'Gagal menghapus pengumuman.'}
          </p>
        )}
        <div className="mt-4 flex justify-end gap-2">
          <Button type="button" variant="secondary" onClick={() => setDeleting(undefined)}>
            Batal
          </Button>
          <Button type="button" variant="danger" loading={deleteAnnouncement.isPending} onClick={handleDelete}>
            Hapus
          </Button>
        </div>
      </Dialog>
    </div>
  );
}
