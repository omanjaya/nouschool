import { useState } from 'react';
import { Navigate } from 'react-router-dom';
import { Check, CreditCard, FileDown, Receipt, Upload, X } from 'lucide-react';
import { Card } from '../../components/ui/Card';
import { Skeleton } from '../../components/ui/Skeleton';
import { EmptyState } from '../../components/ui/EmptyState';
import { ErrorState } from '../../components/ui/ErrorState';
import { Tag } from '../../components/ui/Tag';
import { Button } from '../../components/ui/Button';
import { useToast } from '../../components/ui/Toast';
import { ApiError } from '../../lib/api';
import { formatDate } from '../../lib/date';
import { formatRupiah } from '../../lib/currency';
import { useMe } from '../auth/api';
import { useBilling, usePayInvoice } from './api';
import { UploadProofDialog } from './UploadProofDialog';
import { FEATURE_LABELS, featureLabel } from './featureLabels';
import { INVOICE_STATUS_LABEL, INVOICE_STATUS_TAG_VARIANT, SUBSCRIPTION_STATUS_LABEL, SUBSCRIPTION_STATUS_TAG_VARIANT } from './format';
import type { BillingSubscription, Invoice } from '../../lib/types';

/** Kartu ringkasan langganan — nama plan, status, periode, harga, pemakaian siswa, daftar fitur. */
function SubscriptionCard({ subscription }: { subscription: BillingSubscription }) {
  const usagePct = subscription.max_students > 0
    ? Math.min(100, Math.round((subscription.student_count / subscription.max_students) * 100))
    : 0;

  return (
    <Card className="flex flex-col gap-4">
      <div className="flex items-start justify-between gap-3">
        <div>
          <p className="text-[18px] font-semibold text-ink">{subscription.plan_name}</p>
          <p className="text-[12px] text-muted">
            {formatDate(subscription.starts_on)} – {formatDate(subscription.ends_on)}
          </p>
        </div>
        <Tag variant={SUBSCRIPTION_STATUS_TAG_VARIANT[subscription.status]}>
          {SUBSCRIPTION_STATUS_LABEL[subscription.status]}
        </Tag>
      </div>

      <p className="num text-[21px] font-semibold text-ink">{formatRupiah(subscription.price)}</p>

      <div className="flex flex-col gap-1.5">
        <div className="flex items-center justify-between text-[12px] text-muted">
          <span>
            <span className="num">{subscription.student_count}</span> dari maks{' '}
            <span className="num">{subscription.max_students}</span> siswa
          </span>
          <span className="num">{usagePct}%</span>
        </div>
        <div className="h-1.5 w-full overflow-hidden rounded-full bg-surface-2">
          <div className="h-full rounded-full bg-primary" style={{ width: `${usagePct}%` }} />
        </div>
      </div>

      <div className="flex flex-col gap-2 border-t border-line pt-3">
        <p className="text-[11px] font-semibold uppercase tracking-[0.1em] text-muted">Fitur</p>
        <ul className="flex flex-col gap-1.5">
          {Object.keys(FEATURE_LABELS).map((key) => {
            const active = subscription.features.includes(key);
            return (
              <li key={key} className="flex items-center gap-2 text-[13px] text-muted">
                {active ? (
                  <Check size={16} strokeWidth={2} className="shrink-0 text-primary" aria-hidden="true" />
                ) : (
                  <X size={16} strokeWidth={2} className="shrink-0 text-muted" aria-hidden="true" />
                )}
                <span className={active ? 'text-ink' : ''}>{featureLabel(key)}</span>
              </li>
            );
          })}
        </ul>
      </div>
    </Card>
  );
}

function InvoiceRow({ invoice, onUpload }: { invoice: Invoice; onUpload: () => void }) {
  const payInvoice = usePayInvoice();
  const { showToast } = useToast();

  function handlePay() {
    payInvoice.mutate(invoice.id, {
      onSuccess: (data) => {
        window.location.href = data.redirect_url;
      },
      onError: (err) => {
        showToast(err instanceof ApiError ? err.message : 'Gagal memulai pembayaran online.', 'error');
      },
    });
  }

  return (
    <div className="flex flex-col gap-3 border-b border-line py-3">
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <p className="truncate text-[14px] text-ink">{invoice.number}</p>
          <p className="num text-[12px] text-muted">
            {formatRupiah(invoice.amount)} · Jatuh tempo {formatDate(invoice.due_at)}
          </p>
        </div>
        <Tag variant={INVOICE_STATUS_TAG_VARIANT[invoice.status]}>{INVOICE_STATUS_LABEL[invoice.status]}</Tag>
      </div>

      <div className="flex flex-wrap gap-2">
        {invoice.status === 'unpaid' && (
          <>
            <Button variant="secondary" onClick={handlePay} loading={payInvoice.isPending}>
              <CreditCard size={16} strokeWidth={2} aria-hidden="true" />
              Bayar Online
            </Button>
            <Button variant="secondary" onClick={onUpload}>
              <Upload size={16} strokeWidth={2} aria-hidden="true" />
              Upload Bukti Transfer
            </Button>
          </>
        )}
        <a
          href={invoice.pdf_url}
          target="_blank"
          rel="noreferrer"
          className="inline-flex min-h-11 items-center gap-2 rounded-lg border border-line px-4 text-[14px] font-medium text-ink transition-colors duration-150 hover:bg-surface-2"
        >
          <FileDown size={16} strokeWidth={2} aria-hidden="true" />
          Unduh PDF
        </a>
      </div>
    </div>
  );
}

/** /tagihan — langganan & invoice sekolah (admin & kepsek), docs/09-billing.md. */
export function BillingPage() {
  const { data: me } = useMe();
  const { data, isLoading, isError, refetch } = useBilling();
  const [uploadTarget, setUploadTarget] = useState<Invoice | null>(null);

  if (me && me.role !== 'admin_sekolah' && me.role !== 'kepala_sekolah') {
    return <Navigate to="/" replace />;
  }

  return (
    <div className="mx-auto flex max-w-[640px] flex-col gap-6 px-5 py-6">
      <div>
        <p className="text-[11px] font-semibold uppercase tracking-[0.1em] text-muted">Sekolah</p>
        <h1 className="text-[21px] font-semibold text-ink">Tagihan & Langganan</h1>
      </div>

      {isLoading ? (
        <div className="flex flex-col gap-3">
          <Skeleton className="h-52 w-full" />
          <Skeleton className="h-16 w-full" />
          <Skeleton className="h-16 w-full" />
        </div>
      ) : isError ? (
        <ErrorState message="Gagal memuat data tagihan." onRetry={() => refetch()} />
      ) : !data?.subscription ? (
        <EmptyState icon={Receipt} message="Sekolah belum memiliki langganan. Hubungi NouSchool." />
      ) : (
        <>
          <SubscriptionCard subscription={data.subscription} />

          <div className="flex flex-col gap-1">
            <p className="text-[11px] font-semibold uppercase tracking-[0.1em] text-muted">Tagihan</p>
            {data.invoices.length === 0 ? (
              <EmptyState icon={Receipt} message="Belum ada tagihan." />
            ) : (
              <div>
                {data.invoices.map((invoice) => (
                  <InvoiceRow key={invoice.id} invoice={invoice} onUpload={() => setUploadTarget(invoice)} />
                ))}
              </div>
            )}
          </div>
        </>
      )}

      <UploadProofDialog invoice={uploadTarget} onClose={() => setUploadTarget(null)} />
    </div>
  );
}
