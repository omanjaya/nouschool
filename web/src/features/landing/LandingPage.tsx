import { useState, type FormEvent } from 'react';
import { Link } from 'react-router-dom';
import { CalendarDays, ClipboardCheck, Globe, MonitorPlay } from 'lucide-react';
import { Card } from '../../components/ui/Card';
import { Field, Input, Textarea } from '../../components/ui/Field';
import { Button } from '../../components/ui/Button';
import { ApiError } from '../../lib/api';
import { useSubmitInterest } from './api';
import type { InterestLeadInput } from '../../lib/types';

const FORM_ANCHOR_ID = 'minat';

const FEATURES = [
  {
    icon: ClipboardCheck,
    title: 'Absensi multi-metode',
    description: 'Manual, kartu QR, atau check-in lokasi — sekolah pilih yang paling cocok.',
  },
  {
    icon: MonitorPlay,
    title: 'Monitoring guru & TV dashboard',
    description: 'Lihat siapa yang sedang mengajar, belum masuk kelas, atau izin — real-time.',
  },
  {
    icon: CalendarDays,
    title: 'Izin online',
    description: 'Guru mengajukan izin dari HP, alur persetujuan mengikuti struktur sekolah.',
  },
  {
    icon: Globe,
    title: 'Multi-sekolah, custom domain',
    description: 'Tiap sekolah punya alamat sendiri, atau pakai domain milik sekolah sendiri.',
  },
];

function scrollToForm() {
  const el = document.getElementById(FORM_ANCHOR_ID);
  if (!el) return;
  const prefersReducedMotion = window.matchMedia('(prefers-reduced-motion: reduce)').matches;
  el.scrollIntoView({ behavior: prefersReducedMotion ? 'auto' : 'smooth', block: 'start' });
}

/**
 * `/` di host platform, hanya untuk pengunjung belum login (App.tsx). Halaman
 * marketing publik — bukan bagian AppShell, tanpa fetch `/api/me` di sini.
 */
export function LandingPage() {
  return (
    <div className="flex min-h-dvh flex-col bg-bg text-ink">
      <header className="border-b border-line">
        <div className="mx-auto flex max-w-[1120px] items-center justify-between px-5 py-4">
          <span className="text-[16px] font-semibold text-ink">NouSchool</span>
          <Link to="/login" className="text-[13px] font-medium text-primary hover:opacity-80">
            Masuk
          </Link>
        </div>
      </header>

      <main className="flex-1">
        <section className="mx-auto flex max-w-[720px] flex-col items-center gap-5 px-5 py-16 text-center">
          <h1 className="text-[28px] font-semibold leading-tight text-ink">
            Absensi &amp; monitoring sekolah dalam satu aplikasi.
          </h1>
          <p className="max-w-[520px] text-[14px] text-muted">
            NouSchool membantu sekolah mengelola absensi siswa, kehadiran mengajar guru, dan izin — dari satu
            dashboard, tanpa kertas.
          </p>
          <Button type="button" onClick={scrollToForm}>
            Daftar Minat
          </Button>
        </section>

        <section className="border-t border-line bg-surface-2">
          <div className="mx-auto grid max-w-[1120px] grid-cols-1 gap-8 px-5 py-14 sm:grid-cols-2">
            {FEATURES.map((f) => (
              <div key={f.title} className="flex flex-col gap-2">
                <f.icon size={20} strokeWidth={2} className="text-primary" aria-hidden="true" />
                <p className="text-[16px] font-semibold text-ink">{f.title}</p>
                <p className="text-[13px] text-muted">{f.description}</p>
              </div>
            ))}
          </div>
        </section>

        <section className="mx-auto max-w-[1120px] px-5 py-14">
          <div className="mb-8 text-center">
            <p className="text-[11px] font-semibold uppercase tracking-[0.1em] text-muted">Harga</p>
            <h2 className="text-[21px] font-semibold text-ink">Pilih sesuai kebutuhan sekolah</h2>
          </div>
          <div className="mx-auto grid max-w-[640px] grid-cols-1 gap-4 sm:grid-cols-2">
            <Card className="flex flex-col gap-2">
              <p className="text-[18px] font-semibold text-ink">Basic</p>
              <p className="text-[13px] text-muted">
                Absensi, izin, dan data siswa/guru — mulai dari harga per siswa yang terjangkau untuk sekolah yang
                baru mulai.
              </p>
            </Card>
            <Card className="flex flex-col gap-2">
              <p className="text-[18px] font-semibold text-ink">Pro</p>
              <p className="text-[13px] text-muted">
                Semua fitur Basic ditambah dashboard TV, notifikasi WhatsApp, dan domain sendiri — mulai dari harga
                per siswa yang sepadan dengan fiturnya.
              </p>
            </Card>
          </div>
        </section>

        <section id={FORM_ANCHOR_ID} className="border-t border-line bg-surface-2">
          <div className="mx-auto max-w-[480px] px-5 py-14">
            <div className="mb-6 text-center">
              <p className="text-[11px] font-semibold uppercase tracking-[0.1em] text-muted">Tertarik?</p>
              <h2 className="text-[21px] font-semibold text-ink">Daftarkan Minat Sekolah Anda</h2>
              <p className="mt-1 text-[13px] text-muted">Tim kami akan menghubungi Anda untuk demo & harga.</p>
            </div>
            <InterestForm />
          </div>
        </section>
      </main>

      <footer className="border-t border-line px-5 py-6 text-center">
        <p className="text-[12px] text-muted">© {new Date().getFullYear()} NouSchool.</p>
      </footer>
    </div>
  );
}

