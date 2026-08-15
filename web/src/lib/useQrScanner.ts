import { useCallback, useEffect, useRef, useState, type RefObject } from 'react';

/**
 * BarcodeDetector API native — belum masuk `lib.dom` TypeScript resmi dan
 * dukungan browser masih terbatas (terutama non-Chromium). Deklarasi tipe
 * minimal lokal di sini supaya pemakaiannya type-safe (bukan `any` telanjang)
 * TANPA menambah dependency baru (batasan tugas — dilarang pakai library QR).
 */
interface DetectedBarcodeShape {
  rawValue: string;
  format: string;
}

interface BarcodeDetectorInstance {
  detect(source: CanvasImageSource): Promise<DetectedBarcodeShape[]>;
}

interface BarcodeDetectorConstructor {
  new (options?: { formats?: string[] }): BarcodeDetectorInstance;
}

declare global {
  interface Window {
    BarcodeDetector?: BarcodeDetectorConstructor;
  }
}

const DEFAULT_DETECT_INTERVAL_MS = 400;

export interface UseQrScannerResult {
  /** Pasang ke elemen `<video>` yang menampilkan preview kamera. */
  videoRef: RefObject<HTMLVideoElement | null>;
  /** Stream kamera sudah menyala & siap dipindai (dipakai teks status UI). */
  cameraLive: boolean;
  /** Perangkat mendukung `BarcodeDetector` native. */
  supported: boolean;
  /** Pesan error kalau kamera tidak bisa dipakai (tidak didukung / izin ditolak / tidak tersedia). */
  error: string | null;
  /** Minta izin kamera & mulai deteksi berkelanjutan. Aman dipanggil ulang (idempotent per generasi). */
  start: () => void;
  /** Matikan stream kamera & hentikan deteksi. WAJIB dipanggil saat unmount/tutup layar. */
  stop: () => void;
}

/**
 * Kamera + `BarcodeDetector` native dipakai bersama oleh `ScanPage` (teaching,
 * QR ruangan, docs/06) dan mode scan kartu QR siswa di layar sesi absensi
 * (Fase 8, docs/05). Hook ini TIDAK tahu arti nilai QR yang terbaca — pemanggil
 * yang memfilter prefix & memutuskan aksi lewat callback `onDetect(rawValue)`.
 * Deteksi berjalan tiap `intervalMs` selama kamera aktif; overlap frame dicegah
 * lewat flag internal (tidak menunggu pemrosesan `onDetect` pemanggil selesai —
 * debounce per nilai/token adalah tanggung jawab pemanggil).
 */
export function useQrScanner(
  onDetect: (rawValue: string) => void,
  intervalMs: number = DEFAULT_DETECT_INTERVAL_MS,
): UseQrScannerResult {
  const videoRef = useRef<HTMLVideoElement>(null);
  const streamRef = useRef<MediaStream | null>(null);
  const detectTimerRef = useRef<number | null>(null);
  const detectorRef = useRef<BarcodeDetectorInstance | null>(null);
  const readingRef = useRef(false);
  const generationRef = useRef(0);
  const onDetectRef = useRef(onDetect);
  onDetectRef.current = onDetect;

  const [cameraLive, setCameraLive] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [supported] = useState(() => 'BarcodeDetector' in window && Boolean(window.BarcodeDetector));

  const stop = useCallback(() => {
    // Menaikkan generasi membatalkan init() async yang mungkin masih berjalan
    // (mis. getUserMedia belum resolve saat stop() dipanggil).
    generationRef.current += 1;
    if (detectTimerRef.current !== null) {
      window.clearInterval(detectTimerRef.current);
      detectTimerRef.current = null;
    }
    if (streamRef.current) {
      streamRef.current.getTracks().forEach((track) => track.stop());
      streamRef.current = null;
    }
    setCameraLive(false);
  }, []);

  const runDetect = useCallback(async () => {
    if (readingRef.current) return;
    const video = videoRef.current;
    const detector = detectorRef.current;
    if (!video || !detector || video.readyState < 2) return;
    readingRef.current = true;
    try {
      const barcodes = await detector.detect(video);
      barcodes.forEach((b) => {
        if (b.rawValue) onDetectRef.current(b.rawValue);
      });
    } catch {
      // Frame gagal dibaca (mis. video belum siap) — abaikan, coba lagi di interval berikutnya.
    } finally {
      readingRef.current = false;
    }
  }, []);

  const start = useCallback(() => {
    setError(null);
    const generation = ++generationRef.current;

    async function init() {
      if (!('BarcodeDetector' in window) || !window.BarcodeDetector) {
        setError('Perangkat ini belum mendukung pemindaian QR otomatis.');
        return;
      }
      try {
        const stream = await navigator.mediaDevices.getUserMedia({
          video: { facingMode: 'environment' },
          audio: false,
        });
        if (generationRef.current !== generation) {
          // stop() (atau start() lain) dipanggil sebelum izin kamera selesai — buang stream ini.
          stream.getTracks().forEach((track) => track.stop());
          return;
        }
        streamRef.current = stream;
        if (videoRef.current) {
          videoRef.current.srcObject = stream;
          await videoRef.current.play();
        }
        detectorRef.current = new window.BarcodeDetector!({ formats: ['qr_code'] });
        setCameraLive(true);
        detectTimerRef.current = window.setInterval(runDetect, intervalMs);
      } catch {
        setError('Kamera tidak dapat diakses (izin ditolak atau tidak tersedia).');
      }
    }

    init();
  }, [runDetect, intervalMs]);

  // Jaring pengaman: kalau pemanggil lupa stop() saat unmount, kamera tidak
  // pernah tertinggal menyala di background.
  useEffect(() => stop, [stop]);

  return { videoRef, cameraLive, supported, error, start, stop };
}
