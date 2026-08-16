import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { api } from '../../lib/api';
import type { SubstitutionCreateInput, SubstitutionListResult, SubstitutionRequest } from '../../lib/types';

export const SUBSTITUTION_KEY = 'substitutions' as const;

export type SubstitutionScope = 'mine' | 'for-me' | 'all';

/** GET /api/substitutions?scope=mine|for-me|all&date= */
export function useSubstitutions(scope: SubstitutionScope, date?: string) {
  const qs = new URLSearchParams({ scope });
  if (date) qs.set('date', date);
  return useQuery({
    queryKey: [SUBSTITUTION_KEY, scope, date ?? ''],
    queryFn: () => api.get<SubstitutionListResult>(`/substitutions?${qs.toString()}`).then((r) => r.items),
  });
}

/** POST /api/substitutions {schedule_slot_id,date,substitute_user_id,reason}. */
export function useCreateSubstitution() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (input: SubstitutionCreateInput) => api.post<SubstitutionRequest>('/substitutions', input),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: [SUBSTITUTION_KEY] });
    },
  });
}

/** POST /api/substitutions/{id}/accept (pengganti yang diajukan). */
export function useAcceptSubstitution() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => api.post<SubstitutionRequest>(`/substitutions/${id}/accept`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: [SUBSTITUTION_KEY] });
    },
  });
}

/** POST /api/substitutions/{id}/reject (pengganti yang diajukan). */
export function useRejectSubstitution() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => api.post<SubstitutionRequest>(`/substitutions/${id}/reject`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: [SUBSTITUTION_KEY] });
    },
  });
}

/** POST /api/substitutions/{id}/cancel (pengaju). */
export function useCancelSubstitution() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => api.post<SubstitutionRequest>(`/substitutions/${id}/cancel`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: [SUBSTITUTION_KEY] });
    },
  });
}
