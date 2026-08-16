import { useEffect, useMemo, useRef, useState, type FormEvent, type RefObject } from 'react';
import { Link, useParams } from 'react-router-dom';
import {
  Building2,
  CalendarRange,
  ChevronLeft,
  Copy,
  Eye,
  FileDown,
  History,
  KeyRound,
  LogIn,
  Receipt,
  Search,
  Users,
} from 'lucide-react';
import { Card } from '../../components/ui/Card';
import { Skeleton } from '../../components/ui/Skeleton';
import { EmptyState } from '../../components/ui/EmptyState';
import { ErrorState } from '../../components/ui/ErrorState';
import { ListRow } from '../../components/ui/ListRow';
import { StatTile } from '../../components/ui/StatTile';
import { Tag } from '../../components/ui/Tag';
import { Dialog } from '../../components/ui/Dialog';
import { Button } from '../../components/ui/Button';
import { Checkbox, Field, Input, Select } from '../../components/ui/Field';
import { useToast } from '../../components/ui/Toast';
import {
  ROLE_LABEL,
  useAcademicYears,
  useActivateAcademicYear,
  useCreateAcademicYear,
  useCreateSubscription,
  useExtendSubscription,
  useImpersonateSchool,
  useNotificationChannelSettings,
  usePlans,
  useResetPassword,
  useSchoolAudit,
  useSchoolBilling,
  useSchoolMembers,
  useSchools,
  useSchoolStats,
  useUpdateNotificationChannelSettings,
  useUpdateSchool,
  useVerifyInvoice,
  useVoidInvoice,
} from './api';
import { TIMEZONES } from '../../lib/timezones';
import { formatDate, formatRelativeTime } from '../../lib/date';
import { formatRupiah } from '../../lib/currency';
import { ApiError } from '../../lib/api';
import {
  INVOICE_STATUS_LABEL,
  INVOICE_STATUS_TAG_VARIANT,
  SUBSCRIPTION_STATUS_LABEL,
  SUBSCRIPTION_STATUS_TAG_VARIANT,
} from '../billing/format';
import type { AdminInvoice, AdminAuditLogItem, AdminSchoolMember, NotificationChannel, School } from '../../lib/types';

/**
 * Observer sekali-jalan (docs/11 P3: statistik "hanya agregat", tapi tetap
 * query yang lumayan berat) — set `true` begitu elemen pertama kali masuk
 * viewport, lalu berhenti mengamati; dipakai `StatisticsSection` supaya
 * `GET .../stats` tidak ikut nge-fetch di render awal bersama seksi lain.
 */
function useInViewOnce<T extends HTMLElement>(): [RefObject<T | null>, boolean] {
  const ref = useRef<T | null>(null);
  const [inView, setInView] = useState(false);

  useEffect(() => {
    if (inView) return;
    const node = ref.current;
    if (!node) return;

    const observer = new IntersectionObserver(
      ([entry]) => {
        if (entry.isIntersecting) {
          setInView(true);
          observer.disconnect();
        }
      },
      { rootMargin: '200px' },
    );
    observer.observe(node);
    return () => observer.disconnect();
  }, [inView]);

  return [ref, inView];
}

/** "3.2 KB" / "4.1 MB" — dipakai storage terpakai di `StatisticsSection` (docs/11 P3). */
function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  const kb = bytes / 1024;
  if (kb < 1024) return `${kb.toFixed(1)} KB`;
  const mb = kb / 1024;
  if (mb < 1024) return `${mb.toFixed(1)} MB`;
  return `${(mb / 1024).toFixed(2)} GB`;
}

export function SchoolDetailPage() {
  const { id } = useParams<{ id: string }>();
  const { data: schools, isLoading, isError, refetch } = useSchools();
  const school = useMemo(() => schools?.find((s) => s.id === id), [schools, id]);

  if (isLoading) {
    return (
      <div className="mx-auto flex max-w-[640px] flex-col gap-4 px-5 py-6">
        <Skeleton className="h-6 w-40" />
        <Skeleton className="h-40 w-full" />
      </div>
    );
  }

  if (isError) {
    return (
      <div className="mx-auto max-w-[640px] px-5 py-6">
        <ErrorState message="Gagal memuat data sekolah." onRetry={() => refetch()} />
      </div>
    );
  }

  if (!school || !id) {
    return (
      <div className="mx-auto max-w-[640px] px-5 py-6">
        <EmptyState icon={Building2} message="Sekolah tidak ditemukan." />
      </div>
    );
  }

  return (
    <div className="mx-auto flex max-w-[640px] flex-col gap-6 px-5 py-6 lg:max-w-[860px]">
      <div>
        <Link
          to="/admin/sekolah"
          className="mb-3 inline-flex items-center gap-1 text-[12px] font-medium text-muted hover:text-ink"
        >
          <ChevronLeft size={16} strokeWidth={2} aria-hidden="true" />
          Sekolah
        </Link>
        <h1 className="text-[21px] font-semibold text-ink">{school.name}</h1>
        <p className="text-[12px] text-muted">{school.slug}</p>
      </div>

      <ImpersonateButton school={school} />

      <SchoolEditForm school={school} />
      <StatisticsSection schoolId={school.id} />
      <AcademicYearsSection schoolId={school.id} />
      <BillingSection schoolId={school.id} />
      <NotificationChannelsSection schoolId={school.id} />
      <MembersSection schoolId={school.id} />
      <AuditLogSection schoolId={school.id} />
    </div>
  );
}

