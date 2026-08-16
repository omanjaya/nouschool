import { useNavigate } from 'react-router-dom';
import {
  CheckCircle2,
  ChevronRight,
  Mail,
  MailWarning,
  Megaphone,
  ReceiptText,
  ShieldAlert,
  Tags,
} from 'lucide-react';
import { Card } from '../../components/ui/Card';
import { StatTile } from '../../components/ui/StatTile';
import { ListRow } from '../../components/ui/ListRow';
import { Tag } from '../../components/ui/Tag';
import { Skeleton } from '../../components/ui/Skeleton';
import { EmptyState } from '../../components/ui/EmptyState';
import { ErrorState } from '../../components/ui/ErrorState';
import { formatDate, formatRelativeTime } from '../../lib/date';
import { formatRupiah } from '../../lib/currency';
import { useMe } from '../auth/api';
import { useAdminOverview } from './api';
import type { AdminLastActivity, AdminOverview } from '../../lib/types';

/** /admin — beranda platform super admin (Fase 13 P1, docs/11-superadmin.md). */
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
    <div className="mx-auto flex max-w-[640px] flex-col gap-6 px-5 py-6 lg:max-w-[1120px]">
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
          <Card className="flex flex-col gap-4">
            <p className="text-[11px] font-semibold uppercase tracking-[0.1em] text-muted">Ringkasan</p>
            <div className="grid grid-cols-2 gap-4 sm:grid-cols-3 lg:[grid-template-columns:repeat(auto-fit,minmax(140px,1fr))]">
              <StatTile label="Sekolah Aktif" value={data.stats.schools_active} variant="success" />
              <StatTile label="Grace" value={data.stats.schools_grace} variant="warning" />
              <StatTile label="Readonly" value={data.stats.schools_readonly} variant="danger" />
              <StatTile label="Total Siswa" value={data.stats.total_students} />
              <StatTile label="Pendapatan Tahun Ini" value={formatRupiah(data.stats.revenue_year)} />
              <StatTile label="Leads 7 Hari" value={data.stats.leads_7d} />
            </div>
          </Card>

          <AttentionSection
            overview={data}
            onSchoolClick={(schoolId) => navigate(`/admin/sekolah/${schoolId}`)}
            onOutboxClick={(schoolId) => navigate(`/admin/outbox?school_id=${schoolId}`)}
          />

          <LastActivitySection items={data.last_activity} />

          <div className="flex flex-col gap-3">
            <p className="text-[11px] font-semibold uppercase tracking-[0.1em] text-muted">Jalan Pintas</p>
            <Card variant="plain">
              <div>
                <ListRow
                  leading={<Tags size={20} strokeWidth={2} aria-hidden="true" />}
                  title="Plan & Harga"
                  subtitle="Kelola plan dan bracket harga langganan."
                  trailing={<ChevronRight size={18} strokeWidth={2} className="text-muted" aria-hidden="true" />}
                  onClick={() => navigate('/admin/plans')}
                />
                <ListRow
                  leading={<Mail size={20} strokeWidth={2} aria-hidden="true" />}
                  title="Minat Sekolah"
                  subtitle="Daftar minat dari form landing page."
                  trailing={<ChevronRight size={18} strokeWidth={2} className="text-muted" aria-hidden="true" />}
                  onClick={() => navigate('/admin/minat')}
                />
                <ListRow
                  leading={<MailWarning size={20} strokeWidth={2} aria-hidden="true" />}
                  title="Outbox"
                  subtitle="Notifikasi gagal/dead lintas sekolah."
                  trailing={<ChevronRight size={18} strokeWidth={2} className="text-muted" aria-hidden="true" />}
                  onClick={() => navigate('/admin/outbox')}
                />
                <ListRow
                  leading={<Megaphone size={20} strokeWidth={2} aria-hidden="true" />}
                  title="Pengumuman Platform"
                  subtitle="Kelola pengumuman ke semua sekolah."
                  trailing={<ChevronRight size={18} strokeWidth={2} className="text-muted" aria-hidden="true" />}
                  onClick={() => navigate('/admin/pengumuman')}
                />
                <ListRow
                  leading={<ReceiptText size={20} strokeWidth={2} aria-hidden="true" />}
                  title="Pendapatan"
                  subtitle="Laporan pendapatan per bulan & plan."
                  trailing={<ChevronRight size={18} strokeWidth={2} className="text-muted" aria-hidden="true" />}
                  onClick={() => navigate('/admin/pendapatan')}
                />
              </div>
            </Card>
          </div>
        </>
      )}
    </div>
  );
}

/**
 * Inti dashboard (docs/11 P1): daftar "perlu perhatian" dikelompokkan per Card
 * — kelompok kosong tidak dirender; kalau seluruh kelompok kosong, tampilkan
 * satu EmptyState "semua beres" alih-alih lima kartu kosong berturut-turut.
 */
