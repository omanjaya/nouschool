import { useEffect, useState } from 'react';
import { FileWarning, Search } from 'lucide-react';
import { Dialog } from '../../components/ui/Dialog';
import { Button } from '../../components/ui/Button';
import { Field, Input, Select, Textarea } from '../../components/ui/Field';
import { EmptyState } from '../../components/ui/EmptyState';
import { Skeleton } from '../../components/ui/Skeleton';
import { useToast } from '../../components/ui/Toast';
import { useDebouncedValue } from '../../lib/useDebouncedValue';
import { ApiError } from '../../lib/api';
import { todayISODate } from '../../lib/date';
import { useStudents } from '../students/api';
import { disciplineLetterPdfUrl, useCreateDisciplineRecord, useDisciplineTypes } from './api';
import type { DisciplineIssuedLetter } from '../../lib/types';

export interface RecordViolationPresetStudent {
  id: string;
  name: string;
  nis?: string;
  class_name?: string;
}

interface RecordViolationDialogProps {
  open: boolean;
  onClose: () => void;
  /**
   * Prefill siswa (dipakai dari layar sesi absensi, docs/12 Gelombang A poin
   * 3) — kalau diisi, langkah pencarian siswa dilewati sepenuhnya.
   */
  presetStudent?: RecordViolationPresetStudent;
  attendanceSessionId?: string;
  onRecorded?: () => void;
}

/**
 * Dialog bersama "Catat Pelanggaran" — dipakai `/kedisiplinan` (cari siswa)
 * maupun `AttendanceSessionPage` (siswa & sesi sudah diketahui). Selalu
 * dirender TANPA syarat oleh pemanggil (pola `Dialog` lain di app ini,
 * `open` mengendalikan tampil/tidak) supaya dialog susulan Surat Peringatan
 * tetap bisa muncul setelah dialog utama menutup.
 */
