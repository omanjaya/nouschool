import type { LucideIcon } from 'lucide-react';

/**
 * Diadaptasi dari `@shadcnblocks/feature1`. Bahan asli adalah section
 * "single focus" (satu gambar besar + satu blok teks) — untuk NouSchool
 * diubah strukturnya jadi grid 6 fitur nyata produk (bukan gambar stok),
 * karena itu yang perlu ditunjukkan di landing ini. Tipografi/spacing &
 * kartu hairline mengikuti docs/10-design-system.md; ikon lucide-react.
 */

export interface FeatureItem {
  icon: LucideIcon;
  title: string;
  description: string;
}

interface FeatureGridProps {
  eyebrow: string;
  heading: string;
  items: FeatureItem[];
}

export function FeatureGrid({ eyebrow, heading, items }: FeatureGridProps) {
  return (
    <section className="mx-auto max-w-[1120px] px-5 py-16 lg:py-20">
      <div className="mb-10 text-center">
        <p className="text-[11px] font-semibold uppercase tracking-[0.1em] text-muted">{eyebrow}</p>
        <h2 className="mt-1 text-[21px] font-semibold text-ink lg:text-[28px]">{heading}</h2>
      </div>
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
        {items.map((item) => (
          <div key={item.title} className="flex flex-col gap-3 rounded-lg border border-line bg-surface p-5">
            <span className="flex h-10 w-10 items-center justify-center rounded-lg bg-primary-soft text-primary">
              <item.icon size={20} strokeWidth={2} aria-hidden="true" />
            </span>
            <p className="text-[16px] font-semibold text-ink">{item.title}</p>
            <p className="text-[13px] text-muted">{item.description}</p>
          </div>
        ))}
      </div>
    </section>
  );
}
