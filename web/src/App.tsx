import { Routes, Route } from 'react-router-dom';
import { CalendarX } from 'lucide-react';
import { AppShell } from './components/ui/AppShell';
import { Card } from './components/ui/Card';
import { EmptyState } from './components/ui/EmptyState';

function getGreeting(hour: number) {
  if (hour < 11) return 'Selamat pagi';
  if (hour < 15) return 'Selamat siang';
  if (hour < 18) return 'Selamat sore';
  return 'Selamat malam';
}

function BerandaPage() {
  const greeting = getGreeting(new Date().getHours());

  return (
    <div className="mx-auto flex max-w-[640px] flex-col gap-6 px-5 py-6">
      <div>
        <p className="text-[11px] font-semibold uppercase tracking-[0.1em] text-muted">Beranda</p>
        <h1 className="text-[21px] font-semibold text-ink">{greeting}</h1>
      </div>

      <Card>
        <p className="text-[14px] text-ink">Fase 0 — scaffold berhasil</p>
      </Card>

      <Card variant="plain" className="p-0">
        <EmptyState icon={CalendarX} message="Belum ada jadwal untuk ditampilkan" />
      </Card>
    </div>
  );
}

function App() {
  return (
    <AppShell>
      <Routes>
        <Route path="/" element={<BerandaPage />} />
      </Routes>
    </AppShell>
  );
}

export default App;
