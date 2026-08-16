import { useMemo } from 'react';
import { useNavigate } from 'react-router-dom';
import { ShieldAlert } from 'lucide-react';
import { StatTile } from '../../components/ui/StatTile';
import { DataTable, type DataTableColumn } from '../../components/ui/DataTable';
import { Tag } from '../../components/ui/Tag';
import { Skeleton } from '../../components/ui/Skeleton';
import { EmptyState } from '../../components/ui/EmptyState';
import { ErrorState } from '../../components/ui/ErrorState';
import { PAGE_WIDE } from '../../components/ui/page';
import { formatDate, formatRelativeTime } from '../../lib/date';
import { formatRupiah } from '../../lib/currency';
import { useMe } from '../auth/api';
import { useAdminOverview } from './api';
import type { AdminLastActivity, AdminOverview } from '../../lib/types';

interface AttentionRow {
  key: string;
  jenis: string;
  sekolah: string;
  detail: string;
  schoolId: string;
  target: 'school' | 'outbox';
}

/** /admin — beranda platform super admin (Fase 13 P1, padatkan Fase 16). */
export function DashboardAdminPage() {
  const { data: me } = useMe();
  const { data, isLoading, isError, refetch } = useAdminOverview();
  const navigate = useNavigate();

  if (me && !me.is_super_admin) {
    return (
      <div className="mx-auto max-w-[640px] px-5 py-6">
        <EmptyState icon={ShieldAlert} message="Anda tidak memiliki akses ke halaman ini." />
      </div>
    );
  }

  return (
    <div className={`${PAGE_WIDE} flex flex-col gap-6`}>
      <div>
        <p className="text-[11px] font-semibold uppercase tracking-[0.1em] text-muted">Platform</p>
        <h1 className="text-[21px] font-semibold text-ink">Beranda</h1>
      </div>

      {isLoading ? (
        <div className="flex flex-col gap-4">
          <Skeleton className="h-24 w-full" />
          <Skeleton className="h-40 w-full" />
          <Skeleton className="h-40 w-full" />
        </div>
      ) : isError || !data ? (
        <ErrorState message="Gagal memuat ringkasan platform." onRetry={() => refetch()} />
      ) : (
        <>
          <div className="grid grid-cols-2 gap-4 sm:grid-cols-3 [grid-template-columns:repeat(auto-fit,minmax(150px,1fr))]">
            <StatTile label="Sekolah Aktif" value={data.stats.schools_active} variant="success" />
            <StatTile label="Grace" value={data.stats.schools_grace} variant="warning" />
            <StatTile label="Readonly" value={data.stats.schools_readonly} variant="danger" />
            <StatTile label="Total Siswa" value={data.stats.total_students} />
            <StatTile label="Pendapatan Tahun Ini" value={formatRupiah(data.stats.revenue_year)} />
            <StatTile label="Leads 7 Hari" value={data.stats.leads_7d} />
          </div>

          <AttentionTable
            overview={data}
            onSchoolClick={(schoolId) => navigate(`/admin/sekolah/${schoolId}`)}
            onOutboxClick={(schoolId) => navigate(`/admin/outbox?school_id=${schoolId}`)}
          />

          <LastActivityTable overview={data} onSchoolClick={(schoolId) => navigate(`/admin/sekolah/${schoolId}`)} />
        </>
      )}
    </div>
  );
}