/**
 * Tombol sesi support super admin (Fase 12) — hanya untuk sekolah `active`
 * (sekolah `suspended` tidak boleh diakses lewat sesi support). Backend
 * membuat token sekali pakai; URL tenant dibangun DI CLIENT dari host
 * platform saat ini (`admin.nouschool.id` → `{slug}.nouschool.id`, atau
 * `localhost:5173` → `{slug}.localhost:5173` di dev) supaya jalan di
 * environment mana pun tanpa hardcode domain.
 */
function ImpersonateButton({ school }: { school: School }) {
  const impersonate = useImpersonateSchool(school.id);
  const { showToast } = useToast();

  if (school.status !== 'active') return null;

  function handleClick() {
    impersonate.mutate(undefined, {
      onSuccess: ({ token, slug }) => {
        const baseHost = window.location.host.replace(/^admin\./, '');
        const url = `${window.location.protocol}//${slug}.${baseHost}/impersonate?token=${encodeURIComponent(token)}`;
        window.open(url, '_blank');
      },
      onError: (err) => {
        showToast(err instanceof ApiError ? err.message : 'Gagal memulai sesi support.', 'error');
      },
    });
  }

  return (
    <div className="flex flex-col items-start gap-1.5">
      <Button variant="secondary" loading={impersonate.isPending} onClick={handleClick}>
        <LogIn size={16} strokeWidth={2} aria-hidden="true" />
        Masuk sebagai Sekolah Ini
      </Button>
      <p className="text-[11px] text-muted">Sesi support 2 jam · tercatat di audit log sekolah.</p>
    </div>
  );
}

function SchoolEditForm({ school }: { school: School }) {
  const [name, setName] = useState(school.name);
  const [timezone, setTimezone] = useState(school.timezone);
  const [status, setStatus] = useState(school.status);
  const [customDomain, setCustomDomain] = useState(school.custom_domain ?? '');
  const update = useUpdateSchool(school.id);
  const { showToast } = useToast();

  function handleSubmit(e: FormEvent) {
    e.preventDefault();
    update.mutate(
      {
        name,
        timezone,
        status,
        custom_domain: customDomain.trim() === '' ? null : customDomain.trim(),
      },
      { onSuccess: () => showToast('Perubahan disimpan.') },
    );
  }

  return (
    <Card>
      <form onSubmit={handleSubmit} className="flex flex-col gap-4">
        <p className="text-[14px] font-semibold text-ink">Informasi Sekolah</p>

        <Field label="Nama sekolah" htmlFor="edit-name">
          <Input id="edit-name" value={name} onChange={(e) => setName(e.target.value)} required />
        </Field>
        <Field label="Zona waktu" htmlFor="edit-timezone">
          <Select id="edit-timezone" value={timezone} onChange={(e) => setTimezone(e.target.value)}>
            {TIMEZONES.map((tz) => (
              <option key={tz.value} value={tz.value}>
                {tz.label}
              </option>
            ))}
          </Select>
        </Field>
        <Field label="Status" htmlFor="edit-status">
          <Select
            id="edit-status"
            value={status}
            onChange={(e) => setStatus(e.target.value as 'active' | 'suspended')}
          >
            <option value="active">Aktif</option>
            <option value="suspended">Nonaktif</option>
          </Select>
        </Field>
        <Field label="Custom domain" htmlFor="edit-domain" hint="Kosongkan jika sekolah belum punya domain sendiri.">
          <Input
            id="edit-domain"
            value={customDomain}
            onChange={(e) => setCustomDomain(e.target.value)}
            placeholder="sekolah.sch.id"
          />
        </Field>

        {update.isError && (
          <p className="text-[12px] text-danger">
            {update.error instanceof ApiError ? update.error.message : 'Gagal menyimpan perubahan.'}
          </p>
        )}

        <Button type="submit" variant="secondary" loading={update.isPending} className="self-start">
          Simpan Perubahan
        </Button>
      </form>
    </Card>
  );
}

