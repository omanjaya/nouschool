import type { ReactNode } from 'react';

/**
 * Diadaptasi dari `@shadcnblocks/cta1`. Bahan asli: kartu dua kolom dengan
 * gambar stok di sisi kanan + tombol. Untuk NouSchool, sisi gambar dibuang
 * (tidak ada aset foto, dan aturan UI melarang gambar stok/AI-slop) —
 * dipakai sebagai banner pembuka satu kolom di atas form minat existing
 * (`InterestForm` di `LandingPage.tsx`), bukan tombol link keluar. Border
 * kartu, radius, dan warna ikon disamakan ke token Rapor.
 */

interface CtaBannerProps {
  icon: ReactNode;
  heading: string;
  description: string;
}

export function CtaBanner({ icon, heading, description }: CtaBannerProps) {
  return (
    <div className="mx-auto mb-6 flex max-w-[480px] flex-col items-center gap-2 text-center">
      <span className="flex h-10 w-10 items-center justify-center rounded-lg bg-primary-soft text-primary">
        {icon}
      </span>
      <h2 className="text-[21px] font-semibold text-ink">{heading}</h2>
      <p className="text-[13px] text-muted">{description}</p>
    </div>
  );
}
