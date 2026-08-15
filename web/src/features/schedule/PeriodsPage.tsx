import { useEffect, useState } from 'react';
import { Clock, Plus, Trash2 } from 'lucide-react';
import { Button } from '../../components/ui/Button';
import { Input } from '../../components/ui/Field';
import { Skeleton } from '../../components/ui/Skeleton';
import { EmptyState } from '../../components/ui/EmptyState';
import { ErrorState } from '../../components/ui/ErrorState';
import { useToast } from '../../components/ui/Toast';
import { ApiError } from '../../lib/api';
import { addMinutes, timeToMinutes } from './format';
import { usePeriods, useSavePeriods, type PeriodInput } from './api';

interface PeriodRow {
  key: string;
  starts_at: string;
  ends_at: string;
  label: string;
}

let rowKeySeq = 0;
function nextRowKey(): string {
  rowKeySeq += 1;
  return `row-${rowKeySeq}`;
}

const DEFAULT_DURATION_MIN = 45;

/** /data/jam — editor jam pelajaran (admin): daftar baris inline-editable, satu tombol Simpan (PUT replace-all). */
export function PeriodsPage() {
  const { data: periods, isLoading, isError, refetch } = usePeriods();
  const savePeriods = useSavePeriods();
  const { showToast } = useToast();

  const [rows, setRows] = useState<PeriodRow[] | null>(null);
  const [rowErrors, setRowErrors] = useState<Record<string, string>>({});

  // Inisialisasi state lokal sekali saat data pertama kali datang — perubahan
  // berikutnya (refetch invalidasi setelah save) tidak menimpa draft yang sedang diedit.
  useEffect(() => {
    if (periods && rows === null) {
      setRows(
        periods
          .slice()
          .sort((a, b) => a.number - b.number)
          .map((p) => ({ key: p.id, starts_at: p.starts_at, ends_at: p.ends_at, label: p.label ?? '' })),
      );
    }
  }, [periods, rows]);

  function updateRow(key: string, patch: Partial<PeriodRow>) {
    setRows((prev) => prev?.map((r) => (r.key === key ? { ...r, ...patch } : r)) ?? prev);
  }

  function addRow() {
    setRows((prev) => {
      const list = prev ?? [];
      const last = list[list.length - 1];
      const starts_at = last ? last.ends_at : '07:00';
      const ends_at = addMinutes(starts_at, DEFAULT_DURATION_MIN);
      return [...list, { key: nextRowKey(), starts_at, ends_at, label: '' }];
    });
  }

  function removeRow(key: string) {
    setRows((prev) => prev?.filter((r) => r.key !== key) ?? prev);
  }

  function validate(list: PeriodRow[]): Record<string, string> {
    const errors: Record<string, string> = {};
    list.forEach((row, i) => {
      if (!row.starts_at || !row.ends_at) {
        errors[row.key] = 'Jam mulai & jam selesai wajib diisi.';
        return;
      }
      if (timeToMinutes(row.ends_at) <= timeToMinutes(row.starts_at)) {
        errors[row.key] = 'Jam selesai harus setelah jam mulai.';
        return;
      }
      const prev = list[i - 1];
      if (prev && timeToMinutes(row.starts_at) < timeToMinutes(prev.ends_at)) {
        errors[row.key] = `Bertumpuk dengan jam ke-${i} (${prev.starts_at}–${prev.ends_at}).`;
      }
    });
    return errors;
  }

  function handleSave() {
    if (!rows) return;
    const errors = validate(rows);
    setRowErrors(errors);
    if (Object.keys(errors).length > 0) return;

    const payload: PeriodInput[] = rows.map((r, i) => ({
      number: i + 1,
      starts_at: r.starts_at,
      ends_at: r.ends_at,
      label: r.label.trim() || null,
    }));

    savePeriods.mutate(payload, {
      onSuccess: (data) => {
        setRows(data.slice().sort((a, b) => a.number - b.number).map((p) => ({ key: p.id, starts_at: p.starts_at, ends_at: p.ends_at, label: p.label ?? '' })));
        setRowErrors({});
        showToast('Jam pelajaran tersimpan.');
      },
    });
  }

  if (isLoading || rows === null) {
    return (
      <div className="flex flex-col gap-2">
        <Skeleton className="h-12 w-full" />
        <Skeleton className="h-12 w-full" />
        <Skeleton className="h-12 w-full" />
      </div>
    );
  }

  if (isError) {
    return <ErrorState message="Gagal memuat jam pelajaran." onRetry={() => refetch()} />;
  }

  return (
    <div className="flex flex-col gap-6">
      <div className="flex items-center justify-between gap-3">
        <p className="text-[12px] text-muted">Urutan baris menentukan jam ke- (baris pertama = jam ke-1).</p>
        <Button variant="secondary" onClick={addRow}>
          <Plus size={16} strokeWidth={2} aria-hidden="true" />
          Tambah Baris
        </Button>
      </div>

      {rows.length === 0 ? (
        <EmptyState
          icon={Clock}
          message="Belum ada jam pelajaran. Tambah baris untuk mulai."
          action={
            <Button variant="secondary" onClick={addRow}>
              Tambah Baris
            </Button>
          }
        />
      ) : (
        <div className="flex flex-col gap-2">
          {rows.map((row, i) => (
            <div
              key={row.key}
              className={`flex flex-col gap-2 rounded-lg border border-line p-3 sm:flex-row sm:items-center ${
                row.label.trim() ? 'bg-surface-2' : 'bg-surface'
              }`}
            >
              <span className="num w-16 shrink-0 text-[13px] font-semibold text-muted">Ke-{i + 1}</span>
              <div className="flex flex-1 flex-wrap items-center gap-2">
                <Input
                  type="time"
                  aria-label={`Jam mulai ke-${i + 1}`}
                  value={row.starts_at}
                  onChange={(e) => updateRow(row.key, { starts_at: e.target.value })}
                  className="w-32"
                />
                <span className="text-muted">–</span>
                <Input
                  type="time"
                  aria-label={`Jam selesai ke-${i + 1}`}
                  value={row.ends_at}
                  onChange={(e) => updateRow(row.key, { ends_at: e.target.value })}
                  className="w-32"
                />
                <Input
                  type="text"
                  aria-label={`Label ke-${i + 1}`}
                  placeholder="Label (opsional, mis. Istirahat)"
                  value={row.label}
                  onChange={(e) => updateRow(row.key, { label: e.target.value })}
                  className="min-w-[180px] flex-1"
                />
              </div>
              <button
                type="button"
                onClick={() => removeRow(row.key)}
                aria-label={`Hapus jam ke-${i + 1}`}
                className="flex h-9 w-9 shrink-0 items-center justify-center self-end rounded-lg text-muted transition-colors duration-150 hover:bg-surface hover:text-danger sm:self-center"
              >
                <Trash2 size={16} strokeWidth={2} aria-hidden="true" />
              </button>
              {rowErrors[row.key] && <p className="w-full text-[12px] text-danger">{rowErrors[row.key]}</p>}
            </div>
          ))}
        </div>
      )}

      {savePeriods.isError && (
        <p className="text-[12px] text-danger">
          {savePeriods.error instanceof ApiError ? savePeriods.error.message : 'Gagal menyimpan jam pelajaran.'}
        </p>
      )}

      <div>
        <Button onClick={handleSave} loading={savePeriods.isPending} disabled={rows.length === 0}>
          Simpan
        </Button>
      </div>
    </div>
  );
}
