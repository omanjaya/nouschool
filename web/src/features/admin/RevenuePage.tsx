import { useState } from 'react';
import { Link } from 'react-router-dom';
import { ChevronLeft, Download, ShieldAlert } from 'lucide-react';
import { Card } from '../../components/ui/Card';
import { StatTile } from '../../components/ui/StatTile';
import { DataTable, type DataTableColumn } from '../../components/ui/DataTable';
import { Skeleton } from '../../components/ui/Skeleton';
import { ErrorState } from '../../components/ui/ErrorState';
import { Field, Select } from '../../components/ui/Field';
import { EmptyState } from '../../components/ui/EmptyState';
import { PAGE_WIDE } from '../../components/ui/page';
import { useMe } from '../auth/api';
import { useAdminRevenue, usePlans } from './api';
import { formatRupiah } from '../../lib/currency';
import type { AdminRevenueByPlan, AdminRevenueMonth } from '../../lib/types';

const MONTH_NAMES = [
  'Januari',
  'Februari',
  'Maret',
  'April',
  'Mei',
  'Juni',
  'Juli',
  'Agustus',
  'September',
  'Oktober',
  'November',
  'Desember',
];

function yearOptions(): number[] {
  const current = new Date().getFullYear();
  return [current - 2, current - 1, current, current + 1, current + 2];
}

/** `/admin/pendapatan` — laporan pendapatan platform (Fase 13 Gelombang 2 P6, docs/11 P6). */
export function RevenuePage() {
  const { data: me } = useMe();
  const [year, setYear] = useState(new Date().getFullYear());
  const { data, isLoading, isError, refetch } = useAdminRevenue(year);
  const { data: plans } = usePlans();

  if (me && !me.is_super_admin) {
    return (
      <div className="mx-auto max-w-[640px] px-5 py-6">
        <EmptyState icon={ShieldAlert} message="Anda tidak memiliki akses ke halaman ini." />
      </div>
    );
  }

  const planName = (code: string) => plans?.find((p) => p.code === code)?.name ?? code;

  /** Kolom desktop (docs/10 §5). */
  const monthColumns: DataTableColumn<AdminRevenueMonth>[] = [
    { key: 'month', header: 'Bulan', cell: (m) => MONTH_NAMES[m.month - 1] ?? m.month },
    { key: 'invoice_count', header: 'Invoice', align: 'right', sortable: true, sortValue: (m) => m.invoice_count, cell: (m) => m.invoice_count },
    { key: 'paid_total', header: 'Pendapatan', align: 'right', sortable: true, sortValue: (m) => m.paid_total, cell: (m) => formatRupiah(m.paid_total) },
  ];

  const planColumns: DataTableColumn<AdminRevenueByPlan>[] = [
    { key: 'plan_code', header: 'Plan', sortable: true, sortValue: (p) => planName(p.plan_code), cell: (p) => planName(p.plan_code) },
    { key: 'invoice_count', header: 'Invoice', align: 'right', sortable: true, sortValue: (p) => p.invoice_count, cell: (p) => p.invoice_count },
    { key: 'paid_total', header: 'Pendapatan', align: 'right', sortable: true, sortValue: (p) => p.paid_total, cell: (p) => formatRupiah(p.paid_total) },
  ];

  return (
    <div className={`${PAGE_WIDE} flex flex-col gap-6`}>
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <Link
            to="/admin"
            className="mb-3 inline-flex items-center gap-1 text-[12px] font-medium text-muted hover:text-ink"
          >
            <ChevronLeft size={16} strokeWidth={2} aria-hidden="true" />
            Beranda
          </Link>
          <p className="text-[11px] font-semibold uppercase tracking-[0.1em] text-muted">Platform</p>
          <h1 className="text-[21px] font-semibold text-ink">Pendapatan</h1>
        </div>
        <a
          href={`/api/admin/revenue/export?year=${year}`}
          className="inline-flex min-h-11 items-center gap-2 rounded-lg border border-line bg-surface px-4 text-[14px] font-medium text-ink transition-colors duration-150 hover:bg-surface-2"
        >
          <Download size={16} strokeWidth={2} aria-hidden="true" />
          Unduh Excel
        </a>
      </div>

      <Field label="Tahun" htmlFor="revenue-year" className="max-w-[200px]">
        <Select id="revenue-year" value={year} onChange={(e) => setYear(Number(e.target.value))}>
          {yearOptions().map((y) => (
            <option key={y} value={y}>
              {y}
            </option>
          ))}
        </Select>
      </Field>

      {isLoading ? (
        <div className="flex flex-col gap-4">
          <Skeleton className="h-24 w-full" />
          <Skeleton className="h-64 w-full" />
        </div>
      ) : isError || !data ? (
        <ErrorState message="Gagal memuat laporan pendapatan." onRetry={() => refetch()} />
      ) : (
        <>
          <Card>
            <StatTile label={`Total Pendapatan ${data.year}`} value={formatRupiah(data.total)} />
          </Card>

          <div className="flex flex-col gap-3">
            <p className="text-[11px] font-semibold uppercase tracking-[0.1em] text-muted">Per Bulan</p>
            <DataTable columns={monthColumns} data={data.months} keyField={(m) => String(m.month)} />
          </div>

          <div className="flex flex-col gap-3">
            <p className="text-[11px] font-semibold uppercase tracking-[0.1em] text-muted">Per Plan</p>
            <DataTable
              columns={planColumns}
              data={data.by_plan}
              keyField={(p) => p.plan_code}
              emptyMessage="Belum ada pendapatan tercatat tahun ini."
            />
          </div>
        </>
      )}
    </div>
  );
}
