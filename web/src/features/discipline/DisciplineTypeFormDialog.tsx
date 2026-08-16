import { useEffect, useState, type FormEvent } from 'react';
import { Dialog } from '../../components/ui/Dialog';
import { Button } from '../../components/ui/Button';
import { Checkbox, Field, Input } from '../../components/ui/Field';
import { ApiError } from '../../lib/api';
import { useCreateDisciplineType, useUpdateDisciplineType } from './api';
import type { DisciplineType, DisciplineTypeInput } from '../../lib/types';

interface DisciplineTypeFormDialogProps {
  open: boolean;
  onClose: () => void;
  type?: DisciplineType;
  onSaved?: () => void;
}

const EMPTY: DisciplineTypeInput = { name: '', points: 1, category: '', active: true };

/** Form tambah/ubah jenis pelanggaran — seksi "Jenis Pelanggaran" di `/kedisiplinan/pengaturan`. */
export function DisciplineTypeFormDialog({ open, onClose, type, onSaved }: DisciplineTypeFormDialogProps) {
  const isEdit = Boolean(type);
  const [form, setForm] = useState<DisciplineTypeInput>(EMPTY);
  const [errors, setErrors] = useState<{ name?: string; points?: string; category?: string }>({});
  const createType = useCreateDisciplineType();
  const updateType = useUpdateDisciplineType(type?.id ?? '');
  const mutation = isEdit ? updateType : createType;

  useEffect(() => {
    if (open) {
      setForm(type ? { name: type.name, points: type.points, category: type.category, active: type.active } : EMPTY);
      setErrors({});
      mutation.reset();
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, type]);

  function handleSubmit(e: FormEvent) {
    e.preventDefault();
    const nextErrors: typeof errors = {};
    if (!form.name.trim()) nextErrors.name = 'Nama wajib diisi.';
    if (!Number.isFinite(form.points) || form.points <= 0) nextErrors.points = 'Poin harus angka lebih dari 0.';
    if (!form.category.trim()) nextErrors.category = 'Kategori wajib diisi.';
    setErrors(nextErrors);
    if (Object.keys(nextErrors).length > 0) return;

    mutation.mutate(
      { name: form.name.trim(), points: form.points, category: form.category.trim(), active: form.active },
      {
        onSuccess: () => {
          onSaved?.();
          onClose();
        },
      },
    );
  }

  return (
    <Dialog open={open} onClose={onClose} title={isEdit ? 'Ubah Jenis Pelanggaran' : 'Tambah Jenis Pelanggaran'}>
      <form onSubmit={handleSubmit} className="flex flex-col gap-4">
        <Field label="Nama pelanggaran" htmlFor="dtype-name" error={errors.name}>
          <Input
            id="dtype-name"
            value={form.name}
            onChange={(e) => setForm((f) => ({ ...f, name: e.target.value }))}
            placeholder="Mis. Terlambat masuk kelas"
          />
        </Field>
        <Field label="Poin" htmlFor="dtype-points" error={errors.points} hint="Ditambahkan ke total poin siswa saat dicatat.">
          <Input
            id="dtype-points"
            type="number"
            min={1}
            value={form.points}
            onChange={(e) => setForm((f) => ({ ...f, points: Number(e.target.value) }))}
          />
        </Field>
        <Field label="Kategori" htmlFor="dtype-category" error={errors.category} hint="Mis. Kedisiplinan, Kerapian, Akademik.">
          <Input
            id="dtype-category"
            value={form.category}
            onChange={(e) => setForm((f) => ({ ...f, category: e.target.value }))}
          />
        </Field>
        <Checkbox
          id="dtype-active"
          label="Aktif (bisa dipilih saat mencatat pelanggaran)"
          checked={form.active}
          onChange={(e) => setForm((f) => ({ ...f, active: e.target.checked }))}
        />

        {mutation.isError && (
          <p className="text-[12px] text-danger">
            {mutation.error instanceof ApiError ? mutation.error.message : 'Gagal menyimpan jenis pelanggaran.'}
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
