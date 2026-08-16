import { useEffect, useState } from 'react';
import { Paperclip, Search } from 'lucide-react';
import { Dialog } from '../../components/ui/Dialog';
import { Button } from '../../components/ui/Button';
import { Field, Input, Textarea } from '../../components/ui/Field';
import { EmptyState } from '../../components/ui/EmptyState';
import { Skeleton } from '../../components/ui/Skeleton';
import { useToast } from '../../components/ui/Toast';
import { useDebouncedValue } from '../../lib/useDebouncedValue';
import { ApiError } from '../../lib/api';
import { useStudents } from '../students/api';
import { useCreateCounseling, useUpdateCounseling, useUploadCounselingEvidence } from './api';
import type { CounselingSession } from '../../lib/types';

interface StudentPick {
  id: string;
  name: string;
  nis?: string;
  class_name?: string;
}

interface RecordCounselingDialogProps {
  open: boolean;
  onClose: () => void;
  /** Terisi → mode ubah (siswa & tanggal tidak bisa diganti, hanya isi sesi + bukti). */
  editing?: CounselingSession;
  onSaved?: () => void;
}

const MAX_EVIDENCE_BYTES = 5 * 1024 * 1024;

/**
 * Dialog bersama "Catat Sesi Konseling" / "Ubah Sesi" — pola sama
 * `RecordViolationDialog` (cari siswa via search-select) ditambah 3 Textarea
 * berlabel jelas + upload bukti foto/PDF opsional.
 */
