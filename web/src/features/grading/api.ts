import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { api } from '../../lib/api';
import type {
  ChildRef,
  GradingComponent,
  GradingComponentGradesResult,
  GradingComponentInput,
  GradingComponentListResult,
  GradingGradeInput,
  GradingGradesSaveResult,
  GradingPublicationInput,
  GradingRecapResult,
  GradingSettings,
  GradingStar,
  GradingStarInput,
  GradingStarListResult,
  GradingStatus,
  MyGradesResult,
  MyStarsResult,
} from '../../lib/types';

/* ---- Query keys (dideklarasikan di atas — dipakai lintas hook & realtime mapping App.tsx) ---- */

export const GRADING_STATUS_KEY = ['grading', 'status'] as const;
export const GRADING_COMPONENTS_KEY = 'grading-components' as const;
export const GRADING_COMPONENT_GRADES_KEY = 'grading-component-grades' as const;
export const GRADING_RECAP_KEY = 'grading-recap' as const;
export const MY_GRADES_KEY = 'my-grades' as const;
export const GRADING_STARS_KEY = 'grading-stars' as const;
export const MY_STARS_KEY = 'my-stars' as const;
export const GRADING_SETTINGS_KEY = ['grading-settings'] as const;

/* ---- Status modul (semua role, host tenant) ---- */

/** GET /api/grading/status. Endpoint lain 404 `{code:'grading_disabled'}` kalau `enabled=false`. */
export function useGradingStatus() {
  return useQuery({
    queryKey: GRADING_STATUS_KEY,
    queryFn: () => api.get<GradingStatus>('/grading/status'),
  });
}

/* ---- Konteks kelas + mapel (dipakai Komponen/Input Nilai/Rekap) ---- */

export interface GradingContextFilter {
  classId: string;
  subjectId: string;
}

function contextQuery(filter: GradingContextFilter): string {
  const qs = new URLSearchParams();
  if (filter.classId) qs.set('class_id', filter.classId);
  if (filter.subjectId) qs.set('subject_id', filter.subjectId);
  return qs.toString();
}

/* ---- Komponen penilaian (guru/admin) ---- */

/** GET /api/grading/components?class_id=&subject_id=. */
export function useGradingComponents(filter: GradingContextFilter, enabled = true) {
  return useQuery({
    queryKey: [GRADING_COMPONENTS_KEY, filter.classId, filter.subjectId],
    queryFn: () => api.get<GradingComponentListResult>(`/grading/components?${contextQuery(filter)}`),
    enabled: enabled && Boolean(filter.classId && filter.subjectId),
  });
}

export function useCreateGradingComponent() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (input: GradingComponentInput) => api.post<GradingComponent>('/grading/components', input),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: [GRADING_COMPONENTS_KEY] }),
  });
}

export function useUpdateGradingComponent(id: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (input: GradingComponentInput) => api.patch<GradingComponent>(`/grading/components/${id}`, input),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: [GRADING_COMPONENTS_KEY] }),
  });
}

/** DELETE /api/grading/components/{id} — nilai yang sudah diisi untuk komponen ini ikut terhapus (konfirmasi di pemanggil). */
export function useDeleteGradingComponent() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => api.delete<undefined>(`/grading/components/${id}`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: [GRADING_COMPONENTS_KEY] });
      queryClient.invalidateQueries({ queryKey: [GRADING_RECAP_KEY] });
    },
  });
}

/* ---- Input nilai per komponen (guru/admin) ---- */

/** GET /api/grading/components/{id}/grades. */
export function useComponentGrades(componentId: string, enabled = true) {
  return useQuery({
    queryKey: [GRADING_COMPONENT_GRADES_KEY, componentId],
    queryFn: () => api.get<GradingComponentGradesResult>(`/grading/components/${componentId}/grades`),
    enabled: enabled && Boolean(componentId),
  });
}

/** PUT /api/grading/components/{id}/grades body `{grades}` — simpan bulk (dirty guard di pemanggil, pola `AttendanceSessionPage`). */
export function useSaveComponentGrades(componentId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (grades: GradingGradeInput[]) =>
      api.put<GradingGradesSaveResult>(`/grading/components/${componentId}/grades`, { grades }),
    onSuccess: (data) => {
      queryClient.setQueryData([GRADING_COMPONENT_GRADES_KEY, componentId], data);
      queryClient.invalidateQueries({ queryKey: [GRADING_COMPONENTS_KEY] });
      queryClient.invalidateQueries({ queryKey: [GRADING_RECAP_KEY] });
    },
  });
}

/* ---- Rekap & publikasi (guru/admin) ---- */

