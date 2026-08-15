import { useEffect, useMemo, useRef, useState } from 'react';
import { Navigate } from 'react-router-dom';
import { WifiOff } from 'lucide-react';
import { formatDate } from '../../lib/date';
import { formatClock } from '../schedule/format';
import { useRealtimeEvents, useRealtimeState } from '../../lib/realtime';
import { useMe } from '../auth/api';
import { useTvBoard, TV_BOARD_POLL_MS, TV_BOARD_POLL_WS_CONNECTED_MS } from './api';
import { useServerClock } from './useServerClock';
import { formatClockWithSeconds, formatCountdown, parseWallClock } from './clock';
import type { TeachingStatus, TvTeachingItem, TvAnnouncement } from '../../lib/types';

const ANNOUNCEMENT_ROTATE_MS = 12_000;
const DAILY_RELOAD_HOUR = 3;

const STATUS_BADGE: Record<TeachingStatus, { label: string; bg: string; text: string }> = {
  mengajar: { label: 'Mengajar', bg: 'bg-st-hadir-line', text: 'text-st-hadir' },
  belum_masuk: { label: 'Belum Masuk', bg: 'bg-danger-soft', text: 'text-danger' },
  izin: { label: 'Izin', bg: 'bg-st-izin-line', text: 'text-st-izin' },
  belum_mulai: { label: 'Belum Mulai', bg: 'bg-surface-2', text: 'text-muted' },
  selesai: { label: 'Selesai', bg: 'bg-surface-2', text: 'text-muted' },
};

function roomLine(item: TvTeachingItem): string {
  return item.room_name || 'Tanpa ruangan';
}

/**
 * Fase 12: titik kecil status realtime WS — hijau (`st-hadir`) saat
 * tersambung, abu (`muted`) selain itu (menyambung/reconnecting/putus).
 * Terpisah dari indikator "Koneksi terputus" existing (bottom-right, itu
 * untuk kegagalan fetch REST `/tv/board`, bukan status socket).
 */
function RealtimeStatusDot({ connected }: { connected: boolean }) {
  return (
    <span
      role="status"
      aria-label={connected ? 'Realtime tersambung' : 'Realtime tidak tersambung'}
      title={connected ? 'Realtime tersambung' : 'Realtime tidak tersambung'}
      className={`inline-block h-2.5 w-2.5 shrink-0 rounded-full ${connected ? 'bg-st-hadir' : 'bg-muted'}`}
    />
  );
}

/** Jadwal ke ms sampai jam 03:00 lokal berikutnya — guard auto full-reload harian (redeploy TV). */
function msUntilNextReload(): number {
  const now = new Date();
  const target = new Date(now.getFullYear(), now.getMonth(), now.getDate(), DAILY_RELOAD_HOUR, 0, 0, 0);
  if (target.getTime() <= now.getTime()) target.setDate(target.getDate() + 1);
  return target.getTime() - now.getTime();
}

function TvStatusBadge({ status }: { status: TeachingStatus }) {
  const cfg = STATUS_BADGE[status];
  return (
    <span className={`inline-flex shrink-0 items-center rounded-full px-4 py-2 text-[20px] font-semibold ${cfg.bg} ${cfg.text}`}>
      {cfg.label}
    </span>
  );
}

function TvTeacherRow({ item, muted = false }: { item: TvTeachingItem; muted?: boolean }) {
  return (
    <div className="flex items-center justify-between gap-6 border-b border-line py-4 last:border-b-0">
      <div className="min-w-0 flex-1">
        <p className={`truncate text-[24px] font-bold ${muted ? 'text-muted' : 'text-ink'}`}>{item.teacher_name}</p>
        <p className="truncate text-[20px] text-muted">
          {item.subject_name} · {item.class_name} · {roomLine(item)} · {item.period_starts_at}–{item.period_ends_at}
        </p>
      </div>
      <TvStatusBadge status={item.status} />
    </div>
  );
}

