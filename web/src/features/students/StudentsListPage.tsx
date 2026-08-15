import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { ChevronLeft, ChevronRight, FileSpreadsheet, Plus, Search, Upload, Users } from 'lucide-react';
import { ListRow } from '../../components/ui/ListRow';
import { Skeleton } from '../../components/ui/Skeleton';
import { EmptyState } from '../../components/ui/EmptyState';
import { ErrorState } from '../../components/ui/ErrorState';
import { Button } from '../../components/ui/Button';
import { Tag } from '../../components/ui/Tag';
import { Input, Select } from '../../components/ui/Field';
import { useDebouncedValue } from '../../lib/useDebouncedValue';
import { useClasses } from '../classes/api';
import { importTemplateUrl } from '../import/api';
import { useStudents } from './api';
import { StudentFormDialog } from './StudentFormDialog';
import type { StudentStatus } from '../../lib/types';

const PER_PAGE = 50;

const STATUS_LABEL: Record<StudentStatus, string> = {
  active: 'Aktif',
  graduated: 'Lulus',
  moved: 'Pindah',
  dropped: 'Keluar',
};

export function StudentsListPage() {
  const [qInput, setQInput] = useState('');
  const [classId, setClassId] = useState('');
  const [page, setPage] = useState(1);
  const [dialogOpen, setDialogOpen] = useState(false);
  const navigate = useNavigate();
  const q = useDebouncedValue(qInput, 300);

  const { data: classes } = useClasses();
  const { data, isLoading, isError, isPlaceholderData, refetch } = useStudents({
    q: q || undefined,
    classId: classId || undefined,
    page,
    perPage: PER_PAGE,
  });

  function handleFilterChange(next: Partial<{ q: string; classId: string }>) {
    if (next.q !== undefined) setQInput(next.q);
    if (next.classId !== undefined) setClassId(next.classId);
    setPage(1);
  }

  const total = data?.total ?? 0;
  const start = total === 0 ? 0 : (page - 1) * PER_PAGE + 1;
  const end = Math.min(page * PER_PAGE, total);
  const hasNext = end < total;

  return (
    <div className="flex flex-col gap-6">
      <div className="flex flex-wrap items-center justify-end gap-2">
        <Button variant="secondary" onClick={() => navigate('/data/siswa/import')}>
          <Upload size={16} strokeWidth={2} aria-hidden="true" />
          Import
        </Button>
        <Button onClick={() => setDialogOpen(true)}>
          <Plus size={16} strokeWidth={2} aria-hidden="true" />
          Tambah Siswa
        </Button>
      </div>

      <a
        href={importTemplateUrl('students')}
        className="inline-flex w-fit items-center gap-1.5 text-[12px] font-medium text-primary hover:opacity-80"
      >
        <FileSpreadsheet size={16} strokeWidth={2} aria-hidden="true" />
        Unduh template Excel
      </a>

      <div className="flex flex-col gap-3 sm:flex-row">
        <div className="relative flex-1">
          <Search
            size={16}
            strokeWidth={2}
            className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-muted"
            aria-hidden="true"
          />
          <Input
            value={qInput}
            onChange={(e) => handleFilterChange({ q: e.target.value })}
            placeholder="Cari nama atau NIS..."
            className="pl-9"
            aria-label="Cari siswa"
          />
        </div>
        <Select
          value={classId}
          onChange={(e) => handleFilterChange({ classId: e.target.value })}
          className="sm:w-48"
          aria-label="Filter rombel"
        >
          <option value="">Semua rombel</option>
          {classes?.map((c) => (
            <option key={c.id} value={c.id}>
              {c.name}
            </option>
          ))}
        </Select>
      </div>

      {isLoading ? (
        <div className="flex flex-col gap-2">
          <Skeleton className="h-12 w-full" />
          <Skeleton className="h-12 w-full" />
          <Skeleton className="h-12 w-full" />
          <Skeleton className="h-12 w-full" />
        </div>
      ) : isError ? (
        <ErrorState message="Gagal memuat daftar siswa." onRetry={() => refetch()} />
      ) : data && data.items.length === 0 ? (
        <EmptyState
          icon={Users}
          message="Belum ada siswa. Tambahkan manual atau import dari Excel."
          action={
            <Button variant="secondary" onClick={() => setDialogOpen(true)}>
              Tambah Siswa
            </Button>
          }
        />
      ) : (
        <div className={isPlaceholderData ? 'opacity-60 transition-opacity duration-150' : undefined}>
          {data?.items.map((student) => (
            <ListRow
              key={student.id}
              title={student.name}
              subtitle={`${student.nis} · ${student.class?.name ?? 'Belum ada rombel'}`}
              trailing={
                student.status !== 'active' ? (
                  <Tag variant="neutral">{STATUS_LABEL[student.status]}</Tag>
                ) : undefined
              }
              onClick={() => navigate(`/data/siswa/${student.id}`)}
            />
          ))}
        </div>
      )}

      {data && data.total > 0 && (
        <div className="flex items-center justify-between gap-3">
          <p className="num text-[12px] text-muted">
            {start}–{end} dari {total}
          </p>
          <div className="flex gap-2">
            <Button
              variant="secondary"
              disabled={page <= 1}
              onClick={() => setPage((p) => Math.max(1, p - 1))}
            >
              <ChevronLeft size={16} strokeWidth={2} aria-hidden="true" />
              Sebelumnya
            </Button>
            <Button variant="secondary" disabled={!hasNext} onClick={() => setPage((p) => p + 1)}>
              Berikutnya
              <ChevronRight size={16} strokeWidth={2} aria-hidden="true" />
            </Button>
          </div>
        </div>
      )}

      <StudentFormDialog open={dialogOpen} onClose={() => setDialogOpen(false)} />
    </div>
  );
}
