import { useEffect, useState } from 'react';
import { Search } from 'lucide-react';
import { Dialog } from '../../components/ui/Dialog';
import { Button } from '../../components/ui/Button';
import { Input } from '../../components/ui/Field';
import { EmptyState } from '../../components/ui/EmptyState';
import { Skeleton } from '../../components/ui/Skeleton';
import { useToast } from '../../components/ui/Toast';
import { ApiError } from '../../lib/api';
import { useTeachers } from '../teachers/api';
import { useEmployees } from '../employees/api';
import { useDutyAssignments, useUpdateDutyAssignments } from './api';
import type { Duty } from '../../lib/types';

interface ManageDutyAssigneesDialogProps {
  open: boolean;
  onClose: () => void;
  duty: Duty | undefined;
}

/** Sumber kandidat petugas mengikuti `duty.for_role` (guru dari /api/teachers, pegawai dari /api/employees). */
interface Candidate {
  userId: string;
  name: string;
  subtitle: string | undefined;
}

/** Dialog "Kelola Petugas" — daftar user terpilih untuk satu tugas + tambah via pencarian, lalu PUT user_ids sekaligus (pola `AddStudentsDialog`, tapi start-state sudah berisi petugas saat ini). */
export function ManageDutyAssigneesDialog({ open, onClose, duty }: ManageDutyAssigneesDialogProps) {
  const dutyId = duty?.id ?? '';
  const forRole = duty?.for_role ?? 'guru';
  const { showToast } = useToast();
  const [qInput, setQInput] = useState('');
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [seeded, setSeeded] = useState(false);

  const { data: assignments, isLoading: assignmentsLoading } = useDutyAssignments(dutyId, open && Boolean(dutyId));
  const { data: teachers, isLoading: teachersLoading } = useTeachers();
  const { data: employees, isLoading: employeesLoading } = useEmployees();
  const updateAssignments = useUpdateDutyAssignments(dutyId);

  const candidatesLoading = forRole === 'guru' ? teachersLoading : employeesLoading;
  const candidates: Candidate[] =
    forRole === 'guru'
      ? (teachers ?? []).map((t) => ({ userId: t.user_id, name: t.name, subtitle: t.nip ?? t.email ?? undefined }))
      : (employees ?? []).map((e) => ({ userId: e.user_id, name: e.name, subtitle: e.nip ?? e.email ?? undefined }));

  useEffect(() => {
    if (open && assignments && !seeded) {
      setSelected(new Set(assignments.map((a) => a.user_id)));
      setSeeded(true);
    }
    if (!open) setSeeded(false);
  }, [open, assignments, seeded]);

  const q = qInput.trim().toLowerCase();
  const filtered = q ? candidates.filter((c) => c.name.toLowerCase().includes(q)) : candidates;

  function toggle(userId: string) {
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(userId)) next.delete(userId);
      else next.add(userId);
      return next;
    });
  }

  function handleClose() {
    setQInput('');
    updateAssignments.reset();
    onClose();
  }

  function handleSubmit() {
    updateAssignments.mutate([...selected], {
      onSuccess: () => {
        showToast('Petugas diperbarui.');
        handleClose();
      },
      onError: (err) => {
        showToast(err instanceof ApiError ? err.message : 'Gagal memperbarui petugas.', 'error');
      },
    });
  }

  const isLoading = assignmentsLoading || candidatesLoading || !seeded;

  return (
    <Dialog open={open} onClose={handleClose} title={duty ? `Kelola Petugas — ${duty.name}` : 'Kelola Petugas'}>
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
            placeholder={forRole === 'guru' ? 'Cari nama guru...' : 'Cari nama pegawai...'}
            className="pl-9"
            aria-label="Cari petugas"
          />
        </div>

        {isLoading ? (
          <div className="flex flex-col gap-2">
            <Skeleton className="h-12 w-full" />
            <Skeleton className="h-12 w-full" />
          </div>
        ) : filtered.length === 0 ? (
          <EmptyState
            message={
              forRole === 'guru' ? 'Tidak ada guru yang cocok.' : 'Tidak ada pegawai yang cocok. Tambahkan di tab Pegawai.'
            }
          />
        ) : (
          <div className="max-h-[320px] overflow-y-auto">
            {filtered.map((c) => (
              <label
                key={c.userId}
                className="flex min-h-12 w-full cursor-pointer items-center gap-3 border-b border-line py-3 last:border-b-0"
              >
                <input
                  type="checkbox"
                  checked={selected.has(c.userId)}
                  onChange={() => toggle(c.userId)}
                  className="h-4 w-4 shrink-0 accent-[var(--primary)]"
                />
                <span className="min-w-0 flex-1">
                  <span className="block truncate text-[14px] text-ink">{c.name}</span>
                  {c.subtitle && <span className="block truncate text-[12px] text-muted">{c.subtitle}</span>}
                </span>
              </label>
            ))}
          </div>
        )}

        {updateAssignments.isError && (
          <p className="text-[12px] text-danger">
            {updateAssignments.error instanceof ApiError ? updateAssignments.error.message : 'Gagal memperbarui petugas.'}
          </p>
        )}

        <div className="flex justify-end gap-2">
          <Button type="button" variant="secondary" onClick={handleClose}>
            Batal
          </Button>
          <Button type="button" onClick={handleSubmit} loading={updateAssignments.isPending}>
            Simpan ({selected.size})
          </Button>
        </div>
      </div>
    </Dialog>
  );
}
