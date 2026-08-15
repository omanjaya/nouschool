import { Navigate, Route, Routes, useNavigate } from 'react-router-dom';
import { useLogout, useMe } from './features/auth/api';
import { LoginPage } from './features/auth/LoginPage';
import { RequireLogin } from './features/auth/RequireLogin';
import { AppShell } from './components/ui/AppShell';
import { getNavItems } from './lib/nav';
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
import { TeachersListPage } from './features/teachers/TeachersListPage';
import { SubjectsListPage } from './features/subjects/SubjectsListPage';
import { ImportWizard } from './features/import/ImportWizard';
import { AttendanceClassesPage } from './features/attendance/AttendanceClassesPage';
import { AttendanceSessionPage } from './features/attendance/AttendanceSessionPage';
import { AttendanceRecapPage } from './features/attendance/AttendanceRecapPage';
import { AttendanceHistoryPage } from './features/attendance/AttendanceHistoryPage';
import { Button } from './components/ui/Button';
import type { Me } from './lib/types';

function getGreeting(hour: number) {
  if (hour < 11) return 'Selamat pagi';
  if (hour < 15) return 'Selamat siang';
  if (hour < 18) return 'Selamat sore';
  return 'Selamat malam';
}

function BerandaPage({ me }: { me: Me }) {
  const greeting = getGreeting(new Date().getHours());
  const navigate = useNavigate();
  const canWriteAttendance = me.role === 'admin_sekolah';
  const canViewRecap = me.role === 'admin_sekolah' || me.role === 'kepala_sekolah';

  return (
    <div className="mx-auto flex max-w-[640px] flex-col gap-6 px-5 py-6">
      <div>
        <p className="text-[11px] font-semibold uppercase tracking-[0.1em] text-muted">Beranda</p>
        <h1 className="text-[21px] font-semibold text-ink">
          {greeting}, {me.name}
        </h1>
      </div>

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
      ) : (
        <Card>
          <p className="text-[14px] text-ink">Fase 1 — belum ada modul lain untuk ditampilkan di sini.</p>
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

  // RequireLogin sudah menjamin me ada & sukses saat komponen ini dirender.
  if (!me) return null;

  const isPlatformAdmin = me.is_super_admin && !me.school;
  const navItems = getNavItems(me);

  async function handleLogout() {
    await logout.mutateAsync();
    navigate('/login', { replace: true });
  }

  return (
    <AppShell navItems={navItems} userName={me.name} onLogout={handleLogout}>
      <Routes>
        <Route path="/" element={isPlatformAdmin ? <Navigate to="/admin" replace /> : <BerandaPage me={me} />} />
        <Route path="/admin" element={<SchoolsListPage />} />
        <Route path="/admin/schools/:id" element={<SchoolDetailPage />} />
        <Route path="/pengaturan" element={<SettingsPage />} />
        <Route path="/profil" element={<ProfilePage />} />

        <Route path="/absensi" element={<AttendanceClassesPage />} />
        <Route path="/absensi/sesi/:id" element={<AttendanceSessionPage />} />
        <Route path="/absensi/rekap" element={<AttendanceRecapPage />} />
        <Route path="/kehadiran" element={<AttendanceHistoryPage />} />

        <Route path="/data" element={<DataLayout />}>
          <Route index element={<Navigate to="siswa" replace />} />
          <Route path="siswa" element={<StudentsListPage />} />
          <Route path="rombel" element={<ClassesListPage />} />
          <Route path="guru" element={<TeachersListPage />} />
          <Route path="mapel" element={<SubjectsListPage />} />
        </Route>
        <Route path="/data/siswa/import" element={<ImportWizard entity="students" backTo="/data/siswa" />} />
        <Route path="/data/siswa/:id" element={<StudentDetailPage />} />
        <Route path="/data/rombel/:id" element={<ClassDetailPage />} />
        <Route path="/data/guru/import" element={<ImportWizard entity="teachers" backTo="/data/guru" />} />

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
