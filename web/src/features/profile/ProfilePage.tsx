import { useEffect, useState } from 'react';
import { LogOut } from 'lucide-react';
import { useNavigate } from 'react-router-dom';
import { Card } from '../../components/ui/Card';
import { Button } from '../../components/ui/Button';
import { Skeleton } from '../../components/ui/Skeleton';
import { useToast } from '../../components/ui/Toast';
import { useLogout, useMe } from '../auth/api';
import { disablePush, enablePush, getPushState, isIOS, isStandalone, type PushState } from '../../lib/push';
import { ROLE_LABEL } from '../../lib/roles';

/** Halaman Profil — juga tempat keluar di mobile (sidebar desktop sudah punya tombol keluar sendiri). */
export function ProfilePage() {
  const { data: me, isLoading } = useMe();
  const logout = useLogout();
  const navigate = useNavigate();

  async function handleLogout() {
    await logout.mutateAsync();
    navigate('/login', { replace: true });
  }

  if (isLoading) {
    return (
      <div className="mx-auto flex max-w-[640px] flex-col gap-4 px-5 py-6">
        <Skeleton className="h-6 w-32" />
        <Skeleton className="h-20 w-full" />
      </div>
    );
  }

  if (!me) return null;

  const roleLabel = me.is_super_admin ? 'Admin Platform' : (ROLE_LABEL[me.role] ?? me.role);

  return (
    <div className="mx-auto flex max-w-[640px] flex-col gap-6 px-5 py-6">
      <div>
        <p className="text-[11px] font-semibold uppercase tracking-[0.1em] text-muted">Akun</p>
        <h1 className="text-[21px] font-semibold text-ink">Profil</h1>
      </div>

      <Card className="flex flex-col gap-1">
        <p className="text-[16px] font-semibold text-ink">{me.name}</p>
        <p className="text-[12px] text-muted">{roleLabel}</p>
        {me.school && <p className="text-[12px] text-muted">{me.school.name}</p>}
      </Card>

      <PushSection />

      <Button variant="secondary" onClick={handleLogout} loading={logout.isPending} className="self-start">
        <LogOut size={16} strokeWidth={2} aria-hidden="true" />
        Keluar
      </Button>
    </div>
  );
}

const PUSH_STATE_LABEL: Record<PushState, string> = {
  unsupported: 'Push tidak didukung di perangkat/browser ini.',
  denied: 'Izin notifikasi diblokir. Aktifkan lewat pengaturan browser/perangkat, lalu muat ulang halaman.',
  enabled: 'Notifikasi push aktif di perangkat ini.',
  disabled: 'Notifikasi push belum aktif di perangkat ini.',
};

/** Seksi "Notifikasi Push" — status per perangkat + tombol aktif/nonaktifkan (docs/08). */
function PushSection() {
  const [state, setState] = useState<PushState | null>(null);
  const [loading, setLoading] = useState(false);
  const { showToast } = useToast();

  useEffect(() => {
    let cancelled = false;
    getPushState()
      .then((s) => {
        if (!cancelled) setState(s);
      })
      .catch(() => {
        if (!cancelled) setState('unsupported');
      });
    return () => {
      cancelled = true;
    };
  }, []);

  async function handleEnable() {
    setLoading(true);
    try {
      await enablePush();
      setState('enabled');
      showToast('Notifikasi push diaktifkan.');
    } catch (err) {
      showToast(err instanceof Error ? err.message : 'Gagal mengaktifkan notifikasi push.', 'error');
    } finally {
      setLoading(false);
    }
  }

  async function handleDisable() {
    setLoading(true);
    try {
      await disablePush();
      setState('disabled');
      showToast('Notifikasi push dinonaktifkan.');
    } catch (err) {
      showToast(err instanceof Error ? err.message : 'Gagal menonaktifkan notifikasi push.', 'error');
    } finally {
      setLoading(false);
    }
  }

  const showIOSHint = isIOS() && !isStandalone();

  return (
    <Card className="flex flex-col gap-3">
      <div>
        <p className="text-[14px] font-semibold text-ink">Notifikasi Push</p>
        <p className="text-[12px] text-muted">
          Dapatkan notifikasi langsung di perangkat ini untuk absensi, izin, dan info sekolah lainnya.
        </p>
      </div>

      {state === null ? (
        <Skeleton className="h-9 w-40" />
      ) : (
        <>
          <p className="text-[12px] text-muted">{PUSH_STATE_LABEL[state]}</p>

          {showIOSHint && (
            <p className="text-[12px] text-muted">
              Di iPhone/iPad: install NouSchool ke Layar Utama dulu (tombol Bagikan → Tambah ke Layar Utama) supaya
              notifikasi push bisa aktif.
            </p>
          )}

          {state === 'enabled' && (
            <Button variant="secondary" onClick={handleDisable} loading={loading} className="self-start">
              Nonaktifkan
            </Button>
          )}
          {state === 'disabled' && (
            <Button variant="secondary" onClick={handleEnable} loading={loading} className="self-start">
              Aktifkan
            </Button>
          )}
        </>
      )}
    </Card>
  );
}
