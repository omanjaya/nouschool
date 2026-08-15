import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { api } from '../../lib/api';
import type { AcademicYear, School } from '../../lib/types';

export const SCHOOLS_QUERY_KEY = ['admin', 'schools'] as const;

/** GET /api/admin/schools — tidak ada endpoint GET satuan, detail diambil dari daftar ini. */
export function useSchools() {
  return useQuery({
    queryKey: SCHOOLS_QUERY_KEY,
    queryFn: () => api.get<School[]>('/admin/schools'),
  });
}

interface CreateSchoolInput {
  name: string;
  slug: string;
  timezone: string;
}

export function useCreateSchool() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (input: CreateSchoolInput) => api.post<School>('/admin/schools', input),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: SCHOOLS_QUERY_KEY });
    },
  });
}

interface UpdateSchoolInput {
  name?: string;
  timezone?: string;
  status?: 'active' | 'suspended';
  custom_domain?: string | null;
}

export function useUpdateSchool(schoolId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (input: UpdateSchoolInput) => api.patch<School>(`/admin/schools/${schoolId}`, input),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: SCHOOLS_QUERY_KEY });
    },
  });
}

export function academicYearsQueryKey(schoolId: string) {
  return ['admin', 'schools', schoolId, 'academic-years'] as const;
}

export function useAcademicYears(schoolId: string) {
  return useQuery({
    queryKey: academicYearsQueryKey(schoolId),
    queryFn: () => api.get<AcademicYear[]>(`/admin/schools/${schoolId}/academic-years`),
    enabled: schoolId.length > 0,
  });
}

interface CreateAcademicYearInput {
  name: string;
  starts_on: string;
  ends_on: string;
}

export function useCreateAcademicYear(schoolId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (input: CreateAcademicYearInput) =>
      api.post<AcademicYear>(`/admin/schools/${schoolId}/academic-years`, input),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: academicYearsQueryKey(schoolId) });
    },
  });
}

export function useActivateAcademicYear(schoolId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (academicYearId: string) =>
      api.post<AcademicYear>(`/admin/schools/${schoolId}/academic-years/${academicYearId}/activate`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: academicYearsQueryKey(schoolId) });
    },
  });
}
