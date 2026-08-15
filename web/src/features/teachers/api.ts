import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { api } from '../../lib/api';
import type { Teacher } from '../../lib/types';

export const TEACHERS_QUERY_KEY = ['teachers'] as const;

/** GET /api/teachers — daftar guru (profil kepegawaian). */
export function useTeachers() {
  return useQuery({
    queryKey: TEACHERS_QUERY_KEY,
    queryFn: () => api.get<Teacher[]>('/teachers'),
  });
}

export interface TeacherInput {
  name: string;
  email?: string;
  nip?: string;
}

export function useCreateTeacher() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (input: TeacherInput) => api.post<Teacher>('/teachers', input),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: TEACHERS_QUERY_KEY });
    },
  });
}

export function useUpdateTeacher(id: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (input: Partial<TeacherInput>) => api.patch<Teacher>(`/teachers/${id}`, input),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: TEACHERS_QUERY_KEY });
    },
  });
}
