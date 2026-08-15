import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { api } from '../../lib/api';
import type { AcademicYear, GeneratedInvitation, SchoolClass } from '../../lib/types';

export const CLASSES_QUERY_KEY = 'classes' as const;

export function classesQueryKey(academicYearId?: string) {
  return [CLASSES_QUERY_KEY, academicYearId ?? 'default'] as const;
}

/**
 * GET /api/classes — tanpa `academic_year_id` backend memakai tahun ajaran aktif sekolah
 * (tidak ada endpoint "tahun ajaran aktif" terpisah untuk admin_sekolah di kontrak Fase 2).
 */
export function useClasses(academicYearId?: string) {
  return useQuery({
    queryKey: classesQueryKey(academicYearId),
    queryFn: () =>
      api.get<SchoolClass[]>(`/classes${academicYearId ? `?academic_year_id=${academicYearId}` : ''}`),
  });
}

export const ACADEMIC_YEARS_QUERY_KEY = ['academic-years'] as const;

/** GET /api/academic-years — daftar tahun ajaran sekolah (tenant-scoped lewat host). */
export function useAcademicYears() {
  return useQuery({
    queryKey: ACADEMIC_YEARS_QUERY_KEY,
    queryFn: () => api.get<AcademicYear[]>('/academic-years'),
  });
}

export interface ClassInput {
  name: string;
  grade: string;
  major?: string;
  homeroom_teacher_id?: string;
}

export function useCreateClass() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (input: ClassInput) => api.post<SchoolClass>('/classes', input),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: [CLASSES_QUERY_KEY] });
    },
  });
}

export function useUpdateClass(id: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (input: Partial<ClassInput>) => api.patch<SchoolClass>(`/classes/${id}`, input),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: [CLASSES_QUERY_KEY] });
    },
  });
}

export function useAddClassStudents(classId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (studentIds: string[]) =>
      api.post<undefined>(`/classes/${classId}/students`, { student_ids: studentIds }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: [CLASSES_QUERY_KEY] });
      queryClient.invalidateQueries({ queryKey: ['students'] });
    },
  });
}

export function useRemoveClassStudent(classId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (studentId: string) => api.delete<undefined>(`/classes/${classId}/students/${studentId}`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: [CLASSES_QUERY_KEY] });
      queryClient.invalidateQueries({ queryKey: ['students'] });
    },
  });
}

export function useGenerateInvitations() {
  return useMutation({
    mutationFn: (classId: string) => api.post<GeneratedInvitation[]>('/invitations/generate', { class_id: classId }),
  });
}
