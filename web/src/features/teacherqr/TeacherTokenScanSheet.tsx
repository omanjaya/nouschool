import { useCallback, useEffect, useRef, useState } from 'react';
import { X } from 'lucide-react';
import { Button } from '../../components/ui/Button';
import { Input } from '../../components/ui/Field';
import { useQrScanner } from '../../lib/useQrScanner';
import { TEACHER_QR_PREFIX } from './api';

interface TeacherTokenScanSheetProps {
  open: boolean;
  onClose: () => void;
  /** Dipanggil dengan nilai APA ADANYA — mentah hasil scan (termasuk prefix `nouschool:tqr:`) ATAU hasil ketik manual (kode tanpa prefix, sesuai tampilan `TeacherQrPage`). Backend menerima raw value (docs/12-sion-parity.md Gelombang B alur 2-3: "siswa; kirim raw hasil scan/ketik"). */
  onSubmit: (rawValue: string) => void;
  submitting?: boolean;
  errorMessage?: string | null;
  instruction?: string;
}

/**
 * Overlay kamera + fallback ketik manual untuk memindai QR token guru
 * (pola sama `features/teaching/ScanPage.tsx`, dipakai bersama oleh
 * `MyExitPermitPage` — tiap tahap rantai dispensasi keluar — & `MyLateArrivalPage`,
 * dua fitur yang sama-sama memindai QR guru yang sama bentuknya).
 */
export function TeacherTokenScanSheet({
  open,
  onClose,
  onSubmit,
  submitting = false,
  errorMessage,
  instruction = 'Arahkan kamera ke QR guru',
}: TeacherTokenScanSheetProps) {
  const [showFallback, setShowFallback] = useState(false);
  const [manualCode, setManualCode] = useState('');
  const busyRef = useRef(false);

  const handleDetect = useCallback(
    (rawValue: string) => {
      if (busyRef.current) return;
      if (!rawValue.startsWith(TEACHER_QR_PREFIX)) return;
      busyRef.current = true;
      onSubmit(rawValue);
    },
    [onSubmit],
  );

  const { videoRef, cameraLive, error: scanError, start: startCamera, stop: stopCamera } = useQrScanner(handleDetect);

  // Reset & mulai kamera tiap kali sheet dibuka (dipanggil ulang antar tahap approval).
  useEffect(() => {
    if (!open) return;
    busyRef.current = false;
    setShowFallback(false);
    setManualCode('');
    startCamera();
    return () => stopCamera();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open]);

  // Kamera tidak didukung/izin ditolak → beralih ke input manual.
  useEffect(() => {
    if (scanError) setShowFallback(true);
  }, [scanError]);

  // Submit gagal (kode kedaluwarsa/salah tahap, dsb) → pindah ke tampilan
  // manual supaya pesan errornya bisa pakai token warna biasa (`text-danger`)
  // — overlay kamera gelap dipertahankan HANYA rgba putih seperti `ScanPage`,
  // tidak menambah warna baru di luar token untuk kasus ini.
  useEffect(() => {
    if (errorMessage) setShowFallback(true);
  }, [errorMessage]);

  // Sheet ditutup pemanggil setelah sukses — di sini hanya melepas kunci
  // anti-dobel-submit supaya scan berikutnya (setelah gagal) bisa dicoba lagi.
  useEffect(() => {
    if (!submitting) busyRef.current = false;
  }, [submitting]);

  if (!open) return null;

  function handleManualSubmit() {
    const code = manualCode.trim();
    if (!code) return;
    onSubmit(code);
  }

  return (
    <div className="fixed inset-0 z-40 bg-ink">
      {!showFallback && (
        <>
          <video ref={videoRef} className="h-full w-full object-cover" playsInline muted aria-hidden="true" />
          <div className="pointer-events-none absolute inset-0 flex items-center justify-center px-8">
            <div
              className="aspect-square w-full max-w-[280px] rounded-lg"
              style={{ border: '2px solid rgba(255,255,255,0.85)' }}
            />
          </div>
          <div
            className="absolute inset-x-0 top-0 flex items-center gap-3 px-4 py-3"
            style={{ backgroundColor: 'rgba(23,35,58,0.6)' }}
          >
            <button
              type="button"
              onClick={onClose}
              aria-label="Tutup"
              className="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg text-[rgba(255,255,255,0.92)] transition-colors duration-150 hover:bg-[rgba(255,255,255,0.12)]"
            >
              <X size={20} strokeWidth={2} aria-hidden="true" />
            </button>
            <p className="flex-1 text-center text-[13px] font-medium text-[rgba(255,255,255,0.92)]">
              {cameraLive ? instruction : 'Menyiapkan kamera…'}
            </p>
            <span className="w-10 shrink-0" aria-hidden="true" />
          </div>
          <div
            className="absolute inset-x-0 bottom-0 flex flex-col items-center gap-3 px-4 py-5"
            style={{ backgroundColor: 'rgba(23,35,58,0.6)' }}
          >
            <button
              type="button"
              onClick={() => setShowFallback(true)}
              className="text-[13px] font-medium text-[rgba(255,255,255,0.92)] underline underline-offset-2"
            >
              QR tidak bisa dipindai? Ketik kode
            </button>
          </div>
        </>
      )}

      {showFallback && (
        <div className="flex h-full flex-col bg-bg">
          <div className="flex items-center gap-3 border-b border-line px-4 py-3">
            <button
              type="button"
              onClick={onClose}
              aria-label="Tutup"
              className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg text-muted transition-colors duration-150 hover:bg-surface-2 hover:text-ink"
            >
              <X size={20} strokeWidth={2} aria-hidden="true" />
            </button>
            <div className="min-w-0 flex-1">
              <h1 className="truncate text-[17px] font-semibold text-ink">Ketik Kode QR</h1>
              <p className="truncate text-[12px] text-muted">Kode ditampilkan di layar HP guru.</p>
            </div>
          </div>
          <div className="mx-auto flex w-full max-w-[420px] flex-1 flex-col gap-4 px-5 py-6">
            <Input
              value={manualCode}
              onChange={(e) => setManualCode(e.target.value)}
              placeholder="Kode dari layar guru"
              autoFocus
              className="text-center text-[18px] font-semibold tracking-[0.08em]"
            />
            {errorMessage && <p className="text-[12px] text-danger">{errorMessage}</p>}
            <div className="flex gap-2">
              <Button variant="secondary" className="flex-1" onClick={() => setShowFallback(false)}>
                Pakai Kamera
              </Button>
              <Button className="flex-1" onClick={handleManualSubmit} loading={submitting} disabled={!manualCode.trim()}>
                Kirim
              </Button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
