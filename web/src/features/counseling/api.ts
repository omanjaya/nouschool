import { useInfiniteQuery, useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { api, ApiError } from '../../lib/api';
import type { CounselingCreateInput, CounselingListResult, CounselingSession, CounselingUpdateInput } from '../../lib/types';

export const COUNSELING_KEY = 'counseling' as const;

/**
 * Cek "apakah user ini pemegang flag BK" TANPA mendesain endpoint baru
 * (kontrak Fase 14 Gelombang D): coba `GET /api/counselings?page=1` — 403
 * berarti bukan pemegang `leave_issuance`/bukan admin. Dipakai gating kartu
 * Beranda guru (`useCounselingAccess`) supaya TIDAK pernah merender
 * ErrorState untuk guru yang memang tidak berhak (docs/10 — state error
 * hanya untuk kegagalan sungguhan, bukan hasil gating akses).
 */
export function useCounselingAccess(enabled: boolean) {
  return useQuery({
    queryKey: [COUNSELING_KEY, 'access-probe'],
    queryFn: () => api.get<CounselingListResult>('/counselings?page=1'),
    enabled,
    retry: false,
  });
}

export interface CounselingFilter {
  studentId?: string;
}

function buildQuery(filter: CounselingFilter, page: number): string {
  const qs = new URLSearchParams();
  if (filter.studentId) qs.set('student_id', filter.studentId);
  qs.set('page', String(page));
  return qs.toString();
}

/** GET /api/counselings?student_id=&page= — dimuat bertahap lewat "Muat lebih banyak". */
export function useCounselingSessions(filter: CounselingFilter) {
  return useInfiniteQuery({
    queryKey: [COUNSELING_KEY, 'list', filter],
    queryFn: ({ pageParam }) => api.get<CounselingListResult>(`/counselings?${buildQuery(filter, pageParam)}`),
    initialPageParam: 1,
    getNextPageParam: (lastPage, allPages) => {
      const loaded = allPages.reduce((n, p) => n + p.items.length, 0);
      return loaded < lastPage.total ? allPages.length + 1 : undefined;
    },
  });
}

/** POST /api/counselings — multipart, `evidence` (foto/pdf) opsional. */
export function useCreateCounseling() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (input: CounselingCreateInput) => {
      const fd = new FormData();
      fd.append('student_id', input.student_id);
      fd.append('career_goals', input.career_goals);
      fd.append('problem_description', input.problem_description);
      fd.append('follow_up_plan', input.follow_up_plan);
      if (input.evidence) fd.append('evidence', input.evidence);
      return api.upload<CounselingSession>('/counselings', fd);
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: [COUNSELING_KEY] });
    },
  });
}

/** PATCH /api/counselings/{id} — field teks saja. */
export function useUpdateCounseling(id: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (input: CounselingUpdateInput) => api.patch<CounselingSession>(`/counselings/${id}`, input),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: [COUNSELING_KEY] });
    },
  });
}

/** POST /api/counselings/{id}/evidence — multipart, mengganti bukti (kalau ada) atau menambah baru. */
export function useUploadCounselingEvidence(id: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (file: File) => {
      const fd = new FormData();
      fd.append('evidence', file);
      return api.upload<CounselingSession>(`/counselings/${id}/evidence`, fd);
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: [COUNSELING_KEY] });
    },
  });
}

/** DELETE /api/counselings/{id} (pembuat/admin). */
export function useDeleteCounseling() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => api.delete<undefined>(`/counselings/${id}`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: [COUNSELING_KEY] });
    },
  });
}

export function counselingReportUrl(id: string): string {
  return `/api/counselings/${id}/report/html`;
}

export function isForbidden(err: unknown): boolean {
  return err instanceof ApiError && err.status === 403;
}
