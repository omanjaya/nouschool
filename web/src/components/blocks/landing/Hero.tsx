import { ArrowRight } from 'lucide-react';
import { Button } from '../../ui/Button';
import { StatusChip } from '../../ui/StatusChip';
import { StatTile } from '../../ui/StatTile';
import type { AttendanceStatus } from '../../../lib/types';

/**
 * Diadaptasi dari `@shadcnblocks/hero7` (lihat `components/blocks/README.md`
 * untuk checklist adaptasi). Perubahan dari bahan asli:
 * - Tidak ada avatar/rating sosial palsu (hero7 asli pakai foto & rating
 *   dummy) — diganti panel mock "hari ini" yang disusun dari komponen Rapor
 *   asli (`StatusChip`, `StatTile`) supaya ilustrasinya jujur (bukan gambar
 *   stok) dan tetap 100% token.
 * - Tombol pakai `components/ui/Button` (varian primary/secondary Rapor),
 *   bukan `button.tsx` shadcn bawaan block.
 * - Tipografi & spacing disamakan ke skala docs/10-design-system.md.
 */

interface HeroMockRow {
  name: string;
  detail: string;
  status: AttendanceStatus;
}

const MOCK_ROWS: HeroMockRow[] = [
  { name: 'Kelas X-A', detail: 'Jam ke-1 · Matematika', status: 'hadir' },
  { name: 'Kelas X-B', detail: 'Jam ke-1 · Bahasa Indonesia', status: 'terlambat' },
  { name: 'Kelas XI-IPA 2', detail: 'Jam ke-2 · Fisika', status: 'izin' },
];

interface HeroProps {
  onPrimaryCta: () => void;
  onSecondaryCta: () => void;
}

export function Hero({ onPrimaryCta, onSecondaryCta }: HeroProps) {
  return (
    <section className="mx-auto max-w-[1120px] px-5 py-16 lg:py-24">
      <div className="mx-auto flex max-w-[720px] flex-col items-center gap-5 text-center">
        <h1 className="text-[28px] font-semibold leading-tight text-ink lg:text-[36px]">
          Absensi &amp; monitoring sekolah dalam satu aplikasi.
        </h1>
        <p className="max-w-[560px] text-[14px] text-muted lg:text-[16px]">
          NouSchool membantu sekolah mengelola absensi siswa, kehadiran mengajar guru, izin, hingga rapor — dari
          satu dashboard, tanpa kertas.
        </p>
        <div className="mt-2 flex flex-wrap items-center justify-center gap-3">
          <Button type="button" onClick={onPrimaryCta}>
            Daftar Minat
          </Button>
          <Button type="button" variant="secondary" onClick={onSecondaryCta}>
            Lihat fitur
            <ArrowRight size={16} strokeWidth={2} aria-hidden="true" />
          </Button>
        </div>
      </div>

      <div className="mx-auto mt-14 max-w-[720px] rounded-lg border border-line bg-surface p-5 lg:p-6">
        <div className="mb-4 flex items-center justify-between">
          <div>
            <p className="text-[11px] font-semibold uppercase tracking-[0.1em] text-muted">Dashboard Kepala Sekolah</p>
            <p className="text-[16px] font-semibold text-ink">Senin, 17 Agu 2026</p>
          </div>
          <span className="hidden text-[12px] text-muted sm:inline">Update real-time</span>
        </div>

        <div className="mb-5 grid grid-cols-3 gap-4 border-b border-line pb-5">
          <StatTile label="Guru Mengajar" value={24} variant="success" />
          <StatTile label="Guru Izin" value={2} variant="info" />
          <StatTile label="Kelas Kosong" value={1} variant="warning" />
        </div>

        <div className="flex flex-col gap-3">
          {MOCK_ROWS.map((row) => (
            <div key={row.name} className="flex items-center justify-between gap-3">
              <div className="min-w-0 text-left">
                <p className="truncate text-[13px] font-semibold text-ink">{row.name}</p>
                <p className="truncate text-[12px] text-muted">{row.detail}</p>
              </div>
              <StatusChip status={row.status} size="sm" />
            </div>
          ))}
        </div>
      </div>
    </section>
  );
}
