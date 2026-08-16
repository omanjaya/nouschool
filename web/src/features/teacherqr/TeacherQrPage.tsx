import { useCallback, useEffect, useRef, useState } from 'react';
import { Navigate } from 'react-router-dom';
import { Skeleton } from '../../components/ui/Skeleton';
import { ErrorState } from '../../components/ui/ErrorState';
import { QrImage } from '../../lib/qrcode';
import { useRealtimeEvents } from '../../lib/realtime';
import { useMe } from '../auth/api';
import { useGenerateTeacherQr, TEACHER_QR_PREFIX } from './api';
import type { TeacherQrToken } from '../../lib/types';

const TOKEN_TTL_MS = 60_000;

/**
 * /qr-saya — guru menampilkan QR berumur pendek (TTL 60 detik, sekali pakai)
 * untuk dipindai siswa saat meminta persetujuan dispensasi keluar/terlambat
 * (Fase 14 Gelombang B2, docs/12-sion-parity.md Gelombang B alur 2-3). Token
 * beregenerasi otomatis saat: (1) habis waktu (progress hairline capai 0),
 * ATAU (2) event realtime `teacherqr` diterima — dikirim server begitu siswa
 * MEMAKAI token ini (satu QR = satu approval, tidak boleh dipakai ulang).
 *
 * Kode token juga ditampilkan sebagai teks besar di bawah QR — fallback kalau
 * kamera siswa tidak bisa memindai (siswa mengetik kode secara manual, sesuai
 * pola input token di `MyExitPermitPage`/`MyLateArrivalPage`).
 */
export function TeacherQrPage() {
  const { data: me } = useMe();
  const generate = useGenerateTeacherQr();

  const [tokenData, setTokenData] = useState<TeacherQrToken | null>(null);
  const [remainingMs, setRemainingMs] = useState(TOKEN_TTL_MS);
  const [loadError, setLoadError] = useState(false);
  const busyRef = useRef(false);

  const regenerate = useCallback(() => {
    if (busyRef.current) return;
    busyRef.current = true;
    setLoadError(false);
    generate.mutate(undefined, {
      onSuccess: (data) => {
        busyRef.current = false;
        setTokenData(data);
      },
      onError: () => {
        busyRef.current = false;
        setLoadError(true);
      },
    });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [generate]);

  // Buat token pertama saat halaman dibuka.
  useEffect(() => {
    regenerate();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // Server mengirim event ini begitu siswa berhasil memindai token yang
  // sedang ditampilkan — regenerasi segera, jangan tunggu 60 detik habis.
  useRealtimeEvents({
    teacherqr: () => regenerate(),
  });

  // Countdown — dipicu SEKALI per `tokenData` (bukan tiap tick) supaya
  // kegagalan regenerasi tidak memicu percobaan ulang beruntun tiap 250ms;
  // kegagalan ditangani lewat `ErrorState` + tombol "Coba lagi" manual.
  useEffect(() => {
    if (!tokenData) return;
    let expiredTriggered = false;
    const expiresAt = new Date(tokenData.expires_at).getTime();

    function tick() {
      const remaining = expiresAt - Date.now();
      setRemainingMs(Math.max(0, remaining));
      if (remaining <= 0 && !expiredTriggered) {
        expiredTriggered = true;
        regenerate();
      }
    }
    tick();
    const id = window.setInterval(tick, 250);
    return () => window.clearInterval(id);
  }, [tokenData, regenerate]);

  if (!me) return null;
  if (me.role !== 'guru') return <Navigate to="/" replace />;

  const remainingSec = Math.max(0, Math.ceil(remainingMs / 1000));
  const progressPct = tokenData ? Math.max(0, Math.min(100, (remainingMs / TOKEN_TTL_MS) * 100)) : 0;

  return (
    <div className="mx-auto flex max-w-[480px] flex-col items-center gap-6 px-5 py-8 text-center">
      <div>
        <p className="text-[11px] font-semibold uppercase tracking-[0.1em] text-muted">QR Guru</p>
        <h1 className="text-[21px] font-semibold text-ink">QR Saya</h1>
        <p className="mt-1 text-[13px] text-muted">
          Tunjukkan ke siswa untuk persetujuan izin keluar / keterlambatan.
        </p>
      </div>

      {loadError ? (
        <ErrorState message="Gagal membuat kode QR baru." onRetry={regenerate} />
      ) : !tokenData ? (
        <Skeleton className="h-64 w-64" />
      ) : (
        <>
          <div className="flex flex-col items-center gap-4 rounded-lg border border-line p-6">
            <QrImage value={`${TEACHER_QR_PREFIX}${tokenData.token}`} sizePx={240} />
            <p className="num text-[22px] font-semibold tracking-[0.08em] text-ink">{tokenData.token}</p>
          </div>

          <div className="flex w-full flex-col gap-2">
            <div className="h-1 w-full overflow-hidden rounded-full bg-surface-2" aria-hidden="true">
              <div
                className="h-full rounded-full bg-primary transition-[width] duration-200 ease-linear motion-reduce:transition-none"
                style={{ width: `${progressPct}%` }}
              />
            </div>
            <p className="num text-[12px] text-muted" role="status">
              Berlaku {remainingSec} detik lagi
            </p>
          </div>
        </>
      )}
    </div>
  );
}
