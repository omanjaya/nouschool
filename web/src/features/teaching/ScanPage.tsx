import { useCallback, useEffect, useRef, useState } from 'react';
import { Navigate, useNavigate } from 'react-router-dom';
import { DoorOpen, X } from 'lucide-react';
import { Button } from '../../components/ui/Button';
import { Dialog } from '../../components/ui/Dialog';
import { Field, Select } from '../../components/ui/Field';
import { ListRow } from '../../components/ui/ListRow';
import { Skeleton } from '../../components/ui/Skeleton';
import { EmptyState } from '../../components/ui/EmptyState';
import { ErrorState } from '../../components/ui/ErrorState';
import { useToast } from '../../components/ui/Toast';
import { ApiError } from '../../lib/api';
import { useMe } from '../auth/api';
import { useClasses } from '../classes/api';
import { useSubjects } from '../subjects/api';
import { useRooms } from '../schedule/api';
import { useCreateJournalManual, useTeachingScan } from './api';
import type { TeachingScanResult } from '../../lib/types';

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

const ROOM_QR_PREFIX = 'nouschool:room:';
const DETECT_INTERVAL_MS = 400;

type ScanInput = { room_token: string } | { room_id: string };

/**
 * /scan — "satu scan tiga hasil" (docs/06 #1). Kamera + `BarcodeDetector`
 * native kalau didukung & izin diberikan; kalau tidak, fallback ke daftar
 * ruangan (GET /api/rooms, tanpa token) yang dipilih manual — dianggap lebih
 * manusiawi daripada minta guru mengetik token QR.
 */
