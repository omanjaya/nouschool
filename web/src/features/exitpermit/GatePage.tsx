import { useCallback, useEffect, useRef, useState, type FormEvent } from 'react';
import { Navigate } from 'react-router-dom';
import { CheckCircle2, DoorOpen, XCircle } from 'lucide-react';
import { Button } from '../../components/ui/Button';
import { Field, Input } from '../../components/ui/Field';
import { ListRow } from '../../components/ui/ListRow';
import { Skeleton } from '../../components/ui/Skeleton';
import { EmptyState } from '../../components/ui/EmptyState';
import { ErrorState } from '../../components/ui/ErrorState';
import { ApiError } from '../../lib/api';
import { formatTimeOfDay, todayISODate } from '../../lib/date';
import { useQrScanner } from '../../lib/useQrScanner';
import { useMe } from '../auth/api';
import { useGateHistory, useGateScan, EXIT_PERMIT_GATE_QR_PREFIX } from './api';
import type { ExitPermitGateScanResult } from '../../lib/types';

type ResultOverlay = { kind: 'success'; data: ExitPermitGateScanResult } | { kind: 'error'; message: string };

const AUTO_RESUME_MS = 3000;

function extractGateToken(raw: string): string {
  return raw.startsWith(EXIT_PERMIT_GATE_QR_PREFIX) ? raw.slice(EXIT_PERMIT_GATE_QR_PREFIX.length) : raw;
}

/**
 * /gerbang — pos scan security (Fase 14 Gelombang B2, docs/12-sion-parity.md
 * Gelombang B alur 2: gate token dari QR di HP siswa `nouschool:gate:{token}`).
 * Kamera menyala terus selama halaman terbuka; tiap hasil scan (sukses/gagal)
 * ditampilkan sebagai kartu BESAR berwarna, lalu otomatis kembali siap
 * memindai berikutnya setelah 3 detik — security tidak perlu menyentuh layar
 * antar siswa. Fallback ketik kode manual kalau kamera tidak tersedia atau
 * QR sulit dipindai.
 */
