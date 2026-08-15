import { useEffect, useState } from 'react';
import { Plus, Trash2 } from 'lucide-react';
import { Card } from '../../components/ui/Card';
import { Input, Select } from '../../components/ui/Field';
import { Button } from '../../components/ui/Button';
import { Skeleton } from '../../components/ui/Skeleton';
import { ErrorState } from '../../components/ui/ErrorState';
import { useToast } from '../../components/ui/Toast';
import { ApiError } from '../../lib/api';
import { useLeaveSettings, useUpdateLeaveSettings } from './api';
import type { LeaveChainStep, LeaveType } from '../../lib/types';

const CHAIN_ROLE_OPTIONS: { value: string; label: string }[] = [
  { value: 'kepala_sekolah', label: 'Kepala Sekolah' },
  { value: 'admin_sekolah', label: 'Admin' },
  { value: 'guru', label: 'Guru tertentu' },
];

/** Seksi "Alur Persetujuan Izin" + "Jenis Izin" di halaman /pengaturan (admin). */
export function LeaveSettingsSection() {
  const { data, isLoading, isError, refetch } = useLeaveSettings();
  const update = useUpdateLeaveSettings();
  const { showToast } = useToast();

  const [chain, setChain] = useState<LeaveChainStep[]>([]);
  const [types, setTypes] = useState<LeaveType[]>([]);
  const [formError, setFormError] = useState<string | null>(null);

  useEffect(() => {
    if (data) {
      setChain(data.chain);
      setTypes(data.types);
    }
  }, [data]);

  function addStep() {
    setChain((prev) => [...prev, { role: 'kepala_sekolah' }]);
  }

  function removeStep(idx: number) {
    setChain((prev) => prev.filter((_, i) => i !== idx));
  }

  function updateStepRole(idx: number, role: string) {
    setChain((prev) => prev.map((s, i) => (i === idx ? { ...s, role } : s)));
  }

  function addType() {
    setTypes((prev) => [...prev, { key: '', label: '' }]);
  }

  function removeType(idx: number) {
    setTypes((prev) => prev.filter((_, i) => i !== idx));
  }

  function updateTypeKey(idx: number, key: string) {
    setTypes((prev) => prev.map((t, i) => (i === idx ? { ...t, key } : t)));
  }

  function updateTypeLabel(idx: number, label: string) {
    setTypes((prev) => prev.map((t, i) => (i === idx ? { ...t, label } : t)));
  }

  function handleSave() {
    setFormError(null);
    if (chain.length === 0) {
      setFormError('Alur persetujuan minimal punya satu langkah.');
      return;
    }
    if (types.some((t) => !t.key.trim() || !t.label.trim())) {
      setFormError('Kunci dan label jenis izin tidak boleh kosong.');
      return;
    }
    const keys = types.map((t) => t.key.trim());
    if (new Set(keys).size !== keys.length) {
      setFormError('Kunci jenis izin tidak boleh duplikat.');
      return;
    }
    update.mutate(
      { types: types.map((t) => ({ key: t.key.trim(), label: t.label.trim() })), chain },
      {
        onSuccess: () => showToast('Pengaturan izin disimpan.'),
        onError: (err) => setFormError(err instanceof ApiError ? err.message : 'Gagal menyimpan pengaturan izin.'),
      },
    );
  }

  if (isLoading) {
    return <Skeleton className="h-52 w-full" />;
  }

  if (isError || !data) {
    return <ErrorState message="Gagal memuat pengaturan izin." onRetry={() => refetch()} />;
  }

  return (
    <div className="flex flex-col gap-4">
      <Card className="flex flex-col gap-4">
        <div>
          <p className="text-[16px] font-semibold text-ink">Alur Persetujuan Izin</p>
          <p className="text-[12px] text-muted">Urutan langkah persetujuan — dijalankan berurutan dari langkah 1.</p>
        </div>

        <div className="flex flex-col gap-2">
          {chain.map((step, idx) => (
            <div key={idx} className="flex items-center gap-2">
              <span className="w-5 shrink-0 text-[13px] font-semibold text-muted">{idx + 1}.</span>
              <Select value={step.role} onChange={(e) => updateStepRole(idx, e.target.value)} className="flex-1">
                {CHAIN_ROLE_OPTIONS.map((opt) => (
                  <option key={opt.value} value={opt.value}>
                    {opt.label}
                  </option>
                ))}
              </Select>
              <button
                type="button"
                onClick={() => removeStep(idx)}
                aria-label={`Hapus langkah ${idx + 1}`}
                className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg text-muted transition-colors duration-150 hover:bg-surface-2 hover:text-danger"
              >
                <Trash2 size={16} strokeWidth={2} aria-hidden="true" />
              </button>
            </div>
          ))}
        </div>

        <Button variant="secondary" onClick={addStep} className="self-start">
          <Plus size={16} strokeWidth={2} aria-hidden="true" />
          Tambah Langkah
        </Button>
      </Card>

      <Card className="flex flex-col gap-4">
        <div>
          <p className="text-[16px] font-semibold text-ink">Jenis Izin</p>
          <p className="text-[12px] text-muted">Kunci dipakai sistem, label ditampilkan ke pengguna.</p>
        </div>

        <div className="flex flex-col gap-2">
          {types.map((t, idx) => (
            <div key={idx} className="flex items-center gap-2">
              <Input
                value={t.key}
                onChange={(e) => updateTypeKey(idx, e.target.value)}
                placeholder="kunci (mis. cuti)"
                className="w-28 shrink-0"
              />
              <Input
                value={t.label}
                onChange={(e) => updateTypeLabel(idx, e.target.value)}
                placeholder="Label (mis. Cuti)"
                className="flex-1"
              />
              <button
                type="button"
                onClick={() => removeType(idx)}
                aria-label={`Hapus jenis izin ${t.label || idx + 1}`}
                className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg text-muted transition-colors duration-150 hover:bg-surface-2 hover:text-danger"
              >
                <Trash2 size={16} strokeWidth={2} aria-hidden="true" />
              </button>
            </div>
          ))}
        </div>

        <Button variant="secondary" onClick={addType} className="self-start">
          <Plus size={16} strokeWidth={2} aria-hidden="true" />
          Tambah Jenis
        </Button>
      </Card>

      {formError && <p className="text-[12px] text-danger">{formError}</p>}

      <Button onClick={handleSave} loading={update.isPending} className="self-start">
        Simpan Pengaturan Izin
      </Button>
    </div>
  );
}
