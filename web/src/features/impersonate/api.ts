import { useMutation, useQueryClient } from '@tanstack/react-query';
import { api } from '../../lib/api';
import type { Me } from '../../lib/types';
import { ME_QUERY_KEY } from '../auth/api';

/**
 * POST /api/auth/impersonate {token} — publik, HOST TENANT. Token sekali
 * pakai dibuat lewat panel super admin (`features/admin/SchoolDetailPage`).
 * Sukses: shape sama `/api/me` (auto-login sesi support). Gagal apa pun
 * (token salah/kedaluwarsa/sudah dipakai) → 410 `impersonation_invalid`
 * dengan pesan siap-tampil dari server.
 */
export function useImpersonate() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (token: string) => api.post<Me>('/auth/impersonate', { token }),
    onSuccess: (me) => {
      queryClient.setQueryData(ME_QUERY_KEY, me);
    },
  });
}
