import { useEffect, useState } from 'react';
import { useQueryClient, type QueryClient } from '@tanstack/react-query';
import { Navigate, Route, Routes, useNavigate } from 'react-router-dom';
import { Megaphone, MonitorPlay, QrCode, MapPin, ReceiptText, ShieldAlert } from 'lucide-react';
import { formatTimeOfDay } from './lib/date';
import { hasFeature } from './lib/features';
import { useRealtimeConnection, useRealtimeEvents, useRealtimeState } from './lib/realtime';
import { useLogout, useMe, ME_QUERY_KEY } from './features/auth/api';
import { LoginPage } from './features/auth/LoginPage';
import { RequireLogin } from './features/auth/RequireLogin';
import { AppShell } from './components/ui/AppShell';
import { getNavItems } from './lib/nav';
import { getGreeting } from './lib/greeting';
import { Card } from './components/ui/Card';
import { DashboardAdminPage } from './features/admin/DashboardAdminPage';
import { SchoolsListPage } from './features/admin/SchoolsListPage';
import { SchoolDetailPage } from './features/admin/SchoolDetailPage';
import { SchoolOnboardingWizard } from './features/admin/SchoolOnboardingWizard';
import { InterestLeadsPage } from './features/admin/InterestLeadsPage';
import { OutboxPage } from './features/admin/OutboxPage';
import { PlatformAnnouncementsPage } from './features/admin/PlatformAnnouncementsPage';
import { RevenuePage } from './features/admin/RevenuePage';
import { SettingsPage } from './features/settings/SettingsPage';
import { ProfilePage } from './features/profile/ProfilePage';
import { ActivationPage } from './features/activation/ActivationPage';
import { ImpersonatePage } from './features/impersonate/ImpersonatePage';
import { DataLayout } from './features/data/DataLayout';
import { StudentsListPage } from './features/students/StudentsListPage';
import { StudentDetailPage } from './features/students/StudentDetailPage';
import { ClassesListPage } from './features/classes/ClassesListPage';
import { ClassDetailPage } from './features/classes/ClassDetailPage';
import { ClassQrCardsPage } from './features/classes/ClassQrCardsPage';
import { TeachersListPage } from './features/teachers/TeachersListPage';
import { SubjectsListPage } from './features/subjects/SubjectsListPage';
import { ImportWizard } from './features/import/ImportWizard';
import { AttendanceClassesPage } from './features/attendance/AttendanceClassesPage';
import { AttendanceSessionPage } from './features/attendance/AttendanceSessionPage';
import { AttendanceRecapPage } from './features/attendance/AttendanceRecapPage';
import { AttendanceHistoryPage } from './features/attendance/AttendanceHistoryPage';
import { CheckInPage } from './features/attendance/CheckInPage';
import {
  useSelfCheckinStatus,
  attendanceSessionQueryKey,
  ATTENDANCE_CLASSES_KEY,
  ATTENDANCE_SLOTS_TODAY_KEY,
  ATTENDANCE_SUMMARY_KEY,
  ATTENDANCE_ANOMALIES_KEY,
} from './features/attendance/api';
import { LeavePage } from './features/leave/LeavePage';
import { LeaveDetailPage } from './features/leave/LeaveDetailPage';
import { LeaveApprovalsPage } from './features/leave/LeaveApprovalsPage';
import { LeaveApprovalDetailPage } from './features/leave/LeaveApprovalDetailPage';
import { LeaveRecapPage } from './features/leave/LeaveRecapPage';
import { LEAVE_REQUESTS_KEY, LEAVE_APPROVALS_KEY, LEAVE_SUMMARY_KEY } from './features/leave/api';
import { DisciplinePage } from './features/discipline/DisciplinePage';
import { DisciplineRecordsPage } from './features/discipline/DisciplineRecordsPage';
import { DisciplineRecapPage } from './features/discipline/DisciplineRecapPage';
import { DisciplineLettersPage } from './features/discipline/DisciplineLettersPage';
import { DisciplineSettingsPage } from './features/discipline/DisciplineSettingsPage';
import { MyDisciplinePage } from './features/discipline/MyDisciplinePage';
import {
  DISCIPLINE_RECORDS_KEY,
  DISCIPLINE_SUMMARY_KEY,
  DISCIPLINE_LETTERS_KEY,
  STUDENT_DISCIPLINE_KEY,
} from './features/discipline/api';
import { Button } from './components/ui/Button';
import { PeriodsPage } from './features/schedule/PeriodsPage';
import { RoomsPage } from './features/schedule/RoomsPage';
import { RoomsPrintPage } from './features/schedule/RoomsPrintPage';
import { ScheduleBuilderPage } from './features/schedule/ScheduleBuilderPage';
import { SchedulePage } from './features/schedule/SchedulePage';
import { SCHEDULE_SLOTS_QUERY_KEY, PERIODS_QUERY_KEY, ROOMS_QUERY_KEY } from './features/schedule/api';
import { ScanPage } from './features/teaching/ScanPage';
import { JournalsPage } from './features/teaching/JournalsPage';
import { MonitoringPage } from './features/teaching/MonitoringPage';
import { ComplianceRecapPage } from './features/teaching/ComplianceRecapPage';
import { TEACHING_STATUS_QUERY_KEY, TEACHING_COMPLIANCE_QUERY_KEY } from './features/teaching/api';
import { AnnouncementsPage } from './features/announcements/AnnouncementsPage';
import { ANNOUNCEMENTS_QUERY_KEY } from './features/announcements/api';
import { TvPage } from './features/tv/TvPage';
import { TV_BOARD_QUERY_KEY } from './features/tv/api';
import { KepsekHomePage } from './features/dashboard/KepsekHomePage';
import { NotificationsPage } from './features/notifications/NotificationsPage';
import { PushPromptBanner } from './features/notifications/PushPromptBanner';
import { useUnreadNotificationCount, NOTIFICATIONS_QUERY_KEY } from './features/notifications/api';
import { BillingPage } from './features/billing/BillingPage';
import { SubscriptionBanner } from './features/billing/SubscriptionBanner';
import { BILLING_QUERY_KEY } from './features/billing/api';
import { PlansPage } from './features/admin/PlansPage';
import { LandingPage } from './features/landing/LandingPage';
import { usePublicContext } from './features/branding/api';
import { useAppBranding } from './lib/useAppBranding';
import { Skeleton } from './components/ui/Skeleton';
import { STUDENTS_QUERY_KEY } from './features/students/api';
import { CLASSES_QUERY_KEY } from './features/classes/api';
import { TEACHERS_QUERY_KEY } from './features/teachers/api';
import { SUBJECTS_QUERY_KEY } from './features/subjects/api';
import type { Me } from './lib/types';

