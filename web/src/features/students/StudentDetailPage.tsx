import { useState } from 'react';
import { Link, useParams } from 'react-router-dom';
import { ChevronLeft, LogIn, Pencil, UserCheck, UserX } from 'lucide-react';
import { Card } from '../../components/ui/Card';
import { Skeleton } from '../../components/ui/Skeleton';
import { EmptyState } from '../../components/ui/EmptyState';
import { ErrorState } from '../../components/ui/ErrorState';
import { Tag } from '../../components/ui/Tag';
import { Button } from '../../components/ui/Button';
import { Dialog } from '../../components/ui/Dialog';
import { useToast } from '../../components/ui/Toast';
import { ApiError } from '../../lib/api';
import { formatDate } from '../../lib/date';
import { useMe } from '../auth/api';
import { ImpersonateUserDialog } from '../impersonateuser/ImpersonateUserDialog';
import { useSetStudentStatus, useStudent } from './api';
import { StudentFormDialog } from './StudentFormDialog';
import type { Student, StudentStatus } from '../../lib/types';

const STATUS_LABEL: Record<StudentStatus, string> = {
  active: 'Aktif',
  graduated: 'Lulus',
  moved: 'Pindah',
  dropped: 'Keluar',
};

const GENDER_LABEL: Record<string, string> = { L: 'Laki-laki', P: 'Perempuan' };

function InfoRow({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-center justify-between gap-3 border-b border-line py-2.5 last:border-b-0">
      <span className="text-[12px] text-muted">{label}</span>
      <span className="text-[14px] text-ink">{value}</span>
    </div>
  );
}

export function StudentDetailPage() {
  const { id } = useParams<{ id: string }>();
  const { data: me } = useMe();
  const { data: student, isLoading, isError, refetch } = useStudent(id);
  const [editOpen, setEditOpen] = useState(false);
  const [impersonateOpen, setImpersonateOpen] = useState(false);
  const [statusDialogOpen, setStatusDialogOpen] = useState(false);

  if (isLoading) {
    return (
      <div className="mx-auto flex max-w-[640px] flex-col gap-4 px-5 py-6">
        <Skeleton className="h-6 w-40" />
        <Skeleton className="h-48 w-full" />
      </div>
    );
  }

  if (isError) {
    return (
      <div className="mx-auto max-w-[640px] px-5 py-6">
        <ErrorState message="Gagal memuat data siswa." onRetry={() => refetch()} />
      </div>
    );
  }

  if (!student) {
    return (
      <div className="mx-auto max-w-[640px] px-5 py-6">
        <EmptyState icon={UserX} message="Siswa tidak ditemukan." />
      </div>
    );
  }

  const hasAccount = Boolean(student.user_id);
  const canImpersonate = me?.role === 'admin_sekolah' && hasAccount;
  const canManageStatus = me?.role === 'admin_sekolah' && hasAccount;
  const isInactive = student.membership_status === 'inactive';

  return (
    <div className="mx-auto flex max-w-[640px] flex-col gap-6 px-5 py-6">
      <div>
        <Link
          to="/data/siswa"
          className="mb-3 inline-flex items-center gap-1 text-[12px] font-medium text-muted hover:text-ink"
        >
          <ChevronLeft size={16} strokeWidth={2} aria-hidden="true" />
          Siswa
        </Link>
        <div className="flex items-start justify-between gap-3">
          <div>
            <h1 className="text-[21px] font-semibold text-ink">{student.name}</h1>
            <p className="text-[12px] text-muted">{student.nis}</p>
          </div>
          {student.status !== 'active' && <Tag variant="neutral">{STATUS_LABEL[student.status]}</Tag>}
        </div>
      </div>

      <Card className="flex flex-col gap-0">
        <div className="mb-2 flex items-center justify-between">
          <p className="text-[14px] font-semibold text-ink">Informasi Siswa</p>
          <Button variant="ghost" onClick={() => setEditOpen(true)}>
            <Pencil size={16} strokeWidth={2} aria-hidden="true" />
            Ubah
          </Button>
        </div>
        <InfoRow label="NIS" value={student.nis} />
        <InfoRow label="NISN" value={student.nisn ?? '-'} />
        <InfoRow label="Jenis kelamin" value={student.gender ? (GENDER_LABEL[student.gender] ?? student.gender) : '-'} />
        <InfoRow label="Tanggal lahir" value={student.birth_date ? formatDate(student.birth_date) : '-'} />
        <InfoRow label="Rombel" value={student.class?.name ?? 'Belum ada rombel'} />
        <InfoRow label="Status" value={STATUS_LABEL[student.status]} />
      </Card>

      <div>
        <p className="mb-3 text-[11px] font-semibold uppercase tracking-[0.1em] text-muted">Akun &amp; Undangan</p>
        <Card className="flex items-center justify-between gap-3">
          <div>
            <p className="text-[14px] text-ink">Akun siswa</p>
            <p className="text-[12px] text-muted">
              {hasAccount
                ? 'Siswa sudah mengaktifkan akun dengan kode undangan.'
                : 'Siswa belum mengaktifkan akun. Generate kode undangan dari halaman rombel.'}
            </p>
          </div>
          <Tag variant={hasAccount ? 'now' : 'neutral'}>{hasAccount ? 'Aktif' : 'Belum aktivasi'}</Tag>
        </Card>
        {(canImpersonate || canManageStatus) && (
          <div className="mt-3 flex flex-wrap gap-2">
            {canImpersonate && (
              <Button variant="secondary" onClick={() => setImpersonateOpen(true)}>
                <LogIn size={16} strokeWidth={2} aria-hidden="true" />
                Masuk sebagai Siswa Ini
              </Button>
            )}
            {canManageStatus && (
              <Button variant="secondary" onClick={() => setStatusDialogOpen(true)}>
                {isInactive ? (
                  <UserCheck size={16} strokeWidth={2} aria-hidden="true" />
                ) : (
                  <UserX size={16} strokeWidth={2} aria-hidden="true" />
                )}
                {isInactive ? 'Aktifkan Akun' : 'Nonaktifkan Akun'}
              </Button>
            )}
          </div>
        )}
      </div>

      <StudentFormDialog open={editOpen} onClose={() => setEditOpen(false)} student={student} />
      {canImpersonate && student.user_id && (
        <ImpersonateUserDialog
          open={impersonateOpen}
          onClose={() => setImpersonateOpen(false)}
          userId={student.user_id}
          userName={student.name}
        />
      )}
      {canManageStatus && student.user_id && (
        <DeactivateStudentDialog
          open={statusDialogOpen}
          onClose={() => setStatusDialogOpen(false)}
          student={student}
          userId={student.user_id}
        />
      )}
    </div>
  );
}

