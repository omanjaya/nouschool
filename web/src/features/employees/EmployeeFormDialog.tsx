import { useEffect, useState, type FormEvent } from 'react';
import { Copy } from 'lucide-react';
import { Dialog } from '../../components/ui/Dialog';
import { Button } from '../../components/ui/Button';
import { Field, Input } from '../../components/ui/Field';
import { useToast } from '../../components/ui/Toast';
import { ApiError } from '../../lib/api';
import { useCreateEmployee, useUpdateEmployee } from './api';
import type { Employee, EmployeeInput } from '../../lib/types';

interface EmployeeFormDialogProps {
  open: boolean;
  onClose: () => void;
  employee?: Employee;
  onSaved?: () => void;
}

const EMPTY: EmployeeInput = { name: '', email: '', username: '', nip: '' };

/**
 * Form tambah/ubah pegawai — nama, email/username, NIP. Hasil TAMBAH menampilkan
 * password sementara sekali (pola `ResetPasswordDialog` admin) sebelum dialog ditutup.
 */
export function EmployeeFormDialog({ open, onClose, employee, onSaved }: EmployeeFormDialogProps) {
  const isEdit = Boolean(employee);
  const [form, setForm] = useState<EmployeeInput>(EMPTY);
  const [formError, setFormError] = useState<string | undefined>();
  const [tempPassword, setTempPassword] = useState<string | null>(null);
  const { showToast } = useToast();
  const createEmployee = useCreateEmployee();
  const updateEmployee = useUpdateEmployee(employee?.id ?? '');
  const mutation = isEdit ? updateEmployee : createEmployee;

  useEffect(() => {
    if (open) {
      setForm(
        employee
          ? {
              name: employee.name,
              email: employee.email ?? '',
              username: employee.username ?? '',
              nip: employee.nip ?? '',
            }
          : EMPTY,
      );
      setFormError(undefined);
      setTempPassword(null);
      mutation.reset();
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, employee]);

  function handleSubmit(e: FormEvent) {
    e.preventDefault();
    if (!form.name.trim()) {
      setFormError('Nama wajib diisi.');
      return;
    }
    if (!isEdit && !form.email?.trim() && !form.username?.trim()) {
      setFormError('Isi email atau username untuk login pegawai.');
      return;
    }
    setFormError(undefined);

    const payload: EmployeeInput = {
      name: form.name.trim(),
      email: form.email?.trim() || undefined,
      username: form.username?.trim() || undefined,
      nip: form.nip?.trim() || undefined,
    };

    if (isEdit) {
      updateEmployee.mutate(payload, {
        onSuccess: () => {
          onSaved?.();
          onClose();
        },
      });
    } else {
      createEmployee.mutate(payload, {
        onSuccess: (result) => {
          onSaved?.();
          setTempPassword(result.temp_password);
        },
      });
    }
  }

  async function handleCopy() {
    if (!tempPassword) return;
    try {
      await navigator.clipboard.writeText(tempPassword);
      showToast('Password disalin.');
    } catch {
      showToast('Gagal menyalin password. Salin manual.', 'error');
    }
  }

  if (tempPassword) {
    return (
      <Dialog open={open} onClose={onClose} title="Pegawai Ditambahkan">
        <div className="flex flex-col gap-4">
          <p className="text-[14px] text-ink">
            Password sementara untuk <span className="font-medium">{form.name}</span>.
          </p>
          <div className="rounded-lg border border-line bg-surface-2 px-4 py-4 text-center">
            <span className="num select-all font-mono text-[21px] font-semibold tracking-[0.05em] text-ink">
              {tempPassword}
            </span>
          </div>
          <Button variant="secondary" onClick={handleCopy}>
            <Copy size={16} strokeWidth={2} aria-hidden="true" />
            Salin
          </Button>
          <p className="text-[12px] text-danger">Tampilkan sekali — catat sekarang sebelum menutup dialog ini.</p>
        </div>
        <div className="mt-4 flex justify-end">
          <Button type="button" variant="secondary" onClick={onClose}>
            Tutup
          </Button>
        </div>
      </Dialog>
    );
  }

  return (
    <Dialog open={open} onClose={onClose} title={isEdit ? 'Ubah Pegawai' : 'Tambah Pegawai'}>
      <form onSubmit={handleSubmit} className="flex flex-col gap-4">
        <Field label="Nama" htmlFor="employee-name">
          <Input id="employee-name" value={form.name} onChange={(e) => setForm((f) => ({ ...f, name: e.target.value }))} />
        </Field>
        <Field label="Email" htmlFor="employee-email" hint="Isi salah satu: email atau username">
          <Input
            id="employee-email"
            type="email"
            value={form.email ?? ''}
            onChange={(e) => setForm((f) => ({ ...f, email: e.target.value }))}
          />
        </Field>
        <Field label="Username" htmlFor="employee-username">
          <Input
            id="employee-username"
            value={form.username ?? ''}
            onChange={(e) => setForm((f) => ({ ...f, username: e.target.value }))}
          />
        </Field>
        <Field label="NIP" htmlFor="employee-nip" hint="Opsional">
          <Input id="employee-nip" value={form.nip ?? ''} onChange={(e) => setForm((f) => ({ ...f, nip: e.target.value }))} />
        </Field>

        {(formError || mutation.isError) && (
          <p className="text-[12px] text-danger">
            {formError ?? (mutation.error instanceof ApiError ? mutation.error.message : 'Gagal menyimpan data pegawai.')}
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
