import { useEffect, useState } from 'react';
import { useOutletContext } from 'react-router-dom';
import { Download, FileText, Info } from 'lucide-react';
import { Button } from '../../components/ui/Button';
import { Input } from '../../components/ui/Field';
import { EmptyState } from '../../components/ui/EmptyState';
import { ErrorState } from '../../components/ui/ErrorState';
import { Skeleton } from '../../components/ui/Skeleton';
import { StatTile } from '../../components/ui/StatTile';
import { Tag } from '../../components/ui/Tag';
import { useToast } from '../../components/ui/Toast';
import { ApiError } from '../../lib/api';
import {
  gradingReportExportUrl,
  useGradingAnalysis,
  useGradingManualScores,
  useGradingTpMappings,
  useSaveGradingManualScores,
  useSaveGradingTpMappings,
} from './api';
import type { GradingContext } from './GradingPage';
import type { GradingManualScoreInput, GradingManualScoreStudent, GradingTpMappingInput } from '../../lib/types';

interface SectionProps {
  classId: string;
  subjectId: string;
}

/** Guard `beforeunload` generik dipakai tiap seksi dengan draft lokal — pola sama `GradingInputPage`. */
function useDirtyGuard(dirty: boolean) {
  useEffect(() => {
    function onBeforeUnload(e: BeforeUnloadEvent) {
      if (!dirty) return;
      e.preventDefault();
      e.returnValue = '';
    }
    window.addEventListener('beforeunload', onBeforeUnload);
    return () => window.removeEventListener('beforeunload', onBeforeUnload);
  }, [dirty]);
}

/* ---- 1. Pemetaan TP ---- */

type TpLocal = Record<string, { tp_code: string; description: string }>;

function buildTpLocal(items: { component_id: string; tp_code: string; description: string }[]): TpLocal {
  const next: TpLocal = {};
  items.forEach((i) => {
    next[i.component_id] = { tp_code: i.tp_code, description: i.description };
  });
  return next;
}

function TpMappingSection({ classId, subjectId }: SectionProps) {
  const { showToast } = useToast();
  const filter = { classId, subjectId };
  const { data, isLoading, isError, refetch } = useGradingTpMappings(filter);
  const saveMutation = useSaveGradingTpMappings(filter);

  const [local, setLocal] = useState<TpLocal | null>(null);
  const [dirty, setDirty] = useState(false);
  useDirtyGuard(dirty);

  useEffect(() => {
    if (data) {
      setLocal(buildTpLocal(data.items));
      setDirty(false);
    }
  }, [data]);

  function handleChange(componentId: string, field: 'tp_code' | 'description', value: string) {
    setLocal((prev) => (prev ? { ...prev, [componentId]: { ...prev[componentId], [field]: value } } : prev));
    setDirty(true);
  }

  async function handleSave() {
    if (!data || !local) return;
    const mappings: GradingTpMappingInput[] = data.items.map((item) => ({
      component_id: item.component_id,
      tp_code: local[item.component_id]?.tp_code ?? '',
      description: local[item.component_id]?.description ?? '',
    }));
    try {
      await saveMutation.mutateAsync(mappings);
      setDirty(false);
      showToast('Pemetaan TP tersimpan.');
    } catch (err) {
      showToast(err instanceof ApiError ? err.message : 'Gagal menyimpan pemetaan TP.', 'error');
    }
  }

  return (
    <section className="flex flex-col gap-3">
      <div>
        <p className="text-[16px] font-semibold text-ink">Pemetaan TP</p>
        <p className="text-[12px] text-muted">
          Kaitkan tiap komponen bertipe TP dengan kode & deskripsi Tujuan Pembelajaran untuk ditampilkan di rapor.
        </p>
      </div>

      {isLoading || !local ? (
        <div className="flex flex-col gap-2">
          <Skeleton className="h-14 w-full" />
          <Skeleton className="h-14 w-full" />
        </div>
      ) : isError || !data ? (
        <ErrorState message="Gagal memuat pemetaan TP." onRetry={() => refetch()} />
      ) : data.items.length === 0 ? (
        <EmptyState message="Belum ada komponen bertipe TP untuk kelas & mapel ini." />
      ) : (
        <>
          <div>
            {data.items.map((item) => (
              <div
                key={item.component_id}
                className="flex flex-col gap-2 border-b border-line py-3 last:border-0 sm:flex-row sm:items-center"
              >
                <p className="min-w-0 flex-1 truncate text-[14px] text-ink">{item.component_name}</p>
                <Input
                  value={local[item.component_id]?.tp_code ?? ''}
                  onChange={(e) => handleChange(item.component_id, 'tp_code', e.target.value)}
                  placeholder="Kode TP"
                  aria-label={`Kode TP ${item.component_name}`}
                  className="font-mono sm:w-28"
                />
                <Input
                  value={local[item.component_id]?.description ?? ''}
                  onChange={(e) => handleChange(item.component_id, 'description', e.target.value)}
                  placeholder="Deskripsi TP"
                  aria-label={`Deskripsi TP ${item.component_name}`}
                  className="sm:flex-[2]"
                />
              </div>
            ))}
          </div>
          <Button onClick={handleSave} loading={saveMutation.isPending} disabled={!dirty} className="self-start">
            Simpan Pemetaan TP
          </Button>
        </>
      )}
    </section>
  );
}