function AcademicYearsSection({ schoolId }: { schoolId: string }) {
  const { data: years, isLoading, isError, refetch } = useAcademicYears(schoolId);
  const createYear = useCreateAcademicYear(schoolId);
  const activateYear = useActivateAcademicYear(schoolId);
  const { showToast } = useToast();
  const [name, setName] = useState('');
  const [startsOn, setStartsOn] = useState('');
  const [endsOn, setEndsOn] = useState('');
  const [confirmId, setConfirmId] = useState<string | null>(null);

  function handleCreate(e: FormEvent) {
    e.preventDefault();
    createYear.mutate(
      { name, starts_on: startsOn, ends_on: endsOn },
      {
        onSuccess: () => {
          showToast('Tahun ajaran ditambahkan.');
          setName('');
          setStartsOn('');
          setEndsOn('');
        },
      },
    );
  }

  function handleActivate() {
    if (!confirmId) return;
    activateYear.mutate(confirmId, {
      onSuccess: () => {
        showToast('Tahun ajaran diaktifkan.');
        setConfirmId(null);
      },
    });
  }

  return (
    <div className="flex flex-col gap-4">
      <p className="text-[11px] font-semibold uppercase tracking-[0.1em] text-muted">Tahun Ajaran</p>

      {isLoading ? (
        <div className="flex flex-col gap-2">
          <Skeleton className="h-12 w-full" />
          <Skeleton className="h-12 w-full" />
        </div>
      ) : isError ? (
        <ErrorState message="Gagal memuat tahun ajaran." onRetry={() => refetch()} />
      ) : years && years.length === 0 ? (
        <EmptyState icon={CalendarRange} message="Belum ada tahun ajaran." />
      ) : (
        <div>
          {years?.map((year) => (
            <ListRow
              key={year.id}
              title={year.name}
              subtitle={`${formatDate(year.starts_on)} – ${formatDate(year.ends_on)}`}
              trailing={
                year.is_active ? (
                  <Tag variant="now">Aktif</Tag>
                ) : (
                  <Button variant="secondary" onClick={() => setConfirmId(year.id)}>
                    Jadikan aktif
                  </Button>
                )
              }
            />
          ))}
        </div>
      )}

      <Card>
        <form onSubmit={handleCreate} className="flex flex-col gap-4">
          <p className="text-[14px] font-semibold text-ink">Tambah Tahun Ajaran</p>
          <Field label="Nama" htmlFor="ay-name">
            <Input
              id="ay-name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="2026/2027"
              required
            />
          </Field>
          <div className="grid grid-cols-2 gap-3">
            <Field label="Mulai" htmlFor="ay-start">
              <Input
                id="ay-start"
                type="date"
                value={startsOn}
                onChange={(e) => setStartsOn(e.target.value)}
                required
              />
            </Field>
            <Field label="Selesai" htmlFor="ay-end">
              <Input id="ay-end" type="date" value={endsOn} onChange={(e) => setEndsOn(e.target.value)} required />
            </Field>
          </div>

          {createYear.isError && (
            <p className="text-[12px] text-danger">
              {createYear.error instanceof ApiError ? createYear.error.message : 'Gagal menambahkan tahun ajaran.'}
            </p>
          )}

          <Button type="submit" variant="secondary" loading={createYear.isPending} className="self-start">
            Simpan Tahun Ajaran
          </Button>
        </form>
      </Card>

      <Dialog open={confirmId !== null} onClose={() => setConfirmId(null)} title="Jadikan tahun ajaran aktif?">
        <p className="text-[14px] text-ink">
          Tahun ajaran aktif menentukan periode berjalan untuk seluruh data sekolah ini. Tahun ajaran yang sedang
          aktif akan dinonaktifkan.
        </p>
        <div className="mt-4 flex justify-end gap-2">
          <Button type="button" variant="secondary" onClick={() => setConfirmId(null)}>
            Batal
          </Button>
          <Button type="button" variant="primary" loading={activateYear.isPending} onClick={handleActivate}>
            Jadikan Aktif
          </Button>
        </div>
      </Dialog>
    </div>
  );
}

/**
 * Seksi "Langganan & Tagihan" (Fase 10, docs/09-billing.md "Panel super admin")
 * — status langganan, buat/perpanjang (plan dipilih, bracket & harga dihitung
 * server dari jumlah siswa aktif), perpanjang manual (goodwill), & aksi per
 * invoice (lihat bukti transfer, verifikasi, void).
 */
