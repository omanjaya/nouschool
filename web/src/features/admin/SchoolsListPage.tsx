import { useState, type FormEvent } from 'react';
import { useNavigate } from 'react-router-dom';
import { Building2, Mail, Plus, ShieldAlert, Tags } from 'lucide-react';
import { ListRow } from '../../components/ui/ListRow';
import { Skeleton } from '../../components/ui/Skeleton';
import { EmptyState } from '../../components/ui/EmptyState';
import { ErrorState } from '../../components/ui/ErrorState';
import { Button } from '../../components/ui/Button';
import { Tag } from '../../components/ui/Tag';
import { Dialog } from '../../components/ui/Dialog';
import { Field, Input, Select } from '../../components/ui/Field';
import { useToast } from '../../components/ui/Toast';
import { useCreateSchool, useSchools } from './api';
import { useMe } from '../auth/api';
import { TIMEZONES } from '../../lib/timezones';
import { slugify } from '../../lib/slugify';
import { ApiError } from '../../lib/api';

const SLUG_PATTERN = /^[a-z0-9-]+$/;

export function SchoolsListPage() {
  const { data: me } = useMe();
  const { data: schools, isLoading, isError, refetch } = useSchools();
  const [dialogOpen, setDialogOpen] = useState(false);
  const navigate = useNavigate();

  if (me && !me.is_super_admin) {
    return (
      <div className="mx-auto max-w-[640px] px-5 py-6">
        <EmptyState icon={ShieldAlert} message="Anda tidak memiliki akses ke halaman ini." />
      </div>
    );
  }

  return (
    <div className="mx-auto flex max-w-[640px] flex-col gap-6 px-5 py-6 lg:max-w-[1120px]">
      <div className="flex items-center justify-between gap-3">
        <div>
          <p className="text-[11px] font-semibold uppercase tracking-[0.1em] text-muted">Platform</p>
          <h1 className="text-[21px] font-semibold text-ink">Sekolah</h1>
        </div>
        <div className="flex flex-wrap gap-2">
          <Button variant="secondary" onClick={() => navigate('/admin/minat')}>
            <Mail size={16} strokeWidth={2} aria-hidden="true" />
            Minat Sekolah
          </Button>
          <Button variant="secondary" onClick={() => navigate('/admin/plans')}>
            <Tags size={16} strokeWidth={2} aria-hidden="true" />
            Plan & Harga
          </Button>
          <Button onClick={() => setDialogOpen(true)}>
            <Plus size={16} strokeWidth={2} aria-hidden="true" />
            Tambah Sekolah
          </Button>
        </div>
      </div>

      {isLoading ? (
        <div className="flex flex-col gap-2">
          <Skeleton className="h-12 w-full" />
          <Skeleton className="h-12 w-full" />
          <Skeleton className="h-12 w-full" />
        </div>
      ) : isError ? (
        <ErrorState message="Gagal memuat daftar sekolah." onRetry={() => refetch()} />
      ) : schools && schools.length === 0 ? (
        <EmptyState icon={Building2} message="Belum ada sekolah terdaftar." />
      ) : (
        <div>
          {schools?.map((school) => (
            <ListRow
              key={school.id}
              title={school.name}
              subtitle={school.custom_domain ? `${school.slug} · ${school.custom_domain}` : school.slug}
              trailing={
                <Tag variant={school.status === 'active' ? 'now' : 'danger'}>
                  {school.status === 'active' ? 'Aktif' : 'Nonaktif'}
                </Tag>
              }
              onClick={() => navigate(`/admin/schools/${school.id}`)}
            />
          ))}
        </div>
      )}

      <CreateSchoolDialog open={dialogOpen} onClose={() => setDialogOpen(false)} />
    </div>
  );
}

function CreateSchoolDialog({ open, onClose }: { open: boolean; onClose: () => void }) {
  const [name, setName] = useState('');
  const [slug, setSlug] = useState('');
  const [slugEdited, setSlugEdited] = useState(false);
  const [timezone, setTimezone] = useState<string>(TIMEZONES[0].value);
  const [slugError, setSlugError] = useState<string | undefined>();
  const createSchool = useCreateSchool();
  const { showToast } = useToast();

  function reset() {
    setName('');
    setSlug('');
    setSlugEdited(false);
    setTimezone(TIMEZONES[0].value);
    setSlugError(undefined);
    createSchool.reset();
  }

  function handleClose() {
    reset();
    onClose();
  }

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
    createSchool.mutate(
      { name, slug, timezone },
      {
        onSuccess: () => {
          showToast('Sekolah ditambahkan.');
          handleClose();
        },
      },
    );
  }

  return (
    <Dialog open={open} onClose={handleClose} title="Tambah Sekolah">
      <form onSubmit={handleSubmit} className="flex flex-col gap-4">
        <Field label="Nama sekolah" htmlFor="school-name">
          <Input id="school-name" value={name} onChange={(e) => handleNameChange(e.target.value)} required />
        </Field>
        <Field
          label="Slug"
          htmlFor="school-slug"
          error={slugError}
          hint={!slugError ? `Alamat: ${slug || '...'}.nouschool.id` : undefined}
        >
          <Input
            id="school-slug"
            value={slug}
            onChange={(e) => {
              setSlugEdited(true);
              setSlug(e.target.value);
            }}
            required
          />
        </Field>
        <Field label="Zona waktu" htmlFor="school-timezone">
          <Select id="school-timezone" value={timezone} onChange={(e) => setTimezone(e.target.value)}>
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

        <div className="flex justify-end gap-2 pt-2">
          <Button type="button" variant="secondary" onClick={handleClose}>
            Batal
          </Button>
          <Button type="submit" loading={createSchool.isPending}>
            Simpan
          </Button>
        </div>
      </form>
    </Dialog>
  );
}
