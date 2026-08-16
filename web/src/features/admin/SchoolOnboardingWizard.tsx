import { useEffect, useState, type FormEvent } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { Check, Copy, ShieldAlert } from 'lucide-react';
import { Card } from '../../components/ui/Card';
import { Skeleton } from '../../components/ui/Skeleton';
import { EmptyState } from '../../components/ui/EmptyState';
import { ErrorState } from '../../components/ui/ErrorState';
import { Button } from '../../components/ui/Button';
import { Field, Input, Select } from '../../components/ui/Field';
import { useToast } from '../../components/ui/Toast';
import { useMe } from '../auth/api';
import {
  useCreateAcademicYear,
  useActivateAcademicYear,
  useCreateSchool,
  useCreateSchoolAdmin,
  useCreateSubscription,
  usePlans,
  useSchoolBilling,
} from './api';
import { TIMEZONES } from '../../lib/timezones';
import { slugify } from '../../lib/slugify';
import { formatRupiah } from '../../lib/currency';
import { ApiError } from '../../lib/api';
import { INVOICE_STATUS_LABEL } from '../billing/format';
import type { AcademicYear, Plan, School } from '../../lib/types';

const SLUG_PATTERN = /^[a-z0-9-]+$/;

interface AdminCredential {
  name: string;
  identifier: string;
  temp_password: string;
}

const STEPS = [
  { n: 1, label: 'Sekolah' },
  { n: 2, label: 'Tahun Ajaran' },
  { n: 3, label: 'Admin' },
  { n: 4, label: 'Plan' },
  { n: 5, label: 'Ringkasan' },
] as const;

/** "2026/2027" — tahun ajaran Indonesia dimulai Juli, dipakai default nama langkah 2. */
function defaultAcademicYearName(): string {
  const now = new Date();
  const year = now.getFullYear();
  const startYear = now.getMonth() + 1 >= 7 ? year : year - 1;
  return `${startYear}/${startYear + 1}`;
}

/** Indikator langkah 1–5, hairline (docs/10 — tanpa shadow dekoratif). */
function StepIndicator({ current }: { current: number }) {
  return (
    <div className="flex items-start">
      {STEPS.map((s, i) => (
        <div key={s.n} className="flex flex-1 items-center">
          <div className="flex flex-col items-center gap-1.5">
            <span
              className={`flex h-7 w-7 shrink-0 items-center justify-center rounded-full border text-[12px] font-semibold ${
                s.n < current
                  ? 'border-primary bg-primary text-primary-ink'
                  : s.n === current
                    ? 'border-primary text-primary'
                    : 'border-line text-muted'
              }`}
            >
              {s.n < current ? <Check size={14} strokeWidth={2} aria-hidden="true" /> : s.n}
            </span>
            <span
              className={`hidden text-center text-[11px] sm:block ${s.n === current ? 'font-medium text-ink' : 'text-muted'}`}
            >
              {s.label}
            </span>
          </div>
          {i < STEPS.length - 1 && <div className="mt-3.5 h-px flex-1 bg-line" />}
        </div>
      ))}
    </div>
  );
}

