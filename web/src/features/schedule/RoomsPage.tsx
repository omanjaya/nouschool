import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { DoorOpen, Plus, Printer, QrCode, Trash2 } from 'lucide-react';
import { ListRow } from '../../components/ui/ListRow';
import { DataTable, type DataTableColumn } from '../../components/ui/DataTable';
import { Skeleton } from '../../components/ui/Skeleton';
import { EmptyState } from '../../components/ui/EmptyState';
import { ErrorState } from '../../components/ui/ErrorState';
import { Button } from '../../components/ui/Button';
import { Dialog } from '../../components/ui/Dialog';
import { useToast } from '../../components/ui/Toast';
import { ApiError } from '../../lib/api';
import { useDeleteRoom, useRooms } from './api';
import { RoomFormDialog } from './RoomFormDialog';
import { RoomQrDialog } from './RoomQrDialog';
import type { Room } from '../../lib/types';

/** /data/ruangan — list ruangan + QR per ruangan (admin). */
export function RoomsPage() {
  const { data: rooms, isLoading, isError, refetch } = useRooms();
  const [formOpen, setFormOpen] = useState(false);
  const [editing, setEditing] = useState<Room | undefined>();
  const [qrRoom, setQrRoom] = useState<Room | undefined>();
  const [deleting, setDeleting] = useState<Room | undefined>();
  const deleteRoom = useDeleteRoom();
  const { showToast } = useToast();
  const navigate = useNavigate();

  function openEdit(room: Room) {
    setEditing(room);
    setFormOpen(true);
  }

  function openCreate() {
    setEditing(undefined);
    setFormOpen(true);
  }

  function handleDelete() {
    if (!deleting) return;
    deleteRoom.mutate(deleting.id, {
      onSuccess: () => {
        showToast('Ruangan dihapus.');
        setDeleting(undefined);
      },
    });
  }

  /** Kolom desktop (docs/10 §5) — mobile tetap `ListRow` di bawah. */
  const columns: DataTableColumn<Room>[] = [
    { key: 'name', header: 'Nama', sortable: true, sortValue: (r) => r.name, cell: (r) => <span className="font-medium text-ink">{r.name}</span> },
  ];

  return (
    <div className="flex flex-col gap-6">
      <div className="flex flex-wrap items-center justify-end gap-2">
        <Button variant="secondary" onClick={() => navigate('/data/ruangan/cetak')}>
          <Printer size={16} strokeWidth={2} aria-hidden="true" />
          Cetak Semua QR
        </Button>
        <Button onClick={openCreate}>
          <Plus size={16} strokeWidth={2} aria-hidden="true" />
          Tambah Ruangan
        </Button>
      </div>

      {isLoading ? (
        <div className="flex flex-col gap-2">
          <Skeleton className="h-12 w-full" />
          <Skeleton className="h-12 w-full" />
          <Skeleton className="h-12 w-full" />
        </div>
      ) : isError ? (
        <ErrorState message="Gagal memuat daftar ruangan." onRetry={() => refetch()} />
      ) : rooms && rooms.length === 0 ? (
        <EmptyState
          icon={DoorOpen}
          message="Belum ada ruangan."
          action={
            <Button variant="secondary" onClick={openCreate}>
              Tambah Ruangan
            </Button>
          }
        />
      ) : (
        <>
          <div className="lg:hidden">
            {rooms?.map((room) => (
              <ListRow
                key={room.id}
                title={room.name}
                onClick={() => openEdit(room)}
                trailing={
                  <span className="flex items-center gap-1">
                    <button
                      type="button"
                      onClick={(e) => {
                        e.stopPropagation();
                        setQrRoom(room);
                      }}
                      aria-label={`Lihat QR ${room.name}`}
                      className="flex h-9 w-9 items-center justify-center rounded-lg text-muted transition-colors duration-150 hover:bg-surface-2 hover:text-ink"
                    >
                      <QrCode size={16} strokeWidth={2} aria-hidden="true" />
                    </button>
                    <button
                      type="button"
                      onClick={(e) => {
                        e.stopPropagation();
                        setDeleting(room);
                      }}
                      aria-label={`Hapus ${room.name}`}
                      className="flex h-9 w-9 items-center justify-center rounded-lg text-muted transition-colors duration-150 hover:bg-surface-2 hover:text-danger"
                    >
                      <Trash2 size={16} strokeWidth={2} aria-hidden="true" />
                    </button>
                  </span>
                }
              />
            ))}
          </div>
          <div className="hidden lg:block">
            <DataTable
              columns={columns}
              data={rooms ?? []}
              keyField={(r) => r.id}
              onRowClick={(r) => openEdit(r)}
              emptyIcon={DoorOpen}
              emptyMessage="Belum ada ruangan."
              actions={(r) => [
                { label: 'Lihat QR', icon: QrCode, onClick: () => setQrRoom(r) },
                { label: 'Hapus', icon: Trash2, onClick: () => setDeleting(r), variant: 'danger' },
              ]}
            />
          </div>
        </>
      )}

      <RoomFormDialog open={formOpen} onClose={() => setFormOpen(false)} room={editing} />
      <RoomQrDialog open={qrRoom !== undefined} onClose={() => setQrRoom(undefined)} room={qrRoom} />

      <Dialog open={deleting !== undefined} onClose={() => setDeleting(undefined)} title="Hapus ruangan?">
        <p className="text-[14px] text-ink">
          Ruangan &quot;{deleting?.name}&quot; akan dihapus. Slot jadwal yang memakai ruangan ini akan kehilangan
          referensi ruangannya.
        </p>
        {deleteRoom.isError && (
          <p className="mt-3 text-[12px] text-danger">
            {deleteRoom.error instanceof ApiError ? deleteRoom.error.message : 'Gagal menghapus ruangan.'}
          </p>
        )}
        <div className="mt-4 flex justify-end gap-2">
          <Button type="button" variant="secondary" onClick={() => setDeleting(undefined)}>
            Batal
          </Button>
          <Button type="button" variant="danger" loading={deleteRoom.isPending} onClick={handleDelete}>
            Hapus
          </Button>
        </div>
      </Dialog>
    </div>
  );
}
