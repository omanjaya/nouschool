import { useEffect, useState, type ReactNode } from 'react';
import { Clock, Plus, Trash2 } from 'lucide-react';
import { Button } from '../../components/ui/Button';
import { Input } from '../../components/ui/Field';
import { Skeleton } from '../../components/ui/Skeleton';
import { EmptyState } from '../../components/ui/EmptyState';
import { ErrorState } from '../../components/ui/ErrorState';
import { SegmentedControl, type SegmentedOption } from '../../components/ui/SegmentedControl';
import { useToast } from '../../components/ui/Toast';
import { ApiError } from '../../lib/api';
import { addMinutes, DAY_LABELS, timeToMinutes } from './format';
import { usePeriodOverrides, usePeriods, useSavePeriodOverrides, useSavePeriods, type PeriodInput } from './api';
import type { DayOfWeek } from '../../lib/types';

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

type DayTab = 'default' | DayOfWeek;

const ALL_DAYS: DayOfWeek[] = [1, 2, 3, 4, 5, 6];

const DAY_TAB_OPTIONS: SegmentedOption<string>[] = [
  { value: 'default', label: 'Default' },
  ...ALL_DAYS.map((d) => ({ value: String(d), label: DAY_LABELS[d].slice(0, 3) })),
];

function toRows(list: { number: number; starts_at: string; ends_at: string; label: string | null }[]): PeriodRow[] {
  return list
    .slice()
    .sort((a, b) => a.number - b.number)
    .map((p) => ({ key: nextRowKey(), starts_at: p.starts_at, ends_at: p.ends_at, label: p.label ?? '' }));
}

