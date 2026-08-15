import { useMemo, useState, type FormEvent } from 'react';
import { Link, useParams } from 'react-router-dom';
import { Building2, CalendarRange, ChevronLeft } from 'lucide-react';
import { Card } from '../../components/ui/Card';
import { Skeleton } from '../../components/ui/Skeleton';
import { EmptyState } from '../../components/ui/EmptyState';
import { ErrorState } from '../../components/ui/ErrorState';
import { ListRow } from '../../components/ui/ListRow';
import { Tag } from '../../components/ui/Tag';
import { Dialog } from '../../components/ui/Dialog';
import { Button } from '../../components/ui/Button';
import { Field, Input, Select } from '../../components/ui/Field';
import { useToast } from '../../components/ui/Toast';
import {
  useAcademicYears,
  useActivateAcademicYear,
  useCreateAcademicYear,
  useSchools,
  useUpdateSchool,
} from './api';
import { TIMEZONES } from '../../lib/timezones';
import { formatDate } from '../../lib/date';
import { ApiError } from '../../lib/api';
import type { School } from '../../lib/types';

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
          to="/admin"
          className="mb-3 inline-flex items-center gap-1 text-[12px] font-medium text-muted hover:text-ink"
        >
          <ChevronLeft size={16} strokeWidth={2} aria-hidden="true" />
          Sekolah
        </Link>
        <h1 className="text-[21px] font-semibold text-ink">{school.name}</h1>
        <p className="text-[12px] text-muted">{school.slug}</p>
      </div>

      <SchoolEditForm school={school} />
      <AcademicYearsSection schoolId={school.id} />
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
