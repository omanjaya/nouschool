import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { api } from '../../lib/api';
import type { Branding } from '../../lib/types';

export const BRANDING_QUERY_KEY = ['settings', 'branding'] as const;

export function useBranding() {
  return useQuery({
    queryKey: BRANDING_QUERY_KEY,
    queryFn: () => api.get<Branding>('/settings/branding'),
  });
}

export function useUpdateBranding() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (input: Branding) => api.put<Branding>('/settings/branding', input),
    onSuccess: (data) => {
      queryClient.setQueryData(BRANDING_QUERY_KEY, data);
    },
  });
}
