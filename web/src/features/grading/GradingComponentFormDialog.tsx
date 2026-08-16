import { useEffect, useState, type FormEvent } from 'react';
import { Dialog } from '../../components/ui/Dialog';
import { Button } from '../../components/ui/Button';
import { Field, Input, Select } from '../../components/ui/Field';
import { ApiError } from '../../lib/api';
import { useCreateGradingComponent, useUpdateGradingComponent } from './api';
import { GRADING_TYPE_OPTIONS } from './format';
import type { GradingComponent, GradingComponentType } from '../../lib/types';

interface GradingComponentFormDialogProps {
  open: boolean;
  onClose: () => void;
  classId: string;
  subjectId: string;
  component?: GradingComponent;
}

interface FormState {
  name: string;
  type: GradingComponentType;
  weight: string;
  kktp: string;
}

const EMPTY: FormState = { name: '', type: 'tp', weight: '', kktp: '70' };

/** Form tambah/ubah komponen nilai — Dialog dipakai dari tab "Komponen" `/nilai`. */
export function GradingComponentFormDialog({
  open,
  onClose,
  classId,
  subjectId,
  component,
}: GradingComponentFormDialogProps) {
  const isEdit = Boolean(component);
  const [form, setForm] = useState<FormState>(EMPTY);
  const [errors, setErrors] = useState<{ name?: string; weight?: string; kktp?: string }>({});
  const createComponent = useCreateGradingComponent();
  const updateComponent = useUpdateGradingComponent(component?.id ?? '');
  const mutation = isEdit ? updateComponent : createComponent;

  useEffect(() => {
    if (open) {
      setForm(
        component
          ? { name: component.name, type: component.type, weight: String(component.weight), kktp: String(component.kktp) }
          : EMPTY,
      );
      setErrors({});
      mutation.reset();
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, component]);

  function handleSubmit(e: FormEvent) {
    e.preventDefault();
    const nextErrors: typeof errors = {};
    if (!form.name.trim()) nextErrors.name = 'Nama wajib diisi.';
    const weight = Number(form.weight);
    if (!Number.isFinite(weight) || weight <= 0) nextErrors.weight = 'Bobot harus angka lebih dari 0.';
    const kktp = Number(form.kktp);
    if (!Number.isFinite(kktp) || kktp < 0 || kktp > 100) nextErrors.kktp = 'KKTP harus angka 0-100.';
    setErrors(nextErrors);
    if (Object.keys(nextErrors).length > 0) return;

    mutation.mutate(
      { name: form.name.trim(), type: form.type, weight, kktp, class_id: classId, subject_id: subjectId },
      { onSuccess: () => onClose() },
    );
  }

  return (
    <Dialog open={open} onClose={onClose} title={isEdit ? 'Ubah Komponen Nilai' : 'Tambah Komponen Nilai'}>
      <form onSubmit={handleSubmit} className="flex flex-col gap-4">
        <Field label="Nama komponen" htmlFor="gc-name" error={errors.name}>
          <Input
            id="gc-name"
            value={form.name}
            onChange={(e) => setForm((f) => ({ ...f, name: e.target.value }))}
            placeholder="Mis. Ulangan Harian 1"
          />
        </Field>
        <Field label="Tipe" htmlFor="gc-type">
          <Select
            id="gc-type"
            value={form.type}
            onChange={(e) => setForm((f) => ({ ...f, type: e.target.value as GradingComponentType }))}
          >
            {GRADING_TYPE_OPTIONS.map((opt) => (
              <option key={opt.value} value={opt.value}>
                {opt.label}
              </option>
            ))}
          </Select>
        </Field>
        <Field
          label="Bobot"
          htmlFor="gc-weight"
          error={errors.weight}
          hint="Total bobot boleh berbeda dari 100 — nilai akhir dinormalisasi otomatis."
        >
          <Input
            id="gc-weight"
            type="number"
            min={1}
            value={form.weight}
            onChange={(e) => setForm((f) => ({ ...f, weight: e.target.value }))}
          />
        </Field>
        <Field
          label="KKTP"
          htmlFor="gc-kktp"
          error={errors.kktp}
          hint="Kriteria Ketercapaian Tujuan Pembelajaran — ambang nilai minimal."
        >
          <Input
            id="gc-kktp"
            type="number"
            min={0}
            max={100}
            value={form.kktp}
            onChange={(e) => setForm((f) => ({ ...f, kktp: e.target.value }))}
          />
        </Field>

        {mutation.isError && (
          <p className="text-[12px] text-danger">
            {mutation.error instanceof ApiError ? mutation.error.message : 'Gagal menyimpan komponen nilai.'}
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
