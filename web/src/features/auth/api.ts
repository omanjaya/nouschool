import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { api, ApiError } from '../../lib/api';
import type { AuthResponse, Me } from '../../lib/types';

export const ME_QUERY_KEY = ['me'] as const;

/**
 * GET /api/me — sesi berjalan. 401 = "belum login": kondisi NORMAL, bukan
 * error — dikembalikan sebagai data `null` supaya query settle di status
 * 'success'. Kalau 401 dilempar sebagai error, setiap guard route yang
 * mount/unmount (RequireLogin → Navigate → LoginPage) memicu refetch-on-mount
 * + cancel-revert tanpa henti → banjir request /api/me (bug loop yang pernah
 * terjadi: ±170 req/detik). Jangan diubah kembali menjadi throw.
 */
export function useMe() {
  return useQuery({
    queryKey: ME_QUERY_KEY,
    queryFn: async (): Promise<Me | null> => {
      try {
        // Backend membungkus SEMUA respons auth dalam `{user: ...}` (kontrak
        // terdokumentasi, beda dari konvensi `{data: ...}` yang sudah
        // di-unwrap `lib/api.ts`) — unwrap `.user` di sini supaya query
        // cache berisi `Me` flat, bukan `{user: Me}`.
        const res = await api.get<AuthResponse>('/me');
        return res.user;
      } catch (err) {
        if (err instanceof ApiError && err.status === 401) return null;
        throw err;
      }
    },
    staleTime: 5 * 60 * 1000,
    retry: false,
  });
}

interface LoginInput {
  identifier: string;
  password: string;
}

export function useLogin() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (input: LoginInput) => {
      const res = await api.post<AuthResponse>('/auth/login', input);
      return res.user;
    },
    onSuccess: (me) => {
      queryClient.setQueryData(ME_QUERY_KEY, me);
    },
  });
}

export function useLogout() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: () => api.post<undefined>('/auth/logout'),
    onSuccess: () => {
      queryClient.setQueryData(ME_QUERY_KEY, null);
      queryClient.clear();
    },
  });
}
