import { Link, useNavigate } from 'react-router-dom';
import { Building2, ChevronLeft, Mail, Plus, ShieldAlert, Tags } from 'lucide-react';
import { ListRow } from '../../components/ui/ListRow';
import { DataTable, type DataTableColumn } from '../../components/ui/DataTable';
import { Skeleton } from '../../components/ui/Skeleton';
import { EmptyState } from '../../components/ui/EmptyState';
import { ErrorState } from '../../components/ui/ErrorState';
import { Button } from '../../components/ui/Button';
import { Tag } from '../../components/ui/Tag';
import { PAGE_WIDE } from '../../components/ui/page';
import { useSchools } from './api';
import { useMe } from '../auth/api';
import type { School } from '../../lib/types';

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

  /** Kolom desktop (docs/10 §5) — tanpa kolom Langganan: `GET /api/admin/schools` tidak mengirim status langganan per sekolah (butuh fetch per-sekolah `useSchoolBilling`, tidak dipanggil massal di sini). */
  const columns: DataTableColumn<School>[] = [
    { key: 'name', header: 'Nama', sortable: true, sortValue: (s) => s.name, cell: (s) => <span className="font-medium text-ink">{s.name}</span> },
    { key: 'slug', header: 'Slug', sortable: true, sortValue: (s) => s.slug, cell: (s) => s.slug },
    { key: 'domain', header: 'Domain Custom', cell: (s) => s.custom_domain ?? <span className="text-muted">—</span> },
    {
      key: 'status',
      header: 'Status',
      cell: (s) => <Tag variant={s.status === 'active' ? 'now' : 'danger'}>{s.status === 'active' ? 'Aktif' : 'Nonaktif'}</Tag>,
    },
  ];

  return (
    <div className={`${PAGE_WIDE} flex flex-col gap-6`}>
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
        <>
          <div className="lg:hidden">
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
          <div className="hidden lg:block">
            <DataTable
              columns={columns}
              data={schools ?? []}
              keyField={(s) => s.id}
              onRowClick={(s) => navigate(`/admin/sekolah/${s.id}`)}
              emptyIcon={Building2}
              emptyMessage="Belum ada sekolah terdaftar."
              actions={(s) => [{ label: 'Lihat Detail', onClick: () => navigate(`/admin/sekolah/${s.id}`) }]}
            />
          </div>
        </>
      )}
    </div>
  );
}