/** Langkah 1 — Data sekolah (nama, slug auto+editable, timezone) → POST /api/admin/schools. */
function StepSchool({ onCreated }: { onCreated: (school: School) => void }) {
  const [name, setName] = useState('');
  const [slug, setSlug] = useState('');
  const [slugEdited, setSlugEdited] = useState(false);
  const [timezone, setTimezone] = useState<string>(TIMEZONES[0].value);
  const [slugError, setSlugError] = useState<string | undefined>();
  const createSchool = useCreateSchool();

  function handleNameChange(value: string) {
    setName(value);
    if (!slugEdited) setSlug(slugify(value));
  }

  function handleSubmit(e: FormEvent) {
    e.preventDefault();
    if (!SLUG_PATTERN.test(slug)) {
      setSlugError('Slug hanya boleh huruf kecil, angka, dan tanda hubung.');
      return;
    }
    setSlugError(undefined);
    createSchool.mutate({ name, slug, timezone }, { onSuccess: (school) => onCreated(school) });
  }

  return (
    <Card>
      <form onSubmit={handleSubmit} className="flex flex-col gap-4">
        <p className="text-[14px] font-semibold text-ink">1. Data Sekolah</p>
        <Field label="Nama sekolah" htmlFor="wiz-name">
          <Input id="wiz-name" value={name} onChange={(e) => handleNameChange(e.target.value)} required />
        </Field>
        <Field
          label="Slug"
          htmlFor="wiz-slug"
          error={slugError}
          hint={!slugError ? `Alamat: ${slug || '...'}.nouschool.id` : undefined}
        >
          <Input
            id="wiz-slug"
            value={slug}
            onChange={(e) => {
              setSlugEdited(true);
              setSlug(e.target.value);
            }}
            required
          />
        </Field>
        <Field label="Zona waktu" htmlFor="wiz-timezone">
          <Select id="wiz-timezone" value={timezone} onChange={(e) => setTimezone(e.target.value)}>
            {TIMEZONES.map((tz) => (
              <option key={tz.value} value={tz.value}>
                {tz.label}
              </option>
            ))}
          </Select>
        </Field>

        {createSchool.isError && (
          <p className="text-[12px] text-danger">
            {createSchool.error instanceof ApiError ? createSchool.error.message : 'Gagal menambahkan sekolah.'}
          </p>
        )}

        <Button type="submit" loading={createSchool.isPending} className="self-start">
          Lanjut
        </Button>
      </form>
    </Card>
  );
}

/**
 * Langkah 2 — Tahun ajaran pertama → POST academic-years lalu activate. Dua mutasi berurutan;
 * kalau create sukses tapi activate gagal, tawarkan "coba lagi" HANYA untuk activate (tidak
 * membuat tahun ajaran duplikat).
 */
function StepAcademicYear({ schoolId, onDone }: { schoolId: string; onDone: () => void }) {
  const [name, setName] = useState(defaultAcademicYearName());
  const [startsOn, setStartsOn] = useState('');
  const [endsOn, setEndsOn] = useState('');
  const [createdYear, setCreatedYear] = useState<AcademicYear | null>(null);
  const createYear = useCreateAcademicYear(schoolId);
  const activateYear = useActivateAcademicYear(schoolId);

  function handleActivate(yearId: string) {
    activateYear.mutate(yearId, { onSuccess: () => onDone() });
  }

  function handleCreate(e: FormEvent) {
    e.preventDefault();
    createYear.mutate(
      { name, starts_on: startsOn, ends_on: endsOn },
      {
        onSuccess: (year) => {
          setCreatedYear(year);
          handleActivate(year.id);
        },
      },
    );
  }

  if (createdYear) {
    return (
      <Card className="flex flex-col gap-4">
        <p className="text-[14px] font-semibold text-ink">2. Tahun Ajaran Pertama</p>
        <p className="text-[14px] text-ink">
          Tahun ajaran <span className="font-medium">{createdYear.name}</span> dibuat, sedang diaktifkan…
        </p>
        {activateYear.isError && (
          <>
            <p className="text-[12px] text-danger">
              {activateYear.error instanceof ApiError ? activateYear.error.message : 'Gagal mengaktifkan tahun ajaran.'}
            </p>
            <Button
              variant="secondary"
              loading={activateYear.isPending}
              onClick={() => handleActivate(createdYear.id)}
              className="self-start"
            >
              Coba Aktifkan Lagi
            </Button>
          </>
        )}
      </Card>
    );
  }

  return (
    <Card>
      <form onSubmit={handleCreate} className="flex flex-col gap-4">
        <p className="text-[14px] font-semibold text-ink">2. Tahun Ajaran Pertama</p>
        <Field label="Nama" htmlFor="wiz-ay-name">
          <Input id="wiz-ay-name" value={name} onChange={(e) => setName(e.target.value)} required />
        </Field>
        <div className="grid grid-cols-2 gap-3">
          <Field label="Mulai" htmlFor="wiz-ay-start">
            <Input
              id="wiz-ay-start"
              type="date"
              value={startsOn}
              onChange={(e) => setStartsOn(e.target.value)}
              required
            />
          </Field>
          <Field label="Selesai" htmlFor="wiz-ay-end">
            <Input id="wiz-ay-end" type="date" value={endsOn} onChange={(e) => setEndsOn(e.target.value)} required />
          </Field>
        </div>

        {createYear.isError && (
          <p className="text-[12px] text-danger">
            {createYear.error instanceof ApiError ? createYear.error.message : 'Gagal menambahkan tahun ajaran.'}
          </p>
        )}

        <Button type="submit" loading={createYear.isPending || activateYear.isPending} className="self-start">
          Lanjut
        </Button>
      </form>
    </Card>
  );
}

