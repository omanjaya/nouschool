import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { api } from '../../lib/api';
import type {
  LeaveApprovalQueueItem,
  LeaveDecideInput,
  LeaveRequest,
  LeaveRequestCreateInput,
  LeaveSettings,
  LeaveSummaryRow,
  LeaveType,
} from '../../lib/types';

export const LEAVE_TYPES_KEY = ['leave', 'types'] as const;
export const LEAVE_REQUESTS_KEY = 'leave-requests' as const;
export const LEAVE_APPROVALS_KEY = ['leave', 'approvals'] as const;
export const LEAVE_SUMMARY_KEY = 'leave-summary' as const;
export const LEAVE_SETTINGS_KEY = ['settings', 'leave'] as const;

/** GET /api/leave/types — jenis izin aktif untuk sekolah ini. */
export function useLeaveTypes() {
  return useQuery({
    queryKey: LEAVE_TYPES_KEY,
    queryFn: () => api.get<{ types: LeaveType[] }>('/leave/types').then((r) => r.types),
  });
}

/** GET /api/leave/requests?scope=&status= — `mine` (guru sendiri) atau `all` (kepsek/admin). */
export function useLeaveRequests(scope: 'mine' | 'all', status?: string) {
  const qs = new URLSearchParams({ scope });
  if (status) qs.set('status', status);
  return useQuery({
    queryKey: [LEAVE_REQUESTS_KEY, scope, status ?? ''],
    queryFn: () => api.get<{ items: LeaveRequest[] }>(`/leave/requests?${qs.toString()}`).then((r) => r.items),
  });
}

/** POST /api/leave/requests — multipart (lampiran opsional). */
export function useCreateLeaveRequest() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (input: LeaveRequestCreateInput) => {
      const fd = new FormData();
      fd.append('type', input.type);
      fd.append('date_start', input.date_start);
      fd.append('date_end', input.date_end);
      fd.append('reason', input.reason);
      if (input.attachment) fd.append('attachment', input.attachment);
      return api.upload<LeaveRequest>('/leave/requests', fd);
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: [LEAVE_REQUESTS_KEY] });
      queryClient.invalidateQueries({ queryKey: LEAVE_APPROVALS_KEY });
    },
  });
}

/** POST /api/leave/requests/{id}/cancel */
export function useCancelLeaveRequest() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => api.post<LeaveRequest>(`/leave/requests/${id}/cancel`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: [LEAVE_REQUESTS_KEY] });
      queryClient.invalidateQueries({ queryKey: LEAVE_APPROVALS_KEY });
    },
  });
}

/** GET /api/leave/approvals — antrian menunggu keputusan user ini. */
export function useLeaveApprovals() {
  return useQuery({
    queryKey: LEAVE_APPROVALS_KEY,
    queryFn: () => api.get<{ items: LeaveApprovalQueueItem[] }>('/leave/approvals').then((r) => r.items),
  });
}

/** POST /api/leave/approvals/{step_id}/decide */
export function useDecideLeaveApproval() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ stepId, input }: { stepId: string; input: LeaveDecideInput }) =>
      api.post<LeaveRequest>(`/leave/approvals/${stepId}/decide`, input),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: LEAVE_APPROVALS_KEY });
      queryClient.invalidateQueries({ queryKey: [LEAVE_REQUESTS_KEY] });
    },
  });
}

/** GET /api/leave/summary?from=&to= — rekap izin per guru (kepsek/admin). */
export function useLeaveSummary(from: string, to: string, enabled = true) {
  return useQuery({
    queryKey: [LEAVE_SUMMARY_KEY, from, to],
    queryFn: () => api.get<LeaveSummaryRow[]>(`/leave/summary?from=${from}&to=${to}`),
    enabled,
  });
}

/** GET /api/settings/leave (admin). */
export function useLeaveSettings(enabled = true) {
  return useQuery({
    queryKey: LEAVE_SETTINGS_KEY,
    queryFn: () => api.get<LeaveSettings>('/settings/leave'),
    enabled,
  });
}

/** PUT /api/settings/leave (admin). */
export function useUpdateLeaveSettings() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (input: LeaveSettings) => api.put<LeaveSettings>('/settings/leave', input),
    onSuccess: (data) => {
      queryClient.setQueryData(LEAVE_SETTINGS_KEY, data);
      queryClient.invalidateQueries({ queryKey: LEAVE_TYPES_KEY });
    },
  });
}
