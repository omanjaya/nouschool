import { Navigate, Route, Routes, useNavigate } from 'react-router-dom';
import { Megaphone, QrCode, MapPin } from 'lucide-react';
import { formatTimeOfDay } from './lib/date';
import { useLogout, useMe } from './features/auth/api';
import { LoginPage } from './features/auth/LoginPage';
import { RequireLogin } from './features/auth/RequireLogin';
import { AppShell } from './components/ui/AppShell';
import { getNavItems } from './lib/nav';
import { getGreeting } from './lib/greeting';
import { Card } from './components/ui/Card';
import { SchoolsListPage } from './features/admin/SchoolsListPage';
import { SchoolDetailPage } from './features/admin/SchoolDetailPage';
import { SettingsPage } from './features/settings/SettingsPage';
import { ProfilePage } from './features/profile/ProfilePage';
import { ActivationPage } from './features/activation/ActivationPage';
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
import { useSelfCheckinStatus } from './features/attendance/api';
import { LeavePage } from './features/leave/LeavePage';
import { LeaveDetailPage } from './features/leave/LeaveDetailPage';
import { LeaveApprovalsPage } from './features/leave/LeaveApprovalsPage';
import { LeaveApprovalDetailPage } from './features/leave/LeaveApprovalDetailPage';
import { LeaveRecapPage } from './features/leave/LeaveRecapPage';
import { Button } from './components/ui/Button';
import { PeriodsPage } from './features/schedule/PeriodsPage';
import { RoomsPage } from './features/schedule/RoomsPage';
import { RoomsPrintPage } from './features/schedule/RoomsPrintPage';
import { ScheduleBuilderPage } from './features/schedule/ScheduleBuilderPage';
import { SchedulePage } from './features/schedule/SchedulePage';
import { ScanPage } from './features/teaching/ScanPage';
import { JournalsPage } from './features/teaching/JournalsPage';
import { MonitoringPage } from './features/teaching/MonitoringPage';
import { ComplianceRecapPage } from './features/teaching/ComplianceRecapPage';
import { AnnouncementsPage } from './features/announcements/AnnouncementsPage';
import { TvPage } from './features/tv/TvPage';
import { KepsekHomePage } from './features/dashboard/KepsekHomePage';
import { NotificationsPage } from './features/notifications/NotificationsPage';
import { PushPromptBanner } from './features/notifications/PushPromptBanner';
import { useUnreadNotificationCount } from './features/notifications/api';
import type { Me } from './lib/types';

/** Kartu "Check-in Kehadiran" di Beranda siswa — muncul hanya kalau sekolah mengaktifkan metode self_checkin. */
function SelfCheckinCard() {
  const navigate = useNavigate();
  const { data } = useSelfCheckinStatus(true);

  if (!data || !data.enabled) return null;

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

  return (
    <div className="mx-auto flex max-w-[640px] flex-col gap-6 px-5 py-6">
      <div>
        <p className="text-[11px] font-semibold uppercase tracking-[0.1em] text-muted">Beranda</p>
        <h1 className="text-[21px] font-semibold text-ink">
          {greeting}, {me.name}
        </h1>
      </div>

      <PushPromptBanner />

      {isSiswa && <SelfCheckinCard />}

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

      {!canWriteAttendance && !canViewRecap && !isStaff && !canViewOwnSchedule && !canManageAnnouncements && (
        <Card>
          <p className="text-[14px] text-ink">Belum ada modul lain untuk ditampilkan di sini.</p>
        </Card>
      )}
    </div>
  );
}

/** Rute + AppShell untuk pengguna yang sudah login — nav & sapaan mengikuti /api/me. */
function AuthenticatedShell() {
  const { data: me } = useMe();
  const logout = useLogout();
  const navigate = useNavigate();

  // Hooks harus tetap dipanggil tanpa syarat (react-hooks/rules-of-hooks) —
  // dihitung dari `me` yang mungkin belum ada, baru cabang render (return
  // null / <Navigate>) di bawah setelah semua hook dipanggil.
  const navItems = me ? getNavItems(me) : [];
  // role `display` (dashboard TV) dialihkan ke /tv di bawah tanpa pernah
  // merender AppShell — jangan ikut memicu fetch unread count untuknya.
  const hasNotificationsNav = me?.role !== 'display' && navItems.some((item) => item.to === '/notifikasi');
  const { data: unreadCount } = useUnreadNotificationCount(hasNotificationsNav);

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

  async function handleLogout() {
    await logout.mutateAsync();
    navigate('/login', { replace: true });
  }

  return (
    <AppShell navItems={navItems} userName={me.name} onLogout={handleLogout} badgeCounts={badgeCounts}>
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
        <Route path="/admin" element={<SchoolsListPage />} />
        <Route path="/admin/schools/:id" element={<SchoolDetailPage />} />
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

function App() {
  return (
    <Routes>
      <Route path="/login" element={<LoginPage />} />
      <Route path="/aktivasi" element={<ActivationPage />} />
      {/* Fullscreen, TANPA AppShell (docs/10-design-system.md #5: "layout terpisah /tv") — sibling
          rute, bukan lewat AuthenticatedShell, supaya sidebar/bottom-nav tidak pernah ikut render. */}
      <Route
        path="/tv"
        element={
          <RequireLogin>
            <TvPage />
          </RequireLogin>
        }
      />
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
