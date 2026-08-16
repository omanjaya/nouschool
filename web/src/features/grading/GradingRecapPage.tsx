import { useState } from 'react';
import { useOutletContext } from 'react-router-dom';
import { Download, Star, StarOff } from 'lucide-react';
import { Button } from '../../components/ui/Button';
import { Tag } from '../../components/ui/Tag';
import { Dialog } from '../../components/ui/Dialog';
import { Skeleton } from '../../components/ui/Skeleton';
import { EmptyState } from '../../components/ui/EmptyState';
import { ErrorState } from '../../components/ui/ErrorState';
import { useToast } from '../../components/ui/Toast';
import { ApiError } from '../../lib/api';
import { gradingExportUrl, useGradingRecap, useSetGradingPublication } from './api';
import { StarDialog, type StarDialogStudent } from './StarDialog';
import type { GradingContext } from './GradingPage';

interface StarTarget {
  student: StarDialogStudent;
  delta: 1 | -1;
}

/**
 * Tab "Rekap" `/nilai` — tabel scroll-x (siswa, tiap komponen, Final bold,
 * label), skor di bawah KKTP tampil danger, footer publikasi + export, aksi
 * bintang per baris.
 */
export function GradingRecapPage() {
  const { classId, subjectId } = useOutletContext<GradingContext>();
  const hasContext = Boolean(classId && subjectId);
  const { showToast } = useToast();

  const { data, isLoading, isError, refetch } = useGradingRecap({ classId, subjectId }, hasContext);
  const publishMutation = useSetGradingPublication();

  const [confirmPublishOpen, setConfirmPublishOpen] = useState(false);
  const [starTarget, setStarTarget] = useState<StarTarget | null>(null);

  function doPublish(published: boolean) {
    publishMutation.mutate(
      { class_id: classId, subject_id: subjectId, published },
      {
        onSuccess: () => {
          showToast(published ? 'Nilai dipublikasikan.' : 'Publikasi nilai dibatalkan.');
          setConfirmPublishOpen(false);
        },
        onError: (err) => {
          showToast(err instanceof ApiError ? err.message : 'Gagal mengubah status publikasi.', 'error');
          setConfirmPublishOpen(false);
        },
      },
    );
  }

  function handlePublishClick() {
    if (!data) return;
    if (data.published) {
      doPublish(false);
    } else {
      setConfirmPublishOpen(true);
    }
  }

  if (!hasContext) {
    return <EmptyState message="Pilih kelas & mata pelajaran terlebih dahulu." />;
  }

  if (isLoading) {
    return (
      <div className="flex flex-col gap-2">
        <Skeleton className="h-14 w-full" />
        <Skeleton className="h-14 w-full" />
        <Skeleton className="h-14 w-full" />
      </div>
    );
  }

  if (isError || !data) {
    return <ErrorState message="Gagal memuat rekap nilai." onRetry={() => refetch()} />;
  }

  if (data.students.length === 0) {
    return <EmptyState message="Belum ada siswa di rombel ini." />;
  }

  return (
    <div className="flex flex-col gap-4">
      <div className="overflow-x-auto rounded-lg border border-line">
        <table className="w-full min-w-[560px] border-collapse text-[13px]">
          <thead>
            <tr className="bg-surface-2 text-left text-[11px] font-semibold uppercase tracking-[0.05em] text-muted">
              <th className="px-3 py-2">Siswa</th>
              {data.components.map((c) => (
                <th key={c.id} className="px-3 py-2 text-right">
                  {c.name}
                </th>
              ))}
              <th className="px-3 py-2 text-right">Final</th>
              <th className="px-3 py-2">Label</th>
              <th className="px-3 py-2 text-right">Bintang</th>
            </tr>
          </thead>
          <tbody>
            {data.students.map((s) => (
              <tr key={s.student_id} className="border-b border-line last:border-0">
                <td className="px-3 py-2">
                  <p className="text-ink">{s.name}</p>
                  <p className="num text-[12px] text-muted">{s.nis}</p>
                </td>
                {data.components.map((c) => {
                  const score = s.scores[c.id];
                  const below = s.below_kktp.includes(c.id);
                  return (
                    <td key={c.id} className={`num px-3 py-2 text-right ${below ? 'text-danger' : 'text-ink'}`}>
                      {score === null || score === undefined ? '-' : score}
                    </td>
                  );
                })}
                <td className="num px-3 py-2 text-right text-[15px] font-bold text-ink">{s.final ?? '-'}</td>
                <td className="px-3 py-2">
                  {s.label ? <Tag variant="neutral">{s.label}</Tag> : <span className="text-muted">-</span>}
                </td>
                <td className="px-3 py-2 text-right">
                  <div className="flex justify-end gap-1">
                    <button
                      type="button"
                      onClick={() => setStarTarget({ student: { id: s.student_id, name: s.name }, delta: 1 })}
                      aria-label={`Tambah bintang ${s.name}`}
                      className="flex h-8 w-8 items-center justify-center rounded-lg text-muted transition-colors duration-150 hover:bg-surface-2 hover:text-st-hadir"
                    >
                      <Star size={16} strokeWidth={2} aria-hidden="true" />
                    </button>
                    <button
                      type="button"
                      onClick={() => setStarTarget({ student: { id: s.student_id, name: s.name }, delta: -1 })}
                      aria-label={`Kurangi bintang ${s.name}`}
                      className="flex h-8 w-8 items-center justify-center rounded-lg text-muted transition-colors duration-150 hover:bg-surface-2 hover:text-danger"
                    >
                      <StarOff size={16} strokeWidth={2} aria-hidden="true" />
                    </button>
                  </div>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      <div className="flex flex-wrap items-center justify-between gap-3 border-t border-line pt-4">
        <div className="flex items-center gap-2">
          <Tag variant={data.published ? 'success' : 'neutral'}>
            {data.published ? 'Terpublikasi' : 'Belum dipublikasikan'}
          </Tag>
          <Button variant="secondary" onClick={handlePublishClick} loading={publishMutation.isPending}>
            {data.published ? 'Batalkan Publikasi' : 'Publikasikan'}
          </Button>
        </div>
        <a
          href={gradingExportUrl({ classId, subjectId })}
          className="inline-flex min-h-11 items-center gap-2 rounded-lg border border-line px-4 text-[14px] font-medium text-ink transition-colors duration-150 hover:bg-surface-2"
        >
          <Download size={16} strokeWidth={2} aria-hidden="true" />
          Unduh Excel
        </a>
      </div>

      <Dialog open={confirmPublishOpen} onClose={() => setConfirmPublishOpen(false)} title="Publikasikan nilai?">
        <p className="text-[14px] text-ink">Siswa & orang tua akan melihat nilai.</p>
        <div className="mt-4 flex justify-end gap-2">
          <Button variant="secondary" onClick={() => setConfirmPublishOpen(false)}>
            Batal
          </Button>
          <Button onClick={() => doPublish(true)} loading={publishMutation.isPending}>
            Publikasikan
          </Button>
        </div>
      </Dialog>

      <StarDialog
        open={starTarget !== null}
        onClose={() => setStarTarget(null)}
        student={starTarget?.student ?? null}
        initialDelta={starTarget?.delta ?? 1}
      />
    </div>
  );
}