/**
 * Kartu "Check-in Kehadiran" di Beranda siswa — muncul hanya kalau sekolah
 * mengaktifkan metode self_checkin DAN fitur `self_checkin` aktif di
 * langganan (Fase 10, docs/09-billing.md "Feature gating").
 */
function SelfCheckinCard({ me }: { me: Me }) {
  const navigate = useNavigate();
  const featureOn = hasFeature(me, 'self_checkin');
  const { data } = useSelfCheckinStatus(featureOn);

  if (!featureOn || !data || !data.enabled) return null;

  return (
    <Card className="flex flex-col gap-3">
      <div>
        <p className="text-[14px] font-semibold text-ink">Check-in Kehadiran</p>
        <p className="text-[12px] text-muted">
          {data.today
            ? `Anda sudah check-in pukul ${formatTimeOfDay(data.today.checked_at)}.`
            : 'Absen datang mandiri lewat lokasi HP Anda.'}
        </p>
      </div>
      <div className="flex flex-wrap gap-2">
        <Button onClick={() => navigate('/checkin')}>
          <MapPin size={16} strokeWidth={2} aria-hidden="true" />
          {data.today ? 'Lihat Check-in' : 'Check-in Sekarang'}
        </Button>
      </div>
    </Card>
  );
}

function BerandaPage({ me }: { me: Me }) {
  const greeting = getGreeting(new Date().getHours());
  const navigate = useNavigate();
  const canWriteAttendance = me.role === 'admin_sekolah';
  const canViewRecap = me.role === 'admin_sekolah' || me.role === 'kepala_sekolah';
  // guru & kepala_sekolah sudah punya item nav "Izin" — kartu ini terutama untuk
  // admin_sekolah yang belum punya slot nav untuk /izin (lihat lib/nav.ts).
  const canManageLeave = me.role === 'admin_sekolah';
  const isStaff = me.role !== 'siswa' && me.role !== 'orang_tua';
  // Kartu jalan pintas ke /jadwal — guru & siswa belum tentu punya item nav
  // ke /jadwal (hanya siswa yang punya, lihat lib/nav.ts), jadi kartu ini
  // jadi entry point untuk guru; untuk siswa tetap ditampilkan sebagai jalan pintas.
  const canViewOwnSchedule = me.role === 'guru' || me.role === 'siswa';
  // Kartu jalan pintas ke scan QR ruangan (Fase 6, docs/06-teaching.md) — hanya
  // guru yang mengajar bisa memindai QR untuk membuka jurnal + absensi.
  const canScanQr = me.role === 'guru';
  // Kartu monitoring status mengajar guru — untuk peran yang mengawasi jalannya
  // KBM, bukan guru sendiri.
  const canViewMonitoring = me.role === 'kepala_sekolah' || me.role === 'admin_sekolah';
  // Kartu kelola pengumuman TV/beranda — kepsek sudah punya jalan pintas ini di
  // KepsekHomePage (lihat AuthenticatedShell), jadi kartu Beranda ini khusus admin.
  const canManageAnnouncements = me.role === 'admin_sekolah';
  // Kartu check-in mandiri (Fase 8) — hanya siswa, dan hook di dalamnya
  // sendiri yang memutuskan tampil/tidak berdasar `self_checkin` aktif.
  const isSiswa = me.role === 'siswa';
  // Kartu Tagihan & Langganan (Fase 10) — kepala_sekolah punya jalan pintas
  // setara di KepsekHomePage (QUICK_LINKS), jadi kartu Beranda ini khusus admin.
  const canManageBilling = me.role === 'admin_sekolah';
  // Kartu Kedisiplinan (Fase 14 Gelombang A) — kepala_sekolah punya jalan
  // pintas setara di KepsekHomePage (QUICK_LINKS), jadi kartu Beranda ini
  // untuk guru & admin_sekolah (keduanya bisa mencatat pelanggaran).
  const canManageDiscipline = me.role === 'guru' || me.role === 'admin_sekolah';
  // Kartu "Poin Kedisiplinan" — siswa (miliknya sendiri) & orang tua (anak).
  const isSiswaOrOrtu = me.role === 'siswa' || me.role === 'orang_tua';

  return (
    <div className="mx-auto flex max-w-[640px] flex-col gap-6 px-5 py-6">
      <div>
        <p className="text-[11px] font-semibold uppercase tracking-[0.1em] text-muted">Beranda</p>
        <h1 className="text-[21px] font-semibold text-ink">
          {greeting}, {me.name}
        </h1>
      </div>

      <PushPromptBanner />

      {isSiswa && <SelfCheckinCard me={me} />}

      {canScanQr && (
        <Card className="flex flex-col gap-3">
          <div>
            <p className="text-[14px] font-semibold text-ink">Scan QR Kelas</p>
            <p className="text-[12px] text-muted">Satu scan: jurnal mengajar terisi + absensi siswa siap diisi.</p>
          </div>
          <div className="flex flex-wrap gap-2">
            <Button onClick={() => navigate('/scan')}>
              <QrCode size={16} strokeWidth={2} aria-hidden="true" />
              Scan QR Ruangan
            </Button>
          </div>
        </Card>
      )}

      {canViewMonitoring && (
        <Card className="flex flex-col gap-3">
          <div>
            <p className="text-[14px] font-semibold text-ink">Monitoring Guru</p>
            <p className="text-[12px] text-muted">Lihat guru yang sedang mengajar, izin, atau belum masuk kelas.</p>
          </div>
          <div className="flex flex-wrap gap-2">
            <Button variant="secondary" onClick={() => navigate('/monitoring')}>
              Buka Monitoring
            </Button>
          </div>
        </Card>
      )}

      {canWriteAttendance || canViewRecap ? (
        <Card className="flex flex-col gap-3">
          <div>
            <p className="text-[14px] font-semibold text-ink">Absensi Hari Ini</p>
            <p className="text-[12px] text-muted">Buka absensi rombel atau lihat rekap kehadiran hari ini.</p>
          </div>
          <div className="flex flex-wrap gap-2">
            {canWriteAttendance && (
              <Button variant="secondary" onClick={() => navigate('/absensi')}>
                Buka Absensi
              </Button>
            )}
            {canViewRecap && (
              <Button variant="secondary" onClick={() => navigate('/absensi/rekap')}>
                Lihat Rekap
              </Button>
            )}
          </div>
        </Card>
      ) : null}

      {isStaff && (
        <Card className="flex flex-col gap-3">
          <div>
            <p className="text-[14px] font-semibold text-ink">Izin</p>
            <p className="text-[12px] text-muted">
              {canManageLeave ? 'Ajukan izin atau lihat rekap izin guru.' : 'Ajukan izin atau lihat status pengajuan Anda.'}
            </p>
          </div>
          <div className="flex flex-wrap gap-2">
            <Button variant="secondary" onClick={() => navigate('/izin')}>
              Ajukan Izin
            </Button>
            {canManageLeave && (
              <Button variant="secondary" onClick={() => navigate('/izin/rekap')}>
                Rekap Izin
              </Button>
            )}
          </div>
        </Card>
      )}

      {canViewOwnSchedule && (
        <Card className="flex flex-col gap-3">
          <div>
            <p className="text-[14px] font-semibold text-ink">Jadwal Hari Ini</p>
            <p className="text-[12px] text-muted">
              {me.role === 'guru' ? 'Lihat jadwal mengajar Anda hari ini.' : 'Lihat jadwal pelajaran Anda hari ini.'}
            </p>
          </div>
          <div className="flex flex-wrap gap-2">
            <Button variant="secondary" onClick={() => navigate('/jadwal')}>
              Lihat Jadwal
            </Button>
          </div>
        </Card>
      )}

      {canManageDiscipline && (
        <Card className="flex flex-col gap-3">
          <div>
            <p className="text-[14px] font-semibold text-ink">Kedisiplinan</p>
            <p className="text-[12px] text-muted">Catat pelanggaran siswa & lihat rekap poin kedisiplinan.</p>
          </div>
          <div className="flex flex-wrap gap-2">
            <Button variant="secondary" onClick={() => navigate('/kedisiplinan')}>
              <ShieldAlert size={16} strokeWidth={2} aria-hidden="true" />
              Buka Kedisiplinan
            </Button>
          </div>
        </Card>
      )}

      {isSiswaOrOrtu && (
        <Card className="flex flex-col gap-3">
          <div>
            <p className="text-[14px] font-semibold text-ink">Poin Kedisiplinan</p>
            <p className="text-[12px] text-muted">
              {me.role === 'siswa'
                ? 'Lihat riwayat pelanggaran & Surat Peringatan Anda.'
                : 'Lihat riwayat pelanggaran & Surat Peringatan anak Anda.'}
            </p>
          </div>
          <div className="flex flex-wrap gap-2">
            <Button variant="secondary" onClick={() => navigate('/pelanggaran-saya')}>
              <ShieldAlert size={16} strokeWidth={2} aria-hidden="true" />
              Lihat Poin Kedisiplinan
            </Button>
          </div>
        </Card>
      )}

      {canManageAnnouncements && (
        <Card className="flex flex-col gap-3">
          <div>
            <p className="text-[14px] font-semibold text-ink">Pengumuman</p>
            <p className="text-[12px] text-muted">Kelola pengumuman yang tampil di dashboard TV & beranda.</p>
          </div>
          <div className="flex flex-wrap gap-2">
            <Button variant="secondary" onClick={() => navigate('/pengumuman')}>
              <Megaphone size={16} strokeWidth={2} aria-hidden="true" />
              Kelola Pengumuman
            </Button>
          </div>
        </Card>
      )}

      {canManageBilling && (
        <Card className="flex flex-col gap-3">
          <div>
            <p className="text-[14px] font-semibold text-ink">Tagihan & Langganan</p>
            <p className="text-[12px] text-muted">Lihat status langganan, riwayat tagihan, & pembayaran.</p>
          </div>
          <div className="flex flex-wrap gap-2">
            <Button variant="secondary" onClick={() => navigate('/tagihan')}>
              <ReceiptText size={16} strokeWidth={2} aria-hidden="true" />
              Buka Tagihan
            </Button>
          </div>
        </Card>
      )}

      {!canWriteAttendance &&
        !canViewRecap &&
        !isStaff &&
        !canViewOwnSchedule &&
        !canManageAnnouncements &&
        !canManageBilling &&
        !isSiswaOrOrtu && (
          <Card>
            <p className="text-[14px] text-ink">Belum ada modul lain untuk ditampilkan di sini.</p>
          </Card>
        )}
    </div>
  );
}

