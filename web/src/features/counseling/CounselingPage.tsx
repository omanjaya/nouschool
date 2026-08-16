import { useState } from 'react';
import { ExternalLink, Paperclip, Plus, Search, Users } from 'lucide-react';
import { ListRow } from '../../components/ui/ListRow';
import { Button } from '../../components/ui/Button';
import { Dialog } from '../../components/ui/Dialog';
import { Skeleton } from '../../components/ui/Skeleton';
import { EmptyState } from '../../components/ui/EmptyState';
import { ErrorState } from '../../components/ui/ErrorState';
import { Field, Input } from '../../components/ui/Field';
import { useToast } from '../../components/ui/Toast';
import { useDebouncedValue } from '../../lib/useDebouncedValue';
import { ApiError } from '../../lib/api';
import { formatRelativeTime } from '../../lib/date';
import { useMe } from '../auth/api';
import { useStudents } from '../students/api';
import { counselingReportUrl, useCounselingSessions, useDeleteCounseling } from './api';
import { RecordCounselingDialog } from './RecordCounselingDialog';
import type { CounselingSession } from '../../lib/types';

interface StudentPick {
  id: string;
  name: string;
  nis?: string;
  class_name?: string;
}

/**
 * /konseling — Konseling BK (Fase 14 Gelombang D). Guru pemegang flag
 * `leave_issuance` (BK) & admin bisa mencatat/ubah/hapus; kepsek read-only
 * (siswa/ortu sama sekali tidak melihat halaman ini — kartu Beranda hanya
 * dipasang untuk guru/admin/kepsek, lihat App.tsx).
 */
