import { useEffect, useState } from 'react';
import { Card } from '../../components/ui/Card';
import { Checkbox } from '../../components/ui/Field';
import { Button } from '../../components/ui/Button';
import { Skeleton } from '../../components/ui/Skeleton';
import { ErrorState } from '../../components/ui/ErrorState';
import { useToast } from '../../components/ui/Toast';
import { ApiError } from '../../lib/api';
import { useSecuritySettings, useUpdateSecuritySettings } from './api';

/** Seksi "Keamanan" di /pengaturan (admin) — Fase 15 GAP 4, docs/02-identity.md §session. */
export function SecuritySettingsSection() {
  const { data, isLoading, isError, refetch } = useSecuritySettings();
  const update = useUpdateSecuritySettings();
  const { showToast } = useToast();

  const [singleDevice, setSingleDevice] = useState(false);

  useEffect(() => {
    if (data) {
      setSingleDevice(data.single_device);
    }
  }, [data]);

  function handleSave() {
    update.mutate(
      { single_device: singleDevice },
      { onSuccess: () => showToast('Pengaturan keamanan disimpan.') },
    );
  }

  if (isLoading) {
    return <Skeleton className="h-32 w-full" />;
  }

  if (isError || !data) {
    return <ErrorState message="Gagal memuat pengaturan keamanan." onRetry={() => refetch()} />;
  }

  return (
    <Card className="flex flex-col gap-4">
      <Checkbox
        id="single-device"
        label="Satu perangkat per akun"
        checked={singleDevice}
        onChange={(e) => setSingleDevice(e.target.checked)}
      />
      <p className="-mt-2 text-[12px] text-muted">Login baru mengeluarkan sesi lama di perangkat lain.</p>

      {update.isError && (
        <p className="text-[12px] text-danger">
          {update.error instanceof ApiError ? update.error.message : 'Gagal menyimpan pengaturan keamanan.'}
        </p>
      )}

      <Button onClick={handleSave} loading={update.isPending} className="self-start">
        Simpan
      </Button>
    </Card>
  );
}