/**
 * Fase 12 (docs/06 dkk, realtime WebSocket, kontrak `GET /api/ws`): mapping
 * event server→client ke `invalidateQueries` — event HANYA sinyal, data
 * sebenarnya tetap diambil lewat REST existing. Tiap query key di bawah
 * dicocokkan satu-satu terhadap yang benar-benar diekspor `features/*\/api.ts`
 * (lihat import di atas), BUKAN ditebak. Dipasang sekali di `AuthenticatedShell`
 * — TV (`TvPage`) sengaja punya invalidasi sendiri yang lebih spesifik
 * (lihat `features/tv/TvPage.tsx`), dan koneksi socket-nya sendiri sudah
 * berjalan lebih tinggi lewat `useRealtimeConnection` di `App()`.
 */
function useRealtime(queryClient: QueryClient) {
  useRealtimeEvents({
    // {session_id, class_id, date} — sesi yang sedang dibuka + daftar rombel
    // (badge jumlah tercatat) + jadwal per-mapel guru hari ini.
    'attendance.session': (event) => {
      queryClient.invalidateQueries({ queryKey: attendanceSessionQueryKey(String(event.data.session_id)) });
      queryClient.invalidateQueries({ queryKey: [ATTENDANCE_CLASSES_KEY] });
      queryClient.invalidateQueries({ queryKey: [ATTENDANCE_SLOTS_TODAY_KEY] });
    },
    // {date} — rekap harian per rombel + papan TV + anomali check-in.
    'attendance.summary': () => {
      queryClient.invalidateQueries({ queryKey: [ATTENDANCE_SUMMARY_KEY] });
      queryClient.invalidateQueries({ queryKey: TV_BOARD_QUERY_KEY });
      queryClient.invalidateQueries({ queryKey: [ATTENDANCE_ANOMALIES_KEY] });
    },
    // {date} — monitoring status mengajar + papan TV + rekap ketertiban mengajar.
    'teaching.status': () => {
      queryClient.invalidateQueries({ queryKey: [TEACHING_STATUS_QUERY_KEY] });
      queryClient.invalidateQueries({ queryKey: TV_BOARD_QUERY_KEY });
      queryClient.invalidateQueries({ queryKey: [TEACHING_COMPLIANCE_QUERY_KEY] });
    },
    // {} — daftar notifikasi + ringkasan unread (badge nav AppShell update instan).
    notification: () => {
      queryClient.invalidateQueries({ queryKey: [NOTIFICATIONS_QUERY_KEY] });
    },
    // {request_id} — daftar izin (mine/all) + antrian approval + rekap izin.
    leave: () => {
      queryClient.invalidateQueries({ queryKey: [LEAVE_REQUESTS_KEY] });
      queryClient.invalidateQueries({ queryKey: LEAVE_APPROVALS_KEY });
      queryClient.invalidateQueries({ queryKey: [LEAVE_SUMMARY_KEY] });
    },
    // {} — pengumuman (CRUD & aktif) + papan TV.
    announcement: () => {
      queryClient.invalidateQueries({ queryKey: [ANNOUNCEMENTS_QUERY_KEY] });
      queryClient.invalidateQueries({ queryKey: TV_BOARD_QUERY_KEY });
    },
    // {} — slot jadwal builder + jadwal hari ini (guru/siswa) + jam & ruangan.
    schedule: () => {
      queryClient.invalidateQueries({ queryKey: [SCHEDULE_SLOTS_QUERY_KEY] });
      queryClient.invalidateQueries({ queryKey: ['schedule-today'] });
      queryClient.invalidateQueries({ queryKey: PERIODS_QUERY_KEY });
      queryClient.invalidateQueries({ queryKey: ROOMS_QUERY_KEY });
    },
    // {} — langganan/invoice + /api/me (subscription & features snapshot dipakai feature gating).
    billing: () => {
      queryClient.invalidateQueries({ queryKey: BILLING_QUERY_KEY });
      queryClient.invalidateQueries({ queryKey: ME_QUERY_KEY });
    },
    // {} — data induk siswa/rombel/guru/mapel (mis. import selesai di device lain).
    students: () => {
      queryClient.invalidateQueries({ queryKey: [STUDENTS_QUERY_KEY] });
      queryClient.invalidateQueries({ queryKey: [CLASSES_QUERY_KEY] });
      queryClient.invalidateQueries({ queryKey: TEACHERS_QUERY_KEY });
      queryClient.invalidateQueries({ queryKey: SUBJECTS_QUERY_KEY });
    },
    // {student_id} — Fase 14 Gelombang A: catatan pelanggaran, rekap poin, Surat
    // Peringatan, & riwayat kedisiplinan siswa terkait (`/pelanggaran-saya`).
    discipline: () => {
      queryClient.invalidateQueries({ queryKey: [DISCIPLINE_RECORDS_KEY] });
      queryClient.invalidateQueries({ queryKey: [DISCIPLINE_SUMMARY_KEY] });
      queryClient.invalidateQueries({ queryKey: [DISCIPLINE_LETTERS_KEY] });
      queryClient.invalidateQueries({ queryKey: [STUDENT_DISCIPLINE_KEY] });
    },
  });
}