function TvAnnouncementPanel({ announcements }: { announcements: TvAnnouncement[] }) {
  const [index, setIndex] = useState(0);
  const [visible, setVisible] = useState(true);

  useEffect(() => {
    setIndex(0);
    setVisible(true);
  }, [announcements]);

  useEffect(() => {
    if (announcements.length <= 1) return;
    const reduceMotion = window.matchMedia('(prefers-reduced-motion: reduce)').matches;
    const interval = setInterval(() => {
      if (reduceMotion) {
        setIndex((i) => (i + 1) % announcements.length);
        return;
      }
      setVisible(false);
      setTimeout(() => {
        setIndex((i) => (i + 1) % announcements.length);
        setVisible(true);
      }, 300);
    }, ANNOUNCEMENT_ROTATE_MS);
    return () => clearInterval(interval);
  }, [announcements.length]);

  if (announcements.length === 0) {
    return <p className="text-[20px] text-muted">Belum ada pengumuman.</p>;
  }

  const current = announcements[index];

  return (
    <div className={`transition-opacity duration-300 motion-reduce:transition-none ${visible ? 'opacity-100' : 'opacity-0'}`}>
      <p className="text-[24px] font-semibold text-ink">{current.title}</p>
      <p className="mt-2 text-[20px] leading-snug text-muted">{current.body}</p>
    </div>
  );
}

/**
 * `/tv` — dashboard TV fullscreen (docs/06 "Dashboard TV ruang guru"). TANPA
 * AppShell: layout terpisah, token Rapor sama, ukuran font digandakan untuk
 * dilihat dari jauh (docs/10-design-system.md #5). Akses: role `display`,
 * `kepala_sekolah`, `admin_sekolah` (kontrak `GET /api/tv/board`).
 */
