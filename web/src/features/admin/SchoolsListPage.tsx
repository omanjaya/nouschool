import { Link, useNavigate } from 'react-router-dom';
import { Building2, ChevronLeft, Mail, Plus, ShieldAlert, Tags } from 'lucide-react';
import { ListRow } from '../../components/ui/ListRow';
import { Skeleton } from '../../components/ui/Skeleton';
import { EmptyState } from '../../components/ui/EmptyState';
import { ErrorState } from '../../components/ui/ErrorState';
import { Button } from '../../components/ui/Button';
import { Tag } from '../../components/ui/Tag';
import { useSchools } from './api';
import { useMe } from '../auth/api';

export function SchoolsListPage() {
  const { data: me } = useMe();
  const { data: schools, isLoading, isError, refetch } = useSchools();
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
      <div className="flex items-start justify-between gap-3">
        <div>
          <Link
            to="/admin"
            className="mb-3 inline-flex items-center gap-1 text-[12px] font-medium text-muted hover:text-ink"
          >
            <ChevronLeft size={16} strokeWidth={2} aria-hidden="true" />
            Beranda
          </Link>
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
          <Button onClick={() => navigate('/admin/sekolah/new')}>
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
              onClick={() => navigate(`/admin/sekolah/${school.id}`)}
            />
          ))}
        </div>
      )}
    </div>
  );
}