function validateRows(list: PeriodRow[]): Record<string, string> {
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

function SkeletonRows() {
  return (
    <div className="flex flex-col gap-2">
      <Skeleton className="h-12 w-full" />
      <Skeleton className="h-12 w-full" />
      <Skeleton className="h-12 w-full" />
    </div>
  );
}

interface PeriodRowsEditorProps {
  initialRows: PeriodRow[];
  onSave: (payload: PeriodInput[]) => void;
  saving: boolean;
  saveError?: string | null;
  saveLabel: string;
  /** Tombol tambahan di sebelah Simpan — mis. "Hapus jadwal khusus" untuk override yang sudah ada. */
  extraActions?: ReactNode;
}

/**
 * Editor baris jam inline-editable (satu tombol Simpan) — dipakai baik untuk
 * jam default (`PUT /api/periods`) maupun jam khusus per hari (`PUT
 * /api/periods/overrides`). Di-`key`-kan per tab pemanggil (lihat
 * `PeriodsPage`) supaya state lokal selalu mulai bersih saat berpindah tab.
 */
function PeriodRowsEditor({ initialRows, onSave, saving, saveError, saveLabel, extraActions }: PeriodRowsEditorProps) {
  const [rows, setRows] = useState<PeriodRow[]>(initialRows);
  const [rowErrors, setRowErrors] = useState<Record<string, string>>({});

  function updateRow(key: string, patch: Partial<PeriodRow>) {
    setRows((prev) => prev.map((r) => (r.key === key ? { ...r, ...patch } : r)));
  }

  function addRow() {
    setRows((prev) => {
      const last = prev[prev.length - 1];
      const starts_at = last ? last.ends_at : '07:00';
      const ends_at = addMinutes(starts_at, DEFAULT_DURATION_MIN);
      return [...prev, { key: nextRowKey(), starts_at, ends_at, label: '' }];
    });
  }

  function removeRow(key: string) {
    setRows((prev) => prev.filter((r) => r.key !== key));
  }

  function handleSave() {
    const errors = validateRows(rows);
    setRowErrors(errors);
    if (Object.keys(errors).length > 0) return;

    const payload: PeriodInput[] = rows.map((r, i) => ({
      number: i + 1,
      starts_at: r.starts_at,
      ends_at: r.ends_at,
      label: r.label.trim() || null,
    }));
    onSave(payload);
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

      {saveError && <p className="text-[12px] text-danger">{saveError}</p>}

      <div className="flex flex-wrap items-center gap-2">
        <Button onClick={handleSave} loading={saving} disabled={rows.length === 0}>
          {saveLabel}
        </Button>
        {extraActions}
      </div>
    </div>
  );
}

/**
 * /data/jam — editor jam pelajaran (admin). SegmentedControl hari: "Default"
 * (editor lama, `PUT /api/periods`) atau Senin..Sabtu (jam khusus per hari,
 * Fase 14 Gelombang D §Period day overrides, `GET/PUT /api/periods/overrides`).
 * Hari tanpa jadwal khusus mengikuti jam default sampai admin membuatnya
 * sendiri (prefill dari jam default) — konsisten dengan makna "kosong = ikut
 * default" di kontrak.
 */
export function PeriodsPage() {
  const { showToast } = useToast();
  const [dayTab, setDayTab] = useState<DayTab>('default');
  const [creatingOverride, setCreatingOverride] = useState(false);

  const isDefault = dayTab === 'default';
  const day: DayOfWeek | undefined = isDefault ? undefined : dayTab;

  const { data: periods, isLoading: defaultLoading, isError: defaultError, refetch: refetchDefault } = usePeriods();
  const savePeriods = useSavePeriods();

  const {
    data: overrides,
    isLoading: overridesLoading,
    isError: overridesError,
    refetch: refetchOverrides,
  } = usePeriodOverrides(day ?? 1, !isDefault);
  const saveOverrides = useSavePeriodOverrides();

  useEffect(() => {
    setCreatingOverride(false);
  }, [dayTab]);

  function handleSaveDefault(payload: PeriodInput[]) {
    savePeriods.mutate(payload, {
      onSuccess: () => showToast('Jam pelajaran tersimpan.'),
    });
  }

  function handleSaveOverride(payload: PeriodInput[]) {
    if (!day) return;
    saveOverrides.mutate(
      { day_of_week: day, periods: payload },
      {
        onSuccess: () => {
          showToast(`Jam khusus ${DAY_LABELS[day]} tersimpan.`);
          setCreatingOverride(false);
        },
      },
    );
  }

  function handleDeleteOverride() {
    if (!day) return;
    saveOverrides.mutate(
      { day_of_week: day, periods: [] },
      {
        onSuccess: () => showToast(`Jadwal khusus ${DAY_LABELS[day]} dihapus — kembali mengikuti jam default.`),
      },
    );
  }

  return (
    <div className="flex flex-col gap-6">
      <SegmentedControl
        options={DAY_TAB_OPTIONS}
        value={isDefault ? 'default' : String(dayTab)}
        onChange={(v) => setDayTab(v === 'default' ? 'default' : (Number(v) as DayOfWeek))}
        className="w-full overflow-x-auto"
      />

      {isDefault ? (
        defaultLoading ? (
          <SkeletonRows />
        ) : defaultError ? (
          <ErrorState message="Gagal memuat jam pelajaran." onRetry={() => refetchDefault()} />
        ) : (
          <PeriodRowsEditor
            key="default"
            initialRows={toRows(periods ?? [])}
            onSave={handleSaveDefault}
            saving={savePeriods.isPending}
            saveError={
              savePeriods.isError
                ? savePeriods.error instanceof ApiError
                  ? savePeriods.error.message
                  : 'Gagal menyimpan jam pelajaran.'
                : null
            }
            saveLabel="Simpan"
          />
        )
      ) : overridesLoading ? (
        <SkeletonRows />
      ) : overridesError ? (
        <ErrorState message="Gagal memuat jam khusus." onRetry={() => refetchOverrides()} />
      ) : overrides && overrides.periods.length === 0 && !creatingOverride ? (
        <EmptyState
          icon={Clock}
          message={`Mengikuti jam default. Belum ada jadwal khusus untuk hari ${DAY_LABELS[day!]}.`}
          action={
            <Button variant="secondary" onClick={() => setCreatingOverride(true)}>
              Buat jadwal khusus {DAY_LABELS[day!]}
            </Button>
          }
        />
      ) : (
        <PeriodRowsEditor
          key={dayTab}
          initialRows={
            overrides && overrides.periods.length > 0 ? toRows(overrides.periods) : toRows(periods ?? [])
          }
          onSave={handleSaveOverride}
          saving={saveOverrides.isPending}
          saveError={
            saveOverrides.isError
              ? saveOverrides.error instanceof ApiError
                ? saveOverrides.error.message
                : 'Gagal menyimpan jam khusus.'
              : null
          }
          saveLabel={`Simpan Jam Khusus ${DAY_LABELS[day!]}`}
          extraActions={
            overrides && overrides.periods.length > 0 ? (
              <Button variant="danger" onClick={handleDeleteOverride} loading={saveOverrides.isPending}>
                Hapus jadwal khusus
              </Button>
            ) : undefined
          }
        />
      )}
    </div>
  );
}
