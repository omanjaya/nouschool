import { useEffect, useState, type FormEvent } from 'react';
import { Dialog } from '../../components/ui/Dialog';
import { Button } from '../../components/ui/Button';
import { Field, Input } from '../../components/ui/Field';
import { ApiError } from '../../lib/api';
import { useCreateSubject, useUpdateSubject, type SubjectInput } from './api';
import type { Subject } from '../../lib/types';

interface SubjectFormDialogProps {
  open: boolean;
  onClose: () => void;
  subject?: Subject;
  onSaved?: () => void;
}

const EMPTY: SubjectInput = { code: '', name: '' };

export function SubjectFormDialog({ open, onClose, subject, onSaved }: SubjectFormDialogProps) {
  const isEdit = Boolean(subject);
  const [form, setForm] = useState<SubjectInput>(EMPTY);
  const [errors, setErrors] = useState<{ code?: string; name?: string }>({});
  const createSubject = useCreateSubject();
  const updateSubject = useUpdateSubject(subject?.id ?? '');
  const mutation = isEdit ? updateSubject : createSubject;

  useEffect(() => {
    if (open) {
      setForm(subject ? { code: subject.code, name: subject.name } : EMPTY);
      setErrors({});
      mutation.reset();
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, subject]);

  function handleSubmit(e: FormEvent) {
    e.preventDefault();
    const nextErrors: { code?: string; name?: string } = {};
    if (!form.code.trim()) nextErrors.code = 'Kode wajib diisi.';
    if (!form.name.trim()) nextErrors.name = 'Nama wajib diisi.';
    setErrors(nextErrors);
    if (Object.keys(nextErrors).length > 0) return;

    mutation.mutate(
      { code: form.code.trim(), name: form.name.trim() },
      {
        onSuccess: () => {
          onSaved?.();
          onClose();
        },
      },
    );
  }

  return (
    <Dialog open={open} onClose={onClose} title={isEdit ? 'Ubah Mapel' : 'Tambah Mapel'}>
      <form onSubmit={handleSubmit} className="flex flex-col gap-4">
        <Field label="Kode" htmlFor="subject-code" error={errors.code} hint="Contoh: MTK">
          <Input id="subject-code" value={form.code} onChange={(e) => setForm((f) => ({ ...f, code: e.target.value }))} />
        </Field>
        <Field label="Nama mata pelajaran" htmlFor="subject-name" error={errors.name}>
          <Input id="subject-name" value={form.name} onChange={(e) => setForm((f) => ({ ...f, name: e.target.value }))} />
        </Field>

        {mutation.isError && (
          <p className="text-[12px] text-danger">
            {mutation.error instanceof ApiError ? mutation.error.message : 'Gagal menyimpan mata pelajaran.'}
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
