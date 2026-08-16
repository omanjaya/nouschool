import { useEffect, useState, type FormEvent } from 'react';
import { Dialog } from '../../components/ui/Dialog';
import { Button } from '../../components/ui/Button';
import { Checkbox, Field, Input, Select } from '../../components/ui/Field';
import { Skeleton } from '../../components/ui/Skeleton';
import { ApiError } from '../../lib/api';
import { useCreateDuty, useDutyFlags, useUpdateDuty } from './api';
import type { Duty, DutyForRole, DutyInput } from '../../lib/types';

interface DutyFormDialogProps {
  open: boolean;
  onClose: () => void;
  duty?: Duty;
  onSaved?: () => void;
}

const EMPTY: DutyInput = { name: '', for_role: 'guru', flags: [], active: true };

/** Form tambah/ubah tugas tambahan — nama, diberikan untuk guru/pegawai, flags kapabilitas. */
export function DutyFormDialog({ open, onClose, duty, onSaved }: DutyFormDialogProps) {
  const isEdit = Boolean(duty);
  const [form, setForm] = useState<DutyInput>(EMPTY);
  const [nameError, setNameError] = useState<string | undefined>();
  const { data: flagOptions, isLoading: flagsLoading } = useDutyFlags();
  const createDuty = useCreateDuty();
  const updateDuty = useUpdateDuty(duty?.id ?? '');
  const mutation = isEdit ? updateDuty : createDuty;

  useEffect(() => {
    if (open) {
      setForm(
        duty
          ? { name: duty.name, for_role: duty.for_role, flags: [...duty.flags], active: duty.active }
          : EMPTY,
      );
      setNameError(undefined);
      mutation.reset();
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, duty]);

  function toggleFlag(key: string) {
    setForm((f) => ({
      ...f,
      flags: f.flags.includes(key) ? f.flags.filter((k) => k !== key) : [...f.flags, key],
    }));
  }

  function handleSubmit(e: FormEvent) {
    e.preventDefault();
    if (!form.name.trim()) {
      setNameError('Nama tugas wajib diisi.');
      return;
    }
    setNameError(undefined);
    mutation.mutate(
      { name: form.name.trim(), for_role: form.for_role, flags: form.flags, active: form.active },
      {
        onSuccess: () => {
          onSaved?.();
          onClose();
        },
      },
    );
  }

  return (
    <Dialog open={open} onClose={onClose} title={isEdit ? 'Ubah Tugas Tambahan' : 'Tambah Tugas Tambahan'}>
      <form onSubmit={handleSubmit} className="flex flex-col gap-4">
        <Field label="Nama tugas" htmlFor="duty-name" error={nameError}>
          <Input
            id="duty-name"
            value={form.name}
            onChange={(e) => setForm((f) => ({ ...f, name: e.target.value }))}
            placeholder="Mis. Wali Kelas, Guru BK, Guru Piket"
          />
        </Field>

        <Field label="Diberikan untuk" htmlFor="duty-for-role">
          <Select
            id="duty-for-role"
            value={form.for_role}
            onChange={(e) => setForm((f) => ({ ...f, for_role: e.target.value as DutyForRole }))}
          >
            <option value="guru">Guru</option>
            <option value="pegawai">Pegawai</option>
          </Select>
        </Field>

        <div className="flex flex-col gap-2">
          <p className="text-[12px] font-medium text-muted">Kapabilitas</p>
          {flagsLoading ? (
            <Skeleton className="h-20 w-full" />
          ) : !flagOptions || flagOptions.length === 0 ? (
            <p className="text-[12px] text-muted">Belum ada kapabilitas yang bisa dipilih.</p>
          ) : (
            <div className="flex flex-col gap-2 rounded-lg border border-line px-3 py-3">
              {flagOptions.map((flag) => (
                <Checkbox
                  key={flag.key}
                  id={`duty-flag-${flag.key}`}
                  label={flag.label}
                  checked={form.flags.includes(flag.key)}
                  onChange={() => toggleFlag(flag.key)}
                />
              ))}
            </div>
          )}
        </div>

        <Checkbox
          id="duty-active"
          label="Aktif (bisa ditugaskan ke guru/pegawai)"
          checked={form.active}
          onChange={(e) => setForm((f) => ({ ...f, active: e.target.checked }))}
        />

        {mutation.isError && (
          <p className="text-[12px] text-danger">
            {mutation.error instanceof ApiError ? mutation.error.message : 'Gagal menyimpan tugas tambahan.'}
          </p>
        )}

        <div className="flex justify-end gap-2 pt-2">
          <Button type="button" variant="secondary" onClick={onClose}>
            Batal
          </Button>
          <Button type="submit" loading={mutation.isPending}>
            Simpan
          </Button>
        </div>
      </form>
    </Dialog>
  );
}