function InterestForm() {
  const submitInterest = useSubmitInterest();
  const [form, setForm] = useState<InterestLeadInput>({
    school_name: '',
    contact_name: '',
    phone: '',
    email: '',
    note: '',
  });

  function setField<K extends keyof InterestLeadInput>(key: K, value: InterestLeadInput[K]) {
    setForm((prev) => ({ ...prev, [key]: value }));
  }

  function handleSubmit(e: FormEvent) {
    e.preventDefault();
    submitInterest.mutate({
      school_name: form.school_name.trim(),
      contact_name: form.contact_name.trim(),
      phone: form.phone.trim(),
      email: form.email?.trim() || undefined,
      note: form.note?.trim() || undefined,
    });
  }

  if (submitInterest.isSuccess) {
    return (
      <Card className="flex flex-col items-center gap-2 text-center">
        <p className="text-[16px] font-semibold text-ink">Terima kasih!</p>
        <p className="text-[13px] text-muted">
          Data sudah kami terima. Tim NouSchool akan menghubungi {form.contact_name || 'Anda'} lewat nomor atau
          email yang diisi.
        </p>
      </Card>
    );
  }

  return (
    <Card>
      <form onSubmit={handleSubmit} className="flex flex-col gap-4">
        <Field label="Nama sekolah" htmlFor="interest-school-name">
          <Input
            id="interest-school-name"
            value={form.school_name}
            onChange={(e) => setField('school_name', e.target.value)}
            required
          />
        </Field>
        <Field label="Nama kontak" htmlFor="interest-contact-name">
          <Input
            id="interest-contact-name"
            value={form.contact_name}
            onChange={(e) => setField('contact_name', e.target.value)}
            required
          />
        </Field>
        <Field label="No. HP" htmlFor="interest-phone">
          <Input
            id="interest-phone"
            type="tel"
            value={form.phone}
            onChange={(e) => setField('phone', e.target.value)}
            required
          />
        </Field>
        <Field label="Email" htmlFor="interest-email" hint="Opsional">
          <Input
            id="interest-email"
            type="email"
            value={form.email}
            onChange={(e) => setField('email', e.target.value)}
          />
        </Field>
        <Field label="Catatan" htmlFor="interest-note" hint="Opsional">
          <Textarea
            id="interest-note"
            value={form.note}
            onChange={(e) => setField('note', e.target.value)}
          />
        </Field>

        {submitInterest.isError && (
          <p className="text-[12px] text-danger">
            {submitInterest.error instanceof ApiError ? submitInterest.error.message : 'Gagal mengirim data. Coba lagi.'}
          </p>
        )}

        <Button type="submit" loading={submitInterest.isPending} className="w-full">
          Kirim Minat
        </Button>
      </form>
    </Card>
  );
}
