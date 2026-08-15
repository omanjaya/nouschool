import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { Plus, School2 } from 'lucide-react';
import { ListRow } from '../../components/ui/ListRow';
import { Skeleton } from '../../components/ui/Skeleton';
import { EmptyState } from '../../components/ui/EmptyState';
import { ErrorState } from '../../components/ui/ErrorState';
import { Button } from '../../components/ui/Button';
import { useClasses } from './api';
import { ClassFormDialog } from './ClassFormDialog';

export function ClassesListPage() {
  const { data: classes, isLoading, isError, refetch } = useClasses();
  const [dialogOpen, setDialogOpen] = useState(false);
  const navigate = useNavigate();

  return (
    <div className="flex flex-col gap-6">
      <div className="flex items-center justify-end">
        <Button onClick={() => setDialogOpen(true)}>
          <Plus size={16} strokeWidth={2} aria-hidden="true" />
          Tambah Rombel
        </Button>
      </div>

      {isLoading ? (
        <div className="flex flex-col gap-2">
          <Skeleton className="h-12 w-full" />
          <Skeleton className="h-12 w-full" />
          <Skeleton className="h-12 w-full" />
        </div>
      ) : isError ? (
        <ErrorState message="Gagal memuat daftar rombel." onRetry={() => refetch()} />
      ) : classes && classes.length === 0 ? (
        <EmptyState
          icon={School2}
          message="Belum ada rombel."
          action={
            <Button variant="secondary" onClick={() => setDialogOpen(true)}>
              Tambah Rombel
            </Button>
          }
        />
      ) : (
        <div>
          {classes?.map((c) => (
            <ListRow
              key={c.id}
              title={c.name}
              subtitle={`${c.grade}${c.major ? ` ${c.major}` : ''} · ${c.homeroom_teacher?.name ?? 'Wali kelas belum ditentukan'}`}
              trailing={<span className="num text-[14px] text-muted">{c.student_count} siswa</span>}
              onClick={() => navigate(`/data/rombel/${c.id}`)}
            />
          ))}
        </div>
      )}

      <ClassFormDialog open={dialogOpen} onClose={() => setDialogOpen(false)} />
    </div>
  );
}
