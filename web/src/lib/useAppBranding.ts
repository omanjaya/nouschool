import { useEffect } from 'react';
import { usePublicContext } from '../features/branding/api';
import { applyBrandColor } from './color';

export const DEFAULT_APP_NAME = 'NouSchool';

/**
 * Branding runtime (docs/01-tenant.md §branding, docs/10-design-system.md
 * §1): fetch `/api/public/context` sekali saat app load lewat
 * `usePublicContext` (react-query, di-cache — dipanggil ulang di komponen
 * lain seperti `AppShell` tanpa fetch tambahan), lalu untuk host tenant:
 * set `document.title` = nama aplikasi sekolah + inject `--primary`
 * (dan turunannya `--primary-soft`/`--primary-ink`, lihat `lib/color.ts`).
 * Host platform (mis. admin.nouschool.id) tetap memakai default NouSchool —
 * tidak pernah menimpa token warna.
 *
 * Dipanggil di `App()` (bukan di dalam rute tertentu) supaya berjalan sebelum/
 * paralel dengan `useMe()`, dan tetap berlaku di layar publik (`/login`,
 * `/aktivasi`, landing page).
 */
export function useAppBranding() {
  const query = usePublicContext();

  useEffect(() => {
    if (!query.data) return;

    if (query.data.platform) {
      document.title = DEFAULT_APP_NAME;
      return;
    }

    document.title = query.data.branding.app_name || DEFAULT_APP_NAME;
    applyBrandColor(query.data.branding.primary_color);
  }, [query.data]);

  return query;
}