/* ---- 2. Nilai Manual ---- */

interface ManualLocalRow {
  previous: string;
  manualScore: string;
  manualNote: string;
}
type ManualLocal = Record<string, ManualLocalRow>;

function buildManualLocal(students: GradingManualScoreStudent[]): ManualLocal {
  const next: ManualLocal = {};
  students.forEach((s) => {
    next[s.student_id] = {
      previous: s.previous === null ? '' : String(s.previous),
      manualScore: s.manual ? String(s.manual.score) : '',
      manualNote: s.manual?.note ?? '',
    };
  });
  return next;
}

function ManualScoresSection({ classId, subjectId }: SectionProps) {
  const { showToast } = useToast();
  const filter = { classId, subjectId };
  const { data, isLoading, isError, refetch } = useGradingManualScores(filter);
  const saveMutation = useSaveGradingManualScores(filter);

  const [local, setLocal] = useState<ManualLocal | null>(null);
  const [dirty, setDirty] = useState(false);
  useDirtyGuard(dirty);

  useEffect(() => {
    if (data) {
      setLocal(buildManualLocal(data.students));
      setDirty(false);
    }
  }, [data]);

  function handleChange(studentId: string, field: keyof ManualLocalRow, value: string) {
    setLocal((prev) => (prev ? { ...prev, [studentId]: { ...prev[studentId], [field]: value } } : prev));
    setDirty(true);
  }

  async function handleSave() {
    if (!data || !local) return;
    const items: GradingManualScoreInput[] = [];
    data.students.forEach((s) => {
      const row = local[s.student_id];
      if (!row) return;
      const previousNum = row.previous === '' ? null : Number(row.previous);
      items.push({
        student_id: s.student_id,
        kind: 'previous',
        score: previousNum === null || Number.isNaN(previousNum) ? null : previousNum,
      });
      const manualNum = row.manualScore === '' ? null : Number(row.manualScore);
      items.push({
        student_id: s.student_id,
        kind: 'manual',
        score: manualNum === null || Number.isNaN(manualNum) ? null : manualNum,
        note: row.manualNote.trim() || undefined,
      });
    });
    try {
      await saveMutation.mutateAsync(items);
      setDirty(false);
      showToast('Nilai manual tersimpan.');
    } catch (err) {
      showToast(err instanceof ApiError ? err.message : 'Gagal menyimpan nilai manual.', 'error');
    }
  }

  return (
    <section className="flex flex-col gap-3">
      <div>
        <p className="text-[16px] font-semibold text-ink">Nilai Manual</p>
        <p className="text-[12px] text-muted">
          Isi nilai semester lalu untuk rapor gabungan, atau timpa nilai akhir terhitung dengan nilai manual bila perlu.
        </p>
      </div>

      {isLoading || !local ? (
        <div className="flex flex-col gap-2">
          <Skeleton className="h-16 w-full" />
          <Skeleton className="h-16 w-full" />
        </div>
      ) : isError || !data ? (
        <ErrorState message="Gagal memuat nilai manual." onRetry={() => refetch()} />
      ) : data.students.length === 0 ? (
        <EmptyState message="Belum ada siswa di rombel ini." />
      ) : (
        <>
          <div className="hidden gap-4 border-b border-line px-1 pb-2 text-[11px] font-semibold uppercase tracking-[0.05em] text-muted sm:flex">
            <span className="flex-1">Siswa</span>
            <span className="w-24 text-right">Final Terhitung</span>
            <span className="w-28 text-right">Semester Lalu</span>
            <span className="w-48">Manual (menimpa)</span>
          </div>
          <div>
            {data.students.map((s) => {
              const row = local[s.student_id];
              const hasManual = (row?.manualScore ?? '') !== '';
              return (
                <div
                  key={s.student_id}
                  className="flex flex-col gap-2 border-b border-line py-3 last:border-0 sm:flex-row sm:items-center sm:gap-4"
                >
                  <div className="min-w-0 flex-1">
                    <p className="truncate text-[14px] text-ink">{s.name}</p>
                    <p className="num text-[12px] text-muted">{s.nis}</p>
                  </div>
                  <div className="flex items-center justify-between sm:w-24 sm:justify-end">
                    <span className="text-[11px] uppercase text-muted sm:hidden">Final terhitung</span>
                    <span className="num text-[14px] text-muted">{s.computed_final ?? '-'}</span>
                  </div>
                  <Input
                    type="number"
                    value={row?.previous ?? ''}
                    onChange={(e) => handleChange(s.student_id, 'previous', e.target.value)}
                    placeholder="-"
                    aria-label={`Nilai semester lalu ${s.name}`}
                    className="text-right sm:w-28"
                  />
                  <div className="flex flex-col gap-1 sm:w-48">
                    <div className="flex items-center gap-2">
                      <Input
                        type="number"
                        value={row?.manualScore ?? ''}
                        onChange={(e) => handleChange(s.student_id, 'manualScore', e.target.value)}
                        placeholder="-"
                        aria-label={`Nilai manual ${s.name}`}
                        className="text-right"
                      />
                      {hasManual && <Tag variant="warning">menimpa</Tag>}
                    </div>
                    <Input
                      value={row?.manualNote ?? ''}
                      onChange={(e) => handleChange(s.student_id, 'manualNote', e.target.value)}
                      placeholder="Catatan (opsional)"
                      aria-label={`Catatan nilai manual ${s.name}`}
                      className="h-9 text-[12px]"
                    />
                  </div>
                </div>
              );
            })}
          </div>
          <Button onClick={handleSave} loading={saveMutation.isPending} disabled={!dirty} className="self-start">
            Simpan Nilai Manual
          </Button>
        </>
      )}
    </section>
  );
}

