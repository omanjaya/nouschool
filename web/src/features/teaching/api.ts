import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { api } from '../../lib/api';
import { ATTENDANCE_SLOTS_TODAY_KEY } from '../attendance/api';
import type {
  Journal,
  JournalListResult,
  TeachingComplianceRow,
  TeachingJournalCreateInput,
  TeachingJournalUpdateInput,
  TeachingScanResult,
  TeachingStatusResult,
} from '../../lib/types';

export const JOURNALS_QUERY_KEY = 'teaching-journals' as const;
export const TEACHING_STATUS_QUERY_KEY = 'teaching-status' as const;
export const TEACHING_COMPLIANCE_QUERY_KEY = 'teaching-compliance' as const;

/**
 * POST /api/teaching/scan — "satu scan tiga hasil" (docs/06 #1): kirim
 * `{room_token}` (dari QR fisik, prefix `nouschool:room:` sudah dilepas
 * pemanggil) atau `{room_id}` (fallback pilih ruangan manual).
 */
export function useTeachingScan() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (input: { room_token: string } | { room_id: string }) =>
      api.post<TeachingScanResult>('/teaching/scan', input),
    onSuccess: (result) => {
      if (!result.needs_manual) {
        queryClient.invalidateQueries({ queryKey: [JOURNALS_QUERY_KEY] });
        queryClient.invalidateQueries({ queryKey: [ATTENDANCE_SLOTS_TODAY_KEY] });
      }
    },
  });
}

/** POST /api/teaching/journals — entri jurnal di luar jadwal (guru pengganti/insidental, flag `unscheduled`/`manual_pick`). */
export function useCreateJournalManual() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (input: TeachingJournalCreateInput) => api.post<Journal>('/teaching/journals', input),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: [JOURNALS_QUERY_KEY] }),
  });
}

/** PATCH /api/teaching/journals/{id} — isi/ubah materi & catatan, boleh belakangan hari itu. */
export function useUpdateJournal(id: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (input: TeachingJournalUpdateInput) => api.patch<Journal>(`/teaching/journals/${id}`, input),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: [JOURNALS_QUERY_KEY] }),
  });
}

/** POST /api/teaching/journals/{id}/end — guru menutup sesi mengajar lebih awal secara manual. */
export function useEndJournal(id: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: () => api.post<Journal>(`/teaching/journals/${id}/end`),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: [JOURNALS_QUERY_KEY] }),
  });
}

export type JournalScope = 'mine' | 'all';

export interface JournalFilter {
  date?: string;
  month?: string;
}

function journalFilterQuery(scope: JournalScope, filter: JournalFilter): string {
  const params = new URLSearchParams({ scope });
  if (filter.date) params.set('date', filter.date);
  if (filter.month) params.set('month', filter.month);
  return params.toString();
}

/** GET /api/teaching/journals?scope=mine|all&date=|month= */
export function useJournals(scope: JournalScope, filter: JournalFilter, enabled = true) {
  return useQuery({
    queryKey: [JOURNALS_QUERY_KEY, scope, filter.date ?? '', filter.month ?? ''],
    queryFn: () => api.get<JournalListResult>(`/teaching/journals?${journalFilterQuery(scope, filter)}`),
    enabled,
  });
}

/** GET /api/teaching/status?date= — status mengajar per slot jadwal (kepsek/admin), dipoll 30 dtk. */
export function useTeachingStatus(date: string, enabled = true) {
  return useQuery({
    queryKey: [TEACHING_STATUS_QUERY_KEY, date],
    queryFn: () => api.get<TeachingStatusResult>(`/teaching/status?date=${date}`),
    enabled,
    refetchInterval: 30_000,
  });
}

/**
 * GET /api/teaching/compliance?from=&to= (kepsek/admin) — "ketertiban mengajar":
 * % slot jadwal yang benar-benar terlaksana (journal vs jadwal) per guru,
 * dipakai `/monitoring/rekap` (docs/06 dashboard kepsek).
 */
export function useTeachingCompliance(from: string, to: string, enabled = true) {
  return useQuery({
    queryKey: [TEACHING_COMPLIANCE_QUERY_KEY, from, to],
    queryFn: () => api.get<TeachingComplianceRow[]>(`/teaching/compliance?from=${from}&to=${to}`),
    enabled,
  });
}