/**
 * Garis tipis "Menyambung ulang…" di atas konten (docs/10 — halus, tidak
 * mengganggu). Hanya tampil kalau status `reconnecting` (percobaan ke-2 dst —
 * lihat `RealtimeClient.open`, percobaan PERTAMA statusnya `connecting`)
 * bertahan lebih dari 5 detik, supaya gangguan sekejap tidak membuatnya
 * berkedip dan TIDAK PERNAH muncul saat pemuatan awal.
 */
function RealtimeReconnectBanner() {
  const state = useRealtimeState();
  const [visible, setVisible] = useState(false);

  useEffect(() => {
    if (state !== 'reconnecting') {
      setVisible(false);
      return;
    }
    const id = window.setTimeout(() => setVisible(true), 5000);
    return () => window.clearTimeout(id);
  }, [state]);

  if (!visible) return null;

  return (
    <div
      role="status"
      className="flex items-center justify-center gap-2 bg-st-terlambat-line px-4 py-1.5 text-center text-[11px] font-medium text-st-terlambat"
    >
      Menyambung ulang…
    </div>
  );
}

/**
 * Garis tipis "Mode support" (Fase 12) — tampil hanya untuk sesi impersonation
 * di host TENANT: `me.school` ada (bukan platform admin) DAN `me.is_super_admin`
 * true (super admin sedang "masuk sebagai" sekolah ini, lihat
 * `features/admin/SchoolDetailPage` & `features/impersonate/ImpersonatePage`).
 * Warna `--st-izin` (semantik info, docs/10) — bukan warna baru di luar token.
 */
