import type { GradingComponentType } from '../../lib/types';

/** Label ringkas tipe komponen — dipakai subtitle daftar & tabel rekap/nilai. */
export const GRADING_TYPE_LABEL: Record<GradingComponentType, string> = {
  tp: 'TP',
  sumatif: 'Sumatif',
  praktik: 'Praktik',
  lainnya: 'Lainnya',
};

/** Opsi Select form komponen — label lebih deskriptif dari `GRADING_TYPE_LABEL`. */
export const GRADING_TYPE_OPTIONS: { value: GradingComponentType; label: string }[] = [
  { value: 'tp', label: 'TP (Tujuan Pembelajaran)' },
  { value: 'sumatif', label: 'Sumatif' },
  { value: 'praktik', label: 'Praktik' },
  { value: 'lainnya', label: 'Lainnya' },
];
