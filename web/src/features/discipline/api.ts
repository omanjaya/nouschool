import { useInfiniteQuery, useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { api } from '../../lib/api';
import type {
  ChildRef,
  DisciplineLetterListResult,
  DisciplineRecordCreateInput,
  DisciplineRecordCreateResult,
  DisciplineRecordListResult,
  DisciplineSpSettings,
  DisciplineSummaryRow,
  DisciplineType,
  DisciplineTypeInput,
  StudentDisciplineSummary,
} from '../../lib/types';

export const DISCIPLINE_TYPES_KEY = ['discipline', 'types'] as const;
export const DISCIPLINE_SP_SETTINGS_KEY = ['discipline', 'sp-settings'] as const;
export const DISCIPLINE_RECORDS_KEY = 'discipline-records' as const;
export const DISCIPLINE_LETTERS_KEY = 'discipline-letters' as const;
export const DISCIPLINE_SUMMARY_KEY = 'discipline-summary' as const;
export const STUDENT_DISCIPLINE_KEY = 'student-discipline' as const;

/** Helper generik paginasi "Muat lebih banyak" — berhenti begitu total item termuat mencapai `total` (pola dipakai `useDisciplineRecords` & `useDisciplineLetters`), sedikit lebih presisi dari pola `features/notifications` karena kontrak di sini memang mengirim `total`. */
function useInfinitePagedQuery<TPage extends { items: unknown[]; total: number }>(
  queryKey: readonly unknown[],
  fetchPage: (page: number) => Promise<TPage>,
) {
  return useInfiniteQuery({
    queryKey,
    queryFn: ({ pageParam }) => fetchPage(pageParam),
    initialPageParam: 1,
    getNextPageParam: (lastPage, allPages) => {
      const loaded = allPages.reduce((n, p) => n + p.items.length, 0);
      return loaded < lastPage.total ? allPages.length + 1 : undefined;
    },
  });
}

/* ---- Master jenis pelanggaran (admin) ---- */

/** GET /api/discipline/types. */
export function useDisciplineTypes() {
  return useQuery({
    queryKey: DISCIPLINE_TYPES_KEY,
    queryFn: () => api.get<DisciplineType[]>('/discipline/types'),
  });
}

export function useCreateDisciplineType() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (input: DisciplineTypeInput) => api.post<DisciplineType>('/discipline/types', input),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: DISCIPLINE_TYPES_KEY }),
  });
}

export function useUpdateDisciplineType(id: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (input: DisciplineTypeInput) => api.patch<DisciplineType>(`/discipline/types/${id}`, input),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: DISCIPLINE_TYPES_KEY }),
  });
}

/** DELETE /api/discipline/types/{id} — server bisa 409 kalau jenis sudah dipakai (baca `ApiError.status` di pemanggil). */
export function useDeleteDisciplineType() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => api.delete<undefined>(`/discipline/types/${id}`),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: DISCIPLINE_TYPES_KEY }),
  });
}

/* ---- Ambang Surat Peringatan (admin, per TA aktif) ---- */

/** GET /api/discipline/sp-settings. */
export function useDisciplineSpSettings(enabled = true) {
  return useQuery({
    queryKey: DISCIPLINE_SP_SETTINGS_KEY,
    queryFn: () => api.get<DisciplineSpSettings>('/discipline/sp-settings'),
    enabled,
  });
}

/** PUT /api/discipline/sp-settings. */
export function useUpdateDisciplineSpSettings() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (input: DisciplineSpSettings) => api.put<DisciplineSpSettings>('/discipline/sp-settings', input),
    onSuccess: (data) => queryClient.setQueryData(DISCIPLINE_SP_SETTINGS_KEY, data),
  });
}

/* ---- Catatan pelanggaran ---- */

export interface DisciplineRecordsFilter {
  studentId?: string;
  classId?: string;
  from?: string;
  to?: string;
}

function buildRecordsQuery(filter: DisciplineRecordsFilter, page: number): string {
  const qs = new URLSearchParams();
  if (filter.studentId) qs.set('student_id', filter.studentId);
  if (filter.classId) qs.set('class_id', filter.classId);
  if (filter.from) qs.set('from', filter.from);
  if (filter.to) qs.set('to', filter.to);
  qs.set('page', String(page));
  return qs.toString();
}

/** GET /api/discipline/records?... — dimuat bertahap lewat "Muat lebih banyak". */
export function useDisciplineRecords(filter: DisciplineRecordsFilter) {
  return useInfinitePagedQuery<DisciplineRecordListResult>([DISCIPLINE_RECORDS_KEY, filter], (page) =>
    api.get<DisciplineRecordListResult>(`/discipline/records?${buildRecordsQuery(filter, page)}`),
  );
}

