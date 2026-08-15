import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { MessageSquare, ScanLine } from 'lucide-react';
import { AppBar } from '../../components/ui/AppBar';
import { ListRow } from '../../components/ui/ListRow';
import { StatusChip } from '../../components/ui/StatusChip';
import { Tag } from '../../components/ui/Tag';
import { Button } from '../../components/ui/Button';
import { Dialog } from '../../components/ui/Dialog';
import { Skeleton } from '../../components/ui/Skeleton';
import { ErrorState } from '../../components/ui/ErrorState';
import { useToast } from '../../components/ui/Toast';
import { ApiError } from '../../lib/api';
import { formatDate } from '../../lib/date';
import { hasFeature } from '../../lib/features';
import { useQrScanner } from '../../lib/useQrScanner';
import { useMe } from '../auth/api';
import { useAttendanceScan, useAttendanceSession, useFinalizeAttendanceSession, useSaveAttendanceRecords } from './api';
import { AttendanceNoteDialog } from './AttendanceNoteDialog';
import type { AttendanceRecordInput, AttendanceSessionResult, AttendanceStatus } from '../../lib/types';

/** Nilai sama diabaikan kalau terpindai lagi dalam jendela ini (kartu tetap di depan kamera). */
const SCAN_DEBOUNCE_MS = 3000;
/** Berapa lama flash tepi hijau/merah tampil setelah tiap hasil scan. */
const SCAN_FLASH_MS = 400;
/** Batas jumlah baris di daftar "hasil scan terakhir" (FIFO, terbaru di atas). */
const SCAN_FEED_LIMIT = 20;

interface ScanFeedItem {
  id: string;
  name: string;
  ok: boolean;
  alreadyMarked?: boolean;
  message?: string;
}

const CYCLE: AttendanceStatus[] = ['hadir', 'terlambat', 'izin', 'sakit', 'alpa'];

interface LocalRecord {
  status: AttendanceStatus;
  note: string;
}

type LocalRecords = Record<string, LocalRecord>;

function buildLocalRecords(result: AttendanceSessionResult): LocalRecords {
  const next: LocalRecords = {};
  result.students.forEach((s) => {
    next[s.student_id] = { status: s.record?.status ?? 'hadir', note: s.record?.note ?? '' };
  });
  return next;
}

