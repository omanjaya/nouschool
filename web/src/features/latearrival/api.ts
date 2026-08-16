import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { api } from '../../lib/api';
import type { LateArrivalListResult, LateArrivalScanResult, LateArrivalSummaryRow } from '../../lib/types';

export const LATE_ARRIVAL_KEY = 'late-arrival' as const;
export const LATE_ARRIVAL_SUMMARY_KEY = 'late-arrival-summary' as const;

export type LateArrivalScope = 'mine' | 'today' | 'all';

/** GET /api/late-arrivals?scope=mine|today|all&month= */
export function useLateArrivals(scope: LateArrivalScope, month?: string, enabled = true) {
  const qs = new URLSearchParams({ scope });
  if (month) qs.set('month', month);
  return useQuery({
    queryKey: [LATE_ARRIVAL_KEY, scope, month ?? ''],
    queryFn: () => api.get<LateArrivalListResult>(`/late-arrivals?${qs.toString()}`).then((r) => r.items),
    enabled,
  });
}

/** POST /api/late-arrivals/scan (siswa) `{token}` — kirim mentah hasil scan/ketik (kontrak eksplisit Fase 14 Gelombang B2, sama pola exit-permits). Belum ada record hari ini → scan pertama (QR piket) membuat record baru. */
export function useScanLateArrival() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (token: string) => api.post<LateArrivalScanResult>('/late-arrivals/scan', { token }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: [LATE_ARRIVAL_KEY] }),
  });
}

/** GET /api/late-arrivals/summary?month= — rekap per siswa (array langsung, staf). */
export function useLateArrivalSummary(month: string, enabled = true) {
  return useQuery({
    queryKey: [LATE_ARRIVAL_SUMMARY_KEY, month],
    queryFn: () => api.get<LateArrivalSummaryRow[]>(`/late-arrivals/summary?month=${encodeURIComponent(month)}`),
    enabled,
  });
}