export function TvPage() {
  const { data: me } = useMe();
  const canView = me?.role === 'display' || me?.role === 'kepala_sekolah' || me?.role === 'admin_sekolah';
  const rtState = useRealtimeState();
  const rtConnected = rtState === 'connected';
  // Fase 12: polling tetap ada sebagai fallback, dilonggarkan saat WS
  // tersambung (event realtime jadi jalur update utama, lihat di bawah).
  const { data, isLoading, isError, refetch } = useTvBoard(rtConnected ? TV_BOARD_POLL_WS_CONNECTED_MS : TV_BOARD_POLL_MS);
  // Sinyal saja (docs/06/12) — begitu salah satu event ini datang, refetch
  // REST `/tv/board` segera alih-alih menunggu jadwal polling berikutnya.
  useRealtimeEvents({
    'attendance.summary': () => refetch(),
    'teaching.status': () => refetch(),
    announcement: () => refetch(),
  });
  const reloadScheduled = useRef(false);

  const clientNow = useServerClock(data?.now.date, data?.now.time, data?.generated_at);

  useEffect(() => {
    if (reloadScheduled.current) return;
    reloadScheduled.current = true;
    const id = setTimeout(() => window.location.reload(), msUntilNextReload());
    return () => clearTimeout(id);
  }, []);

  const countdownTarget = useMemo(() => {
    if (!data) return null;
    const targetTime = data.now.current_period ? data.now.current_period.ends_at : data.now.next_starts_at;
    if (!targetTime) return null;
    return parseWallClock(data.now.date, targetTime);
  }, [data]);

  if (me && !canView) {
    return <Navigate to="/" replace />;
  }

  if (!data && isLoading) {
    return (
      <div className="flex min-h-dvh items-center justify-center bg-bg">
        <p className="text-[24px] text-muted">Memuat papan TV…</p>
      </div>
    );
  }

  if (!data && isError) {
    return (
      <div className="flex min-h-dvh flex-col items-center justify-center gap-4 bg-bg">
        <p className="text-[24px] text-muted">Gagal memuat papan TV.</p>
        <button
          type="button"
          onClick={() => refetch()}
          className="rounded-lg bg-primary px-6 py-3 text-[20px] font-semibold text-primary-ink"
        >
          Coba lagi
        </button>
      </div>
    );
  }

  if (!data) return null;

  const currentPeriodNumber = data.now.current_period?.number;
  const countdownLabel = formatCountdown(clientNow, countdownTarget);

  return (
    <div className="flex min-h-dvh w-full flex-col gap-8 bg-bg p-10 text-ink">
      <header className="flex items-center justify-between">
        <p className="text-[28px] font-semibold text-ink">{data.school.name}</p>
        <p className="flex items-center gap-2 text-[16px] uppercase tracking-[0.1em] text-muted">
          <RealtimeStatusDot connected={rtConnected} />
          Dashboard TV
        </p>
      </header>

      <div className="grid flex-1 grid-cols-3 gap-10 overflow-hidden">
        {/* Kiri: sedang mengajar + jam berikutnya (2/3) */}
        <div className="col-span-2 flex min-h-0 flex-col gap-8">
          <section className="flex min-h-0 flex-1 flex-col">
            <h2 className="mb-2 text-[28px] font-semibold text-ink">
              Sedang Mengajar{currentPeriodNumber ? ` — Jam ke-${currentPeriodNumber}` : ''}
            </h2>
            <div className="flex-1 overflow-y-auto">
              {data.teaching.current.length === 0 ? (
                <p className="py-6 text-[22px] text-muted">Tidak ada guru mengajar saat ini.</p>
              ) : (
                data.teaching.current.map((item) => <TvTeacherRow key={`${item.teacher_id}-${item.class_id}-${item.period_starts_at}`} item={item} />)
              )}
            </div>
          </section>

          <section className="flex min-h-0 flex-[0.7] flex-col">
            <h2 className="mb-2 text-[24px] font-semibold text-muted">Jam Berikutnya</h2>
            <div className="flex-1 overflow-y-auto opacity-70">
              {data.teaching.upcoming.length === 0 ? (
                <p className="py-4 text-[20px] text-muted">Tidak ada jadwal jam berikutnya.</p>
              ) : (
                data.teaching.upcoming.map((item) => <TvTeacherRow key={`${item.teacher_id}-${item.class_id}-${item.period_starts_at}`} item={item} muted />)
              )}
            </div>
          </section>
        </div>

        {/* Kanan: jam, kehadiran, pengumuman (1/3) */}
        <div className="col-span-1 flex min-h-0 flex-col gap-8">
          <section className="border-b border-line pb-6 text-right">
            <p className="num text-[72px] font-bold leading-none text-ink">{formatClockWithSeconds(clientNow)}</p>
            <p className="mt-2 text-[22px] text-muted">
              {data.now.day_label}, {formatDate(data.now.date)}
            </p>
            {data.now.current_period && (
              <p className="mt-1 text-[22px] font-medium text-ink">
                Jam ke-{data.now.current_period.number} · {formatClock(data.now.current_period.starts_at)}–
                {formatClock(data.now.current_period.ends_at)}
              </p>
            )}
            {countdownLabel && <p className="mt-1 text-[20px] text-muted">Pergantian dalam {countdownLabel}</p>}
          </section>

          <section className="flex min-h-0 flex-1 flex-col">
            <h2 className="mb-2 text-[22px] font-semibold uppercase tracking-[0.1em] text-muted">Kehadiran Hari Ini</h2>
            <div className="flex-1 overflow-y-auto">
              {data.attendance.length === 0 ? (
                <p className="py-4 text-[18px] text-muted">Belum ada data absensi hari ini.</p>
              ) : (
                data.attendance.map((row) => (
                  <div key={row.class_id} className="flex items-center justify-between gap-3 border-b border-line py-2.5 last:border-b-0">
                    <span className="truncate text-[20px] font-medium text-ink">{row.class_name}</span>
                    <div className="flex shrink-0 items-center gap-2">
                      <span className="num text-[16px] font-semibold text-st-hadir">{row.hadir}H</span>
                      <span className="num text-[16px] font-semibold text-st-terlambat">{row.terlambat}T</span>
                      <span className="num text-[16px] font-semibold text-st-izin">{row.izin}I</span>
                      <span className="num text-[16px] font-semibold text-st-sakit">{row.sakit}S</span>
                      <span className="num text-[16px] font-semibold text-st-alpa">{row.alpa}A</span>
                      {row.session_status === 'finalized' && (
                        <span className="rounded-full bg-surface-2 px-3 py-1 text-[14px] font-medium text-muted">Selesai</span>
                      )}
                    </div>
                  </div>
                ))
              )}
            </div>
          </section>

          <section className="border-t border-line pt-6">
            <h2 className="mb-2 text-[22px] font-semibold uppercase tracking-[0.1em] text-muted">Pengumuman</h2>
            <TvAnnouncementPanel announcements={data.announcements} />
          </section>
        </div>
      </div>

      {isError && (
        <div className="fixed bottom-6 right-6 flex items-center gap-2 rounded-lg border border-line bg-surface px-4 py-3 text-[16px] text-muted shadow-[0_8px_24px_rgba(23,35,58,0.12)]">
          <WifiOff size={18} strokeWidth={2} aria-hidden="true" />
          Koneksi terputus · mencoba ulang
        </div>
      )}
    </div>
  );
}