export function CounselingPage() {
  const { data: me } = useMe();
  const { showToast } = useToast();
  const [studentFilter, setStudentFilter] = useState<StudentPick | null>(null);
  const [qInput, setQInput] = useState('');
  const q = useDebouncedValue(qInput, 300);
  const [pickerOpen, setPickerOpen] = useState(false);
  const [expandedId, setExpandedId] = useState<string | null>(null);
  const [dialogOpen, setDialogOpen] = useState(false);
  const [editing, setEditing] = useState<CounselingSession | undefined>();
  const [deleting, setDeleting] = useState<CounselingSession | undefined>();

  const { data, isLoading, isError, refetch, hasNextPage, fetchNextPage, isFetchingNextPage } = useCounselingSessions({
    studentId: studentFilter?.id,
  });
  const { data: studentsResult, isLoading: studentsLoading } = useStudents({
    q: q || undefined,
    page: 1,
    perPage: 20,
  });
  const deleteSession = useDeleteCounseling();

  const isAdmin = me?.role === 'admin_sekolah';
  const isKepsek = me?.role === 'kepala_sekolah';
  const canRecord = !isKepsek;
  const items = data?.pages.flatMap((p) => p.items) ?? [];

  // Kepemilikan sesi dicek by id konselor (backend mengirim `counselor_id`).
  // Admin selalu boleh kelola; kepsek selalu read-only.
  function canManage(item: CounselingSession): boolean {
    if (isKepsek) return false;
    return isAdmin || String(item.counselor_id) === String(me?.id);
  }

  function openCreate() {
    setEditing(undefined);
    setDialogOpen(true);
  }

  function openEdit(item: CounselingSession) {
    setEditing(item);
    setDialogOpen(true);
  }

  function handleDelete() {
    if (!deleting) return;
    deleteSession.mutate(deleting.id, {
      onSuccess: () => {
        setDeleting(undefined);
        setExpandedId(null);
        showToast('Sesi konseling dihapus.');
      },
      onError: (err) => {
        showToast(err instanceof ApiError ? err.message : 'Gagal menghapus sesi konseling.', 'error');
      },
    });
  }

  return (
    <div className="mx-auto flex max-w-[640px] flex-col gap-5 px-5 py-6 lg:max-w-[720px]">
      <div className="flex items-start justify-between gap-3">
        <div>
          <p className="text-[11px] font-semibold uppercase tracking-[0.1em] text-muted">Bimbingan Konseling</p>
          <h1 className="text-[21px] font-semibold text-ink">Konseling BK</h1>
        </div>
        {canRecord && (
          <Button onClick={openCreate} className="shrink-0">
            <Plus size={16} strokeWidth={2} aria-hidden="true" />
            Catat Sesi
          </Button>
        )}
      </div>

      <div className="flex flex-wrap items-center gap-2">
        {studentFilter ? (
          <div className="flex items-center gap-2 rounded-lg border border-line bg-surface-2 px-3 py-2">
            <span className="text-[13px] text-ink">{studentFilter.name}</span>
            <button
              type="button"
              onClick={() => setStudentFilter(null)}
              className="text-[12px] font-semibold text-primary hover:underline"
            >
              Hapus filter
            </button>
          </div>
        ) : (
          <Button variant="secondary" onClick={() => setPickerOpen(true)}>
            <Search size={16} strokeWidth={2} aria-hidden="true" />
            Filter Siswa
          </Button>
        )}
      </div>

      {isLoading ? (
        <div className="flex flex-col gap-2">
          <Skeleton className="h-16 w-full" />
          <Skeleton className="h-16 w-full" />
          <Skeleton className="h-16 w-full" />
        </div>
      ) : isError ? (
        <ErrorState message="Gagal memuat sesi konseling." onRetry={() => refetch()} />
      ) : items.length === 0 ? (
        <EmptyState
          icon={Users}
          message={
            studentFilter
              ? `Belum ada sesi konseling untuk ${studentFilter.name}.`
              : 'Belum ada sesi konseling tercatat.'
          }
        />
      ) : (
        <div className="flex flex-col gap-3">
          <div>
            {items.map((item) => (
              <div key={item.id} className="border-b border-line last:border-b-0">
                <ListRow
                  className="border-b-0"
                  onClick={() => setExpandedId((cur) => (cur === item.id ? null : item.id))}
                  title={item.student.name}
                  subtitle={`${item.counselor_name} · ${formatRelativeTime(item.created_at)}`}
                  trailing={
                    item.has_evidence ? (
                      <Paperclip size={16} strokeWidth={2} className="text-muted" aria-hidden="true" />
                    ) : undefined
                  }
                />
                {expandedId === item.id && (
                  <div className="flex flex-col gap-3 px-1 pb-4 pt-1">
                    <DetailField label="Tujuan Karir" value={item.career_goals} />
                    <DetailField label="Uraian Masalah" value={item.problem_description} />
                    <DetailField label="Rencana Tindak Lanjut" value={item.follow_up_plan} />

                    <div className="flex flex-wrap items-center gap-2 pt-1">
                      <a
                        href={counselingReportUrl(item.id)}
                        target="_blank"
                        rel="noreferrer"
                        className="inline-flex min-h-9 items-center gap-1.5 rounded-lg border border-line px-3 text-[13px] font-medium text-ink transition-colors duration-150 hover:bg-surface-2"
                      >
                        <ExternalLink size={14} strokeWidth={2} aria-hidden="true" />
                        Buka Laporan
                      </a>
                      {canManage(item) && (
                        <>
                          <Button variant="secondary" onClick={() => openEdit(item)}>
                            Ubah
                          </Button>
                          <Button variant="danger" onClick={() => setDeleting(item)}>
                            Hapus
                          </Button>
                        </>
                      )}
                    </div>
                  </div>
                )}
              </div>
            ))}
          </div>

          {hasNextPage && (
            <Button
              variant="secondary"
              onClick={() => fetchNextPage()}
              loading={isFetchingNextPage}
              className="self-center"
            >
              Muat lebih banyak
            </Button>
          )}
        </div>
      )}

      <RecordCounselingDialog open={dialogOpen} onClose={() => setDialogOpen(false)} editing={editing} />

      <Dialog open={pickerOpen} onClose={() => setPickerOpen(false)} title="Filter Siswa">
        <div className="flex flex-col gap-3">
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
          </Field>
          <div className="max-h-[280px] overflow-y-auto rounded-lg border border-line">
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
                  onClick={() => {
                    setStudentFilter({ id: s.id, name: s.name, nis: s.nis, class_name: s.class?.name });
                    setPickerOpen(false);
                    setQInput('');
                  }}
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
        </div>
      </Dialog>

      <Dialog open={deleting !== undefined} onClose={() => setDeleting(undefined)} title="Hapus sesi konseling?">
        <p className="text-[14px] text-ink">
          Sesi konseling untuk {deleting?.student.name} pada {deleting ? formatRelativeTime(deleting.created_at) : ''}{' '}
          akan dihapus permanen.
        </p>
        {deleteSession.isError && (
          <p className="mt-3 text-[12px] text-danger">
            {deleteSession.error instanceof ApiError ? deleteSession.error.message : 'Gagal menghapus sesi konseling.'}
          </p>
        )}
        <div className="mt-4 flex justify-end gap-2">
          <Button type="button" variant="secondary" onClick={() => setDeleting(undefined)}>
            Batal
          </Button>
          <Button type="button" variant="danger" loading={deleteSession.isPending} onClick={handleDelete}>
            Hapus
          </Button>
        </div>
      </Dialog>
    </div>
  );
}

function DetailField({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex flex-col gap-1 rounded-lg border border-line bg-surface-2 p-3">
      <span className="text-[11px] font-semibold uppercase tracking-[0.1em] text-muted">{label}</span>
      <p className="whitespace-pre-wrap text-[14px] text-ink">{value}</p>
    </div>
  );
}