export function ScanPage() {
  const { data: me } = useMe();
  const navigate = useNavigate();
  const { showToast } = useToast();

  const videoRef = useRef<HTMLVideoElement>(null);
  const streamRef = useRef<MediaStream | null>(null);
  const detectTimerRef = useRef<number | null>(null);
  const detectorRef = useRef<BarcodeDetectorInstance | null>(null);
  const busyRef = useRef(false);

  const [cameraLive, setCameraLive] = useState(false);
  const [fallbackReason, setFallbackReason] = useState<string | null>(null);
  const [showFallback, setShowFallback] = useState(false);
  const [result, setResult] = useState<TeachingScanResult | null>(null);
  const [manualClassId, setManualClassId] = useState('');
  const [manualSubjectId, setManualSubjectId] = useState('');

  const scan = useTeachingScan();
  const createManual = useCreateJournalManual();
  // ASUMSI: GET /api/rooms bisa diakses guru (bukan admin-only) — kontrak
  // docs/04-schedule.md hanya menegaskan `qr_token` dibatasi utk admin,
  // bukan seluruh endpoint. Room di sini hanya dipakai id+name (tanpa token).
  const { data: rooms, isLoading: roomsLoading, isError: roomsError, refetch: refetchRooms } = useRooms();
  const { data: classes } = useClasses();
  const { data: subjects } = useSubjects();

  const stopCamera = useCallback(() => {
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

  const submitScan = useCallback(
    async (input: ScanInput) => {
      if (busyRef.current) return;
      busyRef.current = true;
      try {
        const res = await scan.mutateAsync(input);
        stopCamera();
        setShowFallback(false);
        setResult(res);
      } catch (err) {
        showToast(err instanceof ApiError ? err.message : 'Gagal memindai ruangan. Coba lagi.', 'error');
      } finally {
        busyRef.current = false;
      }
    },
    [scan, showToast, stopCamera],
  );

  const runDetect = useCallback(async () => {
    if (busyRef.current) return;
    const video = videoRef.current;
    const detector = detectorRef.current;
    if (!video || !detector || video.readyState < 2) return;
    try {
      const barcodes = await detector.detect(video);
      const match = barcodes.find((b) => b.rawValue.startsWith(ROOM_QR_PREFIX));
      if (!match) return;
      const token = match.rawValue.slice(ROOM_QR_PREFIX.length).trim();
      if (!token) return;
      await submitScan({ room_token: token });
    } catch {
      // Frame gagal dibaca (mis. video belum siap) — abaikan, coba lagi di interval berikutnya.
    }
  }, [submitScan]);

  // Inisialisasi kamera + detector sekali saat halaman dibuka; cleanup wajib
  // saat unmount/route change (matikan stream & timer, jangan biarkan kamera
  // tetap menyala di background).
  useEffect(() => {
    let cancelled = false;

    async function init() {
      if (!('BarcodeDetector' in window) || !window.BarcodeDetector) {
        setFallbackReason('Perangkat ini belum mendukung pemindaian QR otomatis. Pilih ruangan secara manual.');
        setShowFallback(true);
        return;
      }
      try {
        const stream = await navigator.mediaDevices.getUserMedia({
          video: { facingMode: 'environment' },
          audio: false,
        });
        if (cancelled) {
          stream.getTracks().forEach((track) => track.stop());
          return;
        }
        streamRef.current = stream;
        if (videoRef.current) {
          videoRef.current.srcObject = stream;
          await videoRef.current.play();
        }
        detectorRef.current = new window.BarcodeDetector({ formats: ['qr_code'] });
        setCameraLive(true);
        detectTimerRef.current = window.setInterval(runDetect, DETECT_INTERVAL_MS);
      } catch {
        setFallbackReason('Kamera tidak dapat diakses (izin ditolak atau tidak tersedia). Pilih ruangan secara manual.');
        setShowFallback(true);
      }
    }

    init();

    return () => {
      cancelled = true;
      stopCamera();
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // Reset form pilih kelas/mapel setiap kali dialog "tidak ada jadwal" dibuka ulang.
  useEffect(() => {
    if (result && result.needs_manual) {
      setManualClassId('');
      setManualSubjectId('');
    }
  }, [result]);

  useEffect(() => {
    if (result && !result.needs_manual) {
      showToast(`Jurnal dibuka — ${result.journal.class.name} · ${result.journal.subject?.name ?? 'Tanpa mapel'}`);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [result]);

  if (!me) return null;
  if (me.role !== 'guru') return <Navigate to="/" replace />;

  function handleCancel() {
    stopCamera();
    navigate(-1);
  }

  async function handleManualSubmit() {
    if (!result || result.needs_manual !== true || !manualClassId || !manualSubjectId) return;
    try {
      await createManual.mutateAsync({
        room_id: result.room.id,
        class_id: manualClassId,
        subject_id: manualSubjectId,
      });
      showToast('Jurnal dicatat di luar jadwal.');
      navigate('/jurnal');
    } catch (err) {
      showToast(err instanceof ApiError ? err.message : 'Gagal mencatat jurnal.', 'error');
    }
  }

  const needsManual = result && result.needs_manual === true ? result : null;
  const success = result && result.needs_manual === false ? result : null;

  return (
    <div className="fixed inset-0 z-30 bg-ink">
      {!showFallback && (
        <video ref={videoRef} className="h-full w-full object-cover" playsInline muted aria-hidden="true" />
      )}

      {!showFallback && (
        <>
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
              onClick={handleCancel}
              aria-label="Batal"
              className="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg text-[rgba(255,255,255,0.92)] transition-colors duration-150 hover:bg-[rgba(255,255,255,0.12)]"
            >
              <X size={20} strokeWidth={2} aria-hidden="true" />
            </button>
            <p className="flex-1 text-center text-[13px] font-medium text-[rgba(255,255,255,0.92)]">
              {cameraLive ? 'Arahkan kamera ke QR ruangan' : 'Menyiapkan kamera…'}
            </p>
            <span className="w-10 shrink-0" aria-hidden="true" />
          </div>

          <div
            className="absolute inset-x-0 bottom-0 flex flex-col items-center gap-3 px-4 py-5"
            style={{ backgroundColor: 'rgba(23,35,58,0.6)' }}
          >
            <p className="text-center text-[12px] text-[rgba(255,255,255,0.8)]">
              QR tercetak di setiap ruang kelas. Scan QR untuk membuka jurnal mengajar sekaligus menyiapkan absensi.
            </p>
            <button
              type="button"
              onClick={() => setShowFallback(true)}
              className="text-[13px] font-medium text-[rgba(255,255,255,0.92)] underline underline-offset-2"
            >
              Ruangan tidak bisa dipindai? Pilih manual
            </button>
          </div>
        </>
      )}

      {showFallback && (
        <div className="flex h-full flex-col bg-bg">
          <div className="flex items-center gap-3 border-b border-line px-4 py-3">
            <button
              type="button"
              onClick={handleCancel}
              aria-label="Batal"
              className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg text-muted transition-colors duration-150 hover:bg-surface-2 hover:text-ink"
            >
              <X size={20} strokeWidth={2} aria-hidden="true" />
            </button>
            <div className="min-w-0 flex-1">
              <h1 className="truncate text-[17px] font-semibold text-ink">Pilih Ruangan</h1>
              <p className="truncate text-[12px] text-muted">Kelas tempat Anda mengajar sekarang</p>
            </div>
          </div>

          <div className="mx-auto w-full max-w-[640px] flex-1 overflow-y-auto px-5 py-4">
            {fallbackReason && <p className="mb-4 text-[13px] text-muted">{fallbackReason}</p>}

            {roomsLoading ? (
              <div className="flex flex-col gap-2">
                <Skeleton className="h-14 w-full" />
                <Skeleton className="h-14 w-full" />
                <Skeleton className="h-14 w-full" />
              </div>
            ) : roomsError ? (
              <ErrorState message="Gagal memuat daftar ruangan." onRetry={() => refetchRooms()} />
            ) : rooms && rooms.length === 0 ? (
              <EmptyState icon={DoorOpen} message="Belum ada ruangan terdaftar di sekolah ini." />
            ) : (
              <div>
                {rooms?.map((room) => (
                  <ListRow
                    key={room.id}
                    className="min-h-[52px]"
                    leading={<DoorOpen size={20} strokeWidth={2} aria-hidden="true" />}
                    title={room.name}
                    onClick={() => submitScan({ room_id: room.id })}
                  />
                ))}
              </div>
            )}
          </div>
        </div>
      )}

      <Dialog
        open={Boolean(success)}
        onClose={() => navigate('/jurnal')}
        title="Jurnal Dibuka"
        footer={
          <>
            <Button variant="secondary" onClick={() => navigate('/jurnal')}>
              Nanti
            </Button>
            {success && (
              <Button onClick={() => navigate(`/absensi/sesi/${success.attendance_session_id}`)}>
                Isi Absensi Sekarang
              </Button>
            )}
          </>
        }
      >
        {success && (
          <div className="flex flex-col gap-1">
            <p className="text-[16px] font-semibold text-ink">
              {success.journal.subject?.name ?? 'Tanpa mapel'} · {success.journal.class.name}
            </p>
            <p className="text-[13px] text-muted">
              {success.journal.room?.name ?? 'Tanpa ruangan'} · sesi absensi sudah disiapkan.
            </p>
          </div>
        )}
      </Dialog>

      <Dialog
        open={Boolean(needsManual)}
        onClose={() => navigate('/jurnal')}
        title="Ruangan Tanpa Jadwal"
        footer={
          <>
            <Button variant="secondary" onClick={() => navigate('/jurnal')}>
              Batal
            </Button>
            <Button
              onClick={handleManualSubmit}
              loading={createManual.isPending}
              disabled={!manualClassId || !manualSubjectId}
            >
              Simpan Jurnal
            </Button>
          </>
        }
      >
        {needsManual && (
          <div className="flex flex-col gap-4">
            <p className="text-[14px] text-ink">
              Tidak ada jadwal Anda di jam ini di {needsManual.room.name}. Pilih kelas dan mata pelajaran untuk
              mencatat jurnal mengajar.
            </p>
            <Field label="Kelas" htmlFor="manual-class">
              <Select id="manual-class" value={manualClassId} onChange={(e) => setManualClassId(e.target.value)}>
                <option value="">Pilih kelas</option>
                {classes?.map((c) => (
                  <option key={c.id} value={c.id}>
                    {c.name}
                  </option>
                ))}
              </Select>
            </Field>
            <Field label="Mata pelajaran" htmlFor="manual-subject">
              <Select id="manual-subject" value={manualSubjectId} onChange={(e) => setManualSubjectId(e.target.value)}>
                <option value="">Pilih mapel</option>
                {subjects?.map((s) => (
                  <option key={s.id} value={s.id}>
                    {s.code} — {s.name}
                  </option>
                ))}
              </Select>
            </Field>
            {createManual.isError && (
              <p className="text-[12px] text-danger">
                {createManual.error instanceof ApiError ? createManual.error.message : 'Gagal mencatat jurnal.'}
              </p>
            )}
          </div>
        )}
      </Dialog>
    </div>
  );
}