function ImpersonationBanner({ me, onEndSession }: { me: Me; onEndSession: () => void }) {
  if (!me.school || !me.is_super_admin) return null;

  return (
    <div
      role="status"
      className="flex flex-wrap items-center justify-center gap-x-3 gap-y-1 bg-st-izin-line px-4 py-1.5 text-center text-[11px] font-medium text-st-izin"
    >
      <span>
        Mode support NouSchool — Anda masuk sebagai {me.school.name}. Semua tindakan tercatat.
      </span>
      <button
        type="button"
        onClick={onEndSession}
        className="font-semibold underline underline-offset-2 hover:opacity-80"
      >
        Akhiri Sesi
      </button>
    </div>
  );
}

/** Rute + AppShell untuk pengguna yang sudah login — nav & sapaan mengikuti /api/me. */
function AuthenticatedShell() {
  const { data: me } = useMe();
  const { data: context } = usePublicContext();
  const logout = useLogout();
  const navigate = useNavigate();
  const queryClient = useQueryClient();

  // Hooks harus tetap dipanggil tanpa syarat (react-hooks/rules-of-hooks) —
  // dihitung dari `me` yang mungkin belum ada, baru cabang render (return
  // null / <Navigate>) di bawah setelah semua hook dipanggil.
  const navItems = me ? getNavItems(me) : [];
  // role `display` (dashboard TV) dialihkan ke /tv di bawah tanpa pernah
  // merender AppShell — jangan ikut memicu fetch unread count untuknya.
  const hasNotificationsNav = me?.role !== 'display' && navItems.some((item) => item.to === '/notifikasi');
  const { data: unreadCount } = useUnreadNotificationCount(hasNotificationsNav);
  useRealtime(queryClient);

  // RequireLogin sudah menjamin me ada & sukses saat komponen ini dirender.
  if (!me) return null;

  // Akun display (role TV, docs/06 "Dashboard TV") hanya boleh melihat `/tv`
  // (rute terpisah TANPA AppShell, lihat `App()`) — route lain apa pun di
  // bawah shell ini dialihkan ke sana.
  if (me.role === 'display') {
    return <Navigate to="/tv" replace />;
  }

  const isPlatformAdmin = me.is_super_admin && !me.school;
  const badgeCounts = hasNotificationsNav ? { '/notifikasi': unreadCount ?? 0 } : undefined;
  // Host platform (admin.nouschool.id) tidak punya branding sekolah — AppShell
  // jatuh ke default "NouSchool" (docs/01 §branding).
  const brandName = context && !context.platform ? context.branding.app_name : undefined;
  const brandLogoUrl = context && !context.platform ? context.branding.logo_url : null;

  async function handleLogout() {
    await logout.mutateAsync();
    navigate('/login', { replace: true });
  }

  // Super admin di HOST PLATFORM: hanya rute admin — rute tenant (absensi,
  // jadwal, dst) tidak punya konteks sekolah di host ini (backend juga menolak
  // via allowlist ErrTenantOnlyEndpoint), jadi jangan pernah dirender.
  if (isPlatformAdmin) {
    return (
      <AppShell navItems={navItems} userName={me.name} onLogout={handleLogout}>
        <Routes>
          <Route path="/admin" element={<DashboardAdminPage />} />
          <Route path="/admin/sekolah" element={<SchoolsListPage />} />
          <Route path="/admin/sekolah/new" element={<SchoolOnboardingWizard />} />
          <Route path="/admin/sekolah/:id" element={<SchoolDetailPage />} />
          <Route path="/admin/outbox" element={<OutboxPage />} />
          <Route path="/admin/plans" element={<PlansPage />} />
          <Route path="/admin/minat" element={<InterestLeadsPage />} />
          <Route path="/admin/pengumuman" element={<PlatformAnnouncementsPage />} />
          <Route path="/admin/pendapatan" element={<RevenuePage />} />
          <Route path="/profil" element={<ProfilePage />} />
          <Route path="*" element={<Navigate to="/admin" replace />} />
        </Routes>
      </AppShell>
    );
  }

  return (
    <AppShell
      navItems={navItems}
      userName={me.name}
      onLogout={handleLogout}
      badgeCounts={badgeCounts}
      appName={brandName}
      logoUrl={brandLogoUrl}
    >
      <ImpersonationBanner me={me} onEndSession={handleLogout} />
      <RealtimeReconnectBanner />
      <SubscriptionBanner me={me} />
      <Routes>
        <Route
          path="/"
          element={
            isPlatformAdmin ? (
              <Navigate to="/admin" replace />
            ) : me.role === 'kepala_sekolah' ? (
              <KepsekHomePage me={me} />
            ) : (
              <BerandaPage me={me} />
            )
          }
        />
        {/* Rute admin di bawah ini TIDAK dialihkan ke DashboardAdminPage/OutboxPage baru
            (Fase 13) — hanya escape hatch existing untuk sesi impersonation super admin
            (`me.is_super_admin && me.school`, lihat `ImpersonationBanner`), dipertahankan
            apa adanya, sekadar disamakan path-nya dengan cabang isPlatformAdmin supaya
            tautan "Sekolah" di SchoolDetailPage/PlansPage/InterestLeadsPage tidak putus. */}
        <Route path="/admin" element={<SchoolsListPage />} />
        <Route path="/admin/sekolah" element={<SchoolsListPage />} />
        <Route path="/admin/sekolah/:id" element={<SchoolDetailPage />} />
        <Route path="/admin/plans" element={<PlansPage />} />
        <Route path="/admin/minat" element={<InterestLeadsPage />} />
        <Route path="/tagihan" element={<BillingPage />} />
        <Route path="/pengaturan" element={<SettingsPage />} />
        <Route path="/profil" element={<ProfilePage />} />
        <Route path="/notifikasi" element={<NotificationsPage />} />

        <Route path="/absensi" element={<AttendanceClassesPage />} />
        <Route path="/absensi/sesi/:id" element={<AttendanceSessionPage />} />
        <Route path="/absensi/rekap" element={<AttendanceRecapPage />} />
        <Route path="/kehadiran" element={<AttendanceHistoryPage />} />
        <Route path="/checkin" element={<CheckInPage />} />

        <Route path="/scan" element={<ScanPage />} />
        <Route path="/jurnal" element={<JournalsPage />} />
        <Route path="/monitoring" element={<MonitoringPage />} />
        <Route path="/monitoring/rekap" element={<ComplianceRecapPage />} />
        <Route path="/pengumuman" element={<AnnouncementsPage />} />

        <Route path="/izin" element={<LeavePage />} />
        <Route path="/izin/rekap" element={<LeaveRecapPage />} />
        <Route path="/izin/persetujuan" element={<LeaveApprovalsPage />} />
        <Route path="/izin/persetujuan/:stepId" element={<LeaveApprovalDetailPage />} />
        <Route path="/izin/:id" element={<LeaveDetailPage />} />

        <Route path="/kedisiplinan" element={<DisciplinePage />}>
          <Route index element={<DisciplineRecordsPage />} />
          <Route path="rekap" element={<DisciplineRecapPage />} />
          <Route path="surat" element={<DisciplineLettersPage />} />
        </Route>
        <Route path="/kedisiplinan/pengaturan" element={<DisciplineSettingsPage />} />
        <Route path="/pelanggaran-saya" element={<MyDisciplinePage />} />

        <Route path="/data" element={<DataLayout />}>
          <Route index element={<Navigate to="siswa" replace />} />
          <Route path="siswa" element={<StudentsListPage />} />
          <Route path="rombel" element={<ClassesListPage />} />
          <Route path="guru" element={<TeachersListPage />} />
          <Route path="mapel" element={<SubjectsListPage />} />
          <Route path="jadwal" element={<ScheduleBuilderPage />} />
          <Route path="jam" element={<PeriodsPage />} />
          <Route path="ruangan" element={<RoomsPage />} />
        </Route>
        <Route path="/data/siswa/import" element={<ImportWizard entity="students" backTo="/data/siswa" />} />
        <Route
          path="/data/siswa/import-dapodik"
          element={<ImportWizard entity="dapodik" backTo="/data/siswa" />}
        />
        <Route path="/data/siswa/:id" element={<StudentDetailPage />} />
        <Route path="/data/rombel/:id" element={<ClassDetailPage />} />
        <Route path="/data/rombel/:id/kartu-qr" element={<ClassQrCardsPage />} />
        <Route path="/data/guru/import" element={<ImportWizard entity="teachers" backTo="/data/guru" />} />
        <Route path="/data/jadwal/import" element={<ImportWizard entity="schedule" backTo="/data/jadwal" />} />
        <Route path="/data/ruangan/cetak" element={<RoomsPrintPage />} />

        <Route path="/jadwal" element={<SchedulePage />} />

        <Route path="*" element={<Navigate to="/" replace />} />
      </Routes>
    </AppShell>
  );
}

