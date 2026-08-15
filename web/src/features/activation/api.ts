import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { api } from '../../lib/api';
import type { ActivateInput, InvitationInfo, Me } from '../../lib/types';
import { ME_QUERY_KEY } from '../auth/api';

/** GET /api/auth/invitation/{code} — publik, dipakai untuk menampilkan info sebelum aktivasi. */
export function useInvitationInfo(code: string) {
  return useQuery({
    queryKey: ['invitation', code],
    queryFn: () => api.get<InvitationInfo>(`/auth/invitation/${encodeURIComponent(code)}`),
    enabled: code.length > 0,
    retry: false,
  });
}

/** POST /api/auth/activate — membuat akun dari kode undangan + auto-login (shape sama /api/me). */
export function useActivate() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (input: ActivateInput) => api.post<Me>('/auth/activate', input),
    onSuccess: (me) => {
      queryClient.setQueryData(ME_QUERY_KEY, me);
    },
  });
}
