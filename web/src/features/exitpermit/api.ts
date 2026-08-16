import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { api } from '../../lib/api';
import type {
  ChildRef,
  ExitPermit,
  ExitPermitCreateInput,
  ExitPermitGateHistoryResult,
  ExitPermitGateScanResult,
  ExitPermitListResult,
  ExitPermitRejectInput,
  ExitPermitScanResult,
} from '../../lib/types';

export const EXIT_PERMIT_KEY = 'exit-permit' as const;
export const EXIT_PERMIT_GATE_HISTORY_KEY = 'exit-permit-gate-history' as const;

export type ExitPermitScope = 'mine' | 'active' | 'all';

/** GET /api/exit-permits?scope=mine|active|all&date= */
export function useExitPermits(scope: ExitPermitScope, date?: string, enabled = true) {
  const qs = new URLSearchParams({ scope });
  if (date) qs.set('date', date);
  return useQuery({
    queryKey: [EXIT_PERMIT_KEY, scope, date ?? ''],
    queryFn: () => api.get<ExitPermitListResult>(`/exit-permits?${qs.toString()}`).then((r) => r.items),
    enabled,
  });
}

/** POST /api/exit-permits (siswa) — 409 kalau siswa masih punya dispensasi aktif (belum `exited`/`rejected`/`canceled`). */
export function useCreateExitPermit() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (input: ExitPermitCreateInput) => api.post<ExitPermit>('/exit-permits', input),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: [EXIT_PERMIT_KEY] }),
  });
}

/** POST /api/exit-permits/{id}/scan (siswa) `{token}` — kirim mentah hasil scan kamera ATAU kode yang diketik manual. */
export function useScanExitPermit() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id, token }: { id: string; token: string }) =>
      api.post<ExitPermitScanResult>(`/exit-permits/${id}/scan`, { token }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: [EXIT_PERMIT_KEY] }),
  });
}

/** POST /api/exit-permits/{id}/cancel (siswa, hanya saat masih pending). */
export function useCancelExitPermit() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => api.post<ExitPermit>(`/exit-permits/${id}/cancel`),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: [EXIT_PERMIT_KEY] }),
  });
}

/** POST /api/exit-permits/{id}/reject (admin only) `{comment}` — hanya saat status masih pending_*. */
export function useRejectExitPermit() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id, input }: { id: string; input: ExitPermitRejectInput }) =>
      api.post<ExitPermit>(`/exit-permits/${id}/reject`, input),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: [EXIT_PERMIT_KEY] }),
  });
}

/** POST /api/exit-permits/gate-scan (security/pegawai) `{gate_token}` — 410 kalau kedaluwarsa/sudah dipakai. */
export function useGateScan() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (gate_token: string) => api.post<ExitPermitGateScanResult>('/exit-permits/gate-scan', { gate_token }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: [EXIT_PERMIT_KEY] });
      queryClient.invalidateQueries({ queryKey: [EXIT_PERMIT_GATE_HISTORY_KEY] });
    },
  });
}

/** GET /api/exit-permits/gate-history?date= — riwayat siswa yang sudah keluar gerbang pada tanggal terkait. */
export function useGateHistory(date?: string) {
  const qs = date ? `?date=${encodeURIComponent(date)}` : '';
  return useQuery({
    queryKey: [EXIT_PERMIT_GATE_HISTORY_KEY, date ?? ''],
    queryFn: () => api.get<ExitPermitGateHistoryResult>(`/exit-permits/gate-history${qs}`).then((r) => r.items),
  });
}

/**
 * GET /api/me/children (ortu) — query key SENGAJA sama dengan
 * `features/studentleave/api.ts#useStudentLeaveChildren`/`features/attendance/api.ts#useChildren`
 * (endpoint identik "daftar anak akun ini"; berbagi cache lebih tepat daripada duplikasi).
 */
export function useExitPermitChildren(enabled = true) {
  return useQuery({
    queryKey: ['me-children'],
    queryFn: () => api.get<ChildRef[]>('/me/children'),
    enabled,
  });
}

/** Prefix konten QR gate dari HP siswa (dipindai security) — kontrak Fase 14 Gelombang B2. */
export const EXIT_PERMIT_GATE_QR_PREFIX = 'nouschool:gate:';
/** Prefix konten QR token guru (dipindai siswa tiap tahap approval) — sama dengan `features/teacherqr/api.ts`. */
export const TEACHER_QR_SCAN_PREFIX = 'nouschool:tqr:';