/* ---- 3. Analisis ---- */

function AnalysisSection({ classId, subjectId }: SectionProps) {
  const { data, isLoading, isError, refetch } = useGradingAnalysis({ classId, subjectId });

  return (
    <section className="flex flex-col gap-4">
      <div>
        <p className="text-[16px] font-semibold text-ink">Analisis</p>
        <p className="text-[12px] text-muted">Ringkasan nilai akhir untuk kelas & mata pelajaran terpilih.</p>
      </div>

      {isLoading ? (
        <Skeleton className="h-24 w-full" />
      ) : isError || !data ? (
        <ErrorState message="Gagal memuat analisis nilai." onRetry={() => refetch()} />
      ) : data.count === 0 ? (
        <EmptyState icon={FileText} message="Belum ada nilai untuk dianalisis." />
      ) : (
        <>
          <div className="grid grid-cols-2 gap-4 sm:grid-cols-4">
            <StatTile label="Rata-rata" value={data.avg !== null ? data.avg.toFixed(1) : '-'} />
            <StatTile label="Min" value={data.min ?? '-'} variant="danger" />
            <StatTile label="Maks" value={data.max ?? '-'} variant="success" />
            <StatTile label="Jumlah" value={data.count} />
          </div>

          {data.per_label.length > 0 && (
            <div className="flex flex-col gap-2">
              <p className="text-[11px] font-semibold uppercase tracking-[0.05em] text-muted">Sebaran Label</p>
              {(() => {
                const maxCount = Math.max(...data.per_label.map((r) => r.count), 1);
                return data.per_label.map((row) => (
                  <div key={row.label} className="flex items-center gap-3">
                    <span className="w-20 shrink-0 truncate text-[13px] text-ink">{row.label}</span>
                    <div className="h-2 flex-1 overflow-hidden rounded-full bg-surface-2">
                      <div
                        className="h-full rounded-full bg-primary"
                        style={{ width: `${(row.count / maxCount) * 100}%` }}
                      />
                    </div>
                    <span className="num w-8 shrink-0 text-right text-[13px] text-muted">{row.count}</span>
                  </div>
                ));
              })()}
            </div>
          )}

          <div className="flex flex-col gap-2">
            <p className="text-[11px] font-semibold uppercase tracking-[0.05em] text-muted">Di Bawah KKTP</p>
            {data.below_kktp_per_component.length === 0 ? (
              <p className="text-[13px] text-muted">Tidak ada komponen dengan siswa di bawah KKTP.</p>
            ) : (
              <div>
                {data.below_kktp_per_component.map((row) => (
                  <div
                    key={row.component_id}
                    className="flex items-center justify-between border-b border-line py-2 last:border-0"
                  >
                    <span className="text-[13px] text-ink">{row.name}</span>
                    <span className="num text-[13px] text-danger">{row.count} siswa</span>
                  </div>
                ))}
              </div>
            )}
          </div>

          <div className="flex items-start gap-2 rounded-lg border border-line bg-surface-2 px-3 py-2.5">
            <Info size={16} strokeWidth={2} className="mt-0.5 shrink-0 text-muted" aria-hidden="true" />
            <p className="text-[12px] text-muted">
              Sumber nilai akhir: <span className="num font-semibold text-ink">{data.resolved_source.computed}</span>{' '}
              dihitung otomatis · <span className="num font-semibold text-ink">{data.resolved_source.manual}</span>{' '}
              nilai manual · <span className="num font-semibold text-ink">{data.resolved_source.none}</span> belum
              ada nilai.
            </p>
          </div>
        </>
      )}

      <div className="flex flex-col items-start gap-1 border-t border-line pt-4">
        <a
          href={gradingReportExportUrl(classId)}
          className="inline-flex min-h-11 items-center gap-2 rounded-lg border border-line px-4 text-[14px] font-medium text-ink transition-colors duration-150 hover:bg-surface-2"
        >
          <Download size={16} strokeWidth={2} aria-hidden="true" />
          Unduh Rapor Kelas (Excel)
        </a>
        <p className="text-[12px] text-muted">
          Mengunduh rapor untuk SELURUH mata pelajaran di kelas ini, bukan hanya mapel yang sedang dipilih di atas.
        </p>
      </div>
    </section>
  );
}

/**
 * Tab "Rapor" `/nilai` (Fase 15 GAP 1) — Pemetaan TP + Nilai Manual (guru/admin,
 * disembunyikan untuk kepsek lewat `GradingContext.readOnly` sama pola
 * `GradingRecapPage`) + Analisis (semua role yang boleh masuk `/nilai`,
 * termasuk kepsek baca-saja lewat `grading:read` — GAP 5).
 */
export function GradingReportPage() {
  const { classId, subjectId, readOnly } = useOutletContext<GradingContext>();
  const hasContext = Boolean(classId && subjectId);

  if (!hasContext) {
    return <EmptyState message="Pilih kelas & mata pelajaran terlebih dahulu." />;
  }

  return (
    <div className="flex flex-col gap-8">
      {!readOnly && (
        <>
          <TpMappingSection classId={classId} subjectId={subjectId} />
          <div className="border-t border-line" aria-hidden="true" />
          <ManualScoresSection classId={classId} subjectId={subjectId} />
          <div className="border-t border-line" aria-hidden="true" />
        </>
      )}
      <AnalysisSection classId={classId} subjectId={subjectId} />
    </div>
  );
}
