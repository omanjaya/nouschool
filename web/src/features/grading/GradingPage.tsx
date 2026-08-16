import { useEffect, useMemo } from 'react';
import { Navigate, Outlet, useLocation, useSearchParams } from 'react-router-dom';
import { GraduationCap } from 'lucide-react';
import { Tabs, type TabItem } from '../../components/ui/Tabs';
import { Field, Select } from '../../components/ui/Field';
import { EmptyState } from '../../components/ui/EmptyState';
import { ErrorState } from '../../components/ui/ErrorState';
import { Skeleton } from '../../components/ui/Skeleton';
import { useMe } from '../auth/api';
import { useClasses } from '../classes/api';
import { useSubjects } from '../subjects/api';
import { useScheduleSlots } from '../schedule/api';
import { useGradingStatus } from './api';
import type { ClassRef, ScheduleSlotSubjectRef } from '../../lib/types';
import { PAGE_WIDE } from '../../components/ui/page';

export interface GradingContext {
  classId: string;
  subjectId: string;
  /**
   * `true` untuk kepala_sekolah (Fase 15 GAP 5, `grading:read` lintas kelas
   * tapi tanpa mutasi) — konsumen Outlet (Rekap/Rapor) menyembunyikan tombol
   * publikasi/bintang/simpan saat ini `true`. Selalu `false` untuk guru/admin.
   */
  readOnly: boolean;
}

function uniqueBy<T>(items: T[], key: (item: T) => string): T[] {
  const map = new Map<string, T>();
  items.forEach((item) => map.set(key(item), item));
  return Array.from(map.values());
}

/**
 * /nilai — layout guru/admin: guard `status.enabled` & role, pemilih kelas+mapel
 * (disimpan di query string supaya bertahan lintas tab), tab Komponen/Input
 * Nilai/Rekap lewat `<Outlet context={GradingContext}>`.
 *
 * Pemilih kelas+mapel: GURU derivasi dari `/api/schedule/slots` TANPA parameter
 * (backend infer slot sendiri, pola sama `SchedulePage`) — dua Select berantai
 * (kelas dulu, lalu mapel yang diajarkan guru itu di kelas tsb). ADMIN: dua
 * Select independen dari seluruh `/api/classes` × `/api/subjects`.
 */
export function GradingPage() {
  const { data: me } = useMe();
  const { data: status, isLoading: statusLoading, isError: statusError, refetch: refetchStatus } = useGradingStatus();
  const [searchParams, setSearchParams] = useSearchParams();
  const location = useLocation();

  const classId = searchParams.get('class_id') ?? '';
  const subjectId = searchParams.get('subject_id') ?? '';

  const isAdmin = me?.role === 'admin_sekolah';
  const isGuru = me?.role === 'guru';
  const isKepsek = me?.role === 'kepala_sekolah';
  const allowed = isAdmin || isGuru || isKepsek;
  /** Kepsek: baca-saja lintas kelas, hanya tab Rekap & Rapor (lihat `GradingContext.readOnly`). */
  const canManageComponents = isAdmin || isGuru;

  const { data: adminClasses } = useClasses();
  const { data: adminSubjects } = useSubjects();
  const { data: guruSlots } = useScheduleSlots({}, isGuru);

  const guruClasses: ClassRef[] = useMemo(
    () => uniqueBy((guruSlots ?? []).map((s) => s.class), (c) => c.id),
    [guruSlots],
  );
  const guruSubjectsForClass: ScheduleSlotSubjectRef[] = useMemo(
    () => uniqueBy((guruSlots ?? []).filter((s) => s.class.id === classId).map((s) => s.subject), (s) => s.id),
    [guruSlots, classId],
  );

  const classOptions = isGuru ? guruClasses : (adminClasses ?? []);
  const subjectOptions = isGuru ? guruSubjectsForClass : (adminSubjects ?? []);

  function updateContext(next: { class_id?: string; subject_id?: string }) {
    const params = new URLSearchParams(searchParams);
    Object.entries(next).forEach(([key, value]) => {
      if (value) params.set(key, value);
      else params.delete(key);
    });
    setSearchParams(params, { replace: true });
  }

  // Guru: kalau kelas belum dipilih tapi slot sudah termuat, langsung pilihkan
  // kelas+mapel pertama (kasus umum: guru cuma pegang satu-dua kombinasi).
  useEffect(() => {
    if (!isGuru || classId || guruClasses.length === 0) return;
    const firstClass = guruClasses[0];
    const firstSubject = (guruSlots ?? []).find((s) => s.class.id === firstClass.id)?.subject;
    updateContext({ class_id: firstClass.id, subject_id: firstSubject?.id });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [isGuru, classId, guruClasses, guruSlots]);

  const search = searchParams.toString();
  const qs = search ? `?${search}` : '';
  const TAB_ITEMS: TabItem[] = [
    ...(canManageComponents
      ? [
          { to: `/nilai${qs}`, label: 'Komponen', end: true },
          { to: `/nilai/input${qs}`, label: 'Input Nilai' },
        ]
      : []),
    { to: `/nilai/rekap${qs}`, label: 'Rekap' },
    { to: `/nilai/rapor${qs}`, label: 'Rapor' },
  ];

  if (!me) return null;
  if (!allowed) return <Navigate to="/" replace />;
  // Kepsek tidak punya tab Komponen/Input Nilai — kalau nyasar ke sana lewat
  // URL langsung, lempar ke Rekap (tab pertama yang tersedia untuknya).
  if (isKepsek && (location.pathname === '/nilai' || location.pathname === '/nilai/input')) {
    return <Navigate to={`/nilai/rekap${qs}`} replace />;
  }

  return (
    <div className={`${PAGE_WIDE} flex flex-col gap-5`}>
      <div>
        <p className="text-[11px] font-semibold uppercase tracking-[0.1em] text-muted">Penilaian</p>
        <h1 className="text-[21px] font-semibold text-ink">Nilai</h1>
      </div>

      {statusLoading ? (
        <Skeleton className="h-52 w-full" />
      ) : statusError ? (
        <ErrorState message="Gagal memuat status modul penilaian." onRetry={() => refetchStatus()} />
      ) : !status?.enabled ? (
        <EmptyState icon={GraduationCap} message="Modul penilaian tidak aktif." />
      ) : (
        <>
          <div className="flex flex-col gap-3 sm:flex-row">
            <Field label="Kelas" className="sm:w-56">
              <Select
                value={classId}
                onChange={(e) => updateContext({ class_id: e.target.value, subject_id: '' })}
                aria-label="Pilih kelas"
              >
                <option value="">Pilih kelas...</option>
                {classOptions.map((c) => (
                  <option key={c.id} value={c.id}>
                    {c.name}
                  </option>
                ))}
              </Select>
            </Field>
            <Field label="Mata pelajaran" className="sm:w-56">
              <Select
                value={subjectId}
                onChange={(e) => updateContext({ subject_id: e.target.value })}
                disabled={isGuru && !classId}
                aria-label="Pilih mata pelajaran"
              >
                <option value="">Pilih mapel...</option>
                {subjectOptions.map((s) => (
                  <option key={s.id} value={s.id}>
                    {s.name}
                  </option>
                ))}
              </Select>
            </Field>
          </div>

          <Tabs items={TAB_ITEMS} />

          <div className="pb-8">
            <Outlet context={{ classId, subjectId, readOnly: isKepsek } satisfies GradingContext} />
          </div>
        </>
      )}
    </div>
  );
}