/**
 * Guard fitur `tv_dashboard` (Fase 10) di depan `/tv` — sekolah tanpa fitur
 * ini (mis. plan Basic) melihat pesan alih-alih papan TV kosong/error backend.
 * `me` sudah pasti ada di sini (dibungkus `RequireLogin`).
 */
function TvRouteGuard() {
  const { data: me } = useMe();
  if (!me) return null;
  if (!hasFeature(me, 'tv_dashboard')) {
    return (
      <div className="flex min-h-dvh flex-col items-center justify-center gap-3 bg-bg px-6 text-center">
        <MonitorPlay size={24} strokeWidth={2} className="text-muted" aria-hidden="true" />
        <p className="text-[14px] text-muted">Dashboard TV tidak aktif untuk sekolah ini.</p>
      </div>
    );
  }
  return <TvPage />;
}

/** Placeholder layar penuh saat status login/branding awal belum diketahui — bukan spinner fullscreen (docs/10 #7). */
function BootSkeleton() {
  return (
    <div className="flex min-h-dvh flex-col items-center justify-center gap-3 bg-bg px-6">
      <Skeleton className="h-6 w-40" />
      <Skeleton className="h-4 w-56" />
    </div>
  );
}

function App() {
  // Dipanggil di sini (bukan di dalam satu rute) supaya berjalan paralel dengan
  // `useMe()` sejak app dimuat — berlaku juga di layar publik (docs/01 §branding).
  const branding = useAppBranding();
  const { data: me, isLoading: meLoading } = useMe();
  // Fase 12: koneksi WebSocket dipasang di titik paling atas ini (bukan di
  // `AuthenticatedShell`) supaya role `display` (dialihkan ke `/tv` tanpa
  // pernah merender shell itu) tetap tersambung — lihat komentar di
  // `useRealtimeConnection`. Hanya host TENANT (`me.school` ada) yang punya
  // endpoint ini; host platform/super admin sengaja TIDAK connect.
  useRealtimeConnection(Boolean(me?.school));

  // Tunggu keduanya selesai sekali di awal supaya rute "/" bisa memutuskan
  // landing page vs. app biasa tanpa race (lihat `showLanding` di bawah).
  if (branding.isLoading || meLoading) {
    return <BootSkeleton />;
  }

  // Landing page publik hanya di host platform (context.platform === true)
  // DAN pengunjung belum login — selain itu "/" tetap masuk alur biasa
  // (RequireLogin → /login, atau AuthenticatedShell kalau sudah login).
  const showLanding = branding.data?.platform === true && !me;

  return (
    <Routes>
      <Route path="/login" element={<LoginPage />} />
      <Route path="/aktivasi" element={<ActivationPage />} />
      {/* Fase 12 (sesi support super admin) — layar penuh tanpa AppShell, sibling
          `/aktivasi`; publik karena backend mengesahkan lewat token sekali pakai,
          bukan sesi yang sudah login. */}
      <Route path="/impersonate" element={<ImpersonatePage />} />
      {/* Fullscreen, TANPA AppShell (docs/10-design-system.md #5: "layout terpisah /tv") — sibling
          rute, bukan lewat AuthenticatedShell, supaya sidebar/bottom-nav tidak pernah ikut render. */}
      <Route
        path="/tv"
        element={
          <RequireLogin>
            <TvRouteGuard />
          </RequireLogin>
        }
      />
      {showLanding && <Route path="/" element={<LandingPage />} />}
      <Route
        path="/*"
        element={
          <RequireLogin>
            <AuthenticatedShell />
          </RequireLogin>
        }
      />
    </Routes>
  );
}

export default App;
