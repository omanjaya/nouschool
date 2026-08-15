import { useState } from 'react';
import { Search } from 'lucide-react';
import { Dialog } from '../../components/ui/Dialog';
import { Button } from '../../components/ui/Button';
import { Input } from '../../components/ui/Field';
import { EmptyState } from '../../components/ui/EmptyState';
import { Skeleton } from '../../components/ui/Skeleton';
import { useDebouncedValue } from '../../lib/useDebouncedValue';
import { ApiError } from '../../lib/api';
import { useStudents } from '../students/api';
import { useAddClassStudents } from './api';

interface AddStudentsDialogProps {
  open: boolean;
  onClose: () => void;
  classId: string;
  onAdded?: () => void;
}

/** Cari siswa (yang belum ada di rombel ini) lalu tambahkan beberapa sekaligus. */
export function AddStudentsDialog({ open, onClose, classId, onAdded }: AddStudentsDialogProps) {
  const [qInput, setQInput] = useState('');
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const q = useDebouncedValue(qInput, 300);
  const { data, isLoading } = useStudents({ q: q || undefined, page: 1, perPage: 50 });
  const addStudents = useAddClassStudents(classId);

  const candidates = data?.items.filter((s) => s.class?.id !== classId) ?? [];

  function toggle(id: string) {
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  }

  function handleClose() {
    setQInput('');
    setSelected(new Set());
    addStudents.reset();
    onClose();
  }

  function handleSubmit() {
    addStudents.mutate([...selected], {
      onSuccess: () => {
        onAdded?.();
        handleClose();
      },
    });
  }

  return (
    <Dialog open={open} onClose={handleClose} title="Tambah Siswa ke Rombel">
      <div className="flex flex-col gap-4">
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
          />
        </div>

        {isLoading ? (
          <div className="flex flex-col gap-2">
            <Skeleton className="h-12 w-full" />
            <Skeleton className="h-12 w-full" />
          </div>
        ) : candidates.length === 0 ? (
          <EmptyState message="Tidak ada siswa yang cocok untuk ditambahkan." />
        ) : (
          <div className="max-h-[320px] overflow-y-auto">
            {candidates.map((s) => (
              <label
                key={s.id}
                className="flex min-h-12 w-full cursor-pointer items-center gap-3 border-b border-line py-3 last:border-b-0"
              >
                <input
                  type="checkbox"
                  checked={selected.has(s.id)}
                  onChange={() => toggle(s.id)}
                  className="h-4 w-4 shrink-0 accent-[var(--primary)]"
                />
                <span className="min-w-0 flex-1">
                  <span className="block truncate text-[14px] text-ink">{s.name}</span>
                  <span className="block truncate text-[12px] text-muted">
                    {s.nis} · {s.class?.name ?? 'Belum ada rombel'}
                  </span>
                </span>
              </label>
            ))}
          </div>
        )}

        {addStudents.isError && (
          <p className="text-[12px] text-danger">
            {addStudents.error instanceof ApiError ? addStudents.error.message : 'Gagal menambahkan siswa.'}
          </p>
        )}

        <div className="flex justify-end gap-2">
          <Button type="button" variant="secondary" onClick={handleClose}>
            Batal
          </Button>
          <Button type="button" onClick={handleSubmit} loading={addStudents.isPending} disabled={selected.size === 0}>
            Tambah ({selected.size})
          </Button>
        </div>
      </div>
    </Dialog>
  );
}
