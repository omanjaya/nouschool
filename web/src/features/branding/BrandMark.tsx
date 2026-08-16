import { usePublicContext } from './api';
import { DEFAULT_APP_NAME } from '../../lib/useAppBranding';

interface BrandMarkProps {
  className?: string;
}

/**
 * Logo sekolah (bila ada) + nama aplikasi — dipakai `LoginPage` &
 * `ActivationPage` (docs/01-tenant.md §branding: "app_name sebagai heading").
 * Host platform / belum ada logo → wordmark teks saja, tanpa gambar.
 */
export function BrandMark({ className = '' }: BrandMarkProps) {
  const { data: context } = usePublicContext();
  const appName = context && !context.platform ? context.branding.app_name : DEFAULT_APP_NAME;
  const logoUrl = context && !context.platform ? context.branding.logo_url : null;

  const displayName = appName || DEFAULT_APP_NAME;

  return (
    <div className={`flex flex-col items-center gap-2 ${className}`}>
      {logoUrl ? (
        <img
          src={logoUrl}
          alt=""
          className="h-12 w-12 rounded-lg border border-line object-contain"
        />
      ) : (
        <span
          aria-hidden="true"
          className="flex h-12 w-12 items-center justify-center rounded-lg bg-primary-soft text-[21px] font-semibold text-primary"
        >
          {displayName.charAt(0).toUpperCase()}
        </span>
      )}
      <p className="text-center text-[21px] font-semibold text-ink">{displayName}</p>
    </div>
  );
}