export function GatePage() {
  const { data: me } = useMe();
  const gateScan = useGateScan();
  const { data: history, isLoading: historyLoading, isError: historyError, refetch: refetchHistory } =
    useGateHistory(todayISODate());

  const [showFallback, setShowFallback] = useState(false);
  const [manualToken, setManualToken] = useState('');
  const [overlay, setOverlay] = useState<ResultOverlay | null>(null);
  const busyRef = useRef(false);
  const dismissTimerRef = useRef<number | null>(null);

  const scheduleDismiss = useCallback(() => {
    if (dismissTimerRef.current !== null) window.clearTimeout(dismissTimerRef.current);
    dismissTimerRef.current = window.setTimeout(() => {
      setOverlay(null);
      setManualToken('');
      busyRef.current = false;
    }, AUTO_RESUME_MS);
  }, []);

  const submitToken = useCallback(
    (raw: string) => {
      if (busyRef.current) return;
      const gate_token = extractGateToken(raw.trim());
      if (!gate_token) return;
      busyRef.current = true;
      gateScan.mutate(gate_token, {
        onSuccess: (data) => {
          setOverlay({ kind: 'success', data });
          scheduleDismiss();
        },
        onError: (err) => {
          setOverlay({
            kind: 'error',
            message: err instanceof ApiError ? err.message : 'Gagal memindai QR gerbang. Coba lagi.',
          });
          scheduleDismiss();
        },
      });
    },
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [gateScan, scheduleDismiss],
  );

  const handleDetect = useCallback(
    (rawValue: string) => {
      if (busyRef.current) return;
      if (!rawValue.startsWith(EXIT_PERMIT_GATE_QR_PREFIX)) return;
      submitToken(rawValue);
    },
    [submitToken],
  );

  const { videoRef, cameraLive, error: scanError, start: startCamera, stop: stopCamera } = useQrScanner(handleDetect);

  useEffect(() => {
    startCamera();
    return () => {
      stopCamera();
      if (dismissTimerRef.current !== null) window.clearTimeout(dismissTimerRef.current);
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  useEffect(() => {
    if (scanError) setShowFallback(true);
  }, [scanError]);

  function handleManualSubmit(e: FormEvent) {
    e.preventDefault();
    submitToken(manualToken);
  }

  if (!me) return null;
  if (me.role !== 'pegawai') return <Navigate to="/" replace />;

  return (
    <div className="mx-auto flex max-w-[640px] flex-col gap-6 px-5 py-6">
      <div>
        <p className="text-[11px] font-semibold uppercase tracking-[0.1em] text-muted">Gerbang</p>
        <h1 className="text-[21px] font-semibold text-ink">Scan QR Gerbang</h1>
        <p className="mt-1 text-[13px] text-muted">Pindai QR gerbang di HP siswa saat keluar sekolah.</p>
      </div>

      <div className="relative overflow-hidden rounded-lg bg-ink" style={{ aspectRatio: '3 / 4' }}>
        {!showFallback ? (
          <>
            <video ref={videoRef} className="h-full w-full object-cover" playsInline muted aria-hidden="true" />
            <div className="pointer-events-none absolute inset-0 flex items-center justify-center px-10">
              <div
                className="aspect-square w-full max-w-[240px] rounded-lg"
                style={{ border: '2px solid rgba(255,255,255,0.85)' }}
              />
            </div>
            <div
              className="absolute inset-x-0 bottom-0 flex flex-col items-center gap-2 px-4 py-4"
              style={{ backgroundColor: 'rgba(23,35,58,0.6)' }}
            >
              <p className="text-center text-[13px] font-medium text-[rgba(255,255,255,0.92)]">
                {cameraLive ? 'Arahkan kamera ke QR gerbang siswa' : 'Menyiapkan kamera…'}
              </p>
              <button
                type="button"
                onClick={() => setShowFallback(true)}
                className="text-[12px] font-medium text-[rgba(255,255,255,0.92)] underline underline-offset-2"
              >
                Ketik kode manual
              </button>
            </div>
          </>
        ) : (
          <div className="flex h-full flex-col items-center justify-center gap-4 bg-surface px-6">
            <form onSubmit={handleManualSubmit} className="flex w-full max-w-[320px] flex-col gap-3">
              <Field label="Kode gerbang" htmlFor="gate-manual-token">
                <Input
                  id="gate-manual-token"
                  value={manualToken}
                  onChange={(e) => setManualToken(e.target.value)}
                  placeholder="Kode dari layar HP siswa"
                  autoFocus
                  className="text-center text-[16px] font-semibold tracking-[0.06em]"
                />
              </Field>
              <Button type="submit" loading={gateScan.isPending} disabled={!manualToken.trim()}>
                Periksa
              </Button>
              <Button type="button" variant="secondary" onClick={() => setShowFallback(false)}>
                Pakai Kamera
              </Button>
            </form>
          </div>
        )}

        {overlay && (
          <div
            className={`absolute inset-0 flex flex-col items-center justify-center gap-2 px-6 text-center ${
              overlay.kind === 'success' ? 'bg-st-hadir' : 'bg-danger'
            }`}
            role="status"
          >
            {/* Teks putih di sini AMAN — latar `--st-hadir`/`--danger` keduanya token semantik
                TETAP (tidak ikut warna brand sekolah, docs/10 §1), selalu cukup gelap untuk
                kontras putih; pola sama dengan overlay kamera gelap di `ScanPage`/`TeacherTokenScanSheet`. */}
            <div style={{ color: '#FFFFFF' }} className="flex flex-col items-center gap-2">
              {overlay.kind === 'success' ? (
                <>
                  <CheckCircle2 size={40} strokeWidth={2} aria-hidden="true" />
                  <p className="text-[20px] font-semibold">{overlay.data.student.name}</p>
                  <p className="num text-[13px] opacity-90">
                    {overlay.data.student.nis} · {overlay.data.student.class_name}
                  </p>
                  <p className="text-[13px] opacity-90">{overlay.data.reason}</p>
                  <p className="num text-[13px] font-medium">
                    Berlaku s.d. {formatTimeOfDay(overlay.data.gate_expires_at)}
                  </p>
                </>
              ) : (
                <>
                  <XCircle size={40} strokeWidth={2} aria-hidden="true" />
                  <p className="text-[16px] font-semibold">{overlay.message}</p>
                </>
              )}
            </div>
          </div>
        )}
      </div>

      <div className="flex flex-col gap-3">
        <p className="text-[11px] font-semibold uppercase tracking-[0.1em] text-muted">Riwayat Hari Ini</p>
        {historyLoading ? (
          <Skeleton className="h-16 w-full" />
        ) : historyError ? (
          <ErrorState message="Gagal memuat riwayat gerbang." onRetry={() => refetchHistory()} />
        ) : !history || history.length === 0 ? (
          <EmptyState icon={DoorOpen} message="Belum ada siswa yang keluar gerbang hari ini." />
        ) : (
          <div>
            {history.map((item) => (
              <ListRow
                key={item.id}
                className="min-h-[56px]"
                title={item.student.name}
                subtitle={
                  <span className="num">
                    {item.student.nis} · {item.student.class_name} · {formatTimeOfDay(item.exited_at)}
                  </span>
                }
                trailing={<span className="max-w-[140px] truncate text-[12px] text-muted">{item.reason}</span>}
              />
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
