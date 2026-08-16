import { useMemo, useState } from 'react';
import { Navigate, useNavigate } from 'react-router-dom';
import { FileBarChart, Users } from 'lucide-react';
import { ListRow } from '../../components/ui/ListRow';
import { StatTile } from '../../components/ui/StatTile';
import { Tag } from '../../components/ui/Tag';
import { Button } from '../../components/ui/Button';
import { Input } from '../../components/ui/Field';
import { Skeleton } from '../../components/ui/Skeleton';
import { EmptyState } from '../../components/ui/EmptyState';
import { ErrorState } from '../../components/ui/ErrorState';
import { formatClock } from '../schedule/format';
import { todayISODate } from '../../lib/date';
import { useMe } from '../auth/api';
import { PresenceSummary } from '../presence/PresenceSummary';
import { useTeachingStatus } from './api';
import type { TeachingStatus, TeachingStatusItem } from '../../lib/types';

type StatusTagVariant = 'success' | 'danger' | 'info' | 'done';

const STATUS_CONFIG: Record<TeachingStatus, { label: string; variant: StatusTagVariant }> = {
  mengajar: { label: 'Mengajar', variant: 'success' },
  belum_masuk: { label: 'Belum Masuk', variant: 'danger' },
  izin: { label: 'Izin', variant: 'info' },
  belum_mulai: { label: 'Belum Mulai', variant: 'done' },
  selesai: { label: 'Selesai', variant: 'done' },
};

interface Group {
  heading: string;
  items: TeachingStatusItem[];
}

function periodLabel(item: TeachingStatusItem): string {
  const { period_start, period_end } = item.slot;
  const number = period_start === period_end ? `Jam ke-${period_start}` : `Jam ke-${period_start}–${period_end}`;
  return `${number} · ${item.slot.period_starts_at}–${item.slot.period_ends_at}`;
}

function roomLine(item: TeachingStatusItem): string {
  const scheduled = item.slot.room?.name ?? 'Tanpa ruangan';
  if (item.room_actual && item.room_actual.id !== item.slot.room?.id) {
    return `${scheduled} → ${item.room_actual.name}`;
  }
  return scheduled;
}

/** /monitoring — status mengajar guru derivasi dari jadwal + jurnal (kepsek/admin), docs/06 #2. */
export function MonitoringPage() {
  const { data: me } = useMe();
  const navigate = useNavigate();
  const [date, setDate] = useState(todayISODate());
  const canView = me?.role === 'kepala_sekolah' || me?.role === 'admin_sekolah';
  const { data, isLoading, isError, refetch } = useTeachingStatus(date, canView);

  const groups = useMemo<Group[]>(() => {
    const items = data?.items ?? [];
    // "Sedang berlangsung" ditentukan dari current_period milik server (waktu
    // lokal SEKOLAH) — jangan pakai jam browser yang bisa beda timezone.
    const nowNumber = data?.current_period?.number ?? null;
    const isRunning = (item: TeachingStatusItem) =>
      nowNumber !== null && item.slot.period_start <= nowNumber && nowNumber <= item.slot.period_end;

    const running = items.filter(isRunning);
    const rest = items
      .filter((item) => !isRunning(item))
      .slice()
      .sort((a, b) => a.slot.period_start - b.slot.period_start);

    const result: Group[] = [];
    if (running.length > 0) {
      result.push({ heading: 'Sedang Berlangsung', items: running });
    }

    const restGroups = new Map<string, TeachingStatusItem[]>();
    rest.forEach((item) => {
      const key = `${item.slot.period_start}-${item.slot.period_end}`;
      const bucket = restGroups.get(key);
      if (bucket) bucket.push(item);
      else restGroups.set(key, [item]);
    });
    restGroups.forEach((bucketItems) => {
      result.push({ heading: periodLabel(bucketItems[0]), items: bucketItems });
    });

    return result;
  }, [data]);

  if (me && !canView) {
    return <Navigate to="/" replace />;
  }

  const summary = data?.summary;
  const currentPeriod = data?.current_period;
  const isToday = date === todayISODate();

  return (
    <div className="mx-auto flex max-w-[640px] flex-col gap-6 px-5 py-6 lg:max-w-[1000px]">
      {/* Fase 15 GAP 6b — dipasang di TOP halaman ini (satu-satunya pemakai,
          `canView` di atas sudah menggating admin_sekolah/kepala_sekolah). */}
      <PresenceSummary />

      <div className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <p className="text-[11px] font-semibold uppercase tracking-[0.1em] text-muted">Mengajar</p>
          <h1 className="text-[21px] font-semibold text-ink">Monitoring Guru</h1>
          {isToday && currentPeriod && (
            <p className="mt-1 text-[13px] text-muted">
              Jam ke-{currentPeriod.number} · {formatClock(currentPeriod.starts_at)}–{formatClock(currentPeriod.ends_at)} sedang berjalan
            </p>
          )}
        </div>
        <div className="flex flex-wrap items-start gap-2">
          <div className="max-w-[200px]">
            <Input type="date" value={date} onChange={(e) => setDate(e.target.value)} aria-label="Pilih tanggal" />
          </div>
          <Button variant="secondary" onClick={() => navigate('/monitoring/rekap')}>
            <FileBarChart size={16} strokeWidth={2} aria-hidden="true" />
            Rekap Ketertiban
          </Button>
        </div>
      </div>

      {summary && (
        <div className="grid grid-cols-3 gap-4 border-b border-line pb-6 sm:grid-cols-5">
          <StatTile label="Mengajar" value={summary.mengajar} variant="success" />
          <StatTile label="Belum Masuk" value={summary.belum_masuk} variant="danger" />
          <StatTile label="Izin" value={summary.izin} variant="info" />
          <StatTile label="Belum Mulai" value={summary.belum_mulai} variant="muted" />
          <StatTile label="Selesai" value={summary.selesai} variant="muted" />
        </div>
      )}

      {isLoading ? (
        <div className="flex flex-col gap-2">
          <Skeleton className="h-14 w-full" />
          <Skeleton className="h-14 w-full" />
          <Skeleton className="h-14 w-full" />
        </div>
      ) : isError ? (
        <ErrorState message="Gagal memuat status mengajar." onRetry={() => refetch()} />
      ) : groups.length === 0 ? (
        <EmptyState icon={Users} message="Tidak ada jadwal mengajar pada tanggal ini." />
      ) : (
        <div className="flex flex-col gap-5">
          {groups.map((group) => (
            <div key={group.heading}>
              <p className="mb-1 text-[11px] font-semibold uppercase tracking-[0.1em] text-muted">{group.heading}</p>
              <div>
                {group.items.map((item) => {
                  const cfg = STATUS_CONFIG[item.status];
                  return (
                    <ListRow
                      key={item.slot.id}
                      className="min-h-[56px]"
                      title={item.teacher.name}
                      subtitle={`${item.slot.subject.name} · ${item.slot.class.name} · ${roomLine(item)}`}
                      trailing={<Tag variant={cfg.variant}>{cfg.label}</Tag>}
                    />
                  );
                })}
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
