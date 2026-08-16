import { useMemo } from 'react';
import { useNavigate } from 'react-router-dom';
import { AlertTriangle, ChevronRight, ClipboardList, Megaphone, Users } from 'lucide-react';
import { Card } from '../../components/ui/Card';
import { PAGE_WIDE } from '../../components/ui/page';
import { StatTile } from '../../components/ui/StatTile';
import { DataTable, type DataTableColumn } from '../../components/ui/DataTable';
import { ListRow } from '../../components/ui/ListRow';
import { Tag } from '../../components/ui/Tag';
import { Skeleton } from '../../components/ui/Skeleton';
import { EmptyState } from '../../components/ui/EmptyState';
import { ErrorState } from '../../components/ui/ErrorState';
import { getGreeting } from '../../lib/greeting';
import { todayISODate, isoDateDaysAgo } from '../../lib/date';
import { PushPromptBanner } from '../notifications/PushPromptBanner';
import { useTvBoard } from '../tv/api';
import { useLeaveApprovals } from '../leave/api';
import { useStudentLeaveRequests } from '../studentleave/api';
import { useDisciplineRecords } from '../discipline/api';
import type { AttendanceClassSummary, Me, TvTeachingItem } from '../../lib/types';

function SessionStatusTag({ status }: { status: AttendanceClassSummary['session_status'] }) {
  if (status === 'finalized') return <Tag variant="done">Selesai</Tag>;
  if (status === 'open') return <Tag variant="now">Sedang diisi</Tag>;
  return <Tag variant="neutral">Belum diabsen</Tag>;
}

interface ActionItem {
  key: string;
  title: string;
  subtitle: string;
  to: string;
}

/**
 * Beranda admin sekolah desktop (Fase 16, feedback user: "masih banyak
 * kosong") — satu fetch utama `/api/tv/board` (sama sumber data TV/kepsek)
 * + antrian izin guru/siswa & pelanggaran 7 hari BILA hook-nya ada (tidak
 * mengarang endpoint baru). Menggantikan `BerandaPage` generik untuk role
 * `admin_sekolah` — role lain tetap memakai `BerandaPage`.
 */