export function AttendanceSessionPage() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const { showToast } = useToast();
  const { data: me } = useMe();
  const { data, isLoading, isError, refetch } = useAttendanceSession(id);
  const saveMutation = useSaveAttendanceRecords(id ?? '');
  const finalizeMutation = useFinalizeAttendanceSession(id ?? '');
  const scanMutation = useAttendanceScan(id ?? '');

  const [records, setRecords] = useState<LocalRecords | null>(null);
  const [dirty, setDirty] = useState(false);
  const [hasSaved, setHasSaved] = useState(false);
  const [noteStudentId, setNoteStudentId] = useState<string | null>(null);
  const [confirmFinalizeOpen, setConfirmFinalizeOpen] = useState(false);
  const [confirmLeaveOpen, setConfirmLeaveOpen] = useState(false);

  const [scanOpen, setScanOpen] = useState(false);
  const [scanFeed, setScanFeed] = useState<ScanFeedItem[]>([]);
  const [scanFlash, setScanFlash] = useState<'success' | 'error' | null>(null);
  const lastSeenRef = useRef<Map<string, number>>(new Map());
  const feedIdRef = useRef(0);

  const handleScanDetect = useCallback(
    (rawValue: string) => {
      const now = Date.now();
      const last = lastSeenRef.current.get(rawValue);
      if (last && now - last < SCAN_DEBOUNCE_MS) return;
      lastSeenRef.current.set(rawValue, now);

      scanMutation.mutate(rawValue, {
        onSuccess: (res) => {
          setScanFlash('success');
          window.setTimeout(() => setScanFlash(null), SCAN_FLASH_MS);
          feedIdRef.current += 1;
          setScanFeed((prev) =>
            [{ id: `s-${feedIdRef.current}`, name: res.name, ok: true, alreadyMarked: res.already_marked }, ...prev].slice(
              0,
              SCAN_FEED_LIMIT,
            ),
          );
          setRecords((prev) => {
            if (!prev) return prev;
            const cur = prev[res.student_id];
            return { ...prev, [res.student_id]: { status: res.status, note: cur?.note ?? '' } };
          });
          setHasSaved(true);
        },
        onError: (err) => {
          setScanFlash('error');
          window.setTimeout(() => setScanFlash(null), SCAN_FLASH_MS);
          feedIdRef.current += 1;
          setScanFeed((prev) =>
            [
              {
                id: `e-${feedIdRef.current}`,
                name: err instanceof ApiError ? err.message : 'Gagal memindai kartu.',
                ok: false,
              },
              ...prev,
            ].slice(0, SCAN_FEED_LIMIT),
          );
        },
      });
    },
    [scanMutation],
  );

  const { videoRef: scanVideoRef, cameraLive: scanCameraLive, error: scanCamError, start: startScan, stop: stopScan } =
    useQrScanner(handleScanDetect);

  useEffect(() => {
    if (!scanOpen) return;
    startScan();
    return () => stopScan();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [scanOpen]);

  // Reset daftar hasil & dedupe token setiap kali overlay scan dibuka ulang.
  useEffect(() => {
    if (scanOpen) {
      setScanFeed([]);
      lastSeenRef.current = new Map();
    }
  }, [scanOpen]);

  // Inisialisasi state lokal sekali dari data server (default siswa tanpa record = hadir).
  useEffect(() => {
    if (data && records === null) {
      setRecords(buildLocalRecords(data));
      if (data.students.some((s) => s.record)) setHasSaved(true);
    }
  }, [data, records]);

  // Peringatkan sebelum tutup tab/reload kalau ada perubahan belum tersimpan.
  useEffect(() => {
    function onBeforeUnload(e: BeforeUnloadEvent) {
      if (!dirty) return;
      e.preventDefault();
      e.returnValue = '';
    }
    window.addEventListener('beforeunload', onBeforeUnload);
    return () => window.removeEventListener('beforeunload', onBeforeUnload);
  }, [dirty]);

  const isReadOnly = data ? data.session.status === 'finalized' && me?.role !== 'admin_sekolah' : true;

  const counts = useMemo(() => {
    const c: Record<AttendanceStatus, number> = { hadir: 0, terlambat: 0, izin: 0, sakit: 0, alpa: 0 };
    if (records) Object.values(records).forEach((r) => c[r.status]++);
    return c;
  }, [records]);

  function syncFromServer(result: AttendanceSessionResult) {
    setRecords(buildLocalRecords(result));
    setDirty(false);
  }

  function goBack() {
    navigate('/absensi');
  }

  function handleBack() {
    if (dirty) {
      setConfirmLeaveOpen(true);
      return;
    }
    goBack();
  }

  function cycleStatus(studentId: string) {
    if (isReadOnly) return;
    setRecords((prev) => {
      if (!prev) return prev;
      const cur = prev[studentId];
      const idx = CYCLE.indexOf(cur.status);
      const next = CYCLE[(idx + 1) % CYCLE.length];
      return { ...prev, [studentId]: { ...cur, status: next } };
    });
    setDirty(true);
  }

  function markAllHadir() {
    if (isReadOnly) return;
    setRecords((prev) => {
      if (!prev) return prev;
      const next: LocalRecords = {};
      Object.entries(prev).forEach(([sid, r]) => {
        next[sid] = { ...r, status: 'hadir' };
      });
      return next;
    });
    setDirty(true);
  }

  function saveNote(studentId: string, note: string) {
    setRecords((prev) => {
      if (!prev) return prev;
      return { ...prev, [studentId]: { ...prev[studentId], note } };
    });
    setDirty(true);
    setNoteStudentId(null);
  }

  async function handleSave() {
    if (!data || !records) return;
    const payload: AttendanceRecordInput[] = data.students.map((s) => {
      const r = records[s.student_id];
      return { student_id: s.student_id, status: r.status, note: r.note ? r.note : undefined };
    });
    try {
      const result = await saveMutation.mutateAsync(payload);
      syncFromServer(result);
      setHasSaved(true);
      showToast('Absensi tersimpan');
    } catch (err) {
      showToast(err instanceof ApiError ? err.message : 'Gagal menyimpan absensi.', 'error');
    }
  }

  async function handleFinalize() {
    try {
      const result = await finalizeMutation.mutateAsync();
      syncFromServer(result);
      setConfirmFinalizeOpen(false);
      showToast('Sesi absensi dikunci.');
    } catch (err) {
      setConfirmFinalizeOpen(false);
      showToast(err instanceof ApiError ? err.message : 'Gagal mengunci sesi.', 'error');
    }
  }

  if (isLoading || (data && !records)) {
    return (
      <div className="mx-auto flex max-w-[640px] flex-col gap-4 px-5 py-6">
        <Skeleton className="h-6 w-40" />
        <Skeleton className="h-14 w-full" />
        <Skeleton className="h-14 w-full" />
        <Skeleton className="h-14 w-full" />
      </div>
    );
  }

  if (isError || !data || !records) {
    return (
      <div className="mx-auto max-w-[640px] px-5 py-6">
        <ErrorState message="Gagal memuat sesi absensi." onRetry={() => refetch()} />
      </div>
    );
  }

  const noteStudent = noteStudentId ? data.students.find((s) => s.student_id === noteStudentId) : undefined;

  return (
    <div className="flex min-h-dvh flex-col">
      <div className="sticky top-0 z-20 flex flex-col">
        <AppBar
          title={data.session.class_name}
          subtitle={`Absensi harian · ${formatDate(data.session.date)}`}
          onBack={handleBack}
          action={
            data.session.status === 'open' && hasFeature(me, 'qr_card') ? (
              <Button variant="secondary" onClick={() => setScanOpen(true)}>
                <ScanLine size={16} strokeWidth={2} aria-hidden="true" />
                Scan Kartu
              </Button>
            ) : data.session.status === 'finalized' ? (
              <Tag variant="done">Terkunci</Tag>
            ) : undefined
          }
        />
        <div className="flex flex-wrap items-center gap-2 border-b border-line bg-surface px-4 py-2.5">
          {CYCLE.map((status) => (
            <StatusChip key={status} status={status} size="sm" count={counts[status]} />
          ))}
        </div>
      </div>

      <div className="mx-auto w-full max-w-[640px] flex-1 px-5 py-4">
        <div className="mb-1 flex items-center justify-between">
          <p className="text-[11px] font-semibold uppercase tracking-[0.1em] text-muted">
            {data.students.length} siswa
          </p>
          {!isReadOnly && (
            <button
              type="button"
              onClick={markAllHadir}
              className="text-[12px] font-medium text-primary hover:underline"
            >
              Semua hadir
            </button>
          )}
        </div>

        <div>
          {data.students.map((s) => {
            const r = records[s.student_id];
            return (
              <ListRow
                key={s.student_id}
                className="min-h-[52px]"
                title={s.name}
                subtitle={
                  <span className="flex flex-col gap-0.5">
                    <span className="num truncate">{s.nis}</span>
                    {r.note && <span className="truncate">{r.note}</span>}
                  </span>
                }
                trailing={
                  <div className="flex items-center gap-1.5">
                    <button
                      type="button"
                      onClick={() => setNoteStudentId(s.student_id)}
                      aria-label={r.note ? `Ubah catatan ${s.name}` : `Tambah catatan ${s.name}`}
                      className={`flex h-9 w-9 items-center justify-center rounded-lg transition-colors duration-150 hover:bg-surface-2 ${
                        r.note ? 'text-primary' : 'text-muted hover:text-ink'
                      }`}
                    >
                      <MessageSquare size={16} strokeWidth={2} aria-hidden="true" />
                    </button>
                    <StatusChip status={r.status} size="md" onClick={isReadOnly ? undefined : () => cycleStatus(s.student_id)} />
                  </div>
                }
              />
            );
          })}
        </div>
      </div>

      {!isReadOnly && (
        <div className="sticky bottom-[68px] z-20 flex gap-2 border-t border-line bg-surface px-5 py-3 lg:bottom-0">
          <Button className="flex-1" onClick={handleSave} loading={saveMutation.isPending}>
            Simpan Absensi
          </Button>
          {data.session.status === 'open' && hasSaved && (
            <Button variant="secondary" onClick={() => setConfirmFinalizeOpen(true)} loading={finalizeMutation.isPending}>
              Kunci Sesi
            </Button>
          )}
        </div>
      )}

      {noteStudent && (
        <AttendanceNoteDialog
          open={Boolean(noteStudentId)}
          studentName={noteStudent.name}
          initialNote={records[noteStudent.student_id]?.note ?? ''}
          readOnly={isReadOnly}
          onClose={() => setNoteStudentId(null)}
          onSave={(note) => saveNote(noteStudent.student_id, note)}
        />
      )}

      <Dialog open={confirmFinalizeOpen} onClose={() => setConfirmFinalizeOpen(false)} title="Kunci sesi absensi?">
        <p className="text-[14px] text-ink">Setelah dikunci hanya admin yang bisa mengubah.</p>
        <div className="mt-4 flex justify-end gap-2">
          <Button variant="secondary" onClick={() => setConfirmFinalizeOpen(false)}>
            Batal
          </Button>
          <Button onClick={handleFinalize} loading={finalizeMutation.isPending}>
            Kunci Sesi
          </Button>
        </div>
      </Dialog>

      <Dialog open={confirmLeaveOpen} onClose={() => setConfirmLeaveOpen(false)} title="Keluar tanpa menyimpan?">
        <p className="text-[14px] text-ink">Perubahan absensi belum disimpan. Yakin ingin keluar?</p>
        <div className="mt-4 flex justify-end gap-2">
          <Button variant="secondary" onClick={() => setConfirmLeaveOpen(false)}>
            Batal
          </Button>
          <Button variant="danger" onClick={goBack}>
            Keluar Tanpa Simpan
          </Button>
        </div>
      </Dialog>

      {scanOpen && (
        <div className="fixed inset-0 z-30 bg-ink">
          <video ref={scanVideoRef} className="h-full w-full object-cover" playsInline muted aria-hidden="true" />

          {/* Flash tepi tipis: hijau = tersimpan, merah = gagal (docs/10 — tanpa glow/shadow dekoratif, hanya inset border). */}
          <div
            aria-hidden="true"
            className="pointer-events-none absolute inset-0 transition-opacity duration-200"
            style={{
              opacity: scanFlash ? 1 : 0,
              boxShadow:
                scanFlash === 'success'
                  ? 'inset 0 0 0 6px rgba(14,107,78,0.85)'
                  : scanFlash === 'error'
                    ? 'inset 0 0 0 6px rgba(179,56,42,0.85)'
                    : 'none',
            }}
          />

          <div className="pointer-events-none absolute inset-0 flex items-center justify-center px-10 pb-[40%]">
            <div
              className="aspect-square w-full max-w-[240px] rounded-lg"
              style={{ border: '2px solid rgba(255,255,255,0.85)' }}
            />
          </div>

          <div
            className="absolute inset-x-0 top-0 flex items-center gap-3 px-4 py-3"
            style={{ backgroundColor: 'rgba(23,35,58,0.6)' }}
          >
            <p className="flex-1 text-center text-[13px] font-medium text-[rgba(255,255,255,0.92)]">
              {scanCamError ?? (scanCameraLive ? 'Arahkan kamera ke kartu QR siswa' : 'Menyiapkan kamera…')}
            </p>
          </div>

          <div className="absolute inset-x-0 bottom-0 flex max-h-[45%] flex-col gap-2 border-t border-line bg-surface px-4 py-3">
            <div className="flex items-center justify-between gap-3">
              <p className="text-[11px] font-semibold uppercase tracking-[0.1em] text-muted">Hasil Scan Terakhir</p>
              <Button variant="secondary" onClick={() => setScanOpen(false)}>
                Selesai Scan
              </Button>
            </div>
            <div className="flex-1 overflow-y-auto">
              {scanFeed.length === 0 ? (
                <p className="py-4 text-center text-[13px] text-muted">Belum ada kartu terpindai.</p>
              ) : (
                <div>
                  {scanFeed.map((item) => (
                    <ListRow
                      key={item.id}
                      title={item.name}
                      trailing={
                        item.ok ? (
                          <Tag variant={item.alreadyMarked ? 'neutral' : 'success'}>
                            {item.alreadyMarked ? 'Sudah tercatat' : 'Hadir'}
                          </Tag>
                        ) : (
                          <span className="text-[12px] text-danger">{item.message}</span>
                        )
                      }
                    />
                  ))}
                </div>
              )}
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