function AttentionSection({
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

  const allEmpty =
    invoices_awaiting.length === 0 &&
    schools_grace.length === 0 &&
    schools_readonly.length === 0 &&
    schools_no_active_year.length === 0 &&
    outbox_dead.length === 0;

  return (
    <div className="flex flex-col gap-4">
      <p className="text-[11px] font-semibold uppercase tracking-[0.1em] text-muted">Perlu Perhatian</p>

      {allEmpty ? (
        <Card>
          <EmptyState icon={CheckCircle2} message="Semua beres — tidak ada yang perlu perhatian." />
        </Card>
      ) : (
        <div className="flex flex-col gap-4">
          {invoices_awaiting.length > 0 && (
            <Card>
              <p className="mb-1 text-[14px] font-semibold text-ink">
                Invoice Menunggu Verifikasi <span className="num text-muted">({invoices_awaiting.length})</span>
              </p>
              <div>
                {invoices_awaiting.map((invoice) => (
                  <ListRow
                    key={invoice.invoice_id}
                    title={invoice.number}
                    subtitle={`${invoice.school_name} · ${formatRupiah(invoice.amount)}`}
                    trailing={<ChevronRight size={18} strokeWidth={2} className="text-muted" aria-hidden="true" />}
                    onClick={() => onSchoolClick(invoice.school_id)}
                  />
                ))}
              </div>
            </Card>
          )}

          {(schools_grace.length > 0 || schools_readonly.length > 0) && (
            <Card>
              <p className="mb-1 text-[14px] font-semibold text-ink">
                Sekolah Grace / Readonly{' '}
                <span className="num text-muted">({schools_grace.length + schools_readonly.length})</span>
              </p>
              <div>
                {schools_grace.map((school) => (
                  <ListRow
                    key={school.school_id}
                    title={school.name}
                    subtitle={`Berakhir ${formatDate(school.ends_on)} · grace sampai ${formatDate(school.grace_until)}`}
                    trailing={<Tag variant="warning">Grace</Tag>}
                    onClick={() => onSchoolClick(school.school_id)}
                  />
                ))}
                {schools_readonly.map((school) => (
                  <ListRow
                    key={school.school_id}
                    title={school.name}
                    subtitle="Akses sekolah dibatasi hanya-baca."
                    trailing={<Tag variant="danger">Readonly</Tag>}
                    onClick={() => onSchoolClick(school.school_id)}
                  />
                ))}
              </div>
            </Card>
          )}

          {schools_no_active_year.length > 0 && (
            <Card>
              <p className="mb-1 text-[14px] font-semibold text-ink">
                Sekolah Tanpa Tahun Ajaran Aktif{' '}
                <span className="num text-muted">({schools_no_active_year.length})</span>
              </p>
              <div>
                {schools_no_active_year.map((school) => (
                  <ListRow
                    key={school.school_id}
                    title={school.name}
                    subtitle="Belum ada tahun ajaran yang diaktifkan."
                    trailing={<ChevronRight size={18} strokeWidth={2} className="text-muted" aria-hidden="true" />}
                    onClick={() => onSchoolClick(school.school_id)}
                  />
                ))}
              </div>
            </Card>
          )}

          {outbox_dead.length > 0 && (
            <Card>
              <p className="mb-1 text-[14px] font-semibold text-ink">
                Outbox Menumpuk (Dead) <span className="num text-muted">({outbox_dead.length})</span>
              </p>
              <div>
                {outbox_dead.map((school) => (
                  <ListRow
                    key={school.school_id}
                    title={school.school_name}
                    subtitle={`${school.dead_count} pesan gagal permanen`}
                    trailing={<Tag variant="danger">{school.dead_count}</Tag>}
                    onClick={() => onOutboxClick(school.school_id)}
                  />
                ))}
              </div>
            </Card>
          )}
        </div>
      )}
    </div>
  );
}

/** Deteksi pelanggan tidak aktif (docs/11 P1) — login/sesi absen terakhir per sekolah. */
function LastActivitySection({ items }: { items: AdminLastActivity[] }) {
  return (
    <div className="flex flex-col gap-3">
      <p className="text-[11px] font-semibold uppercase tracking-[0.1em] text-muted">Aktivitas Terakhir</p>
      <Card>
        {items.length === 0 ? (
          <EmptyState message="Belum ada data aktivitas sekolah." />
        ) : (
          <div>
            {items.map((item) => (
              <div
                key={item.school_id}
                className="flex flex-col gap-2 border-b border-line py-3 last:border-0 sm:flex-row sm:items-center sm:justify-between"
              >
                <p className="truncate text-[14px] text-ink">{item.name}</p>
                <div className="flex flex-wrap items-center gap-x-4 gap-y-1.5 text-[12px] text-muted">
                  <span className="flex items-center gap-1.5">
                    Login:
                    {item.last_login ? (
                      <span className="text-ink">{formatRelativeTime(item.last_login)}</span>
                    ) : (
                      <Tag variant="done">Belum pernah</Tag>
                    )}
                  </span>
                  <span className="flex items-center gap-1.5">
                    Sesi absen:
                    {item.last_attendance_session ? (
                      <span className="text-ink">{formatRelativeTime(item.last_attendance_session)}</span>
                    ) : (
                      <Tag variant="done">Belum pernah</Tag>
                    )}
                  </span>
                </div>
              </div>
            ))}
          </div>
        )}
      </Card>
    </div>
  );
}
