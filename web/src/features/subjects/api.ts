import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { api } from '../../lib/api';
import type { Subject } from '../../lib/types';

export const SUBJECTS_QUERY_KEY = ['subjects'] as const;

/** GET /api/subjects — daftar mata pelajaran. */
export function useSubjects() {
  return useQuery({
    queryKey: SUBJECTS_QUERY_KEY,
    queryFn: () => api.get<Subject[]>('/subjects'),
  });
}

export interface SubjectInput {
  code: string;
  name: string;
}

export function useCreateSubject() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (input: SubjectInput) => api.post<Subject>('/subjects', input),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: SUBJECTS_QUERY_KEY });
    },
  });
}

export function useUpdateSubject(id: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (input: Partial<SubjectInput>) => api.patch<Subject>(`/subjects/${id}`, input),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: SUBJECTS_QUERY_KEY });
    },
  });
}
