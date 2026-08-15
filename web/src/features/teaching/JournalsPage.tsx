import { useMemo, useState } from 'react';
import { Navigate, useNavigate } from 'react-router-dom';
import { NotebookPen, QrCode } from 'lucide-react';
import { ListRow } from '../../components/ui/ListRow';
import { Skeleton } from '../../components/ui/Skeleton';
import { EmptyState } from '../../components/ui/EmptyState';
import { ErrorState } from '../../components/ui/ErrorState';
import { Tag } from '../../components/ui/Tag';
import { Button } from '../../components/ui/Button';
import { SegmentedControl } from '../../components/ui/SegmentedControl';
import { currentISOMonth, formatTimeOfDay, todayISODate } from '../../lib/date';
import { useMe } from '../auth/api';
import { useJournals } from './api';
import { JournalDetailDialog } from './JournalDetailDialog';
import type { Journal } from '../../lib/types';

type RangeOption = 'today' | 'month';

const FLAG_LABEL: Record<string, string> = {
  room_mismatch: 'Ruang beda',
  unscheduled: 'Di luar jadwal',
  manual_pick: 'Dipilih manual',
};

/** /jurnal — jurnal mengajar guru sendiri: Hari Ini | Bulan Ini, tap baris untuk isi/ubah materi. */
export function JournalsPage() {
  const { data: me } = useMe();
  const navigate = useNavigate();
  const [range, setRange] = useState<RangeOption>('today');
  const [selected, setSelected] = useState<Journal | null>(null);

  const today = todayISODate();
  const filter = range === 'today' ? { date: today } : { month: currentISOMonth() };
  const isGuru = me?.role === 'guru';
  const { data, isLoading, isError, refetch } = useJournals('mine', filter, isGuru);

  const items = useMemo(() => {
    const list = data?.items ?? [];
    return list.slice().sort((a, b) => (a.started_at < b.started_at ? 1 : -1));
  }, [data]);

  if (me && !isGuru) {
    return <Navigate to="/" replace />;
  }

  return (
    <div className="mx-auto flex max-w-[640px] flex-col gap-6 px-5 py-6">
      <div className="flex items-start justify-between gap-3">
        <div>
          <p className="text-[11px] font-semibold uppercase tracking-[0.1em] text-muted">Mengajar</p>
          <h1 className="text-[21px] font-semibold text-ink">Jurnal Mengajar</h1>
        </div>
        <Button variant="secondary" onClick={() => navigate('/scan')}>
          <QrCode size={16} strokeWidth={2} aria-hidden="true" />
          Scan
        </Button>
      </div>

      <SegmentedControl
        options={[
          { value: 'today', label: 'Hari Ini' },
          { value: 'month', label: 'Bulan Ini' },
        ]}
        value={range}
        onChange={setRange}
      />

      {isLoading ? (
        <div className="flex flex-col gap-2">
          <Skeleton className="h-16 w-full" />
          <Skeleton className="h-16 w-full" />
          <Skeleton className="h-16 w-full" />
        </div>
      ) : isError ? (
        <ErrorState message="Gagal memuat jurnal mengajar." onRetry={() => refetch()} />
      ) : items.length === 0 ? (
        <EmptyState
          icon={NotebookPen}
          message="Belum ada jurnal. Scan QR di kelas untuk memulai."
          action={
            <Button variant="secondary" onClick={() => navigate('/scan')}>
              Scan QR Ruangan
            </Button>
          }
        />
      ) : (
        <div>
          {items.map((j) => {
            const timeLine = `${formatTimeOfDay(j.started_at)}${
              j.ended_at ? `–${formatTimeOfDay(j.ended_at)}` : ''
            } · ${j.room?.name ?? 'Tanpa ruangan'}`;
            const missingMaterial = j.status === 'done' && !j.material;

            return (
              <ListRow
                key={j.id}
                className="min-h-[56px]"
                title={`${j.subject?.name ?? 'Tanpa mapel'} · ${j.class.name}`}
                subtitle={
                  <span className="flex flex-col gap-1">
                    <span className="truncate">{timeLine}</span>
                    {j.flags.length > 0 && (
                      <span className="flex flex-wrap gap-1">
                        {j.flags.map((flag) => (
                          <Tag key={flag} variant="neutral">
                            {FLAG_LABEL[flag] ?? flag}
                          </Tag>
                        ))}
                      </span>
                    )}
                  </span>
                }
                trailing={
                  <div className="flex flex-col items-end gap-1">
                    {j.status === 'ongoing' ? <Tag variant="now">Berlangsung</Tag> : <Tag variant="done">Selesai</Tag>}
                    {missingMaterial && <Tag variant="warning">Materi belum diisi</Tag>}
                  </div>
                }
                onClick={() => setSelected(j)}
              />
            );
          })}
        </div>
      )}

      <JournalDetailDialog open={selected !== null} journal={selected} onClose={() => setSelected(null)} />
    </div>
  );
}