export function RecordViolationDialog({
  open,
  onClose,
  presetStudent,
  attendanceSessionId,
  onRecorded,
}: RecordViolationDialogProps) {
  const { showToast } = useToast();
  const [qInput, setQInput] = useState('');
  const q = useDebouncedValue(qInput, 300);
  const [selectedStudent, setSelectedStudent] = useState<RecordViolationPresetStudent | null>(presetStudent ?? null);
  const [violationTypeId, setViolationTypeId] = useState('');
  const [occurredOn, setOccurredOn] = useState(todayISODate());
  const [note, setNote] = useState('');
  const [formError, setFormError] = useState<string | null>(null);
  const [issuedLetter, setIssuedLetter] = useState<DisciplineIssuedLetter | null>(null);

  const { data: studentsResult, isLoading: studentsLoading } = useStudents({
    q: q || undefined,
    page: 1,
    perPage: 20,
  });
  const { data: types } = useDisciplineTypes();
  const activeTypes = (types ?? []).filter((t) => t.active);
  const createRecord = useCreateDisciplineRecord();

  useEffect(() => {
    if (open) {
      setSelectedStudent(presetStudent ?? null);
      setQInput('');
      setViolationTypeId('');
      setOccurredOn(todayISODate());
      setNote('');
      setFormError(null);
      setIssuedLetter(null);
      createRecord.reset();
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, presetStudent]);

  function handleSubmit() {
    setFormError(null);
    if (!selectedStudent) {
      setFormError('Pilih siswa terlebih dahulu.');
      return;
    }
    if (!violationTypeId) {
      setFormError('Pilih jenis pelanggaran.');
      return;
    }

    createRecord.mutate(
      {
        student_id: selectedStudent.id,
        violation_type_id: violationTypeId,
        attendance_session_id: attendanceSessionId,
        note: note.trim() || undefined,
        occurred_on: occurredOn || undefined,
      },
      {
        onSuccess: (result) => {
          showToast(`Tercatat (+${result.record.violation.points} poin, total ${result.total_points})`);
          onRecorded?.();
          onClose();
          if (result.issued_letter) setIssuedLetter(result.issued_letter);
        },
        onError: (err) => {
          setFormError(err instanceof ApiError ? err.message : 'Gagal mencatat pelanggaran.');
        },
      },
    );
  }

  return (
    <>
      <Dialog open={open} onClose={onClose} title="Catat Pelanggaran">
        <div className="flex flex-col gap-4">
          {presetStudent ? (
            <Field label="Siswa">
              <div className="rounded-lg border border-line bg-surface-2 px-3 py-2.5">
                <p className="truncate text-[14px] text-ink">{presetStudent.name}</p>
                {(presetStudent.nis || presetStudent.class_name) && (
                  <p className="truncate text-[12px] text-muted">
                    {[presetStudent.nis, presetStudent.class_name].filter(Boolean).join(' · ')}
                  </p>
                )}
              </div>
            </Field>
          ) : selectedStudent ? (
            <Field label="Siswa">
              <div className="flex items-center justify-between gap-3 rounded-lg border border-line bg-surface-2 px-3 py-2.5">
                <div className="min-w-0">
                  <p className="truncate text-[14px] text-ink">{selectedStudent.name}</p>
                  {(selectedStudent.nis || selectedStudent.class_name) && (
                    <p className="truncate text-[12px] text-muted">
                      {[selectedStudent.nis, selectedStudent.class_name].filter(Boolean).join(' · ')}
                    </p>
                  )}
                </div>
                <button
                  type="button"
                  onClick={() => setSelectedStudent(null)}
                  className="shrink-0 text-[12px] font-semibold text-primary hover:underline"
                >
                  Ganti
                </button>
              </div>
            </Field>
          ) : (
            <Field label="Cari siswa">
              <div className="relative">
                <Search
                  size={16}
                  strokeWidth={2}
                  className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-muted"
                  aria-hidden="true"
                />
                <Input
                  value={qInput}
                  onChange={(e) => setQInput(e.target.value)}
                  placeholder="Cari nama atau NIS..."
                  className="pl-9"
                  aria-label="Cari siswa"
                  autoFocus
                />
              </div>
              <div className="mt-2 max-h-[220px] overflow-y-auto rounded-lg border border-line">
                {studentsLoading ? (
                  <div className="flex flex-col gap-2 p-3">
                    <Skeleton className="h-10 w-full" />
                    <Skeleton className="h-10 w-full" />
                  </div>
                ) : !studentsResult || studentsResult.items.length === 0 ? (
                  <EmptyState message="Siswa tidak ditemukan." />
                ) : (
                  studentsResult.items.map((s) => (
                    <button
                      key={s.id}
                      type="button"
                      onClick={() =>
                        setSelectedStudent({
                          id: s.id,
                          name: s.name,
                          nis: s.nis,
                          class_name: s.class?.name ?? 'Belum ada rombel',
                        })
                      }
                      className="flex w-full flex-col items-start gap-0.5 border-b border-line px-3 py-2.5 text-left last:border-b-0 hover:bg-surface-2"
                    >
                      <span className="truncate text-[14px] text-ink">{s.name}</span>
                      <span className="truncate text-[12px] text-muted">
                        {s.nis} · {s.class?.name ?? 'Belum ada rombel'}
                      </span>
                    </button>
                  ))
                )}
              </div>
            </Field>
          )}

          <Field label="Jenis pelanggaran" htmlFor="violation-type">
            <Select id="violation-type" value={violationTypeId} onChange={(e) => setViolationTypeId(e.target.value)}>
              <option value="">Pilih jenis...</option>
              {activeTypes.map((t) => (
                <option key={t.id} value={t.id}>
                  {t.name} (+{t.points} poin)
                </option>
              ))}
            </Select>
          </Field>

          <Field label="Tanggal" htmlFor="violation-date">
            <Input
              id="violation-date"
              type="date"
              value={occurredOn}
              onChange={(e) => setOccurredOn(e.target.value)}
            />
          </Field>

          <Field label="Catatan (opsional)" htmlFor="violation-note">
            <Textarea
              id="violation-note"
              value={note}
              onChange={(e) => setNote(e.target.value)}
              rows={3}
              placeholder="Mis. kronologi singkat kejadian..."
            />
          </Field>

          {formError && <p className="text-[12px] text-danger">{formError}</p>}

          <div className="flex justify-end gap-2 pt-1">
            <Button type="button" variant="secondary" onClick={onClose}>
              Batal
            </Button>
            <Button type="button" onClick={handleSubmit} loading={createRecord.isPending}>
              Catat Pelanggaran
            </Button>
          </div>
        </div>
      </Dialog>

      <Dialog open={issuedLetter !== null} onClose={() => setIssuedLetter(null)} title="Surat Peringatan Terbit">
        <div className="flex flex-col items-center gap-3 px-2 py-3 text-center">
          <FileWarning size={24} strokeWidth={2} className="text-danger" aria-hidden="true" />
          <p className="text-[14px] text-ink">
            Surat Peringatan SP{issuedLetter?.level} otomatis terbit — No. {issuedLetter?.number}
          </p>
        </div>
        <div className="mt-2 flex justify-end gap-2">
          <Button type="button" variant="secondary" onClick={() => setIssuedLetter(null)}>
            Tutup
          </Button>
          {issuedLetter && (
            <a
              href={disciplineLetterPdfUrl(issuedLetter.id)}
              target="_blank"
              rel="noreferrer"
              className="inline-flex min-h-11 items-center justify-center gap-2 rounded-lg bg-primary px-4 text-[14px] font-semibold text-primary-ink transition-opacity duration-150 hover:opacity-90"
            >
              Lihat Surat
            </a>
          )}
        </div>
      </Dialog>
    </>
  );
}
