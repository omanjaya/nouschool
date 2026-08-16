import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { api } from '../../lib/api';
import type { Employee, EmployeeCreateResult, EmployeeInput, MemberStatusResult, MembershipStatus } from '../../lib/types';

export const EMPLOYEES_QUERY_KEY = ['employees'] as const;

/** GET /api/employees (admin) — daftar pegawai non-guru (role `pegawai`). */
export function useEmployees() {
  return useQuery({
    queryKey: EMPLOYEES_QUERY_KEY,
    queryFn: () => api.get<Employee[]>('/employees'),
  });
}

/** POST /api/employees — respons berisi `temp_password` (ditampilkan sekali, pola `ResetPasswordDialog`). */
export function useCreateEmployee() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (input: EmployeeInput) => api.post<EmployeeCreateResult>('/employees', input),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: EMPLOYEES_QUERY_KEY });
    },
  });
}

export function useUpdateEmployee(id: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (input: Partial<EmployeeInput>) => api.patch<Employee>(`/employees/${id}`, input),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: EMPLOYEES_QUERY_KEY });
    },
  });
}

/** PATCH /api/members/{userId}/status {role:'pegawai', status} (admin) — Fase 15 GAP 6a, sama pola `teachers/api.ts`. */
export function useSetEmployeeStatus(userId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (status: MembershipStatus) =>
      api.patch<MemberStatusResult>(`/members/${userId}/status`, { role: 'pegawai', status }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: EMPLOYEES_QUERY_KEY });
    },
  });
}
