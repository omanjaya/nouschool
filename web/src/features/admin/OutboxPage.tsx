import { useEffect, useState } from 'react';
import { useSearchParams } from 'react-router-dom';
import { Inbox, RefreshCw, ShieldAlert } from 'lucide-react';
import { Card } from '../../components/ui/Card';
import { ListRow } from '../../components/ui/ListRow';
import { Tag } from '../../components/ui/Tag';
import { Button } from '../../components/ui/Button';
import { Field, Select } from '../../components/ui/Field';
import { SegmentedControl } from '../../components/ui/SegmentedControl';
import { Skeleton } from '../../components/ui/Skeleton';
import { EmptyState } from '../../components/ui/EmptyState';
import { ErrorState } from '../../components/ui/ErrorState';
import { Dialog } from '../../components/ui/Dialog';
import { useToast } from '../../components/ui/Toast';
import { useMe } from '../auth/api';
import { useOutbox, useRetryAllOutbox, useRetryOutboxItem, useSchools } from './api';
import { ApiError } from '../../lib/api';
import { formatRelativeTime } from '../../lib/date';
import type { NotificationChannel, OutboxStatus } from '../../lib/types';

const STATUS_OPTIONS: { value: OutboxStatus; label: string }[] = [
  { value: 'dead', label: 'Dead' },
  { value: 'failed', label: 'Failed' },
  { value: 'pending', label: 'Pending' },
];

const STATUS_LABEL: Record<OutboxStatus, string> = {
  dead: 'Dead',
  failed: 'Failed',
  pending: 'Pending',
  sent: 'Terkirim',
};

const CHANNEL_LABEL: Record<NotificationChannel, string> = {
  in_app: 'In-app',
  web_push: 'Web Push',
  whatsapp: 'WhatsApp',
  email: 'Email',
};

const PER_PAGE = 20;