export function RecordCounselingDialog({ open, onClose, editing, onSaved }: RecordCounselingDialogProps) {
  const { showToast } = useToast();
  const [qInput, setQInput] = useState('');
  const q = useDebouncedValue(qInput, 300);
  const [selectedStudent, setSelectedStudent] = useState<StudentPick | null>(null);
  const [careerGoals, setCareerGoals] = useState('');
  const [problemDescription, setProblemDescription] = useState('');
  const [followUpPlan, setFollowUpPlan] = useState('');
  const [evidence, setEvidence] = useState<File | null>(null);
  const [formError, setFormError] = useState<string | null>(null);

  const { data: studentsResult, isLoading: studentsLoading } = useStudents({
    q: q || undefined,
    page: 1,
    perPage: 20,
  });
  const createSession = useCreateCounseling();
  const updateSession = useUpdateCounseling(editing?.id ?? '');
  const uploadEvidence = useUploadCounselingEvidence(editing?.id ?? '');
  const isSaving = createSession.isPending || updateSession.isPending || uploadEvidence.isPending;

  useEffect(() => {
    if (!open) return;
    if (editing) {
      setSelectedStudent({
        id: editing.student.id,
        name: editing.student.name,
        nis: editing.student.nis,
        class_name: editing.student.class_name,
      });
      setCareerGoals(editing.career_goals);
      setProblemDescription(editing.problem_description);
      setFollowUpPlan(editing.follow_up_plan);
    } else {
      setSelectedStudent(null);
      setCareerGoals('');
      setProblemDescription('');
      setFollowUpPlan('');
    }
    setQInput('');
    setEvidence(null);
    setFormError(null);
    createSession.reset();
    updateSession.reset();
    uploadEvidence.reset();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, editing]);

  function handlePickEvidence(file: File | null) {
    if (!file) return;
    if (file.size > MAX_EVIDENCE_BYTES) {
      setFormError('Ukuran bukti maksimal 5MB.');
      return;
    }
    setFormError(null);
    setEvidence(file);
  }

  async function handleSubmit() {
    setFormError(null);
    if (!selectedStudent) {
      setFormError('Pilih siswa terlebih dahulu.');
      return;
    }
    if (!careerGoals.trim() || !problemDescription.trim() || !followUpPlan.trim()) {
      setFormError('Tujuan karir, uraian masalah, dan rencana tindak lanjut wajib diisi.');
      return;
    }

    try {
      if (editing) {
        await updateSession.mutateAsync({
          career_goals: careerGoals.trim(),
          problem_description: problemDescription.trim(),
          follow_up_plan: followUpPlan.trim(),
        });
        if (evidence) await uploadEvidence.mutateAsync(evidence);
        showToast('Sesi konseling diperbarui.');
      } else {
        await createSession.mutateAsync({
          student_id: selectedStudent.id,
          career_goals: careerGoals.trim(),
          problem_description: problemDescription.trim(),
          follow_up_plan: followUpPlan.trim(),
          evidence,
        });
        showToast('Sesi konseling tercatat.');
      }
      onSaved?.();
      onClose();
    } catch (err) {
      setFormError(err instanceof ApiError ? err.message : 'Gagal menyimpan sesi konseling.');
    }
  }

  return (
    <Dialog open={open} onClose={onClose} title={editing ? 'Ubah Sesi Konseling' : 'Catat Sesi Konseling'}>
      <div className="flex flex-col gap-4">
        {editing || selectedStudent ? (
          <Field label="Siswa">
            <div className="flex items-center justify-between gap-3 rounded-lg border border-line bg-surface-2 px-3 py-2.5">
              <div className="min-w-0">
                <p className="truncate text-[14px] text-ink">{selectedStudent?.name}</p>
                {(selectedStudent?.nis || selectedStudent?.class_name) && (
                  <p className="truncate text-[12px] text-muted">
                    {[selectedStudent?.nis, selectedStudent?.class_name].filter(Boolean).join(' · ')}
                  </p>
                )}
              </div>
              {!editing && (
                <button
                  type="button"
                  onClick={() => setSelectedStudent(null)}
                  className="shrink-0 text-[12px] font-semibold text-primary hover:underline"
                >
                  Ganti
                </button>
              )}
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

        <Field label="Tujuan Karir" htmlFor="counseling-career-goals">
          <Textarea
            id="counseling-career-goals"
            value={careerGoals}
            onChange={(e) => setCareerGoals(e.target.value)}
            rows={2}
            placeholder="Mis. melanjutkan ke jurusan Teknik Informatika..."
          />
        </Field>

        <Field label="Uraian Masalah" htmlFor="counseling-problem">
          <Textarea
            id="counseling-problem"
            value={problemDescription}
            onChange={(e) => setProblemDescription(e.target.value)}
            rows={3}
            placeholder="Ceritakan masalah yang dibahas dalam sesi ini..."
          />
        </Field>

        <Field label="Rencana Tindak Lanjut" htmlFor="counseling-follow-up">
          <Textarea
            id="counseling-follow-up"
            value={followUpPlan}
            onChange={(e) => setFollowUpPlan(e.target.value)}
            rows={2}
            placeholder="Langkah berikutnya yang disepakati..."
          />
        </Field>

        <div className="flex flex-col gap-1.5">
          <label className="text-[12px] font-medium text-muted">Bukti (foto/PDF, opsional)</label>
          <label className="flex min-h-11 w-fit cursor-pointer items-center gap-2 rounded-lg border border-line bg-surface px-4 text-[14px] font-semibold text-ink transition-colors duration-150 hover:bg-surface-2">
            <Paperclip size={16} strokeWidth={2} aria-hidden="true" />
            {evidence ? evidence.name : editing?.has_evidence ? 'Ganti bukti' : 'Pilih berkas'}
            <input
              type="file"
              accept="image/png,image/jpeg,application/pdf"
              className="hidden"
              onChange={(e) => {
                handlePickEvidence(e.target.files?.[0] ?? null);
                e.target.value = '';
              }}
            />
          </label>
          <span className="text-[12px] text-muted">JPG/PNG/PDF, maksimal 5MB.</span>
        </div>

        {formError && <p className="text-[12px] text-danger">{formError}</p>}

        <div className="flex justify-end gap-2 pt-1">
          <Button type="button" variant="secondary" onClick={onClose}>
            Batal
          </Button>
          <Button type="button" onClick={handleSubmit} loading={isSaving}>
            {editing ? 'Simpan Perubahan' : 'Catat Sesi'}
          </Button>
        </div>
      </div>
    </Dialog>
  );
}
