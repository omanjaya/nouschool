import { useState } from 'react';
import { Link } from 'react-router-dom';
import { ChevronLeft, Download, ShieldAlert } from 'lucide-react';
import { Card } from '../../components/ui/Card';
import { StatTile } from '../../components/ui/StatTile';
import { Skeleton } from '../../components/ui/Skeleton';
import { ErrorState } from '../../components/ui/ErrorState';
import { Field, Select } from '../../components/ui/Field';
import { EmptyState } from '../../components/ui/EmptyState';
import { useMe } from '../auth/api';
import { useAdminRevenue, usePlans } from './api';
import { formatRupiah } from '../../lib/currency';

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

  return (
    <div className="mx-auto flex max-w-[640px] flex-col gap-6 px-5 py-6 lg:max-w-[1000px]">
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
            <Card variant="plain">
              <div className="overflow-x-auto">
                <table className="w-full min-w-[420px] border-collapse text-[13px]">
                  <thead>
                    <tr className="bg-surface-2 text-left text-[11px] font-semibold uppercase tracking-[0.05em] text-muted">
                      <th className="px-3 py-2">Bulan</th>
                      <th className="px-3 py-2 text-right">Invoice</th>
                      <th className="px-3 py-2 text-right">Pendapatan</th>
                    </tr>
                  </thead>
                  <tbody>
                    {data.months.map((m) => {
                      const isEmpty = m.invoice_count === 0;
                      return (
                        <tr key={m.month} className={`border-b border-line last:border-0 ${isEmpty ? 'text-muted' : 'text-ink'}`}>
                          <td className="px-3 py-2">{MONTH_NAMES[m.month - 1] ?? m.month}</td>
                          <td className="num px-3 py-2 text-right">{m.invoice_count}</td>
                          <td className="num px-3 py-2 text-right">{formatRupiah(m.paid_total)}</td>
                        </tr>
                      );
                    })}
                  </tbody>
                </table>
              </div>
            </Card>
          </div>

          <div className="flex flex-col gap-3">
            <p className="text-[11px] font-semibold uppercase tracking-[0.1em] text-muted">Per Plan</p>
            {data.by_plan.length === 0 ? (
              <Card>
                <p className="text-[14px] text-muted">Belum ada pendapatan tercatat tahun ini.</p>
              </Card>
            ) : (
              <Card variant="plain">
                <div>
                  {data.by_plan.map((row) => (
                    <div
                      key={row.plan_code}
                      className="flex items-center justify-between gap-3 border-b border-line py-3 last:border-0"
                    >
                      <div>
                        <p className="text-[14px] text-ink">{planName(row.plan_code)}</p>
                        <p className="num text-[12px] text-muted">{row.invoice_count} invoice</p>
                      </div>
                      <p className="num text-[14px] font-semibold text-ink">{formatRupiah(row.paid_total)}</p>
                    </div>
                  ))}
                </div>
              </Card>
            )}
          </div>
        </>
      )}
    </div>
  );
}
