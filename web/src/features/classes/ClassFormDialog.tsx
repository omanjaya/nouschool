import { useEffect, useState, type FormEvent } from 'react';
import { Dialog } from '../../components/ui/Dialog';
import { Button } from '../../components/ui/Button';
import { Field, Input, Select } from '../../components/ui/Field';
import { ApiError } from '../../lib/api';
import { useTeachers } from '../teachers/api';
import { useCreateClass, useUpdateClass, type ClassInput } from './api';
import type { SchoolClass } from '../../lib/types';

const GRADES = ['X', 'XI', 'XII', 'XIII'];

interface ClassFormDialogProps {
  open: boolean;
  onClose: () => void;
  schoolClass?: SchoolClass;
  onSaved?: () => void;
}

const EMPTY: ClassInput = { name: '', grade: GRADES[0], major: '', homeroom_teacher_id: '' };

export function ClassFormDialog({ open, onClose, schoolClass, onSaved }: ClassFormDialogProps) {
  const { data: teachers } = useTeachers();
  const isEdit = Boolean(schoolClass);
  const [form, setForm] = useState<ClassInput>(EMPTY);
  const [nameError, setNameError] = useState<string | undefined>();
  const createClass = useCreateClass();
  const updateClass = useUpdateClass(schoolClass?.id ?? '');
  const mutation = isEdit ? updateClass : createClass;

  useEffect(() => {
    if (open) {
      setForm(
        schoolClass
          ? {
              name: schoolClass.name,
              grade: schoolClass.grade,
              major: schoolClass.major ?? '',
              homeroom_teacher_id: schoolClass.homeroom_teacher?.id ?? '',
            }
          : EMPTY,
      );
      setNameError(undefined);
      mutation.reset();
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, schoolClass]);

  function handleSubmit(e: FormEvent) {
    e.preventDefault();
    if (!form.name.trim()) {
      setNameError('Nama rombel wajib diisi.');
      return;
    }
    setNameError(undefined);

    const payload: ClassInput = {
      name: form.name.trim(),
      grade: form.grade,
      major: form.major?.trim() || undefined,
      homeroom_teacher_id: form.homeroom_teacher_id || undefined,
    };

    mutation.mutate(payload, {
      onSuccess: () => {
        onSaved?.();
        onClose();
      },
    });
  }

  return (
    <Dialog open={open} onClose={onClose} title={isEdit ? 'Ubah Rombel' : 'Tambah Rombel'}>
      <form onSubmit={handleSubmit} className="flex flex-col gap-4">
        <Field label="Nama rombel" htmlFor="class-name" error={nameError} hint="Contoh: XII RPL 1">
          <Input id="class-name" value={form.name} onChange={(e) => setForm((f) => ({ ...f, name: e.target.value }))} />
        </Field>
        <div className="grid grid-cols-2 gap-3">
          <Field label="Tingkat" htmlFor="class-grade">
            <Select id="class-grade" value={form.grade} onChange={(e) => setForm((f) => ({ ...f, grade: e.target.value }))}>
              {GRADES.map((g) => (
                <option key={g} value={g}>
                  {g}
                </option>
              ))}
            </Select>
          </Field>
          <Field label="Jurusan" htmlFor="class-major" hint="Opsional">
            <Input
              id="class-major"
              value={form.major ?? ''}
              onChange={(e) => setForm((f) => ({ ...f, major: e.target.value }))}
              placeholder="RPL, IPA, ..."
            />
          </Field>
        </div>
        <Field label="Wali kelas" htmlFor="class-homeroom">
          <Select
            id="class-homeroom"
            value={form.homeroom_teacher_id ?? ''}
            onChange={(e) => setForm((f) => ({ ...f, homeroom_teacher_id: e.target.value }))}
          >
            <option value="">Belum ditentukan</option>
            {teachers?.map((t) => (
              <option key={t.id} value={t.id}>
                {t.name}
              </option>
            ))}
          </Select>
        </Field>

        {mutation.isError && (
          <p className="text-[12px] text-danger">
            {mutation.error instanceof ApiError ? mutation.error.message : 'Gagal menyimpan data rombel.'}
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
