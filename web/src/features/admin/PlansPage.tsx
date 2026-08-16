import { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { ChevronLeft, ShieldAlert, Tag as TagIcon } from 'lucide-react';
import { Card } from '../../components/ui/Card';
import { Skeleton } from '../../components/ui/Skeleton';
import { EmptyState } from '../../components/ui/EmptyState';
import { ErrorState } from '../../components/ui/ErrorState';
import { Button } from '../../components/ui/Button';
import { Checkbox, Field, Input } from '../../components/ui/Field';
import { useToast } from '../../components/ui/Toast';
import { ApiError } from '../../lib/api';
import { formatRupiah } from '../../lib/currency';
import { useMe } from '../auth/api';
import { featureLabel } from '../billing/featureLabels';
import { usePlans, useUpdatePlan } from './api';
import type { Plan, PlanPrice } from '../../lib/types';
import { PAGE_WIDE } from '../../components/ui/page';

function PlanCard({ plan }: { plan: Plan }) {
  const update = useUpdatePlan();
  const { showToast } = useToast();
  const [features, setFeatures] = useState<Record<string, boolean>>(plan.features);
  const [prices, setPrices] = useState<PlanPrice[]>(plan.prices);

  useEffect(() => {
    setFeatures(plan.features);
    setPrices(plan.prices);
  }, [plan]);

  function toggleFeature(key: string) {
    setFeatures((prev) => ({ ...prev, [key]: !prev[key] }));
  }

  function updateMaxStudents(index: number, value: string) {
    const n = Number(value);
    setPrices((prev) => prev.map((p, i) => (i === index ? { ...p, max_students: Number.isFinite(n) ? n : 0 } : p)));
  }

  function updatePriceYearly(index: number, value: string) {
    const n = Number(value);
    setPrices((prev) => prev.map((p, i) => (i === index ? { ...p, price_yearly: Number.isFinite(n) ? n : 0 } : p)));
  }

  function handleSave() {
    update.mutate(
      { code: plan.code, input: { name: plan.name, features, prices } },
      {
        onSuccess: () => showToast(`Plan ${plan.name} disimpan.`),
        onError: (err) => showToast(err instanceof ApiError ? err.message : 'Gagal menyimpan plan.', 'error'),
      },
    );
  }

  return (
    <Card className="flex flex-col gap-4">
      <div>
        <p className="text-[18px] font-semibold text-ink">{plan.name}</p>
        <p className="num text-[12px] text-muted">{plan.code}</p>
      </div>

      <div className="flex flex-col gap-2">
        <p className="text-[11px] font-semibold uppercase tracking-[0.1em] text-muted">Fitur</p>
        <div className="flex flex-col gap-2">
          {Object.keys(features).map((key) => (
            <Checkbox
              key={key}
              id={`plan-${plan.code}-feat-${key}`}
              label={featureLabel(key)}
              checked={features[key]}
              onChange={() => toggleFeature(key)}
            />
          ))}
        </div>
      </div>

      <div className="flex flex-col gap-3 border-t border-line pt-3">
        <p className="text-[11px] font-semibold uppercase tracking-[0.1em] text-muted">Harga per bracket siswa</p>
        {prices.map((price, index) => (
          <div key={index} className="flex flex-wrap items-end gap-3">
            <Field label="Maks siswa" htmlFor={`plan-${plan.code}-max-${index}`} className="min-w-[120px] flex-1">
              <Input
                id={`plan-${plan.code}-max-${index}`}
                type="number"
                min={1}
                value={price.max_students}
                onChange={(e) => updateMaxStudents(index, e.target.value)}
              />
            </Field>
            <Field label="Harga/tahun (Rp)" htmlFor={`plan-${plan.code}-price-${index}`} className="min-w-[160px] flex-1">
              <Input
                id={`plan-${plan.code}-price-${index}`}
                type="number"
                min={0}
                step={1000}
                value={price.price_yearly}
                onChange={(e) => updatePriceYearly(index, e.target.value)}
              />
            </Field>
          </div>
        ))}
        <p className="text-[12px] text-muted">{prices.map((p) => formatRupiah(p.price_yearly)).join(' · ')}</p>
      </div>

      {update.isError && (
        <p className="text-[12px] text-danger">
          {update.error instanceof ApiError ? update.error.message : 'Gagal menyimpan plan.'}
        </p>
      )}

      <Button variant="secondary" onClick={handleSave} loading={update.isPending} className="self-start">
        Simpan
      </Button>
    </Card>
  );
}

/** /admin/plans — kelola plan & harga bracket siswa (super admin), docs/09-billing.md "Panel super admin". */
export function PlansPage() {
  const { data: me } = useMe();
  const { data: plans, isLoading, isError, refetch } = usePlans();

  if (me && !me.is_super_admin) {
    return (
      <div className={PAGE_WIDE}>
        <EmptyState icon={ShieldAlert} message="Anda tidak memiliki akses ke halaman ini." />
      </div>
    );
  }

  return (
    <div className={`${PAGE_WIDE} flex flex-col gap-6`}>
      <div>
        <Link
          to="/admin/sekolah"
          className="mb-3 inline-flex items-center gap-1 text-[12px] font-medium text-muted hover:text-ink"
        >
          <ChevronLeft size={16} strokeWidth={2} aria-hidden="true" />
          Sekolah
        </Link>
        <p className="text-[11px] font-semibold uppercase tracking-[0.1em] text-muted">Platform</p>
        <h1 className="text-[21px] font-semibold text-ink">Plan & Harga</h1>
      </div>

      {isLoading ? (
        <div className="flex flex-col gap-4">
          <Skeleton className="h-64 w-full" />
          <Skeleton className="h-64 w-full" />
        </div>
      ) : isError ? (
        <ErrorState message="Gagal memuat daftar plan." onRetry={() => refetch()} />
      ) : !plans || plans.length === 0 ? (
        <EmptyState icon={TagIcon} message="Belum ada plan terdaftar." />
      ) : (
        <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
          {plans.map((plan) => (
            <PlanCard key={plan.code} plan={plan} />
          ))}
        </div>
      )}
    </div>
  );
}
