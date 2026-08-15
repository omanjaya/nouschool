import { useEffect, useState, type FormEvent } from 'react';
import { Dialog } from '../../components/ui/Dialog';
import { Button } from '../../components/ui/Button';
import { Field, Input } from '../../components/ui/Field';
import { ApiError } from '../../lib/api';
import { useCreateTeacher, useUpdateTeacher, type TeacherInput } from './api';
import type { Teacher } from '../../lib/types';

interface TeacherFormDialogProps {
  open: boolean;
  onClose: () => void;
  teacher?: Teacher;
  onSaved?: () => void;
}

const EMPTY: TeacherInput = { name: '', email: '', nip: '' };

export function TeacherFormDialog({ open, onClose, teacher, onSaved }: TeacherFormDialogProps) {
  const isEdit = Boolean(teacher);
  const [form, setForm] = useState<TeacherInput>(EMPTY);
  const [nameError, setNameError] = useState<string | undefined>();
  const createTeacher = useCreateTeacher();
  const updateTeacher = useUpdateTeacher(teacher?.id ?? '');
  const mutation = isEdit ? updateTeacher : createTeacher;

  useEffect(() => {
    if (open) {
      setForm(teacher ? { name: teacher.name, email: teacher.email ?? '', nip: teacher.nip ?? '' } : EMPTY);
      setNameError(undefined);
      mutation.reset();
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, teacher]);

  function handleSubmit(e: FormEvent) {
    e.preventDefault();
    if (!form.name.trim()) {
      setNameError('Nama wajib diisi.');
      return;
    }
    setNameError(undefined);
    mutation.mutate(
      { name: form.name.trim(), email: form.email?.trim() || undefined, nip: form.nip?.trim() || undefined },
      {
        onSuccess: () => {
          onSaved?.();
          onClose();
        },
      },
    );
  }

  return (
    <Dialog open={open} onClose={onClose} title={isEdit ? 'Ubah Guru' : 'Tambah Guru'}>
      <form onSubmit={handleSubmit} className="flex flex-col gap-4">
        <Field label="Nama" htmlFor="teacher-name" error={nameError}>
          <Input id="teacher-name" value={form.name} onChange={(e) => setForm((f) => ({ ...f, name: e.target.value }))} />
        </Field>
        <Field label="Email" htmlFor="teacher-email" hint="Opsional">
          <Input
            id="teacher-email"
            type="email"
            value={form.email ?? ''}
            onChange={(e) => setForm((f) => ({ ...f, email: e.target.value }))}
          />
        </Field>
        <Field label="NIP/NUPTK" htmlFor="teacher-nip" hint="Opsional">
          <Input id="teacher-nip" value={form.nip ?? ''} onChange={(e) => setForm((f) => ({ ...f, nip: e.target.value }))} />
        </Field>

        {mutation.isError && (
          <p className="text-[12px] text-danger">
            {mutation.error instanceof ApiError ? mutation.error.message : 'Gagal menyimpan data guru.'}
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
