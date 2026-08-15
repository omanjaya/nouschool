import { useEffect, useState, type FormEvent } from 'react';
import { ShieldAlert } from 'lucide-react';
import { useMe } from '../auth/api';
import { Skeleton } from '../../components/ui/Skeleton';
import { ErrorState } from '../../components/ui/ErrorState';
import { EmptyState } from '../../components/ui/EmptyState';
import { Card } from '../../components/ui/Card';
import { Field, Input } from '../../components/ui/Field';
import { Button } from '../../components/ui/Button';
import { useToast } from '../../components/ui/Toast';
import { useBranding, useUpdateBranding } from './api';
import { ApiError } from '../../lib/api';
import { LeaveSettingsSection } from '../leave/LeaveSettingsSection';

/** /pengaturan — hanya admin_sekolah, area sekolah (host tenant). */
export function SettingsPage() {
  const { data: me, isLoading } = useMe();

  if (isLoading) {
    return (
      <div className="mx-auto flex max-w-[640px] flex-col gap-4 px-5 py-6">
        <Skeleton className="h-6 w-40" />
        <Skeleton className="h-52 w-full" />
      </div>
    );
  }

  if (!me || me.role !== 'admin_sekolah') {
    return (
      <div className="mx-auto max-w-[640px] px-5 py-6">
        <EmptyState icon={ShieldAlert} message="Anda tidak memiliki akses ke halaman ini." />
      </div>
    );
  }

  return (
    <div className="mx-auto flex max-w-[640px] flex-col gap-8 px-5 py-6">
      <BrandingForm />
      <div className="flex flex-col gap-4 border-t border-line pt-6">
        <div>
          <p className="text-[11px] font-semibold uppercase tracking-[0.1em] text-muted">Izin</p>
          <h2 className="text-[18px] font-semibold text-ink">Pengaturan Izin Guru</h2>
        </div>
        <LeaveSettingsSection />
      </div>
    </div>
  );
}

function BrandingForm() {
  const { data: branding, isLoading, isError, refetch } = useBranding();
  const update = useUpdateBranding();
  const { showToast } = useToast();
  const [appName, setAppName] = useState('');
  const [primaryColor, setPrimaryColor] = useState('#0E6B4E');

  useEffect(() => {
    if (branding) {
      setAppName(branding.app_name);
      setPrimaryColor(branding.primary_color);
    }
  }, [branding]);

  function handleSubmit(e: FormEvent) {
    e.preventDefault();
    update.mutate(
      { app_name: appName, primary_color: primaryColor },
      { onSuccess: () => showToast('Pengaturan disimpan.') },
    );
  }

  return (
    <div className="flex flex-col gap-6">
      <div>
        <p className="text-[11px] font-semibold uppercase tracking-[0.1em] text-muted">Sekolah</p>
        <h1 className="text-[21px] font-semibold text-ink">Pengaturan</h1>
      </div>

      {isLoading ? (
        <Skeleton className="h-52 w-full" />
      ) : isError ? (
        <ErrorState message="Gagal memuat pengaturan tampilan." onRetry={() => refetch()} />
      ) : (
        <Card>
          <form onSubmit={handleSubmit} className="flex flex-col gap-4">
            <Field label="Nama aplikasi" htmlFor="app-name">
              <Input id="app-name" value={appName} onChange={(e) => setAppName(e.target.value)} required />
            </Field>

            <Field label="Warna utama" htmlFor="primary-color">
              <div className="flex items-center gap-3">
                <input
                  id="primary-color"
                  type="color"
                  value={primaryColor}
                  onChange={(e) => setPrimaryColor(e.target.value)}
                  className="h-11 w-14 cursor-pointer rounded-lg border border-line bg-surface p-1"
                />
                <span className="num text-[14px] text-ink">{primaryColor.toUpperCase()}</span>
              </div>
            </Field>

            {update.isError && (
              <p className="text-[12px] text-danger">
                {update.error instanceof ApiError ? update.error.message : 'Gagal menyimpan pengaturan.'}
              </p>
            )}

            <Button type="submit" loading={update.isPending} className="self-start">
              Simpan Pengaturan
            </Button>
          </form>
        </Card>
      )}
    </div>
  );
}