/** Dialog konfirmasi nonaktifkan/aktifkan akun siswa (Fase 15 GAP 6a) — sama pola `DeactivateTeacherDialog`. */
function DeactivateStudentDialog({
  open,
  onClose,
  student,
  userId,
}: {
  open: boolean;
  onClose: () => void;
  student: Student;
  userId: string;
}) {
  const isInactive = student.membership_status === 'inactive';
  const setStatus = useSetStudentStatus(student.id, userId);
  const { showToast } = useToast();

  function handleConfirm() {
    setStatus.mutate(isInactive ? 'active' : 'inactive', {
      onSuccess: () => {
        showToast(isInactive ? 'Akun siswa diaktifkan kembali.' : 'Akun siswa dinonaktifkan.');
        onClose();
      },
      onError: (err) => showToast(err instanceof ApiError ? err.message : 'Gagal mengubah status akun.', 'error'),
    });
  }

  return (
    <Dialog open={open} onClose={onClose} title={isInactive ? 'Aktifkan akun siswa?' : 'Nonaktifkan akun siswa?'}>
      <p className="text-[14px] text-ink">
        {isInactive ? (
          <>
            Akun <span className="font-medium">{student.name}</span> bisa login kembali seperti biasa.
          </>
        ) : (
          <>
            Akun <span className="font-medium">{student.name}</span> tidak bisa login sampai diaktifkan lagi; semua
            sesinya keluar.
          </>
        )}
      </p>
      {setStatus.isError && (
        <p className="mt-2 text-[12px] text-danger">
          {setStatus.error instanceof ApiError ? setStatus.error.message : 'Gagal mengubah status akun.'}
        </p>
      )}
      <div className="mt-4 flex justify-end gap-2">
        <Button type="button" variant="secondary" onClick={onClose}>
          Batal
        </Button>
        <Button
          type="button"
          variant={isInactive ? 'primary' : 'danger'}
          loading={setStatus.isPending}
          onClick={handleConfirm}
        >
          {isInactive ? 'Aktifkan' : 'Nonaktifkan'}
        </Button>
      </div>
    </Dialog>
  );
}
