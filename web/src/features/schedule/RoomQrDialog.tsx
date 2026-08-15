import { useEffect, useState } from 'react';
import { RefreshCw } from 'lucide-react';
import { Dialog } from '../../components/ui/Dialog';
import { Button } from '../../components/ui/Button';
import { useToast } from '../../components/ui/Toast';
import { ApiError } from '../../lib/api';
import { roomQrImageUrl, useRegenerateRoomQr } from './api';
import type { Room } from '../../lib/types';

interface RoomQrDialogProps {
  open: boolean;
  onClose: () => void;
  room?: Room;
}

/** Dialog "Lihat QR": tampilkan QR ruangan + tombol Regenerate (dua langkah: konfirmasi dulu). */
export function RoomQrDialog({ open, onClose, room }: RoomQrDialogProps) {
  const [confirming, setConfirming] = useState(false);
  const [cacheBust, setCacheBust] = useState(0);
  const regenerate = useRegenerateRoomQr();
  const { showToast } = useToast();

  useEffect(() => {
    if (open) {
      setConfirming(false);
      regenerate.reset();
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, room]);

  if (!room) return null;

  function handleRegenerate() {
    regenerate.mutate(room!.id, {
      onSuccess: () => {
        setConfirming(false);
        setCacheBust((n) => n + 1);
        showToast('QR ruangan diperbarui. QR lama tidak berlaku lagi.');
      },
    });
  }

  return (
    <Dialog open={open} onClose={onClose} title="QR Ruangan">
      <div className="flex flex-col items-center gap-4">
        <img
          key={cacheBust}
          src={`${roomQrImageUrl(room.id)}?v=${cacheBust}`}
          alt={`QR ruangan ${room.name}`}
          className="h-56 w-56 rounded-lg border border-line object-contain"
        />
        <p className="text-[16px] font-semibold text-ink">{room.name}</p>

        {regenerate.isError && (
          <p className="text-[12px] text-danger">
            {regenerate.error instanceof ApiError ? regenerate.error.message : 'Gagal membuat ulang QR.'}
          </p>
        )}

        {confirming ? (
          <div className="flex w-full flex-col gap-3 rounded-lg border border-line bg-surface-2 p-3">
            <p className="text-[13px] text-ink">
              QR lama akan langsung tidak berlaku. Guru/siswa yang sudah menyimpan QR lama tidak bisa scan lagi. Lanjutkan?
            </p>
            <div className="flex justify-end gap-2">
              <Button type="button" variant="secondary" onClick={() => setConfirming(false)}>
                Batal
              </Button>
              <Button type="button" variant="danger" loading={regenerate.isPending} onClick={handleRegenerate}>
                Ya, Regenerate
              </Button>
            </div>
          </div>
        ) : (
          <Button type="button" variant="secondary" onClick={() => setConfirming(true)}>
            <RefreshCw size={16} strokeWidth={2} aria-hidden="true" />
            Regenerate QR
          </Button>
        )}
      </div>
    </Dialog>
  );
}
