import { useState } from 'react';
import { Navigate, useNavigate } from 'react-router-dom';
import { CalendarClock, ClipboardList, FileBarChart } from 'lucide-react';
import { ListRow } from '../../components/ui/ListRow';
import { Skeleton } from '../../components/ui/Skeleton';
import { EmptyState } from '../../components/ui/EmptyState';
import { ErrorState } from '../../components/ui/ErrorState';
import { Tag } from '../../components/ui/Tag';
import { Button } from '../../components/ui/Button';
import { useToast } from '../../components/ui/Toast';
import { ApiError } from '../../lib/api';
import { formatDateWithDay, formatTimeOfDay, todayISODate } from '../../lib/date';
import { useMe } from '../auth/api';
import { useAttendanceClasses, useAttendanceSlotsToday, useOpenAttendanceSession, useOpenAttendanceSessionForSlot } from './api';
import type { AttendanceClassListItem, AttendanceSlotToday } from '../../lib/types';

function SessionTag({ item }: { item: AttendanceClassListItem }) {
  if (!item.session) return <Tag variant="neutral">Belum diabsen</Tag>;
  if (item.session.status === 'finalized') return <Tag variant="done">Selesai</Tag>;
  return (
    <Tag variant="now">
      Sedang diisi ({item.session.marked_count}/{item.student_count})
    </Tag>
  );
}

function SlotSessionTag({ item }: { item: AttendanceSlotToday }) {
  if (!item.session) return <Tag variant="neutral">Belum diabsen</Tag>;
  if (item.session.status === 'finalized') return <Tag variant="done">Selesai</Tag>;
  return (
    <Tag variant="now">
      Sedang diisi ({item.session.marked_count}/{item.session.total})
    </Tag>
  );
}

export function AttendanceClassesPage() {
  const { data: me } = useMe();
  const today = todayISODate();
  const { data: classes, isLoading, isError, error, refetch } = useAttendanceClasses(today);
  const openSession = useOpenAttendanceSession();
  const [openingId, setOpeningId] = useState<string | null>(null);
  const navigate = useNavigate();
  const { showToast } = useToast();

  const isGuru = me?.role === 'guru';
  const {
    data: slotsToday,
    isLoading: slotsLoading,
    isError: slotsError,
    refetch: refetchSlots,
  } = useAttendanceSlotsToday(isGuru);
  const openSlotSession = useOpenAttendanceSessionForSlot();
  const [openingSlotId, setOpeningSlotId] = useState<string | null>(null);

  if (!me) return null;

  // kepala_sekolah hanya punya attendance:report — layar ini (buka/isi sesi)
  // bukan untuk mereka, langsung arahkan ke rekap.
  if (me.role === 'kepala_sekolah') {
    return <Navigate to="/absensi/rekap" replace />;
  }
  if (me.role === 'siswa' || me.role === 'orang_tua') {
    return <Navigate to="/" replace />;
  }

  const canSeeRecap = me.role === 'admin_sekolah' || me.role === 'kepala_sekolah';
  const hasSlots = isGuru && (slotsToday?.length ?? 0) > 0;

  async function handleOpen(item: AttendanceClassListItem) {
    if (openingId) return;
    if (item.session) {
      navigate(`/absensi/sesi/${item.session.id}`);
      return;
    }
    setOpeningId(item.class_id);
    try {
      const result = await openSession.mutateAsync(item.class_id);
      navigate(`/absensi/sesi/${result.session.id}`);
    } catch (err) {
      showToast(err instanceof ApiError ? err.message : 'Gagal membuka sesi absensi.', 'error');
    } finally {
      setOpeningId(null);
    }
  }

  async function handleOpenSlot(item: AttendanceSlotToday) {
    if (openingSlotId) return;
    if (item.session) {
      navigate(`/absensi/sesi/${item.session.id}`);
      return;
    }
    setOpeningSlotId(item.slot.id);
    try {
      const result = await openSlotSession.mutateAsync(item.slot.id);
      navigate(`/absensi/sesi/${result.session.id}`);
    } catch (err) {
      showToast(err instanceof ApiError ? err.message : 'Gagal membuka sesi absensi.', 'error');
    } finally {
      setOpeningSlotId(null);
    }
  }

  return (
    <div className="mx-auto flex max-w-[640px] flex-col gap-6 px-5 py-6">
      <div className="flex items-start justify-between gap-3">
        <div>
          <p className="text-[11px] font-semibold uppercase tracking-[0.1em] text-muted">Absensi</p>
          <h1 className="text-[21px] font-semibold text-ink">{formatDateWithDay(today)}</h1>
        </div>
        {canSeeRecap && (
          <Button variant="secondary" onClick={() => navigate('/absensi/rekap')}>
            <FileBarChart size={16} strokeWidth={2} aria-hidden="true" />
            Rekap
          </Button>
        )}
      </div>

      {isGuru && slotsLoading && (
        <div className="flex flex-col gap-2">
          <Skeleton className="h-14 w-full" />
          <Skeleton className="h-14 w-full" />
        </div>
      )}

      {isGuru && !slotsLoading && slotsError && (
        <ErrorState message="Gagal memuat jadwal mengajar hari ini." onRetry={() => refetchSlots()} />
      )}

      {hasSlots && (
        <div>
          <p className="mb-1 flex items-center gap-1.5 text-[11px] font-semibold uppercase tracking-[0.1em] text-muted">
            <CalendarClock size={14} strokeWidth={2} aria-hidden="true" />
            Jadwal Mengajar Hari Ini
          </p>
          <div>
            {slotsToday?.map((item) => (
              <ListRow
                key={item.slot.id}
                className="min-h-[52px]"
                title={`${item.slot.subject.name} · ${item.slot.class.name}`}
                subtitle={`${formatTimeOfDay(item.slot.starts_at)}–${formatTimeOfDay(item.slot.ends_at)}`}
                trailing={<SlotSessionTag item={item} />}
                onClick={() => handleOpenSlot(item)}
              />
            ))}
          </div>
        </div>
      )}

      <div>
        {hasSlots && (
          <p className="mb-1 text-[11px] font-semibold uppercase tracking-[0.1em] text-muted">Absensi Harian</p>
        )}

        {isLoading ? (
          <div className="flex flex-col gap-2">
            <Skeleton className="h-14 w-full" />
            <Skeleton className="h-14 w-full" />
            <Skeleton className="h-14 w-full" />
          </div>
        ) : isError ? (
          <ErrorState
            message={error instanceof ApiError ? error.message : 'Gagal memuat daftar rombel.'}
            onRetry={() => refetch()}
          />
        ) : classes && classes.length === 0 ? (
          <EmptyState icon={ClipboardList} message="Belum ada rombel yang diajarkan hari ini." />
        ) : (
          <div>
            {classes?.map((item) => (
              <ListRow
                key={item.class_id}
                className="min-h-[52px]"
                title={item.name}
                subtitle={`${item.student_count} siswa`}
                trailing={<SessionTag item={item} />}
                onClick={() => handleOpen(item)}
              />
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