/**
 * Langkah 3 — Akun admin sekolah → POST admins. Password sementara ditampilkan sekali
 * (pola `ResetPasswordDialog` di `SchoolDetailPage`: monospace besar + salin + peringatan).
 */
function StepAdmin({ schoolId, onDone }: { schoolId: string; onDone: (result: AdminCredential) => void }) {
  const [name, setName] = useState('');
  const [email, setEmail] = useState('');
  const [username, setUsername] = useState('');
  const [identifierError, setIdentifierError] = useState<string | undefined>();
  const [tempPassword, setTempPassword] = useState<string | null>(null);
  const createAdmin = useCreateSchoolAdmin(schoolId);
  const { showToast } = useToast();

  function handleSubmit(e: FormEvent) {
    e.preventDefault();
    if (!email.trim() && !username.trim()) {
      setIdentifierError('Isi email atau username.');
      return;
    }
    setIdentifierError(undefined);
    createAdmin.mutate(
      { name, email: email.trim() || undefined, username: username.trim() || undefined },
      { onSuccess: (result) => setTempPassword(result.temp_password) },
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
    const identifier = email.trim() || username.trim();
    return (
      <Card className="flex flex-col gap-4">
        <p className="text-[14px] font-semibold text-ink">3. Akun Admin Sekolah</p>
        <p className="text-[14px] text-ink">
          Akun untuk <span className="font-medium">{name}</span> ({identifier}) dibuat.
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
        <p className="text-[12px] text-danger">Tampilkan sekali — catat sekarang.</p>
        <Button onClick={() => onDone({ name, identifier, temp_password: tempPassword })} className="self-start">
          Lanjut
        </Button>
      </Card>
    );
  }

  return (
    <Card>
      <form onSubmit={handleSubmit} className="flex flex-col gap-4">
        <p className="text-[14px] font-semibold text-ink">3. Akun Admin Sekolah</p>
        <Field label="Nama" htmlFor="wiz-admin-name">
          <Input id="wiz-admin-name" value={name} onChange={(e) => setName(e.target.value)} required />
        </Field>
        <Field label="Email (opsional)" htmlFor="wiz-admin-email">
          <Input id="wiz-admin-email" type="email" value={email} onChange={(e) => setEmail(e.target.value)} />
        </Field>
        <Field label="Username (opsional)" htmlFor="wiz-admin-username">
          <Input id="wiz-admin-username" value={username} onChange={(e) => setUsername(e.target.value)} />
        </Field>
        {identifierError && <p className="text-[12px] text-danger">{identifierError}</p>}

        {createAdmin.isError && (
          <p className="text-[12px] text-danger">
            {createAdmin.error instanceof ApiError ? createAdmin.error.message : 'Gagal membuat akun admin.'}
          </p>
        )}

        <Button type="submit" loading={createAdmin.isPending} className="self-start">
          Buat Akun
        </Button>
      </form>
    </Card>
  );
}

/** Langkah 4 — Pilih plan (kartu) → POST subscriptions; invoice pertama dibuat di server. */
function StepPlan({ schoolId, onDone }: { schoolId: string; onDone: (plan: Plan) => void }) {
  const { data: plans, isLoading, isError, refetch } = usePlans();
  const [selected, setSelected] = useState('');
  const createSubscription = useCreateSubscription(schoolId);
  const { showToast } = useToast();

  useEffect(() => {
    if (plans && plans.length > 0 && !selected) setSelected(plans[0].code);
  }, [plans, selected]);

  function handleSubmit() {
    const plan = plans?.find((p) => p.code === selected);
    if (!plan) return;
    createSubscription.mutate(selected, {
      onSuccess: () => {
        showToast('Langganan dibuat.');
        onDone(plan);
      },
      onError: (err) => showToast(err instanceof ApiError ? err.message : 'Gagal membuat langganan.', 'error'),
    });
  }

  if (isLoading) {
    return <Skeleton className="h-40 w-full" />;
  }
  if (isError || !plans) {
    return <ErrorState message="Gagal memuat plan." onRetry={() => refetch()} />;
  }
  if (plans.length === 0) {
    return <EmptyState message="Belum ada plan tersedia. Tambahkan plan di /admin/plans terlebih dahulu." />;
  }

  return (
    <Card className="flex flex-col gap-4">
      <p className="text-[14px] font-semibold text-ink">4. Pilih Plan</p>
      <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
        {plans.map((plan) => (
          <button
            key={plan.code}
            type="button"
            onClick={() => setSelected(plan.code)}
            className={`flex flex-col gap-1 rounded-lg border p-3 text-left transition-colors duration-150 ${
              selected === plan.code ? 'border-primary bg-primary-soft' : 'border-line hover:bg-surface-2'
            }`}
          >
            <span className="text-[14px] font-semibold text-ink">{plan.name}</span>
            <span className="num text-[12px] text-muted">
              {plan.prices.map((p) => formatRupiah(p.price_yearly)).join(' · ')}
            </span>
          </button>
        ))}
      </div>

      {createSubscription.isError && (
        <p className="text-[12px] text-danger">
          {createSubscription.error instanceof ApiError ? createSubscription.error.message : 'Gagal membuat langganan.'}
        </p>
      )}

      <Button onClick={handleSubmit} loading={createSubscription.isPending} className="self-start">
        Buat Langganan
      </Button>
    </Card>
  );
}

/**
 * Langkah 5 — Ringkasan siap-salin: URL sekolah (pola `ImpersonateButton`), kredensial admin,
 * status invoice terbaru, tombol "Salin Semua" (teks multiline) + tautan detail sekolah.
 */
function StepSummary({ school, admin, plan }: { school: School; admin: AdminCredential | null; plan: Plan | null }) {
  const { data: billing } = useSchoolBilling(school.id);
  const { showToast } = useToast();
  const navigate = useNavigate();

  const baseHost = window.location.host.replace(/^admin\./, '');
  const schoolUrl = `${window.location.protocol}//${school.slug}.${baseHost}`;
  const latestInvoice = billing?.invoices[0];

  async function handleCopyAll() {
    const lines = [
      `Sekolah: ${school.name}`,
      `URL: ${schoolUrl}`,
      admin ? `Admin: ${admin.name}` : null,
      admin ? `Login: ${admin.identifier}` : null,
      admin ? `Password sementara: ${admin.temp_password}` : null,
      plan ? `Plan: ${plan.name}` : null,
      latestInvoice ? `Invoice: ${latestInvoice.number} — ${INVOICE_STATUS_LABEL[latestInvoice.status]}` : null,
    ]
      .filter((line): line is string => line !== null)
      .join('\n');

    try {
      await navigator.clipboard.writeText(lines);
      showToast('Ringkasan disalin.');
    } catch {
      showToast('Gagal menyalin. Salin manual.', 'error');
    }
  }

  return (
    <Card className="flex flex-col gap-4">
      <p className="text-[14px] font-semibold text-ink">5. Ringkasan</p>

      <div className="flex flex-col gap-2 text-[14px] text-ink">
        <p>
          <span className="text-muted">URL sekolah: </span>
          <span className="font-medium">{schoolUrl}</span>
        </p>
        {admin && (
          <>
            <p>
              <span className="text-muted">Admin: </span>
              {admin.name}
            </p>
            <p>
              <span className="text-muted">Login: </span>
              {admin.identifier}
            </p>
            <p className="num">
              <span className="text-muted">Password sementara: </span>
              <span className="font-mono">{admin.temp_password}</span>
            </p>
          </>
        )}
        {plan && (
          <p>
            <span className="text-muted">Plan: </span>
            {plan.name}
          </p>
        )}
        {latestInvoice && (
          <p>
            <span className="text-muted">Invoice: </span>
            {latestInvoice.number} — {INVOICE_STATUS_LABEL[latestInvoice.status]}
          </p>
        )}
      </div>

      <p className="text-[12px] text-danger">Password sementara hanya ditampilkan di sini — catat sekarang.</p>

      <div className="flex flex-wrap gap-2">
        <Button onClick={handleCopyAll}>
          <Copy size={16} strokeWidth={2} aria-hidden="true" />
          Salin Semua
        </Button>
        <Button variant="secondary" onClick={() => navigate(`/admin/sekolah/${school.id}`)}>
          Lihat Detail Sekolah
        </Button>
      </div>
    </Card>
  );
}

/**
 * `/admin/sekolah/new` — wizard onboarding sekolah baru (Fase 13 Gelombang 2 P2, docs/11 P2),
 * menggantikan dialog "Tambah Sekolah" lama. Tiap langkah bisa gagal independen tanpa
 * mereset progres — state (school/admin/plan) disimpan di komponen ini, langkah lanjut
 * hanya jadi aktif setelah langkah sebelumnya sukses.
 */
export function SchoolOnboardingWizard() {
  const { data: me } = useMe();
  const [step, setStep] = useState(1);
  const [school, setSchool] = useState<School | null>(null);
  const [admin, setAdmin] = useState<AdminCredential | null>(null);
  const [plan, setPlan] = useState<Plan | null>(null);

  if (me && !me.is_super_admin) {
    return (
      <div className="mx-auto max-w-[640px] px-5 py-6">
        <EmptyState icon={ShieldAlert} message="Anda tidak memiliki akses ke halaman ini." />
      </div>
    );
  }

  return (
    <div className="mx-auto flex max-w-[640px] flex-col gap-6 px-5 py-6 lg:max-w-[720px]">
      <div>
        <Link
          to="/admin/sekolah"
          className="mb-3 inline-flex items-center gap-1 text-[12px] font-medium text-muted hover:text-ink"
        >
          Sekolah
        </Link>
        <p className="text-[11px] font-semibold uppercase tracking-[0.1em] text-muted">Platform</p>
        <h1 className="text-[21px] font-semibold text-ink">Tambah Sekolah</h1>
      </div>

      <StepIndicator current={step} />

      {step === 1 && (
        <StepSchool
          onCreated={(s) => {
            setSchool(s);
            setStep(2);
          }}
        />
      )}
      {step === 2 && school && <StepAcademicYear schoolId={school.id} onDone={() => setStep(3)} />}
      {step === 3 && school && (
        <StepAdmin
          schoolId={school.id}
          onDone={(result) => {
            setAdmin(result);
            setStep(4);
          }}
        />
      )}
      {step === 4 && school && (
        <StepPlan
          schoolId={school.id}
          onDone={(p) => {
            setPlan(p);
            setStep(5);
          }}
        />
      )}
      {step === 5 && school && <StepSummary school={school} admin={admin} plan={plan} />}
    </div>
  );
}
