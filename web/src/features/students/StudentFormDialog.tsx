import { useEffect, useState, type FormEvent } from 'react';
import { Dialog } from '../../components/ui/Dialog';
import { Button } from '../../components/ui/Button';
import { Field, Input, Select } from '../../components/ui/Field';
import { ApiError } from '../../lib/api';
import { useClasses } from '../classes/api';
import { useCreateStudent, useUpdateStudent, type StudentInput } from './api';
import type { Student } from '../../lib/types';

interface StudentFormDialogProps {
  open: boolean;
  onClose: () => void;
  /** Kosong = mode tambah; diisi = mode edit. */
  student?: Student;
  onSaved?: () => void;
}

const EMPTY: StudentInput = { nis: '', nisn: '', name: '', gender: '', birth_date: '', class_id: '' };

export function StudentFormDialog({ open, onClose, student, onSaved }: StudentFormDialogProps) {
  const { data: classes } = useClasses();
  const isEdit = Boolean(student);
  const [form, setForm] = useState<StudentInput>(EMPTY);
  const [nisError, setNisError] = useState<string | undefined>();
  const [nameError, setNameError] = useState<string | undefined>();
  const createStudent = useCreateStudent();
  const updateStudent = useUpdateStudent(student?.id ?? '');
  const mutation = isEdit ? updateStudent : createStudent;

  useEffect(() => {
    if (open) {
      setForm(
        student
          ? {
              nis: student.nis,
              nisn: student.nisn ?? '',
              name: student.name,
              gender: student.gender ?? '',
              birth_date: student.birth_date ?? '',
              class_id: student.class?.id ?? '',
            }
          : EMPTY,
      );
      setNisError(undefined);
      setNameError(undefined);
      mutation.reset();
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, student]);

  function handleSubmit(e: FormEvent) {
    e.preventDefault();
    let hasError = false;
    if (!form.nis.trim()) {
      setNisError('NIS wajib diisi.');
      hasError = true;
    } else {
      setNisError(undefined);
    }
    if (!form.name.trim()) {
      setNameError('Nama wajib diisi.');
      hasError = true;
    } else {
      setNameError(undefined);
    }
    if (hasError) return;

    const payload: StudentInput = {
      nis: form.nis.trim(),
      name: form.name.trim(),
      nisn: form.nisn?.trim() || undefined,
      gender: form.gender || undefined,
      birth_date: form.birth_date || undefined,
      class_id: form.class_id || undefined,
    };

    mutation.mutate(payload, {
      onSuccess: () => {
        onSaved?.();
        onClose();
      },
    });
  }

  return (
    <Dialog open={open} onClose={onClose} title={isEdit ? 'Ubah Siswa' : 'Tambah Siswa'}>
      <form onSubmit={handleSubmit} className="flex flex-col gap-4">
        <Field label="NIS" htmlFor="student-nis" error={nisError}>
          <Input id="student-nis" value={form.nis} onChange={(e) => setForm((f) => ({ ...f, nis: e.target.value }))} />
        </Field>
        <Field label="NISN" htmlFor="student-nisn">
          <Input
            id="student-nisn"
            value={form.nisn ?? ''}
            onChange={(e) => setForm((f) => ({ ...f, nisn: e.target.value }))}
          />
        </Field>
        <Field label="Nama" htmlFor="student-name" error={nameError}>
          <Input
            id="student-name"
            value={form.name}
            onChange={(e) => setForm((f) => ({ ...f, name: e.target.value }))}
          />
        </Field>
        <div className="grid grid-cols-2 gap-3">
          <Field label="Jenis kelamin" htmlFor="student-gender">
            <Select
              id="student-gender"
              value={form.gender ?? ''}
              onChange={(e) => setForm((f) => ({ ...f, gender: e.target.value }))}
            >
              <option value="">Pilih</option>
              <option value="L">Laki-laki</option>
              <option value="P">Perempuan</option>
            </Select>
          </Field>
          <Field label="Tanggal lahir" htmlFor="student-birth">
            <Input
              id="student-birth"
              type="date"
              value={form.birth_date ?? ''}
              onChange={(e) => setForm((f) => ({ ...f, birth_date: e.target.value }))}
            />
          </Field>
        </div>
        <Field label="Rombel" htmlFor="student-class">
          <Select
            id="student-class"
            value={form.class_id ?? ''}
            onChange={(e) => setForm((f) => ({ ...f, class_id: e.target.value }))}
          >
            <option value="">Belum ada rombel</option>
            {classes?.map((c) => (
              <option key={c.id} value={c.id}>
                {c.name}
              </option>
            ))}
          </Select>
        </Field>

        {mutation.isError && (
          <p className="text-[12px] text-danger">
            {mutation.error instanceof ApiError ? mutation.error.message : 'Gagal menyimpan data siswa.'}
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
