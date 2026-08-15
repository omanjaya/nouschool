import { useMemo, useState } from 'react';
import { Link, useParams } from 'react-router-dom';
import { ChevronLeft, Copy, Download, KeyRound, Pencil, Trash2, UserPlus, Users } from 'lucide-react';
import { Card } from '../../components/ui/Card';
import { Skeleton } from '../../components/ui/Skeleton';
import { EmptyState } from '../../components/ui/EmptyState';
import { ErrorState } from '../../components/ui/ErrorState';
import { ListRow } from '../../components/ui/ListRow';
import { Button } from '../../components/ui/Button';
import { Dialog } from '../../components/ui/Dialog';
import { useToast } from '../../components/ui/Toast';
import { ApiError } from '../../lib/api';
import { useStudents } from '../students/api';
import { useClasses, useGenerateInvitations, useRemoveClassStudent } from './api';
import { ClassFormDialog } from './ClassFormDialog';
import { AddStudentsDialog } from './AddStudentsDialog';
import type { GeneratedInvitation } from '../../lib/types';

const ROSTER_PER_PAGE = 200;

function downloadInvitationsCsv(className: string, rows: GeneratedInvitation[]) {
  const header = 'Nama,Kode Siswa,Kode Wali\n';
  const body = rows
    .map((r) => `"${r.student_name.replace(/"/g, '""')}",${r.student_code ?? ''},${r.guardian_code ?? ''}`)
    .join('\n');
  const blob = new Blob([header + body], { type: 'text/csv;charset=utf-8;' });
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = `kode-undangan-${className.replace(/\s+/g, '-').toLowerCase()}.csv`;
  document.body.appendChild(a);
  a.click();
  a.remove();
  URL.revokeObjectURL(url);
}

