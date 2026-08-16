import { useEffect, useState } from 'react';
import { Plus, Trash2 } from 'lucide-react';
import { Card } from '../../components/ui/Card';
import { Checkbox, Field, Input } from '../../components/ui/Field';
import { Button } from '../../components/ui/Button';
import { Skeleton } from '../../components/ui/Skeleton';
import { ErrorState } from '../../components/ui/ErrorState';
import { useToast } from '../../components/ui/Toast';
import { ApiError } from '../../lib/api';
import { useGradingSettings, useUpdateGradingSettings } from './api';
import type { GradingRange, GradingSettings } from '../../lib/types';

interface RangeForm {
  min: string;
  max: string;
  label: string;
}

function toRangeForm(ranges: GradingRange[]): RangeForm[] {
  return ranges.map((r) => ({ min: String(r.min), max: String(r.max), label: r.label }));
}

const DEFAULT_RANGES: RangeForm[] = [
  { min: '0', max: '59', label: 'D' },
  { min: '60', max: '74', label: 'C' },
  { min: '75', max: '89', label: 'B' },
  { min: '90', max: '100', label: 'A' },
];

/** Seksi "Penilaian" di `/pengaturan` (admin) — toggle modul + editor rentang label nilai (Fase 14 Gelombang C). */
export function GradingSettingsSection() {
  const { data, isLoading, isError, refetch } = useGradingSettings();
  const update = useUpdateGradingSettings();
  const { showToast } = useToast();

  const [enabled, setEnabled] = useState(false);
  const [ranges, setRanges] = useState<RangeForm[]>(DEFAULT_RANGES);
  const [formError, setFormError] = useState<string | null>(null);

  useEffect(() => {
    if (data) {
      setEnabled(data.enabled);
      setRanges(data.ranges.length > 0 ? toRangeForm(data.ranges) : DEFAULT_RANGES);
    }
  }, [data]);

  function updateRange(index: number, field: keyof RangeForm, value: string) {
    setRanges((prev) => prev.map((r, i) => (i === index ? { ...r, [field]: value } : r)));
  }

  function addRange() {
    setRanges((prev) => [...prev, { min: '', max: '', label: '' }]);
  }

  function removeRange(index: number) {
    setRanges((prev) => prev.filter((_, i) => i !== index));
  }

  function handleSave() {
    setFormError(null);

    const parsed = ranges.map((r) => ({ min: Number(r.min), max: Number(r.max), label: r.label.trim() }));

    if (parsed.length === 0) {
      setFormError('Minimal harus ada satu rentang nilai.');
      return;
    }
    if (parsed.some((r) => !r.label || !Number.isFinite(r.min) || !Number.isFinite(r.max) || r.min > r.max)) {
      setFormError('Setiap rentang harus punya label & nilai min ≤ max.');
      return;
    }

    const sorted = parsed.slice().sort((a, b) => a.min - b.min);
    if (sorted[0].min !== 0) {
      setFormError('Rentang pertama harus mulai dari 0.');
      return;
    }
    if (sorted[sorted.length - 1].max !== 100) {
      setFormError('Rentang terakhir harus berakhir di 100.');
      return;
    }
    for (let i = 1; i < sorted.length; i++) {
      if (sorted[i].min !== sorted[i - 1].max + 1) {
        setFormError('Rentang harus berurutan tanpa celah/tumpang tindih, menutup 0-100.');
        return;
      }
    }

    const payload: GradingSettings = { enabled, ranges: sorted };
    update.mutate(payload, {
      onSuccess: () => showToast('Pengaturan penilaian disimpan.'),
      onError: (err) => setFormError(err instanceof ApiError ? err.message : 'Gagal menyimpan pengaturan penilaian.'),
    });
  }

  if (isLoading) {
    return <Skeleton className="h-52 w-full" />;
  }

  if (isError || !data) {
    return <ErrorState message="Gagal memuat pengaturan penilaian." onRetry={() => refetch()} />;
  }

  return (
    <div className="flex flex-col gap-4">
      <Card className="flex flex-col gap-3">
        <Checkbox
          id="grading-enabled"
          label="Aktifkan modul penilaian"
          checked={enabled}
          onChange={(e) => setEnabled(e.target.checked)}
        />
        <p className="text-[12px] text-muted">Menonaktifkan menyembunyikan seluruh menu nilai.</p>
      </Card>

      <Card className="flex flex-col gap-4">
        <div>
          <p className="text-[16px] font-semibold text-ink">Rentang Nilai & Label</p>
          <p className="text-[12px] text-muted">Dipakai memetakan nilai akhir ke label rapor (mis. A/B/C/D).</p>
        </div>

        <div className="flex flex-col gap-3">
          {ranges.map((r, i) => (
            <div key={i} className="flex flex-wrap items-end gap-2">
              <Field label="Min" className="w-20">
                <Input type="number" min={0} max={100} value={r.min} onChange={(e) => updateRange(i, 'min', e.target.value)} />
              </Field>
              <Field label="Max" className="w-20">
                <Input type="number" min={0} max={100} value={r.max} onChange={(e) => updateRange(i, 'max', e.target.value)} />
              </Field>
              <Field label="Label" className="min-w-[100px] flex-1">
                <Input value={r.label} onChange={(e) => updateRange(i, 'label', e.target.value)} placeholder="Mis. A" />
              </Field>
              <button
                type="button"
                onClick={() => removeRange(i)}
                aria-label="Hapus rentang"
                className="flex h-11 w-11 shrink-0 items-center justify-center rounded-lg text-muted transition-colors duration-150 hover:bg-surface-2 hover:text-danger"
              >
                <Trash2 size={16} strokeWidth={2} aria-hidden="true" />
              </button>
            </div>
          ))}
          <Button variant="secondary" onClick={addRange} className="self-start">
            <Plus size={16} strokeWidth={2} aria-hidden="true" />
            Tambah Rentang
          </Button>
        </div>
      </Card>

      {formError && <p className="text-[12px] text-danger">{formError}</p>}

      <Button onClick={handleSave} loading={update.isPending} className="self-start">
        Simpan Pengaturan Penilaian
      </Button>
    </div>
  );
}