/** GET /api/grading/recap?class_id=&subject_id=. */
export function useGradingRecap(filter: GradingContextFilter, enabled = true) {
  return useQuery({
    queryKey: [GRADING_RECAP_KEY, filter.classId, filter.subjectId],
    queryFn: () => api.get<GradingRecapResult>(`/grading/recap?${contextQuery(filter)}`),
    enabled: enabled && Boolean(filter.classId && filter.subjectId),
  });
}

/** PUT /api/grading/publication — publikasikan/batalkan publikasi nilai kelas-mapel. */
export function useSetGradingPublication() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (input: GradingPublicationInput) => api.put<undefined>('/grading/publication', input),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: [GRADING_RECAP_KEY] });
      queryClient.invalidateQueries({ queryKey: [MY_GRADES_KEY] });
    },
  });
}

/** `<a href>` langsung (bukan fetch) — pola sama `disciplineExportUrl`. */
export function gradingExportUrl(filter: GradingContextFilter): string {
  return `/api/grading/export?${contextQuery(filter)}`;
}

/* ---- Nilai siswa/ortu ---- */

/** GET /api/my-grades (siswa: tanpa param, backend infer diri sendiri; ortu: `?student_id=`). */
export function useMyGrades(studentId?: string) {
  return useQuery({
    queryKey: [MY_GRADES_KEY, studentId ?? ''],
    queryFn: () => api.get<MyGradesResult>(`/my-grades${studentId ? `?student_id=${studentId}` : ''}`),
  });
}

/* ---- Bintang kelas ---- */

/** POST /api/grading/stars {student_id,delta,note?,visibility}. */
export function useCreateGradingStar() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (input: GradingStarInput) => api.post<GradingStar>('/grading/stars', input),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: [GRADING_STARS_KEY] });
      queryClient.invalidateQueries({ queryKey: [MY_STARS_KEY] });
    },
  });
}

/** DELETE /api/grading/stars/{id}. */
export function useDeleteGradingStar() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => api.delete<undefined>(`/grading/stars/${id}`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: [GRADING_STARS_KEY] });
      queryClient.invalidateQueries({ queryKey: [MY_STARS_KEY] });
    },
  });
}

export interface GradingStarsFilter {
  studentId?: string;
  classId?: string;
}

/** GET /api/grading/stars?student_id=|class_id= (guru/admin) — lihat ASUMSI bentuk respons di `lib/types.ts#GradingStar`. */
export function useGradingStars(filter: GradingStarsFilter, enabled = true) {
  const qs = new URLSearchParams();
  if (filter.studentId) qs.set('student_id', filter.studentId);
  if (filter.classId) qs.set('class_id', filter.classId);
  return useQuery({
    queryKey: [GRADING_STARS_KEY, filter.studentId ?? '', filter.classId ?? ''],
    queryFn: () => api.get<GradingStarListResult>(`/grading/stars?${qs.toString()}`),
    enabled: enabled && Boolean(filter.studentId || filter.classId),
  });
}

/** GET /api/my-stars (siswa: tanpa param; ortu: `?student_id=`). */
export function useMyStars(studentId?: string) {
  return useQuery({
    queryKey: [MY_STARS_KEY, studentId ?? ''],
    queryFn: () => api.get<MyStarsResult>(`/my-stars${studentId ? `?student_id=${studentId}` : ''}`),
  });
}

/**
 * GET /api/me/children (ortu) — query key SENGAJA sama dengan
 * `features/discipline/api.ts#useDisciplineChildren` (data "daftar anak akun
 * ini" identik lintas fitur, berbagi cache lebih tepat daripada duplikasi).
 */
export function useGradingChildren(enabled = true) {
  return useQuery({
    queryKey: ['me-children'],
    queryFn: () => api.get<ChildRef[]>('/me/children'),
    enabled,
  });
}

/* ---- Pengaturan admin (`/pengaturan` seksi "Penilaian") ---- */

/** GET /api/settings/grading. */
export function useGradingSettings() {
  return useQuery({
    queryKey: GRADING_SETTINGS_KEY,
    queryFn: () => api.get<GradingSettings>('/settings/grading'),
  });
}

/** PUT /api/settings/grading — juga memengaruhi `GET /api/grading/status` (di-invalidate bersamaan). */
export function useUpdateGradingSettings() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (input: GradingSettings) => api.put<GradingSettings>('/settings/grading', input),
    onSuccess: (data) => {
      queryClient.setQueryData(GRADING_SETTINGS_KEY, data);
      queryClient.invalidateQueries({ queryKey: GRADING_STATUS_KEY });
    },
  });
}
