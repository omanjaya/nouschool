import { Check } from 'lucide-react';
import { Button } from '../../ui/Button';
import { Tag } from '../../ui/Tag';

/**
 * Diadaptasi dari `@shadcnblocks/pricing1`. Bahan asli menampilkan 4 paket
 * dengan harga dolar dummy ($0/$20/$80/$199) dan gambar ilustrasi — untuk
 * NouSchool disederhanakan jadi 2 paket nyata (Basic/Pro, sesuai
 * `docs/09-billing.md`: harga = tier × bracket siswa, ditentukan saat
 * demo) TANPA angka harga palsu — memakai teks "mulai dari" seperti
 * diminta. Kartu grid shadcn (`bg-card`, `shadow-sm`, border generik)
 * diganti kartu hairline token Rapor; tombol pakai `components/ui/Button`.
 */

interface PricingPlan {
  name: string;
  tagline: string;
  description: string;
  features: string[];
  highlighted?: boolean;
}

const PLANS: PricingPlan[] = [
  {
    name: 'Basic',
    tagline: 'Mulai dari harga per siswa yang terjangkau',
    description: 'Untuk sekolah yang baru mulai merapikan absensi dan izin dari kertas ke digital.',
    features: [
      'Absensi siswa 3 metode (manual, QR, lokasi)',
      'Izin guru & approval berjenjang',
      'Data siswa, guru, dan rombel',
      'Notifikasi in-app & web push',
    ],
  },
  {
    name: 'Pro',
    tagline: 'Mulai dari harga per siswa yang sepadan dengan fiturnya',
    description: 'Untuk sekolah yang butuh monitoring penuh, dashboard TV, dan domain sendiri.',
    features: [
      'Semua fitur Basic',
      'Monitoring guru + dashboard TV ruang guru',
      'Notifikasi WhatsApp',
      'Custom domain sekolah sendiri',
      'Kedisiplinan & SP otomatis',
    ],
    highlighted: true,
  },
];

interface PricingProps {
  onSelectPlan: () => void;
}

export function Pricing({ onSelectPlan }: PricingProps) {
  return (
    <section className="mx-auto max-w-[1120px] px-5 py-16 lg:py-20">
      <div className="mb-10 text-center">
        <p className="text-[11px] font-semibold uppercase tracking-[0.1em] text-muted">Harga</p>
        <h2 className="mt-1 text-[21px] font-semibold text-ink lg:text-[28px]">Pilih sesuai kebutuhan sekolah</h2>
        <p className="mx-auto mt-2 max-w-[480px] text-[13px] text-muted">
          Langganan tahunan, harga ditentukan tier fitur dan jumlah siswa. Tim kami bantu hitungkan saat demo.
        </p>
      </div>
      <div className="mx-auto grid max-w-[720px] grid-cols-1 gap-4 sm:grid-cols-2">
        {PLANS.map((plan) => (
          <div
            key={plan.name}
            className={`flex flex-col gap-4 rounded-lg border p-6 ${
              plan.highlighted ? 'border-primary bg-primary-soft' : 'border-line bg-surface'
            }`}
          >
            <div className="flex items-center justify-between gap-2">
              <p className="text-[18px] font-semibold text-ink">{plan.name}</p>
              {plan.highlighted && <Tag variant="now">Populer</Tag>}
            </div>
            <p className="text-[13px] font-semibold text-ink">{plan.tagline}</p>
            <p className="text-[13px] text-muted">{plan.description}</p>
            <ul className="flex flex-col gap-2 border-t border-line pt-4">
              {plan.features.map((feature) => (
                <li key={feature} className="flex items-start gap-2 text-[13px] text-ink">
                  <Check size={16} strokeWidth={2} className="mt-0.5 shrink-0 text-primary" aria-hidden="true" />
                  <span>{feature}</span>
                </li>
              ))}
            </ul>
            <Button
              type="button"
              variant={plan.highlighted ? 'primary' : 'secondary'}
              onClick={onSelectPlan}
              className="mt-2"
            >
              Tanya Harga {plan.name}
            </Button>
          </div>
        ))}
      </div>
    </section>
  );
}
