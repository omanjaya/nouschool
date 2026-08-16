import { useEffect, useRef } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { ShieldAlert } from 'lucide-react';
import { BrandMark } from '../branding/BrandMark';
import { Skeleton } from '../../components/ui/Skeleton';
import { ApiError } from '../../lib/api';
import { useImpersonate } from './api';

/**
 * Halaman publik `/impersonate` (host TENANT), layar penuh tanpa AppShell —
 * pasangan sibling `/aktivasi` di App.tsx. Dibuka dari tombol "Masuk sebagai
 * Sekolah Ini" di panel super admin (`window.open`, tab baru).
 *
 * Token di query string dikirim SEKALI (token sekali pakai di backend) —
 * `requested` (useRef, bukan state) menjaga effect dari StrictMode
 * double-invoke di dev supaya tidak mengirim POST kedua yang pasti gagal.
 * Gagal → kartu pesan, TANPA retry otomatis (token yang sama tidak akan
 * pernah berhasil dipakai dua kali).
 */
export function ImpersonatePage() {
  const [searchParams] = useSearchParams();
  const token = searchParams.get('token') ?? '';
  const navigate = useNavigate();
  const impersonate = useImpersonate();
  const requested = useRef(false);

  useEffect(() => {
    if (requested.current) return;
    if (!token) return;
    requested.current = true;
    impersonate.mutate(token, {
      onSuccess: () => navigate('/', { replace: true }),
    });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [token]);

  if (!token) {
    return <ImpersonateErrorCard message="Tautan tidak lengkap — token sesi support tidak ditemukan." />;
  }

  if (impersonate.isError) {
    const message =
      impersonate.error instanceof ApiError ? impersonate.error.message : 'Sesi support gagal dimulai. Coba lagi.';
    return <ImpersonateErrorCard message={message} />;
  }

  return (
    <div className="flex min-h-dvh flex-col items-center justify-center gap-6 bg-bg px-5 py-10">
      <BrandMark />
      <div className="flex w-full max-w-[400px] flex-col items-center gap-3 text-center">
        <p className="text-[14px] text-muted">Menyiapkan sesi support…</p>
        <Skeleton className="h-4 w-full" />
        <Skeleton className="h-4 w-2/3" />
      </div>
    </div>
  );
}

function ImpersonateErrorCard({ message }: { message: string }) {
  return (
    <div className="flex min-h-dvh flex-col items-center justify-center gap-6 bg-bg px-5 py-10 text-center">
      <BrandMark />
      <div className="flex w-full max-w-[400px] flex-col items-center gap-3">
        <ShieldAlert size={24} strokeWidth={2} className="text-danger" aria-hidden="true" />
        <p className="text-[14px] text-ink">{message}</p>
        <p className="text-[12px] text-muted">Minta tautan baru dari panel super admin.</p>
      </div>
    </div>
  );
}
