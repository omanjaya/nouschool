import { useNavigate } from 'react-router-dom';
import { ChevronRight, ClipboardCheck, FileBarChart, Megaphone, Users, type LucideIcon } from 'lucide-react';
import { Card } from '../../components/ui/Card';
import { ListRow } from '../../components/ui/ListRow';
import { StatTile } from '../../components/ui/StatTile';
import { Skeleton } from '../../components/ui/Skeleton';
import { ErrorState } from '../../components/ui/ErrorState';
import { useTvBoard } from '../tv/api';
import { getGreeting } from '../../lib/greeting';
import { PushPromptBanner } from '../notifications/PushPromptBanner';
import type { Me } from '../../lib/types';

interface QuickLink {
  label: string;
  description: string;
  to: string;
  icon: LucideIcon;
}

const QUICK_LINKS: QuickLink[] = [
  { label: 'Monitoring Guru', description: 'Status mengajar guru saat ini.', to: '/monitoring', icon: Users },
  { label: 'Rekap Absensi', description: 'Rekap kehadiran siswa harian.', to: '/absensi/rekap', icon: ClipboardCheck },
  { label: 'Rekap Izin', description: 'Rekap izin guru per rentang.', to: '/izin/rekap', icon: FileBarChart },
  { label: 'Pengumuman', description: 'Kelola pengumuman untuk TV & beranda.', to: '/pengumuman', icon: Megaphone },
  { label: 'Rekap Ketertiban', description: 'Persentase slot mengajar terlaksana.', to: '/monitoring/rekap', icon: FileBarChart },
];

/**
 * Beranda kepala sekolah — ringkasan dari `/api/tv/board` (satu fetch, sama
 * dengan sumber data `/tv`) + jalan pintas ke laporan (docs/06 "Dashboard
 * kepala sekolah"). Menggantikan `BerandaPage` generik untuk role ini.
 */
export function KepsekHomePage({ me }: { me: Me }) {
  const navigate = useNavigate();
  const { data, isLoading, isError, refetch } = useTvBoard();
  const greeting = getGreeting(new Date().getHours());

  const totalHadir = data?.attendance.reduce((sum, row) => sum + row.hadir, 0) ?? 0;
  const totalSiswa = data?.attendance.reduce((sum, row) => sum + row.total, 0) ?? 0;

  return (
    <div className="mx-auto flex max-w-[640px] flex-col gap-6 px-5 py-6 lg:max-w-[1000px]">
      <div>
        <p className="text-[11px] font-semibold uppercase tracking-[0.1em] text-muted">Beranda</p>
        <h1 className="text-[21px] font-semibold text-ink">
          {greeting}, {me.name}
        </h1>
      </div>

      <PushPromptBanner />

      {isLoading ? (
        <div className="flex flex-col gap-2">
          <Skeleton className="h-24 w-full" />
          <Skeleton className="h-24 w-full" />
        </div>
      ) : isError ? (
        <ErrorState message="Gagal memuat ringkasan hari ini." onRetry={() => refetch()} />
      ) : (
        data && (
          <>
            <Card className="flex flex-col gap-3">
              <p className="text-[12px] font-semibold uppercase tracking-[0.1em] text-muted">Status Mengajar</p>
              <div className="grid grid-cols-2 gap-4">
                <StatTile label="Mengajar" value={data.teaching.summary.mengajar} variant="success" />
                <StatTile label="Belum Masuk" value={data.teaching.summary.belum_masuk} variant="danger" />
              </div>
            </Card>

            <Card className="flex flex-col gap-3">
              <p className="text-[12px] font-semibold uppercase tracking-[0.1em] text-muted">Kehadiran Hari Ini</p>
              <div className="grid grid-cols-1 gap-4">
                <StatTile label="Total Hadir" value={totalHadir} variant="success" />
              </div>
              <p className="text-[12px] text-muted">
                dari {totalSiswa} siswa di {data.attendance.length} rombel
              </p>
            </Card>
          </>
        )
      )}

      <Card variant="plain">
        <p className="mb-1 text-[12px] font-semibold uppercase tracking-[0.1em] text-muted">Jalan Pintas</p>
        <div>
          {QUICK_LINKS.map((link) => (
            <ListRow
              key={link.to}
              leading={<link.icon size={20} strokeWidth={2} aria-hidden="true" />}
              title={link.label}
              subtitle={link.description}
              trailing={<ChevronRight size={18} strokeWidth={2} className="text-muted" aria-hidden="true" />}
              onClick={() => navigate(link.to)}
            />
          ))}
        </div>
      </Card>
    </div>
  );
}
