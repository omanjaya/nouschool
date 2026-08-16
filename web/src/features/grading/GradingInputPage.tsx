import { useEffect, useRef, useState, type KeyboardEvent } from 'react';
import { useOutletContext } from 'react-router-dom';
import { Field, Input, Select } from '../../components/ui/Field';
import { Button } from '../../components/ui/Button';
import { ListRow } from '../../components/ui/ListRow';
import { Skeleton } from '../../components/ui/Skeleton';
import { EmptyState } from '../../components/ui/EmptyState';
import { ErrorState } from '../../components/ui/ErrorState';
import { useToast } from '../../components/ui/Toast';
import { ApiError } from '../../lib/api';
import { useComponentGrades, useGradingComponents, useSaveComponentGrades } from './api';
import type { GradingContext } from './GradingPage';
import type { GradingGradeInput } from '../../lib/types';

/** `''` = kosong/belum diisi — dipertahankan sebagai string mentah supaya input kosong tidak dipaksa jadi 0. */
type LocalScores = Record<string, string>;

function buildLocalScores(students: { student_id: string; score: number | null }[]): LocalScores {
  const next: LocalScores = {};
  students.forEach((s) => {
    next[s.student_id] = s.score === null ? '' : String(s.score);
  });
  return next;
}

/**
 * Tab "Input Nilai" `/nilai` — Select komponen lalu daftar siswa dengan Input
 * number 0-100 (kosong = belum), navigasi Enter ke baris berikut, penanda
 * merah tipis bila di bawah KKTP, simpan bulk (dirty guard pola
 * `AttendanceSessionPage`).
 */
export function GradingInputPage() {
  const { classId, subjectId } = useOutletContext<GradingContext>();
  const hasContext = Boolean(classId && subjectId);
  const { showToast } = useToast();

  const { data: componentsResult } = useGradingComponents({ classId, subjectId }, hasContext);
  const components = componentsResult?.items ?? [];
  const [componentId, setComponentId] = useState('');

  // Pilih komponen pertama otomatis begitu daftar termuat, atau kalau komponen
  // yang sedang dipilih sudah tidak ada lagi (mis. dihapus di tab Komponen).
  useEffect(() => {
    if (components.length === 0) {
      if (componentId) setComponentId('');
      return;
    }
    if (!components.some((c) => c.id === componentId)) setComponentId(components[0].id);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [components]);

  const selectedComponent = components.find((c) => c.id === componentId);
  const { data, isLoading, isError, refetch } = useComponentGrades(componentId, Boolean(componentId));
  const saveMutation = useSaveComponentGrades(componentId);

  const [scores, setScores] = useState<LocalScores | null>(null);
  const [dirty, setDirty] = useState(false);
  const inputRefs = useRef<Array<HTMLInputElement | null>>([]);

  useEffect(() => {
    if (data) {
      setScores(buildLocalScores(data.students));
      setDirty(false);
    }
  }, [data]);

  useEffect(() => {
    function onBeforeUnload(e: BeforeUnloadEvent) {
      if (!dirty) return;
      e.preventDefault();
      e.returnValue = '';
    }
    window.addEventListener('beforeunload', onBeforeUnload);
    return () => window.removeEventListener('beforeunload', onBeforeUnload);
  }, [dirty]);

  function handleChange(studentId: string, value: string) {
    setScores((prev) => (prev ? { ...prev, [studentId]: value } : prev));
    setDirty(true);
  }

  function handleKeyDown(e: KeyboardEvent<HTMLInputElement>, index: number) {
    if (e.key !== 'Enter') return;
    e.preventDefault();
    inputRefs.current[index + 1]?.focus();
  }

  async function handleSave() {
    if (!data || !scores) return;
    const grades: GradingGradeInput[] = data.students.map((s) => {
      const raw = scores[s.student_id] ?? '';
      const parsed = raw === '' ? null : Number(raw);
      return { student_id: s.student_id, score: parsed === null || Number.isNaN(parsed) ? null : parsed };
    });
    try {
      await saveMutation.mutateAsync(grades);
      setDirty(false);
      showToast('Nilai tersimpan.');
    } catch (err) {
      showToast(err instanceof ApiError ? err.message : 'Gagal menyimpan nilai.', 'error');
    }
  }

  if (!hasContext) {
    return <EmptyState message="Pilih kelas & mata pelajaran terlebih dahulu." />;
  }

  if (components.length === 0) {
    return <EmptyState message="Belum ada komponen nilai — tambahkan dulu di tab Komponen." />;
  }

  return (
    <div className="flex flex-col gap-4">
      <Field label="Komponen">
        <Select value={componentId} onChange={(e) => setComponentId(e.target.value)} aria-label="Pilih komponen nilai">
          {components.map((c) => (
            <option key={c.id} value={c.id}>
              {c.name}
            </option>
          ))}
        </Select>
      </Field>

      {isLoading || !scores ? (
        <div className="flex flex-col gap-2">
          <Skeleton className="h-12 w-full" />
          <Skeleton className="h-12 w-full" />
          <Skeleton className="h-12 w-full" />
        </div>
      ) : isError || !data ? (
        <ErrorState message="Gagal memuat nilai." onRetry={() => refetch()} />
      ) : data.students.length === 0 ? (
        <EmptyState message="Belum ada siswa di rombel ini." />
      ) : (
        <>
          <div>
            {data.students.map((s, index) => {
              const raw = scores[s.student_id] ?? '';
              const numeric = raw === '' ? null : Number(raw);
              const belowKktp =
                selectedComponent && numeric !== null && !Number.isNaN(numeric) && numeric < selectedComponent.kktp;
              return (
                <ListRow
                  key={s.student_id}
                  title={s.name}
                  subtitle={s.nis}
                  trailing={
                    <Input
                      ref={(el) => {
                        inputRefs.current[index] = el;
                      }}
                      type="number"
                      min={0}
                      max={100}
                      value={raw}
                      onChange={(e) => handleChange(s.student_id, e.target.value)}
                      onKeyDown={(e) => handleKeyDown(e, index)}
                      placeholder="-"
                      aria-label={`Nilai ${s.name}`}
                      className={`w-20 text-right ${belowKktp ? 'border-danger text-danger' : ''}`}
                    />
                  }
                />
              );
            })}
          </div>

          <Button onClick={handleSave} loading={saveMutation.isPending} disabled={!dirty} className="self-start">
            Simpan Nilai
          </Button>
        </>
      )}
    </div>
  );
}