/** POST /api/discipline/records — mencatat pelanggaran; bisa memicu Surat Peringatan otomatis (`issued_letter`). */
export function useCreateDisciplineRecord() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (input: DisciplineRecordCreateInput) =>
      api.post<DisciplineRecordCreateResult>('/discipline/records', input),
    onSuccess: (_data, variables) => {
      queryClient.invalidateQueries({ queryKey: [DISCIPLINE_RECORDS_KEY] });
      queryClient.invalidateQueries({ queryKey: [DISCIPLINE_SUMMARY_KEY] });
      queryClient.invalidateQueries({ queryKey: [DISCIPLINE_LETTERS_KEY] });
      queryClient.invalidateQueries({ queryKey: [STUDENT_DISCIPLINE_KEY, variables.student_id] });
    },
  });
}

/** DELETE /api/discipline/records/{id} (admin). */
export function useDeleteDisciplineRecord() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => api.delete<undefined>(`/discipline/records/${id}`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: [DISCIPLINE_RECORDS_KEY] });
      queryClient.invalidateQueries({ queryKey: [DISCIPLINE_SUMMARY_KEY] });
      queryClient.invalidateQueries({ queryKey: [DISCIPLINE_LETTERS_KEY] });
      queryClient.invalidateQueries({ queryKey: [STUDENT_DISCIPLINE_KEY] });
    },
  });
}

/* ---- Riwayat pelanggaran satu siswa (siswa/ortu: milik sendiri) ---- */

/** GET /api/students/{id}/discipline. */
export function useStudentDiscipline(studentId: string | undefined) {
  return useQuery({
    queryKey: [STUDENT_DISCIPLINE_KEY, studentId ?? ''],
    queryFn: () => api.get<StudentDisciplineSummary>(`/students/${studentId}/discipline`),
    enabled: Boolean(studentId),
  });
}

/**
 * GET /api/me/children (ortu) — query key SENGAJA sama dengan
 * `features/attendance/api.ts#useChildren` (dua fitur berbeda memanggil
 * endpoint identik "daftar anak akun ini"; berbagi cache lebih tepat
 * daripada duplikasi query key untuk data yang sama).
 */
export function useDisciplineChildren(enabled = true) {
  return useQuery({
    queryKey: ['me-children'],
    queryFn: () => api.get<ChildRef[]>('/me/children'),
    enabled,
  });
}

/* ---- Surat Peringatan ---- */

export interface DisciplineLettersFilter {
  level?: '1' | '2' | '3';
}

function buildLettersQuery(filter: DisciplineLettersFilter, page: number): string {
  const qs = new URLSearchParams();
  if (filter.level) qs.set('level', filter.level);
  qs.set('page', String(page));
  return qs.toString();
}

/** GET /api/discipline/letters?page=&level= — dimuat bertahap lewat "Muat lebih banyak". */
export function useDisciplineLetters(filter: DisciplineLettersFilter) {
  return useInfinitePagedQuery<DisciplineLetterListResult>([DISCIPLINE_LETTERS_KEY, filter], (page) =>
    api.get<DisciplineLetterListResult>(`/discipline/letters?${buildLettersQuery(filter, page)}`),
  );
}

export function disciplineLetterPdfUrl(id: string): string {
  return `/api/discipline/letters/${id}/pdf`;
}

/* ---- Rekap & export ---- */

export interface DisciplineSummaryFilter {
  classId?: string;
  from?: string;
  to?: string;
}

function buildSummaryQuery(filter: DisciplineSummaryFilter): string {
  const qs = new URLSearchParams();
  if (filter.classId) qs.set('class_id', filter.classId);
  if (filter.from) qs.set('from', filter.from);
  if (filter.to) qs.set('to', filter.to);
  return qs.toString();
}

/** GET /api/discipline/summary?class_id=&from=&to= — rekap per siswa (admin/guru/kepsek). */
export function useDisciplineSummary(filter: DisciplineSummaryFilter, enabled = true) {
  return useQuery({
    queryKey: [DISCIPLINE_SUMMARY_KEY, filter],
    queryFn: () => api.get<DisciplineSummaryRow[]>(`/discipline/summary?${buildSummaryQuery(filter)}`),
    enabled,
  });
}

export function disciplineExportUrl(filter: DisciplineSummaryFilter): string {
  return `/api/discipline/export?${buildSummaryQuery(filter)}`;
}
