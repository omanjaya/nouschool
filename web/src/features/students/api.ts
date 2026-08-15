import { keepPreviousData, useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { api } from '../../lib/api';
import type { Student, StudentListResult } from '../../lib/types';

export const STUDENTS_QUERY_KEY = 'students' as const;

export interface StudentsQueryParams {
  q?: string;
  classId?: string;
  page: number;
  perPage: number;
}

function buildStudentsQuery(params: StudentsQueryParams): string {
  const search = new URLSearchParams();
  if (params.q) search.set('q', params.q);
  if (params.classId) search.set('class_id', params.classId);
  search.set('page', String(params.page));
  search.set('per_page', String(params.perPage));
  return search.toString();
}

export function studentsQueryKey(params: StudentsQueryParams) {
  return [STUDENTS_QUERY_KEY, params] as const;
}

/** GET /api/students — daftar siswa dengan pencarian, filter rombel & paginasi. */
export function useStudents(params: StudentsQueryParams) {
  return useQuery({
    queryKey: studentsQueryKey(params),
    queryFn: () => api.get<StudentListResult>(`/students?${buildStudentsQuery(params)}`),
    placeholderData: keepPreviousData,
  });
}

export function studentQueryKey(id: string) {
  return [STUDENTS_QUERY_KEY, 'detail', id] as const;
}

/** GET /api/students/{id} — dipakai halaman detail siswa. */
export function useStudent(id: string | undefined) {
  return useQuery({
    queryKey: studentQueryKey(id ?? ''),
    queryFn: () => api.get<Student>(`/students/${id}`),
    enabled: Boolean(id),
  });
}

export interface StudentInput {
  nis: string;
  nisn?: string;
  name: string;
  gender?: string;
  birth_date?: string;
  class_id?: string;
}

export function useCreateStudent() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (input: StudentInput) => api.post<Student>('/students', input),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: [STUDENTS_QUERY_KEY] });
    },
  });
}

export function useUpdateStudent(id: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (input: Partial<StudentInput>) => api.patch<Student>(`/students/${id}`, input),
    onSuccess: (data) => {
      queryClient.setQueryData(studentQueryKey(id), data);
      queryClient.invalidateQueries({ queryKey: [STUDENTS_QUERY_KEY] });
    },
  });
}
