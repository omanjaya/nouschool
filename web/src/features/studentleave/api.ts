import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { api } from '../../lib/api';
import type {
  ChildRef,
  StudentLeaveCreateInput,
  StudentLeaveDecideInput,
  StudentLeaveRequest,
  StudentLeaveVerifyResult,
} from '../../lib/types';

export const STUDENT_LEAVE_KEY = 'student-leave' as const;

export type StudentLeaveScope = 'mine' | 'queue' | 'all';

/** GET /api/student-leave?scope=mine|queue|all&status= */
export function useStudentLeaveRequests(scope: StudentLeaveScope, status?: string, enabled = true) {
  const qs = new URLSearchParams({ scope });
  if (status) qs.set('status', status);
  return useQuery({
    queryKey: [STUDENT_LEAVE_KEY, scope, status ?? ''],
    queryFn: () => api.get<{ items: StudentLeaveRequest[]; total: number }>(`/student-leave?${qs.toString()}`).then((r) => r.items),
    enabled,
  });
}

/** POST /api/student-leave — multipart; siswa untuk diri sendiri, ortu untuk anak (sertakan `student_id`). */
export function useCreateStudentLeaveRequest() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (input: StudentLeaveCreateInput) => {
      const fd = new FormData();
      fd.append('type', input.type);
      fd.append('date_start', input.date_start);
      fd.append('date_end', input.date_end);
      fd.append('reason', input.reason);
      if (input.attachment) fd.append('attachment', input.attachment);
      if (input.student_id) fd.append('student_id', input.student_id);
      return api.upload<StudentLeaveRequest>('/student-leave', fd);
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: [STUDENT_LEAVE_KEY] });
    },
  });
}

/** POST /api/student-leave/{id}/cancel */
export function useCancelStudentLeaveRequest() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => api.post<StudentLeaveRequest>(`/student-leave/${id}/cancel`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: [STUDENT_LEAVE_KEY] });
    },
  });
}

/** POST /api/student-leave/{id}/review — keputusan tahap wali kelas (status `pending_homeroom`). */
export function useReviewStudentLeaveRequest() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id, input }: { id: string; input: StudentLeaveDecideInput }) =>
      api.post<StudentLeaveRequest>(`/student-leave/${id}/review`, input),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: [STUDENT_LEAVE_KEY] });
    },
  });
}

/** POST /api/student-leave/{id}/issue — keputusan tahap BK, menerbitkan surat bila disetujui (status `pending_bk`). */
export function useIssueStudentLeaveRequest() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id, input }: { id: string; input: StudentLeaveDecideInput }) =>
      api.post<StudentLeaveRequest>(`/student-leave/${id}/issue`, input),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: [STUDENT_LEAVE_KEY] });
    },
  });
}

export function studentLeaveLetterPdfUrl(id: string): string {
  return `/api/student-leave/${id}/letter/pdf`;
}

/** URL halaman publik verifikasi surat (dibuka dari scan QR di surat PDF) — bukan endpoint API. */
export function studentLeaveVerifyPageUrl(token: string): string {
  return `${window.location.origin}/verifikasi-surat?token=${encodeURIComponent(token)}`;
}

/**
 * GET /api/me/children (ortu) — query key SENGAJA sama dengan
 * `features/attendance/api.ts#useChildren` / `features/discipline/api.ts#useDisciplineChildren`
 * (fitur berbeda memanggil endpoint identik "daftar anak akun ini"; berbagi cache lebih tepat
 * daripada duplikasi query key untuk data yang sama).
 */
export function useStudentLeaveChildren(enabled = true) {
  return useQuery({
    queryKey: ['me-children'],
    queryFn: () => api.get<ChildRef[]>('/me/children'),
    enabled,
  });
}

/** GET /api/public/leave-verify?token= — PUBLIK, tanpa auth. */
export function useLeaveVerify(token: string) {
  return useQuery({
    queryKey: ['leave-verify', token],
    queryFn: () => api.get<StudentLeaveVerifyResult>(`/public/leave-verify?token=${encodeURIComponent(token)}`),
    enabled: Boolean(token),
    retry: false,
  });
}
