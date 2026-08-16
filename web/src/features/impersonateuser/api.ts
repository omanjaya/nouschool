import { useMutation, useQueryClient } from '@tanstack/react-query';
import { api } from '../../lib/api';
import type { Me } from '../../lib/types';
import { ME_QUERY_KEY } from '../auth/api';

/**
 * "Masuk sebagai user ini" (admin sekolah, Fase 14 Gelombang D) — BEDA dari
 * `features/impersonate` (sesi support SUPER ADMIN lintas sekolah lewat
 * token sekali pakai `/auth/impersonate`): ini admin sekolah masuk sebagai
 * guru/siswa/pegawai DI SEKOLAHNYA SENDIRI, cookie sesi langsung diganti
 * server (tanpa token perantara).
 */
export const IMPERSONATE_USER_MUTATION_KEY = ['impersonate-user'] as const;

/**
 * Ganti identitas sesi total (bukan invalidate per-key) — cache lama (data
 * siswa/guru/dsb dari sudut pandang admin) tidak valid lagi untuk sudut
 * pandang user target, jadi `queryClient.clear()` dulu baru isi ME baru.
 */
function replaceIdentity(queryClient: ReturnType<typeof useQueryClient>, me: Me) {
  queryClient.clear();
  queryClient.setQueryData(ME_QUERY_KEY, me);
}

/** POST /api/users/{id}/impersonate (admin) → shape sama `/api/me`, `impersonated_by` terisi nama admin. */
export function useImpersonateUser() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (userId: string) => api.post<Me>(`/users/${userId}/impersonate`),
    onSuccess: (me) => replaceIdentity(queryClient, me),
  });
}

/** POST /api/auth/impersonation/stop → shape sama `/api/me` (kembali ke akun admin, `impersonated_by` kosong lagi). */
export function useStopImpersonation() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: () => api.post<Me>('/auth/impersonation/stop'),
    onSuccess: (me) => replaceIdentity(queryClient, me),
  });
}