/** /admin/outbox — antrean notifikasi lintas sekolah (Fase 13 P4, docs/11 P4). */
export function OutboxPage() {
  const { data: me } = useMe();
  const [searchParams] = useSearchParams();
  const { data: schools } = useSchools();
  const { showToast } = useToast();

  const [status, setStatus] = useState<OutboxStatus>('dead');
  const [schoolId, setSchoolId] = useState<string>(searchParams.get('school_id') ?? '');
  const [page, setPage] = useState(1);
  const [retryAllOpen, setRetryAllOpen] = useState(false);

  const { data, isLoading, isError, refetch } = useOutbox({ status, schoolId: schoolId || undefined, page });
  const retryItem = useRetryOutboxItem();
  const retryAll = useRetryAllOutbox();

  useEffect(() => {
    setPage(1);
  }, [status, schoolId]);

  if (me && !me.is_super_admin) {
    return (
      <div className="mx-auto max-w-[640px] px-5 py-6">
        <EmptyState icon={ShieldAlert} message="Anda tidak memiliki akses ke halaman ini." />
      </div>
    );
  }

  function handleRetryItem(id: string) {
    retryItem.mutate(id, {
      onSuccess: () => showToast('Pesan diantrikan ulang.'),
      onError: (err) => showToast(err instanceof ApiError ? err.message : 'Gagal mengantrikan ulang pesan.', 'error'),
    });
  }

  function handleRetryAll() {
    retryAll.mutate(
      { status, schoolId: schoolId || undefined },
      {
        onSuccess: (result) => {
          showToast(`${result.retried} pesan diantrikan ulang.`);
          setRetryAllOpen(false);
        },
        onError: (err) => {
          showToast(err instanceof ApiError ? err.message : 'Gagal mengantrikan ulang pesan.', 'error');
          setRetryAllOpen(false);
        },
      },
    );
  }

  const canRetry = status !== 'pending';
  const totalPages = data ? Math.max(1, Math.ceil(data.total / PER_PAGE)) : 1;

  return (
    <div className="mx-auto flex max-w-[640px] flex-col gap-6 px-5 py-6 lg:max-w-[900px]">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <p className="text-[11px] font-semibold uppercase tracking-[0.1em] text-muted">Platform</p>
          <h1 className="text-[21px] font-semibold text-ink">Outbox</h1>
        </div>
        <Button variant="secondary" onClick={() => setRetryAllOpen(true)}>
          <RefreshCw size={16} strokeWidth={2} aria-hidden="true" />
          Retry Semua ({STATUS_LABEL[status]})
        </Button>
      </div>

      <div className="flex flex-wrap items-end gap-3">
        <SegmentedControl options={STATUS_OPTIONS} value={status} onChange={setStatus} />
        <Field label="Sekolah" htmlFor="outbox-school" className="min-w-[200px] flex-1">
          <Select id="outbox-school" value={schoolId} onChange={(e) => setSchoolId(e.target.value)}>
            <option value="">Semua sekolah</option>
            {schools?.map((school) => (
              <option key={school.id} value={school.id}>
                {school.name}
              </option>
            ))}
          </Select>
        </Field>
      </div>

      {isLoading ? (
        <div className="flex flex-col gap-2">
          <Skeleton className="h-14 w-full" />
          <Skeleton className="h-14 w-full" />
          <Skeleton className="h-14 w-full" />
        </div>
      ) : isError || !data ? (
        <ErrorState message="Gagal memuat antrean outbox." onRetry={() => refetch()} />
      ) : data.items.length === 0 ? (
        <EmptyState icon={Inbox} message={`Tidak ada pesan ${status}.`} />
      ) : (
        <Card variant="plain">
          <div>
            {data.items.map((item) => (
              <ListRow
                key={item.id}
                title={item.event}
                subtitle={
                  <>
                    {item.school_name} · {item.user_name ?? '—'} ·{' '}
                    <span className="num">{item.attempts}x</span> · {formatRelativeTime(item.created_at)}
                  </>
                }
                trailing={
                  <div className="flex items-center gap-2">
                    <Tag variant="neutral">{CHANNEL_LABEL[item.channel] ?? item.channel}</Tag>
                    {canRetry && (
                      <Button
                        variant="ghost"
                        loading={retryItem.isPending && retryItem.variables === item.id}
                        onClick={() => handleRetryItem(item.id)}
                      >
                        <RefreshCw size={16} strokeWidth={2} aria-hidden="true" />
                        Retry
                      </Button>
                    )}
                  </div>
                }
              />
            ))}
          </div>
        </Card>
      )}

      {data && data.items.length > 0 && totalPages > 1 && (
        <div className="flex items-center justify-between gap-3">
          <Button variant="secondary" disabled={page <= 1} onClick={() => setPage((p) => Math.max(1, p - 1))}>
            Sebelumnya
          </Button>
          <p className="num text-[12px] text-muted">
            Halaman {page} dari {totalPages}
          </p>
          <Button
            variant="secondary"
            disabled={page >= totalPages}
            onClick={() => setPage((p) => Math.min(totalPages, p + 1))}
          >
            Berikutnya
          </Button>
        </div>
      )}

      <Dialog open={retryAllOpen} onClose={() => setRetryAllOpen(false)} title="Retry semua pesan?">
        <p className="text-[14px] text-ink">
          Semua pesan berstatus <span className="font-medium">{STATUS_LABEL[status]}</span>
          {schoolId ? ' pada sekolah yang dipilih' : ' lintas sekolah'} akan diantrikan ulang untuk dikirim.
        </p>
        <div className="mt-4 flex justify-end gap-2">
          <Button type="button" variant="secondary" onClick={() => setRetryAllOpen(false)}>
            Batal
          </Button>
          <Button type="button" variant="primary" loading={retryAll.isPending} onClick={handleRetryAll}>
            Retry Semua
          </Button>
        </div>
      </Dialog>
    </div>
  );
}
