import { useState, type FormEvent } from 'react';
import { Link } from 'react-router-dom';
import { BellRing, ClipboardCheck, GraduationCap, MessageCircle, MonitorPlay, QrCode, ShieldAlert } from 'lucide-react';
import { Card } from '../../components/ui/Card';
import { Field, Input, Textarea } from '../../components/ui/Field';
import { Button } from '../../components/ui/Button';
import { BrandMark } from '../branding/BrandMark';
import { Hero } from '../../components/blocks/landing/Hero';
import { FeatureGrid, type FeatureItem } from '../../components/blocks/landing/FeatureGrid';
import { Pricing } from '../../components/blocks/landing/Pricing';
import { CtaBanner } from '../../components/blocks/landing/Cta';
import { ApiError } from '../../lib/api';
import { useSubmitInterest } from './api';
import type { InterestLeadInput } from '../../lib/types';

const FORM_ANCHOR_ID = 'minat';
const FEATURES_ANCHOR_ID = 'fitur';

const FEATURES: FeatureItem[] = [
  {
    icon: ClipboardCheck,
    title: 'Absensi 3 metode',
    description: 'Manual, kartu QR, atau check-in lokasi — sekolah pilih yang paling cocok untuk tiap kelas.',
  },
  {
    icon: QrCode,
    title: 'Izin & dispensasi QR',
    description: 'Guru dan siswa mengajukan izin dari HP, alur persetujuan mengikuti struktur sekolah.',
  },
  {
    icon: ShieldAlert,
    title: 'Kedisiplinan & SP otomatis',
    description: 'Pelanggaran tercatat rapi, surat peringatan terbit otomatis sesuai ambang yang diatur sekolah.',
  },
  {
    icon: GraduationCap,
    title: 'Penilaian & rapor',
    description: 'Nilai per mapel terkumpul jadi rapor siswa, siap diunduh tanpa rekap manual.',
  },
  {
    icon: MonitorPlay,
    title: 'Monitoring guru + TV',
    description: 'Lihat siapa yang sedang mengajar, belum masuk kelas, atau izin — real-time di dashboard TV.',
  },
  {
    icon: BellRing,
    title: 'Notifikasi WA/Push',
    description: 'Orang tua dan guru dapat kabar penting lewat WhatsApp atau notifikasi aplikasi, otomatis.',
  },
];

function scrollToId(id: string) {
  const el = document.getElementById(id);
  if (!el) return;
  const prefersReducedMotion = window.matchMedia('(prefers-reduced-motion: reduce)').matches;
  el.scrollIntoView({ behavior: prefersReducedMotion ? 'auto' : 'smooth', block: 'start' });
}

/**
 * `/` di host platform, hanya untuk pengunjung belum login (App.tsx). Halaman
 * marketing publik — bukan bagian AppShell, tanpa fetch `/api/me` di sini.
 *
 * Disusun dari block shadcnblocks (hero7, feature1, pricing1, cta1) yang
 * diadaptasi penuh ke design system "Rapor" — lihat komentar adaptasi di
 * masing-masing file `components/blocks/landing/*` dan checklist di
 * `components/blocks/README.md`.
 */
export function LandingPage() {
  return (
    <div className="flex min-h-dvh flex-col bg-bg text-ink">
      <header className="border-b border-line">
        <div className="mx-auto flex max-w-[1120px] items-center justify-between px-5 py-4">
          <BrandMark layout="row" />
          <Link to="/login" className="text-[13px] font-medium text-primary hover:opacity-80">
            Masuk
          </Link>
        </div>
      </header>

      <main className="flex-1">
        <Hero onPrimaryCta={() => scrollToId(FORM_ANCHOR_ID)} onSecondaryCta={() => scrollToId(FEATURES_ANCHOR_ID)} />

        <div id={FEATURES_ANCHOR_ID} className="border-t border-line bg-surface-2">
          <FeatureGrid eyebrow="Fitur" heading="Semua yang sekolah butuhkan, satu aplikasi" items={FEATURES} />
        </div>

        <Pricing onSelectPlan={() => scrollToId(FORM_ANCHOR_ID)} />

        <section id={FORM_ANCHOR_ID} className="border-t border-line bg-surface-2">
          <div className="mx-auto max-w-[480px] px-5 py-16 lg:py-20">
            <CtaBanner
              icon={<MessageCircle size={20} strokeWidth={2} aria-hidden="true" />}
              heading="Daftarkan Minat Sekolah Anda"
              description="Tim kami akan menghubungi Anda untuk demo & harga."
            />
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