export function AdminHomePage({ me }: { me: Me }) {
  const navigate = useNavigate();
  const greeting = getGreeting(new Date().getHours());
  const { data, isLoading, isError, refetch } = useTvBoard();

  // Antrian izin guru — gerbang kasar RBAC (docs/02): 200 kosong kalau tidak
  // ada step aktif milik admin, bukan 403. Kalau tetap error, tile dilewati
  // (lihat render StatTile "Izin Guru" di bawah).
  const leaveApprovals = useLeaveApprovals();
  // Antrian izin siswa (scope "all", khusus admin_sekolah — docs Fase 14 B1).
  const studentLeaveAll = useStudentLeaveRequests('all', undefined, true);
  const today = todayISODate();
  const disciplineWeek = useDisciplineRecords({ from: isoDateDaysAgo(7, today), to: today });
  const disciplineCount = disciplineWeek.data?.pages[0]?.total;

  const totalHadir = data?.attendance.reduce((sum, row) => sum + row.hadir, 0) ?? 0;
  const belumDiabsen = data?.attendance.filter((row) => row.session_status === null).length ?? 0;
  const studentLeavePending =
    studentLeaveAll.data?.filter((r) => r.status === 'pending_homeroom' || r.status === 'pending_bk').length;

  const actionItems: ActionItem[] = useMemo(() => {
    const items: ActionItem[] = [];
    if (data) {
      data.attendance
        .filter((row) => row.session_status === null)
        .forEach((row) =>
          items.push({
            key: `unmarked-${row.class_id}`,
            title: `${row.class_name} belum diabsen`,
            subtitle: 'Belum ada sesi absensi dibuka hari ini.',
            to: '/absensi',
          }),
        );
      data.attendance
        .filter((row) => row.session_status === 'open')
        .forEach((row) =>
          items.push({
            key: `open-${row.class_id}`,
            title: `${row.class_name} sesi absen belum selesai`,
            subtitle: 'Sesi sudah dibuka, belum di-finalize.',
            to: '/absensi',
          }),
        );
    }
    if (leaveApprovals.data && leaveApprovals.data.length > 0) {
      items.push({
        key: 'leave-approvals',
        title: `${leaveApprovals.data.length} pengajuan izin guru menunggu`,
        subtitle: 'Perlu persetujuan Anda.',
        to: '/izin/persetujuan',
      });
    }
    if (studentLeavePending && studentLeavePending > 0) {
      items.push({
        key: 'student-leave',
        title: `${studentLeavePending} pengajuan izin siswa menunggu`,
        subtitle: 'Surat izin siswa dalam proses persetujuan.',
        to: '/izin-siswa',
      });
    }
    return items;
  }, [data, leaveApprovals.data, studentLeavePending]);

  /** Kolom desktop absensi per rombel — sama pola `AttendanceRecapPage`. */
  const attendanceColumns: DataTableColumn<AttendanceClassSummary>[] = [
    { key: 'class_name', header: 'Rombel', sortable: true, sortValue: (r) => r.class_name, cell: (r) => <span className="font-medium text-ink">{r.class_name}</span> },
    { key: 'hadir', header: 'Hadir', align: 'right', sortable: true, sortValue: (r) => r.hadir, cell: (r) => r.hadir },
    { key: 'terlambat', header: 'Terlambat', align: 'right', sortable: true, sortValue: (r) => r.terlambat, cell: (r) => r.terlambat },
    { key: 'izin', header: 'Izin', align: 'right', sortable: true, sortValue: (r) => r.izin, cell: (r) => r.izin },
    { key: 'sakit', header: 'Sakit', align: 'right', sortable: true, sortValue: (r) => r.sakit, cell: (r) => r.sakit },
    { key: 'alpa', header: 'Alpa', align: 'right', sortable: true, sortValue: (r) => r.alpa, cell: (r) => r.alpa },
    { key: 'unmarked', header: 'Belum', align: 'right', sortable: true, sortValue: (r) => r.unmarked, cell: (r) => r.unmarked },
    { key: 'status', header: 'Status', cell: (r) => <SessionStatusTag status={r.session_status} /> },
  ];

  const teachingColumns: DataTableColumn<TvTeachingItem>[] = [
    { key: 'teacher', header: 'Guru', sortable: true, sortValue: (t) => t.teacher_name, cell: (t) => <span className="font-medium text-ink">{t.teacher_name}</span> },
    { key: 'subject', header: 'Mapel', sortable: true, sortValue: (t) => t.subject_name, cell: (t) => t.subject_name },
    { key: 'class', header: 'Kelas', sortable: true, sortValue: (t) => t.class_name, cell: (t) => t.class_name },
    { key: 'room', header: 'Ruangan', cell: (t) => t.room_name ?? <span className="text-muted">—</span> },
  ];

  return (
    <div className={`${PAGE_WIDE} flex flex-col gap-6`}>
      <div>
        <p className="text-[11px] font-semibold uppercase tracking-[0.1em] text-muted">Beranda</p>
        <h1 className="text-[21px] font-semibold text-ink">
          {greeting}, {me.name}
        </h1>
      </div>

      <PushPromptBanner />

      {isLoading ? (
        <div className="flex flex-col gap-4">
          <Skeleton className="h-24 w-full" />
          <Skeleton className="h-64 w-full" />
        </div>
      ) : isError || !data ? (
        <ErrorState message="Gagal memuat ringkasan hari ini." onRetry={() => refetch()} />
      ) : (
        <>
          <div className="grid grid-cols-2 gap-4 sm:grid-cols-3 [grid-template-columns:repeat(auto-fit,minmax(150px,1fr))]">
            <StatTile label="Hadir Hari Ini" value={totalHadir} variant="success" />
            <StatTile label="Belum Diabsen" value={belumDiabsen} variant={belumDiabsen > 0 ? 'warning' : 'muted'} />
            <StatTile label="Guru Mengajar" value={data.teaching.summary.mengajar} variant="success" />
            <StatTile label="Belum Masuk Kelas" value={data.teaching.summary.belum_masuk} variant="danger" />
            {leaveApprovals.isSuccess && (
              <StatTile
                label="Izin Guru Menunggu"
                value={leaveApprovals.data.length}
                variant={leaveApprovals.data.length > 0 ? 'warning' : 'muted'}
              />
            )}
            {disciplineCount !== undefined && (
              <StatTile
                label="Pelanggaran 7 Hari"
                value={disciplineCount}
                variant={disciplineCount > 0 ? 'warning' : 'muted'}
              />
            )}
          </div>

          <div className="grid gap-6 lg:grid-cols-2">
            <div className="flex flex-col gap-3">
              <p className="text-[11px] font-semibold uppercase tracking-[0.1em] text-muted">Perlu Tindakan</p>
              <Card variant="plain">
                {actionItems.length === 0 ? (
                  <p className="px-1 py-3 text-[13px] text-muted">Semua beres — tidak ada yang perlu tindakan.</p>
                ) : (
                  <div>
                    {actionItems.map((item) => (
                      <ListRow
                        key={item.key}
                        leading={<AlertTriangle size={18} strokeWidth={2} className="text-st-terlambat" aria-hidden="true" />}
                        title={item.title}
                        subtitle={item.subtitle}
                        trailing={<ChevronRight size={18} strokeWidth={2} className="text-muted" aria-hidden="true" />}
                        onClick={() => navigate(item.to)}
                      />
                    ))}
                  </div>
                )}
              </Card>
            </div>

            <div className="flex flex-col gap-3">
              <p className="text-[11px] font-semibold uppercase tracking-[0.1em] text-muted">Absensi per Rombel Hari Ini</p>
              <DataTable
                columns={attendanceColumns}
                data={data.attendance}
                keyField={(r) => r.class_id}
                emptyIcon={ClipboardList}
                emptyMessage="Belum ada rombel."
              />
            </div>
          </div>

          <div className="grid gap-6 lg:grid-cols-2">
            <div className="flex flex-col gap-3">
              <p className="text-[11px] font-semibold uppercase tracking-[0.1em] text-muted">Guru Mengajar Sekarang</p>
              <DataTable
                columns={teachingColumns}
                data={data.teaching.current}
                keyField={(t) => `${t.teacher_id}-${t.class_id}`}
                emptyIcon={Users}
                emptyMessage="Belum ada guru yang mengajar saat ini."
              />
            </div>

            <div className="flex flex-col gap-3">
              <p className="text-[11px] font-semibold uppercase tracking-[0.1em] text-muted">Pengumuman Aktif</p>
              <Card variant="plain">
                {data.announcements.length === 0 ? (
                  <EmptyState icon={Megaphone} message="Belum ada pengumuman aktif." />
                ) : (
                  <div>
                    {data.announcements.map((a) => (
                      <ListRow
                        key={a.id}
                        leading={<Megaphone size={18} strokeWidth={2} aria-hidden="true" />}
                        title={a.title}
                        subtitle={a.body}
                        trailing={a.is_platform ? <Tag variant="neutral">Platform</Tag> : undefined}
                      />
                    ))}
                  </div>
                )}
              </Card>
            </div>
          </div>
        </>
      )}
    </div>
  );
}

