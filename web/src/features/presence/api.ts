import { useQuery } from '@tanstack/react-query';
import { api } from '../../lib/api';
import type { PresenceData } from '../../lib/types';

export const PRESENCE_QUERY_KEY = ['presence'] as const;

const POLL_INTERVAL_MS = 30_000;

/**
 * GET /api/presence (admin/kepsek) — Fase 15 GAP 6b, docs/02-identity.md.
 * Tidak ada event realtime khusus untuk ini (kontrak API pasti) — polling
 * 30 detik cukup, dipasang hanya di `MonitoringPage` (sudah digating role
 * kepala_sekolah/admin_sekolah di sana).
 */
export function usePresence(enabled: boolean) {
  return useQuery({
    queryKey: PRESENCE_QUERY_KEY,
    queryFn: () => api.get<PresenceData>('/presence'),
    enabled,
    refetchInterval: POLL_INTERVAL_MS,
  });
}
