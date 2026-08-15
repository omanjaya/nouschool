import { useState } from 'react';
import { Navigate, useNavigate } from 'react-router-dom';
import { CheckCircle2, MapPin } from 'lucide-react';
import { Button } from '../../components/ui/Button';
import { Skeleton } from '../../components/ui/Skeleton';
import { ErrorState } from '../../components/ui/ErrorState';
import { EmptyState } from '../../components/ui/EmptyState';
import { StatusChip } from '../../components/ui/StatusChip';
import { useToast } from '../../components/ui/Toast';
import { ApiError } from '../../lib/api';
import { formatTimeOfDay } from '../../lib/date';
import { hasFeature } from '../../lib/features';
import { useMe } from '../auth/api';
import { useSelfCheckin, useSelfCheckinStatus } from './api';
import type { SelfCheckinResult } from '../../lib/types';

const GEOLOCATION_TIMEOUT_MS = 10_000;

function formatHHMM(value: string): string {
  return value.replace(':', '.');
}

/** /checkin — check-in kehadiran mandiri siswa lewat lokasi GPS (Fase 8, docs/05). */
export function CheckInPage() {
  const { data: me } = useMe();
  const navigate = useNavigate();
  const { showToast } = useToast();
  const { data, isLoading, isError, refetch } = useSelfCheckinStatus(me?.role === 'siswa');
  const checkin = useSelfCheckin();

  const [locating, setLocating] = useState(false);
  const [geoError, setGeoError] = useState<string | null>(null);
  const [justCheckedIn, setJustCheckedIn] = useState<SelfCheckinResult | null>(null);

  if (me && me.role !== 'siswa') {
    return <Navigate to="/" replace />;
  }

  // Fase 10 (docs/09-billing.md): fitur `self_checkin` non-aktif di
  // langganan sekolah — server tetap menolak POST-nya, halaman ini hanya
  // menghindari UI mati (tombol check-in yang pasti gagal).
  if (me && !hasFeature(me, 'self_checkin')) {
    return <Navigate to="/" replace />;
  }

  function handleCheckIn() {
    setGeoError(null);
    if (!('geolocation' in navigator)) {
      setGeoError('Perangkat ini tidak mendukung layanan lokasi. Check-in tidak bisa dilakukan.');
      return;
    }
    setLocating(true);
    navigator.geolocation.getCurrentPosition(
      (position) => {
        setLocating(false);
        checkin.mutate(
          {
            lat: position.coords.latitude,
            lng: position.coords.longitude,
            accuracy: position.coords.accuracy,
          },
          {
            onSuccess: (result) => {
              setJustCheckedIn(result);
              showToast('Check-in berhasil.');
            },
            onError: (err) => {
              showToast(err instanceof ApiError ? err.message : 'Gagal check-in. Coba lagi.', 'error');
            },
          },
        );
      },
      () => {
        setLocating(false);
        setGeoError('Izinkan akses lokasi untuk check-in.');
      },
      { enableHighAccuracy: true, timeout: GEOLOCATION_TIMEOUT_MS },
    );
  }

  const today = justCheckedIn
    ? { status: justCheckedIn.status, checked_at: justCheckedIn.checked_at }
    : data?.today ?? null;

  return (
    <div className="mx-auto flex max-w-[640px] flex-col gap-6 px-5 py-6">
      <div>
        <p className="text-[11px] font-semibold uppercase tracking-[0.1em] text-muted">Kehadiran</p>
        <h1 className="text-[21px] font-semibold text-ink">Check-in Kehadiran</h1>
      </div>

      {isLoading ? (
        <div className="flex flex-col gap-3">
          <Skeleton className="h-24 w-full" />
          <Skeleton className="h-11 w-full" />
        </div>
      ) : isError ? (
        <ErrorState message="Gagal memuat status check-in." onRetry={() => refetch()} />
      ) : !data || !data.enabled ? (
        <EmptyState icon={MapPin} message="Check-in mandiri belum diaktifkan sekolah Anda." />
      ) : today ? (
        <div className="flex flex-col items-center gap-4 rounded-lg border border-line px-6 py-10 text-center">
          <CheckCircle2 size={24} strokeWidth={2} className="text-st-hadir" aria-hidden="true" />
          <div className="flex flex-col items-center gap-2">
            <StatusChip status={today.status} size="md" />
            <p className="num text-[14px] text-ink">Check-in pukul {formatTimeOfDay(today.checked_at)}</p>
          </div>
          <p className="text-[13px] text-muted">Anda sudah tercatat hadir hari ini.</p>
        </div>
      ) : (
        <div className="flex flex-col gap-5">
          <div className="flex flex-col gap-1 rounded-lg border border-line px-4 py-4">
            <p className="text-[14px] font-semibold text-ink">Jendela check-in hari ini</p>
            {data.window ? (
              <p className="num text-[13px] text-muted">
                {formatHHMM(data.window.open_from)}–{formatHHMM(data.window.close_at)}
                {data.late_after ? ` (toleransi s.d. ${formatHHMM(data.late_after)})` : ''}
              </p>
            ) : (
              <p className="text-[13px] text-muted">Jadwal check-in belum diatur sekolah.</p>
            )}
          </div>

          <Button
            onClick={handleCheckIn}
            loading={locating || checkin.isPending}
            className="min-h-14 w-full text-[16px]"
          >
            <MapPin size={20} strokeWidth={2} aria-hidden="true" />
            Check-in Sekarang
          </Button>

          {geoError && <p className="text-center text-[13px] text-danger">{geoError}</p>}
        </div>
      )}

      <Button variant="ghost" onClick={() => navigate('/')} className="self-center">
        Kembali ke Beranda
      </Button>
    </div>
  );
}