function BillingSection({ schoolId }: { schoolId: string }) {
  const { data, isLoading, isError, refetch } = useSchoolBilling(schoolId);
  const { data: plans } = usePlans();
  const createSubscription = useCreateSubscription(schoolId);
  const extendSubscription = useExtendSubscription(schoolId);
  const verifyInvoice = useVerifyInvoice(schoolId);
  const voidInvoice = useVoidInvoice(schoolId);
  const { showToast } = useToast();

  const [subDialogOpen, setSubDialogOpen] = useState(false);
  const [selectedPlan, setSelectedPlan] = useState('');
  const [extendOpen, setExtendOpen] = useState(false);
  const [extendDays, setExtendDays] = useState('30');
  const [verifyTarget, setVerifyTarget] = useState<AdminInvoice | null>(null);
  const [voidTarget, setVoidTarget] = useState<AdminInvoice | null>(null);

  useEffect(() => {
    if (plans && plans.length > 0 && !selectedPlan) setSelectedPlan(plans[0].code);
  }, [plans, selectedPlan]);

  if (isLoading) {
    return <Skeleton className="h-44 w-full" />;
  }

  if (isError || !data) {
    return <ErrorState message="Gagal memuat data tagihan." onRetry={() => refetch()} />;
  }

  const studentCount = data.subscription?.student_count ?? null;
  const currentPlan = plans?.find((p) => p.code === selectedPlan);
  const estimatedBracket =
    currentPlan && studentCount !== null && currentPlan.prices.length > 0
      ? (currentPlan.prices.find((p) => p.max_students >= studentCount) ?? currentPlan.prices[currentPlan.prices.length - 1])
      : null;

  function handleCreateSubscription() {
    if (!selectedPlan) return;
    createSubscription.mutate(selectedPlan, {
      onSuccess: () => {
        showToast('Langganan diperbarui.');
        setSubDialogOpen(false);
      },
      onError: (err) => showToast(err instanceof ApiError ? err.message : 'Gagal membuat langganan.', 'error'),
    });
  }

  function handleExtend() {
    const days = Number(extendDays);
    if (!Number.isFinite(days) || days <= 0) {
      showToast('Masukkan jumlah hari yang valid.', 'error');
      return;
    }
    extendSubscription.mutate(days, {
      onSuccess: () => {
        showToast('Langganan diperpanjang.');
        setExtendOpen(false);
      },
      onError: (err) => showToast(err instanceof ApiError ? err.message : 'Gagal memperpanjang langganan.', 'error'),
    });
  }

  function handleVerify() {
    if (!verifyTarget) return;
    verifyInvoice.mutate(verifyTarget.id, {
      onSuccess: () => {
        showToast('Pembayaran diverifikasi.');
        setVerifyTarget(null);
      },
      onError: (err) => showToast(err instanceof ApiError ? err.message : 'Gagal memverifikasi pembayaran.', 'error'),
    });
  }

  function handleVoid() {
    if (!voidTarget) return;
    voidInvoice.mutate(voidTarget.id, {
      onSuccess: () => {
        showToast('Tagihan dibatalkan.');
        setVoidTarget(null);
      },
      onError: (err) => showToast(err instanceof ApiError ? err.message : 'Gagal membatalkan tagihan.', 'error'),
    });
  }

  return (
    <div className="flex flex-col gap-4">
      <p className="text-[11px] font-semibold uppercase tracking-[0.1em] text-muted">Langganan & Tagihan</p>

      <Card className="flex flex-col gap-3">
        {data.subscription ? (
          <>
            <div className="flex items-start justify-between gap-3">
              <div>
                <p className="text-[14px] font-semibold text-ink">{data.subscription.plan_name}</p>
                <p className="text-[12px] text-muted">
                  {formatDate(data.subscription.starts_on)} – {formatDate(data.subscription.ends_on)}
                </p>
              </div>
              <Tag variant={SUBSCRIPTION_STATUS_TAG_VARIANT[data.subscription.status]}>
                {SUBSCRIPTION_STATUS_LABEL[data.subscription.status]}
              </Tag>
            </div>
            <p className="text-[12px] text-muted">
              <span className="num">{data.subscription.student_count}</span> dari maks{' '}
              <span className="num">{data.subscription.max_students}</span> siswa · {formatRupiah(data.subscription.price)}
            </p>
          </>
        ) : (
          <p className="text-[14px] text-muted">Belum ada langganan.</p>
        )}

        <div className="flex flex-wrap gap-2">
          <Button variant="secondary" onClick={() => setSubDialogOpen(true)}>
            Buat/Perpanjang Langganan
          </Button>
          {data.subscription && (
            <Button variant="secondary" onClick={() => setExtendOpen(true)}>
              Perpanjang Manual
            </Button>
          )}
        </div>
      </Card>

      <div>
        {data.invoices.length === 0 ? (
          <EmptyState icon={Receipt} message="Belum ada tagihan." />
        ) : (
          data.invoices.map((invoice) => (
            <div key={invoice.id} className="flex flex-col gap-2 border-b border-line py-3">
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
                {invoice.proof_url && (
                  <a
                    href={invoice.proof_url}
                    target="_blank"
                    rel="noreferrer"
                    className="inline-flex min-h-11 items-center gap-2 rounded-lg border border-line px-4 text-[14px] font-medium text-ink transition-colors duration-150 hover:bg-surface-2"
                  >
                    <Eye size={16} strokeWidth={2} aria-hidden="true" />
                    Lihat Bukti
                  </a>
                )}
                {(invoice.status === 'unpaid' || invoice.status === 'awaiting_verification') && (
                  <Button variant="secondary" onClick={() => setVerifyTarget(invoice)}>
                    Verifikasi Pembayaran
                  </Button>
                )}
                {invoice.status !== 'paid' && invoice.status !== 'void' && (
                  <Button variant="danger" onClick={() => setVoidTarget(invoice)}>
                    Void
                  </Button>
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
          ))
        )}
      </div>

      <Dialog open={subDialogOpen} onClose={() => setSubDialogOpen(false)} title="Buat/Perpanjang Langganan">
        <div className="flex flex-col gap-4">
          <Field label="Plan" htmlFor="sub-plan">
            <Select id="sub-plan" value={selectedPlan} onChange={(e) => setSelectedPlan(e.target.value)}>
              {!plans || plans.length === 0 ? (
                <option value="">Memuat…</option>
              ) : (
                plans.map((plan) => (
                  <option key={plan.code} value={plan.code}>
                    {plan.name}
                  </option>
                ))
              )}
            </Select>
          </Field>

          {currentPlan && (
            <div className="rounded-lg border border-line p-3">
              <p className="mb-2 text-[12px] font-medium text-muted">Bracket harga</p>
              <ul className="flex flex-col gap-1">
                {currentPlan.prices.map((price) => (
                  <li
                    key={price.max_students}
                    className={`flex items-center justify-between text-[13px] ${
                      estimatedBracket?.max_students === price.max_students ? 'font-semibold text-ink' : 'text-muted'
                    }`}
                  >
                    <span className="num">Sampai {price.max_students.toLocaleString('id-ID')} siswa</span>
                    <span className="num">{formatRupiah(price.price_yearly)}</span>
                  </li>
                ))}
              </ul>
              <p className="mt-2 text-[12px] text-muted">
                {studentCount !== null
                  ? `Estimasi untuk ${studentCount} siswa aktif saat ini.`
                  : 'Bracket & harga final dihitung otomatis dari jumlah siswa aktif saat langganan dibuat.'}
              </p>
            </div>
          )}
        </div>
        <div className="mt-4 flex justify-end gap-2">
          <Button type="button" variant="secondary" onClick={() => setSubDialogOpen(false)}>
            Batal
          </Button>
          <Button type="button" variant="primary" loading={createSubscription.isPending} onClick={handleCreateSubscription}>
            Simpan
          </Button>
        </div>
      </Dialog>

      <Dialog open={extendOpen} onClose={() => setExtendOpen(false)} title="Perpanjang Manual">
        <div className="flex flex-col gap-4">
          <p className="text-[14px] text-ink">Perpanjangan goodwill — tidak membuat invoice baru.</p>
          <Field label="Jumlah hari" htmlFor="extend-days">
            <Input
              id="extend-days"
              type="number"
              min={1}
              value={extendDays}
              onChange={(e) => setExtendDays(e.target.value)}
            />
          </Field>
        </div>
        <div className="mt-4 flex justify-end gap-2">
          <Button type="button" variant="secondary" onClick={() => setExtendOpen(false)}>
            Batal
          </Button>
          <Button type="button" variant="primary" loading={extendSubscription.isPending} onClick={handleExtend}>
            Perpanjang
          </Button>
        </div>
      </Dialog>

      <Dialog open={verifyTarget !== null} onClose={() => setVerifyTarget(null)} title="Verifikasi pembayaran?">
        <p className="text-[14px] text-ink">
          Tagihan <span className="num font-medium">{verifyTarget?.number}</span> akan ditandai lunas & langganan
          diaktifkan.
        </p>
        <div className="mt-4 flex justify-end gap-2">
          <Button type="button" variant="secondary" onClick={() => setVerifyTarget(null)}>
            Batal
          </Button>
          <Button type="button" variant="primary" loading={verifyInvoice.isPending} onClick={handleVerify}>
            Verifikasi
          </Button>
        </div>
      </Dialog>

      <Dialog open={voidTarget !== null} onClose={() => setVoidTarget(null)} title="Batalkan tagihan ini?">
        <p className="text-[14px] text-ink">
          Tagihan <span className="num font-medium">{voidTarget?.number}</span> akan dibatalkan & tidak bisa dibayar
          lagi.
        </p>
        <div className="mt-4 flex justify-end gap-2">
          <Button type="button" variant="secondary" onClick={() => setVoidTarget(null)}>
            Batal
          </Button>
          <Button type="button" variant="danger" loading={voidInvoice.isPending} onClick={handleVoid}>
            Void
          </Button>
        </div>
      </Dialog>
    </div>
  );
}

const CHANNEL_OPTIONS: { value: NotificationChannel; label: string; alwaysOn?: boolean }[] = [
  { value: 'in_app', label: 'In-app (selalu aktif)', alwaysOn: true },
  { value: 'web_push', label: 'Web Push' },
  { value: 'whatsapp', label: 'WhatsApp' },
  { value: 'email', label: 'Email' },
];

/**
 * Seksi "Notifikasi" — channel aktif per sekolah (docs/08-notification.md: super
 * admin yang mengatur, bukan admin sekolah, karena WA/email pakai kredensial &
 * biaya platform). `in_app` selalu aktif, tidak bisa dimatikan.
 */
function NotificationChannelsSection({ schoolId }: { schoolId: string }) {
  const { data, isLoading, isError, refetch } = useNotificationChannelSettings(schoolId);
  const update = useUpdateNotificationChannelSettings(schoolId);
  const { showToast } = useToast();
  const [channels, setChannels] = useState<NotificationChannel[]>(['in_app']);

  useEffect(() => {
    if (!data) return;
    setChannels(data.channels.includes('in_app') ? data.channels : ['in_app', ...data.channels]);
  }, [data]);

  function toggle(channel: NotificationChannel) {
    if (channel === 'in_app') return;
    setChannels((prev) => (prev.includes(channel) ? prev.filter((c) => c !== channel) : [...prev, channel]));
  }

  function handleSave() {
    update.mutate({ channels }, { onSuccess: () => showToast('Pengaturan notifikasi disimpan.') });
  }

  if (isLoading) {
    return <Skeleton className="h-44 w-full" />;
  }

  if (isError || !data) {
    return <ErrorState message="Gagal memuat pengaturan notifikasi." onRetry={() => refetch()} />;
  }

  return (
    <Card className="flex flex-col gap-4">
      <div>
        <p className="text-[14px] font-semibold text-ink">Notifikasi</p>
        <p className="text-[12px] text-muted">Channel yang aktif untuk mengirim notifikasi ke pengguna sekolah ini.</p>
      </div>

      <div className="flex flex-col gap-2">
        {CHANNEL_OPTIONS.map((opt) => (
          <Checkbox
            key={opt.value}
            id={`notif-channel-${opt.value}`}
            label={opt.label}
            checked={channels.includes(opt.value)}
            disabled={opt.alwaysOn}
            onChange={() => toggle(opt.value)}
          />
        ))}
      </div>

      {update.isError && (
        <p className="text-[12px] text-danger">
          {update.error instanceof ApiError ? update.error.message : 'Gagal menyimpan pengaturan notifikasi.'}
        </p>
      )}

      <Button variant="secondary" onClick={handleSave} loading={update.isPending} className="self-start">
        Simpan Pengaturan Notifikasi
      </Button>
    </Card>
  );
}

/**
 * Seksi "Statistik" (Fase 13 P3, docs/11 P3) — agregat kesehatan pemakaian
 * (bukan data per-siswa, lihat "Aturan desain yang mengikat" #3). Query hanya
 * jalan setelah kartu ini terlihat (`useInViewOnce`) supaya detail sekolah
 * tidak selalu menanggung fetch statistik kalau admin tidak menggulir ke sana.
 */
function StatisticsSection({ schoolId }: { schoolId: string }) {
  const [ref, inView] = useInViewOnce<HTMLDivElement>();
  const { data, isLoading, isError, refetch } = useSchoolStats(schoolId, inView);

  return (
    <div ref={ref} className="flex flex-col gap-4">
      <p className="text-[11px] font-semibold uppercase tracking-[0.1em] text-muted">Statistik</p>
      <Card className="flex flex-col gap-4">
        {!inView || isLoading ? (
          <Skeleton className="h-40 w-full" />
        ) : isError || !data ? (
          <ErrorState message="Gagal memuat statistik sekolah." onRetry={() => refetch()} />
        ) : (
          <>
            <div className="grid grid-cols-2 gap-4 sm:grid-cols-3">
              <StatTile label="Guru" value={data.teachers} />
              <StatTile label="Siswa" value={data.students} />
              <StatTile label="Rombel" value={data.classes} />
              <StatTile label="Sesi Absen 7 Hari" value={data.attendance_sessions_7d} />
              <StatTile label="Jurnal 7 Hari" value={data.journals_7d} />
              <StatTile label="Storage" value={formatBytes(data.uploads_bytes)} />
            </div>

            <div className="border-t border-line pt-3">
              <p className="mb-2 text-[12px] font-medium text-muted">Notifikasi 30 Hari</p>
              <div className="flex flex-wrap gap-4 text-[13px]">
                <span className="text-st-hadir">
                  <span className="num font-semibold">{data.notifications_30d.sent}</span> terkirim
                </span>
                <span className="text-st-terlambat">
                  <span className="num font-semibold">{data.notifications_30d.failed}</span> gagal
                </span>
                <span className="text-danger">
                  <span className="num font-semibold">{data.notifications_30d.dead}</span> dead
                </span>
              </div>
            </div>

            <div className="border-t border-line pt-3">
              <p className="mb-2 text-[12px] font-medium text-muted">Login Terakhir per Peran</p>
              {data.last_logins.length === 0 ? (
                <p className="text-[13px] text-muted">Belum ada login tercatat.</p>
              ) : (
                <div className="flex flex-col gap-1.5">
                  {data.last_logins.map((entry) => (
                    <div key={entry.role} className="flex items-center justify-between text-[13px]">
                      <span className="text-ink">{ROLE_LABEL[entry.role] ?? entry.role}</span>
                      <span className="text-muted">{formatRelativeTime(entry.at)}</span>
                    </div>
                  ))}
                </div>
              )}
            </div>
          </>
        )}
      </Card>
    </div>
  );
}

/**
 * Seksi "Anggota" (Fase 13 P4, docs/11 P4) — daftar user sekolah + aksi darurat
 * reset password (kasus paling sering: admin sekolah lupa password).
 */
function MembersSection({ schoolId }: { schoolId: string }) {
  const { data, isLoading, isError, refetch } = useSchoolMembers(schoolId);
  const [resetTarget, setResetTarget] = useState<AdminSchoolMember | null>(null);

  return (
    <div className="flex flex-col gap-4">
      <p className="text-[11px] font-semibold uppercase tracking-[0.1em] text-muted">Anggota</p>

      <Card variant="plain">
        {isLoading ? (
          <div className="flex flex-col gap-2">
            <Skeleton className="h-12 w-full" />
            <Skeleton className="h-12 w-full" />
          </div>
        ) : isError || !data ? (
          <ErrorState message="Gagal memuat anggota sekolah." onRetry={() => refetch()} />
        ) : data.length === 0 ? (
          <EmptyState icon={Users} message="Belum ada anggota terdaftar di sekolah ini." />
        ) : (
          <div>
            {data.map((member) => (
              <ListRow
                key={member.user_id}
                title={member.name}
                subtitle={
                  <>
                    {member.email ?? member.username ?? '—'} · {ROLE_LABEL[member.role] ?? member.role} ·{' '}
                    {member.last_login ? formatRelativeTime(member.last_login) : 'Belum pernah login'}
                  </>
                }
                trailing={
                  <Button variant="ghost" onClick={() => setResetTarget(member)}>
                    <KeyRound size={16} strokeWidth={2} aria-hidden="true" />
                    Reset Password
                  </Button>
                }
              />
            ))}
          </div>
        )}
      </Card>

      <ResetPasswordDialog schoolId={schoolId} member={resetTarget} onClose={() => setResetTarget(null)} />
    </div>
  );
}

/**
 * Dialog konfirmasi → reset → hasil (password sementara ditampilkan SEKALI,
 * docs/11 P4). Ditutup lewat state `member` (dikendalikan `MembersSection`)
 * supaya satu instance dialog dipakai ulang untuk anggota mana pun yang diklik.
 */
function ResetPasswordDialog({
  schoolId,
  member,
  onClose,
}: {
  schoolId: string;
  member: AdminSchoolMember | null;
  onClose: () => void;
}) {
  const resetPassword = useResetPassword();
  const { showToast } = useToast();
  const [tempPassword, setTempPassword] = useState<string | null>(null);

  function handleClose() {
    setTempPassword(null);
    resetPassword.reset();
    onClose();
  }

  function handleConfirm() {
    if (!member) return;
    resetPassword.mutate(
      { userId: member.user_id, schoolId },
      {
        onSuccess: (result) => setTempPassword(result.temp_password),
        onError: (err) => showToast(err instanceof ApiError ? err.message : 'Gagal mereset password.', 'error'),
      },
    );
  }

  async function handleCopy() {
    if (!tempPassword) return;
    try {
      await navigator.clipboard.writeText(tempPassword);
      showToast('Password disalin.');
    } catch {
      showToast('Gagal menyalin password. Salin manual.', 'error');
    }
  }

  if (tempPassword) {
    return (
      <Dialog open={member !== null} onClose={handleClose} title="Password Sementara">
        <div className="flex flex-col gap-4">
          <p className="text-[14px] text-ink">
            Password sementara untuk <span className="font-medium">{member?.name}</span>.
          </p>
          <div className="rounded-lg border border-line bg-surface-2 px-4 py-4 text-center">
            <span className="num select-all font-mono text-[21px] font-semibold tracking-[0.05em] text-ink">
              {tempPassword}
            </span>
          </div>
          <Button variant="secondary" onClick={handleCopy}>
            <Copy size={16} strokeWidth={2} aria-hidden="true" />
            Salin
          </Button>
          <p className="text-[12px] text-danger">
            Tampilkan sekali — catat sekarang. Semua sesi user ini keluar otomatis.
          </p>
        </div>
        <div className="mt-4 flex justify-end">
          <Button type="button" variant="secondary" onClick={handleClose}>
            Tutup
          </Button>
        </div>
      </Dialog>
    );
  }

  return (
    <Dialog open={member !== null} onClose={handleClose} title="Reset password?">
      <p className="text-[14px] text-ink">
        Password <span className="font-medium">{member?.name}</span> akan diganti dengan password sementara & semua
        sesi user ini keluar otomatis.
      </p>
      <div className="mt-4 flex justify-end gap-2">
        <Button type="button" variant="secondary" onClick={handleClose}>
          Batal
        </Button>
        <Button type="button" variant="danger" loading={resetPassword.isPending} onClick={handleConfirm}>
          Reset Password
        </Button>
      </div>
    </Dialog>
  );
}

const AUDIT_PER_PAGE = 20;

/**
 * Seksi "Audit Log" (Fase 13 P4, docs/11 P4) — 20 terbaru + filter prefix
 * action + "Muat lebih banyak" (bukan tabel kompleks, sengaja tetap ListRow).
 * Reset filter action → halaman & akumulasi item ikut direset ke awal.
 */
function AuditLogSection({ schoolId }: { schoolId: string }) {
  const [actionInput, setActionInput] = useState('');
  const [actionFilter, setActionFilter] = useState('');
  const [page, setPage] = useState(1);
  const [items, setItems] = useState<AdminAuditLogItem[]>([]);

  const { data, isLoading, isError, refetch } = useSchoolAudit(schoolId, {
    page,
    perPage: AUDIT_PER_PAGE,
    action: actionFilter || undefined,
  });

  useEffect(() => {
    if (!data) return;
    setItems((prev) => (page === 1 ? data.items : [...prev, ...data.items]));
  }, [data, page]);

  function handleFilterSubmit(e: FormEvent) {
    e.preventDefault();
    setPage(1);
    setItems([]);
    setActionFilter(actionInput.trim());
  }

  const hasMore = data ? items.length < data.total : false;

  return (
    <div className="flex flex-col gap-4">
      <p className="text-[11px] font-semibold uppercase tracking-[0.1em] text-muted">Audit Log</p>

      <form onSubmit={handleFilterSubmit} className="flex flex-wrap items-end gap-2">
        <Field label="Filter action (awalan)" htmlFor="audit-action-filter" className="min-w-[200px] flex-1">
          <Input
            id="audit-action-filter"
            value={actionInput}
            onChange={(e) => setActionInput(e.target.value)}
            placeholder="mis. billing."
          />
        </Field>
        <Button type="submit" variant="secondary">
          <Search size={16} strokeWidth={2} aria-hidden="true" />
          Cari
        </Button>
      </form>

      <Card variant="plain">
        {isLoading && page === 1 ? (
          <div className="flex flex-col gap-2">
            <Skeleton className="h-10 w-full" />
            <Skeleton className="h-10 w-full" />
            <Skeleton className="h-10 w-full" />
          </div>
        ) : isError ? (
          <ErrorState message="Gagal memuat audit log." onRetry={() => refetch()} />
        ) : items.length === 0 ? (
          <EmptyState icon={History} message="Belum ada aktivitas tercatat." />
        ) : (
          <div>
            {items.map((item) => (
              <ListRow
                key={item.id}
                title={<span className="font-mono text-[13px]">{item.action}</span>}
                subtitle={`${item.user_name ?? 'Sistem'} · ${formatRelativeTime(item.at)}`}
              />
            ))}
          </div>
        )}
      </Card>

      {hasMore && (
        <Button
          variant="secondary"
          loading={isLoading && page > 1}
          onClick={() => setPage((p) => p + 1)}
          className="self-center"
        >
          Muat lebih banyak
        </Button>
      )}
    </div>
  );
}
