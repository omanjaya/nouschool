import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { api } from '../../lib/api';
import type { Duty, DutyAssignee, DutyFlagOption, DutyInput } from '../../lib/types';

export const DUTIES_KEY = ['duties'] as const;
export const DUTY_FLAGS_KEY = ['duties', 'flags'] as const;
export const dutyAssignmentsKey = (dutyId: string) => ['duties', dutyId, 'assignments'] as const;

/** GET /api/duties (admin) — tugas tambahan per TA aktif. */
export function useDuties() {
  return useQuery({
    queryKey: DUTIES_KEY,
    queryFn: () => api.get<Duty[]>('/duties'),
  });
}

/** GET /api/duties/flags — daftar capability flag yang bisa dipilih. */
export function useDutyFlags() {
  return useQuery({
    queryKey: DUTY_FLAGS_KEY,
    queryFn: () => api.get<DutyFlagOption[]>('/duties/flags'),
  });
}

export function useCreateDuty() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (input: DutyInput) => api.post<Duty>('/duties', input),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: DUTIES_KEY }),
  });
}

export function useUpdateDuty(id: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (input: DutyInput) => api.patch<Duty>(`/duties/${id}`, input),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: DUTIES_KEY }),
  });
}

/** DELETE /api/duties/{id} — server bisa 409 kalau tugas masih punya petugas (baca `ApiError.status` di pemanggil, tawarkan nonaktif). */
export function useDeleteDuty() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => api.delete<undefined>(`/duties/${id}`),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: DUTIES_KEY }),
  });
}

/** GET /api/duties/{id}/assignments — daftar user yang saat ini memegang tugas ini (TA aktif). */
export function useDutyAssignments(dutyId: string, enabled = true) {
  return useQuery({
    queryKey: dutyAssignmentsKey(dutyId),
    queryFn: () => api.get<DutyAssignee[]>(`/duties/${dutyId}/assignments`),
    enabled,
  });
}

/** PUT /api/duties/{id}/assignments — ganti seluruh daftar petugas TA aktif. */
export function useUpdateDutyAssignments(dutyId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (userIds: string[]) =>
      api.put<DutyAssignee[]>(`/duties/${dutyId}/assignments`, { user_ids: userIds }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: dutyAssignmentsKey(dutyId) });
      queryClient.invalidateQueries({ queryKey: DUTIES_KEY });
    },
  });
}
