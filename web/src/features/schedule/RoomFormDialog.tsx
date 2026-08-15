import { useEffect, useState, type FormEvent } from 'react';
import { Dialog } from '../../components/ui/Dialog';
import { Button } from '../../components/ui/Button';
import { Field, Input } from '../../components/ui/Field';
import { ApiError } from '../../lib/api';
import { useCreateRoom, useUpdateRoom, type RoomInput } from './api';
import type { Room } from '../../lib/types';

interface RoomFormDialogProps {
  open: boolean;
  onClose: () => void;
  room?: Room;
}

const EMPTY: RoomInput = { name: '' };

/** Dialog tambah/ubah ruangan — hanya nama (qr_token dikelola lewat regenerate, bukan form ini). */
export function RoomFormDialog({ open, onClose, room }: RoomFormDialogProps) {
  const isEdit = Boolean(room);
  const [form, setForm] = useState<RoomInput>(EMPTY);
  const [nameError, setNameError] = useState<string | undefined>();
  const createRoom = useCreateRoom();
  const updateRoom = useUpdateRoom(room?.id ?? '');
  const mutation = isEdit ? updateRoom : createRoom;

  useEffect(() => {
    if (open) {
      setForm(room ? { name: room.name } : EMPTY);
      setNameError(undefined);
      mutation.reset();
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, room]);

  function handleSubmit(e: FormEvent) {
    e.preventDefault();
    if (!form.name.trim()) {
      setNameError('Nama ruangan wajib diisi.');
      return;
    }
    setNameError(undefined);
    mutation.mutate(
      { name: form.name.trim() },
      {
        onSuccess: () => onClose(),
      },
    );
  }

  return (
    <Dialog open={open} onClose={onClose} title={isEdit ? 'Ubah Ruangan' : 'Tambah Ruangan'}>
      <form onSubmit={handleSubmit} className="flex flex-col gap-4">
        <Field label="Nama ruangan" htmlFor="room-name" error={nameError} hint="Contoh: R. 12, Lab TKJ 1">
          <Input id="room-name" value={form.name} onChange={(e) => setForm({ name: e.target.value })} />
        </Field>

        {mutation.isError && (
          <p className="text-[12px] text-danger">
            {mutation.error instanceof ApiError ? mutation.error.message : 'Gagal menyimpan ruangan.'}
          </p>
        )}

        <div className="flex justify-end gap-2 pt-2">
          <Button type="button" variant="secondary" onClick={onClose}>
            Batal
          </Button>
          <Button type="submit" loading={mutation.isPending}>
            Simpan
          </Button>
        </div>
      </form>
    </Dialog>
  );
}
