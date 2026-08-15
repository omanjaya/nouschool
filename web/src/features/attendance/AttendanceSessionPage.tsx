import { useEffect, useMemo, useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { MessageSquare } from 'lucide-react';
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
import { useMe } from '../auth/api';
import { useAttendanceSession, useFinalizeAttendanceSession, useSaveAttendanceRecords } from './api';
import { AttendanceNoteDialog } from './AttendanceNoteDialog';
import type { AttendanceRecordInput, AttendanceSessionResult, AttendanceStatus } from '../../lib/types';

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

  const [records, setRecords] = useState<LocalRecords | null>(null);
  const [dirty, setDirty] = useState(false);
  const [hasSaved, setHasSaved] = useState(false);
  const [noteStudentId, setNoteStudentId] = useState<string | null>(null);
  const [confirmFinalizeOpen, setConfirmFinalizeOpen] = useState(false);
  const [confirmLeaveOpen, setConfirmLeaveOpen] = useState(false);

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
          action={data.session.status === 'finalized' ? <Tag variant="done">Terkunci</Tag> : undefined}
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
    </div>
  );
}
