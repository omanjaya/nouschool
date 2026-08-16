import { useNavigate } from 'react-router-dom';
import { Dialog } from '../../components/ui/Dialog';
import { Button } from '../../components/ui/Button';
import { ApiError } from '../../lib/api';
import { useImpersonateUser } from './api';

interface ImpersonateUserDialogProps {
  open: boolean;
  onClose: () => void;
  userId: string;
  userName: string;
}

/**
 * Dialog konfirmasi "Masuk sebagai user ini" — dipakai `/data/siswa/:id`,
 * `/data/guru`, `/data/pegawai` (admin sekolah, Fase 14 Gelombang D). Sukses
 * → identitas sesi berganti total (`useImpersonateUser`), lalu navigasi ke
 * `/` (Beranda user target, sama seperti login normal).
 */
export function ImpersonateUserDialog({ open, onClose, userId, userName }: ImpersonateUserDialogProps) {
  const navigate = useNavigate();
  const impersonate = useImpersonateUser();

  function handleConfirm() {
    impersonate.mutate(userId, {
      onSuccess: () => {
        onClose();
        navigate('/', { replace: true });
      },
    });
  }

  return (
    <Dialog open={open} onClose={onClose} title="Masuk sebagai user ini?">
      <div className="flex flex-col gap-4">
        <p className="text-[14px] text-ink">
          Sesi Anda berpindah ke akun {userName} selama maks 1 jam — semua tindakan tercatat.
        </p>
        {impersonate.isError && (
          <p className="text-[12px] text-danger">
            {impersonate.error instanceof ApiError ? impersonate.error.message : 'Gagal masuk sebagai user ini.'}
          </p>
        )}
        <div className="flex justify-end gap-2">
          <Button type="button" variant="secondary" onClick={onClose}>
            Batal
          </Button>
          <Button type="button" onClick={handleConfirm} loading={impersonate.isPending}>
            Masuk sebagai {userName}
          </Button>
        </div>
      </div>
    </Dialog>
  );
}
