import { useQuery } from '@tanstack/react-query';
import { api } from '../../lib/api';
import type { PublicContext } from '../../lib/types';

export const PUBLIC_CONTEXT_QUERY_KEY = ['public-context'] as const;

/**
 * GET /api/public/context — publik (tanpa login), dipanggil sekali saat app
 * load lewat `lib/useAppBranding`. Dipakai juga oleh `AppShell`/`LoginPage`/
 * `ActivationPage` untuk menampilkan nama & logo sekolah (query di-cache,
 * bukan fetch ulang per komponen).
 */
export function usePublicContext() {
  return useQuery({
    queryKey: PUBLIC_CONTEXT_QUERY_KEY,
    queryFn: () => api.get<PublicContext>('/public/context'),
    staleTime: 5 * 60 * 1000,
    retry: false,
  });
}
