import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { api } from '../../lib/api';
import type { RolePermissionOverridesInput, RolePermissionsData } from '../../lib/types';

export const ROLE_PERMISSIONS_QUERY_KEY = ['role-permissions'] as const;

/** GET /api/role-permissions (admin) — Fase 15 GAP 2, docs/02-identity.md. */
export function useRolePermissions() {
  return useQuery({
    queryKey: ROLE_PERMISSIONS_QUERY_KEY,
    queryFn: () => api.get<RolePermissionsData>('/role-permissions'),
  });
}

/**
 * PUT /api/role-permissions {overrides} — hanya kirim sel yang benar-benar
 * diubah sesi ini (delta): `true`/`false` = override eksplisit, `null` =
 * kembali ke bawaan. Role `admin_sekolah` & permission `settings:manage`
 * ditolak server kalau disertakan — UI tidak pernah mengirim keduanya
 * (lihat `RolePermissionsPage`).
 */
export function useUpdateRolePermissions() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (overrides: RolePermissionOverridesInput) =>
      api.put<RolePermissionsData>('/role-permissions', { overrides }),
    onSuccess: (data) => {
      queryClient.setQueryData(ROLE_PERMISSIONS_QUERY_KEY, data);
    },
  });
}