export function ClassDetailPage() {
  const { id } = useParams<{ id: string }>();
  const { data: classes, isLoading, isError, refetch } = useClasses();
  const schoolClass = useMemo(() => classes?.find((c) => c.id === id), [classes, id]);

  const [editOpen, setEditOpen] = useState(false);
  const [addOpen, setAddOpen] = useState(false);
  const [removeId, setRemoveId] = useState<string | null>(null);
  const [invitations, setInvitations] = useState<GeneratedInvitation[] | null>(null);
  const { showToast } = useToast();

  const roster = useStudents({ classId: id, page: 1, perPage: ROSTER_PER_PAGE });
  const removeStudent = useRemoveClassStudent(id ?? '');
  const generateInvitations = useGenerateInvitations();

  if (isLoading) {
    return (
      <div className="mx-auto flex max-w-[640px] flex-col gap-4 px-5 py-6">
        <Skeleton className="h-6 w-40" />
        <Skeleton className="h-40 w-full" />
      </div>
    );
  }

  if (isError) {
    return (
      <div className="mx-auto max-w-[640px] px-5 py-6">
        <ErrorState message="Gagal memuat data rombel." onRetry={() => refetch()} />
      </div>
    );
  }

  if (!schoolClass || !id) {
    return (
      <div className="mx-auto max-w-[640px] px-5 py-6">
        <EmptyState icon={Users} message="Rombel tidak ditemukan." />
      </div>
    );
  }

  function handleRemove() {
    if (!removeId) return;
    removeStudent.mutate(removeId, {
      onSuccess: () => {
        showToast('Siswa dikeluarkan dari rombel.');
        setRemoveId(null);
      },
    });
  }

  function handleGenerate() {
    generateInvitations.mutate(id!, {
      onSuccess: (data) => setInvitations(data),
    });
  }

  async function copyCode(code: string) {
    try {
      await navigator.clipboard.writeText(code);
      showToast('Kode disalin.');
    } catch {
      showToast('Gagal menyalin kode.', 'error');
    }
  }

  return (
    <div className="mx-auto flex max-w-[640px] flex-col gap-6 px-5 py-6">
      <div>
        <Link
          to="/data/rombel"
          className="mb-3 inline-flex items-center gap-1 text-[12px] font-medium text-muted hover:text-ink"
        >
          <ChevronLeft size={16} strokeWidth={2} aria-hidden="true" />
          Rombel
        </Link>
        <div className="flex items-start justify-between gap-3">
          <div>
            <h1 className="text-[21px] font-semibold text-ink">{schoolClass.name}</h1>
            <p className="text-[12px] text-muted">
              {schoolClass.grade}
              {schoolClass.major ? ` ${schoolClass.major}` : ''} · Wali: {schoolClass.homeroom_teacher?.name ?? '-'}
            </p>
          </div>
          <Button variant="ghost" onClick={() => setEditOpen(true)}>
            <Pencil size={16} strokeWidth={2} aria-hidden="true" />
            Ubah
          </Button>
        </div>
      </div>

      <div className="flex flex-wrap gap-2">
        <Button variant="secondary" onClick={() => setAddOpen(true)}>
          <UserPlus size={16} strokeWidth={2} aria-hidden="true" />
          Tambah Siswa
        </Button>
        <Button variant="secondary" onClick={handleGenerate} loading={generateInvitations.isPending}>
          <KeyRound size={16} strokeWidth={2} aria-hidden="true" />
          Generate Kode Undangan
        </Button>
      </div>

      {generateInvitations.isError && (
        <p className="text-[12px] text-danger">
          {generateInvitations.error instanceof ApiError
            ? generateInvitations.error.message
            : 'Gagal membuat kode undangan.'}
        </p>
      )}

      {invitations && (
        <Card className="flex flex-col gap-3">
          <div className="flex items-center justify-between gap-3">
            <p className="text-[14px] font-semibold text-ink">Kode Undangan</p>
            <Button variant="secondary" onClick={() => downloadInvitationsCsv(schoolClass.name, invitations)}>
              <Download size={16} strokeWidth={2} aria-hidden="true" />
              Unduh CSV
            </Button>
          </div>
          <div className="flex flex-col">
            {invitations.map((inv) => (
              <div key={inv.student_id} className="flex flex-col gap-2 border-b border-line py-3 last:border-b-0">
                <p className="text-[14px] text-ink">{inv.student_name}</p>
                <div className="flex flex-wrap gap-4">
                  <CodeField label="Kode siswa" code={inv.student_code} onCopy={copyCode} />
                  <CodeField label="Kode wali" code={inv.guardian_code} onCopy={copyCode} />
                </div>
              </div>
            ))}
          </div>
        </Card>
      )}

      <div>
        <p className="mb-3 text-[11px] font-semibold uppercase tracking-[0.1em] text-muted">
          Anggota ({schoolClass.student_count})
        </p>
        {roster.isLoading ? (
          <div className="flex flex-col gap-2">
            <Skeleton className="h-12 w-full" />
            <Skeleton className="h-12 w-full" />
          </div>
        ) : roster.isError ? (
          <ErrorState message="Gagal memuat anggota rombel." onRetry={() => roster.refetch()} />
        ) : roster.data && roster.data.items.length === 0 ? (
          <EmptyState
            icon={Users}
            message="Belum ada siswa di rombel ini."
            action={
              <Button variant="secondary" onClick={() => setAddOpen(true)}>
                Tambah Siswa
              </Button>
            }
          />
        ) : (
          <div>
            {roster.data?.items.map((s) => (
              <ListRow
                key={s.id}
                title={s.name}
                subtitle={s.nis}
                trailing={
                  <button
                    type="button"
                    onClick={() => setRemoveId(s.id)}
                    aria-label={`Keluarkan ${s.name} dari rombel`}
                    className="flex h-9 w-9 items-center justify-center rounded-lg text-muted transition-colors duration-150 hover:bg-surface-2 hover:text-danger"
                  >
                    <Trash2 size={16} strokeWidth={2} aria-hidden="true" />
                  </button>
                }
              />
            ))}
          </div>
        )}
      </div>

      <ClassFormDialog open={editOpen} onClose={() => setEditOpen(false)} schoolClass={schoolClass} />
      <AddStudentsDialog open={addOpen} onClose={() => setAddOpen(false)} classId={id} />

      <Dialog open={removeId !== null} onClose={() => setRemoveId(null)} title="Keluarkan siswa dari rombel?">
        <p className="text-[14px] text-ink">Siswa akan dikeluarkan dari rombel ini. Data siswa tidak dihapus.</p>
        <div className="mt-4 flex justify-end gap-2">
          <Button type="button" variant="secondary" onClick={() => setRemoveId(null)}>
            Batal
          </Button>
          <Button type="button" variant="danger" loading={removeStudent.isPending} onClick={handleRemove}>
            Keluarkan
          </Button>
        </div>
      </Dialog>
    </div>
  );
}

function CodeField({ label, code, onCopy }: { label: string; code: string | null; onCopy: (code: string) => void }) {
  if (!code) {
    return (
      <div>
        <p className="text-[11px] text-muted">{label}</p>
        <p className="text-[14px] text-muted">-</p>
      </div>
    );
  }
  return (
    <div>
      <p className="text-[11px] text-muted">{label}</p>
      <button
        type="button"
        onClick={() => onCopy(code)}
        className="num inline-flex items-center gap-1.5 text-[14px] font-semibold text-ink hover:text-primary"
      >
        {code}
        <Copy size={14} strokeWidth={2} aria-hidden="true" />
      </button>
    </div>
  );
}
