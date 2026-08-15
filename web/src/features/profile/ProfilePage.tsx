import { LogOut } from 'lucide-react';
import { useNavigate } from 'react-router-dom';
import { Card } from '../../components/ui/Card';
import { Button } from '../../components/ui/Button';
import { Skeleton } from '../../components/ui/Skeleton';
import { useLogout, useMe } from '../auth/api';

const ROLE_LABEL: Record<string, string> = {
  admin_sekolah: 'Admin Sekolah',
  guru: 'Guru',
  siswa: 'Siswa',
  orang_tua: 'Orang Tua',
};

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

      <Button variant="secondary" onClick={handleLogout} loading={logout.isPending} className="self-start">
        <LogOut size={16} strokeWidth={2} aria-hidden="true" />
        Keluar
      </Button>
    </div>
  );
}