/** "Perlu Perhatian" dipadatkan jadi satu DataTable gabungan (Fase 16) — kelima kategori digabung satu baris per item, bukan lima Card bertumpuk. */
function AttentionTable({
  overview,
  onSchoolClick,
  onOutboxClick,
}: {
  overview: AdminOverview;
  onSchoolClick: (schoolId: string) => void;
  onOutboxClick: (schoolId: string) => void;
}) {
  const { invoices_awaiting, schools_grace, schools_readonly, schools_no_active_year, outbox_dead } =
    overview.attention;

  const rows: AttentionRow[] = useMemo(() => {
    const result: AttentionRow[] = [];
    invoices_awaiting.forEach((i) =>
      result.push({
        key: `invoice-${i.invoice_id}`,
        jenis: 'Invoice Menunggu',
        sekolah: i.school_name,
        detail: `${i.number} · ${formatRupiah(i.amount)}`,
        schoolId: i.school_id,
        target: 'school',
      }),
    );
    schools_grace.forEach((s) =>
      result.push({
        key: `grace-${s.school_id}`,
        jenis: 'Grace',
        sekolah: s.name,
        detail: `Berakhir ${formatDate(s.ends_on)} · grace sampai ${formatDate(s.grace_until)}`,
        schoolId: s.school_id,
        target: 'school',
      }),
    );
    schools_readonly.forEach((s) =>
      result.push({
        key: `readonly-${s.school_id}`,
        jenis: 'Readonly',
        sekolah: s.name,
        detail: 'Akses sekolah dibatasi hanya-baca.',
        schoolId: s.school_id,
        target: 'school',
      }),
    );
    schools_no_active_year.forEach((s) =>
      result.push({
        key: `no-year-${s.school_id}`,
        jenis: 'Tanpa Tahun Ajaran',
        sekolah: s.name,
        detail: 'Belum ada tahun ajaran yang diaktifkan.',
        schoolId: s.school_id,
        target: 'school',
      }),
    );
    outbox_dead.forEach((o) =>
      result.push({
        key: `outbox-${o.school_id}`,
        jenis: 'Outbox Dead',
        sekolah: o.school_name,
        detail: `${o.dead_count} pesan gagal permanen`,
        schoolId: o.school_id,
        target: 'outbox',
      }),
    );
    return result;
  }, [invoices_awaiting, schools_grace, schools_readonly, schools_no_active_year, outbox_dead]);

  const columns: DataTableColumn<AttentionRow>[] = [
    {
      key: 'jenis',
      header: 'Jenis',
      sortable: true,
      sortValue: (r) => r.jenis,
      cell: (r) => <Tag variant={r.jenis === 'Readonly' || r.jenis === 'Outbox Dead' ? 'danger' : 'warning'}>{r.jenis}</Tag>,
    },
    { key: 'sekolah', header: 'Sekolah', sortable: true, sortValue: (r) => r.sekolah, cell: (r) => <span className="font-medium text-ink">{r.sekolah}</span> },
    { key: 'detail', header: 'Detail', cell: (r) => r.detail },
  ];

  return (
    <div className="flex flex-col gap-3">
      <p className="text-[11px] font-semibold uppercase tracking-[0.1em] text-muted">
        Perlu Perhatian {rows.length > 0 && <span className="num text-muted">({rows.length})</span>}
      </p>
      <DataTable
        columns={columns}
        data={rows}
        keyField={(r) => r.key}
        onRowClick={(r) => (r.target === 'outbox' ? onOutboxClick(r.schoolId) : onSchoolClick(r.schoolId))}
        emptyMessage="Semua beres — tidak ada yang perlu perhatian."
        actions={(r) => [
          {
            label: 'Buka',
            onClick: () => (r.target === 'outbox' ? onOutboxClick(r.schoolId) : onSchoolClick(r.schoolId)),
          },
        ]}
      />
    </div>
  );
}

/**
 * "Aktivitas Terakhir" (deteksi pelanggan tidak aktif, docs/11 P1). Kolom
 * "Status" DITURUNKAN dari `overview.attention` (grace/readonly) yang sudah
 * ada di payload yang sama — BUKAN endpoint baru; sekolah yang tidak masuk
 * salah satu daftar itu ditandai "Aktif" (asumsi, backend tidak mengirim
 * status langganan eksplisit per baris di sini).
 */
function LastActivityTable({
  overview,
  onSchoolClick,
}: {
  overview: AdminOverview;
  onSchoolClick: (schoolId: string) => void;
}) {
  const statusBySchool = useMemo(() => {
    const map = new Map<string, 'grace' | 'readonly'>();
    overview.attention.schools_grace.forEach((s) => map.set(s.school_id, 'grace'));
    overview.attention.schools_readonly.forEach((s) => map.set(s.school_id, 'readonly'));
    return map;
  }, [overview.attention]);

  const columns: DataTableColumn<AdminLastActivity>[] = [
    { key: 'name', header: 'Sekolah', sortable: true, sortValue: (i) => i.name, cell: (i) => <span className="font-medium text-ink">{i.name}</span> },
    {
      key: 'last_login',
      header: 'Login Terakhir',
      sortable: true,
      sortValue: (i) => i.last_login ?? '',
      cell: (i) => (i.last_login ? formatRelativeTime(i.last_login) : <Tag variant="done">Belum pernah</Tag>),
    },
    {
      key: 'last_attendance_session',
      header: 'Sesi Absen Terakhir',
      sortable: true,
      sortValue: (i) => i.last_attendance_session ?? '',
      cell: (i) => (i.last_attendance_session ? formatRelativeTime(i.last_attendance_session) : <Tag variant="done">Belum pernah</Tag>),
    },
    {
      key: 'status',
      header: 'Status',
      cell: (i) => {
        const status = statusBySchool.get(i.school_id);
        if (status === 'readonly') return <Tag variant="danger">Readonly</Tag>;
        if (status === 'grace') return <Tag variant="warning">Grace</Tag>;
        return <Tag variant="success">Aktif</Tag>;
      },
    },
  ];

  return (
    <div className="flex flex-col gap-3">
      <p className="text-[11px] font-semibold uppercase tracking-[0.1em] text-muted">Aktivitas Terakhir</p>
      <DataTable
        columns={columns}
        data={overview.last_activity}
        keyField={(i) => i.school_id}
        onRowClick={(i) => onSchoolClick(i.school_id)}
        emptyMessage="Belum ada data aktivitas sekolah."
      />
    </div>
  );
}
