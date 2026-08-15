import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { api } from '../../lib/api';
import type {
  AttendanceClassListItem,
  AttendanceClassSummary,
  AttendanceRecordInput,
  AttendanceSessionResult,
  ChildRef,
  StudentAttendanceHistory,
} from '../../lib/types';

export const ATTENDANCE_CLASSES_KEY = 'attendance-classes' as const;
export const ATTENDANCE_SESSION_KEY = 'attendance-session' as const;
export const ATTENDANCE_SUMMARY_KEY = 'attendance-summary' as const;
export const ATTENDANCE_HISTORY_KEY = 'attendance-history' as const;

/** GET /api/attendance/classes?date= — daftar rombel guru/admin untuk satu tanggal. */
export function useAttendanceClasses(date: string) {
  return useQuery({
    queryKey: [ATTENDANCE_CLASSES_KEY, date],
    queryFn: () => api.get<AttendanceClassListItem[]>(`/attendance/classes?date=${date}`),
  });
}

/** POST /api/attendance/sessions — buka (atau ambil) sesi absensi hari ini untuk satu rombel. */
export function useOpenAttendanceSession() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (classId: string) =>
      api.post<AttendanceSessionResult>('/attendance/sessions', { class_id: classId }),
    onSuccess: (data) => {
      queryClient.setQueryData(attendanceSessionQueryKey(data.session.id), data);
      queryClient.invalidateQueries({ queryKey: [ATTENDANCE_CLASSES_KEY] });
    },
  });
}

export function attendanceSessionQueryKey(id: string) {
  return [ATTENDANCE_SESSION_KEY, id] as const;
}

/** GET /api/attendance/sessions/{id} */
export function useAttendanceSession(id: string | undefined) {
  return useQuery({
    queryKey: attendanceSessionQueryKey(id ?? ''),
    queryFn: () => api.get<AttendanceSessionResult>(`/attendance/sessions/${id}`),
    enabled: Boolean(id),
  });
}

/** PUT /api/attendance/sessions/{id}/records — simpan bulk record sesi. */
export function useSaveAttendanceRecords(id: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (records: AttendanceRecordInput[]) =>
      api.put<AttendanceSessionResult>(`/attendance/sessions/${id}/records`, { records }),
    onSuccess: (data) => {
      queryClient.setQueryData(attendanceSessionQueryKey(id), data);
      queryClient.invalidateQueries({ queryKey: [ATTENDANCE_CLASSES_KEY] });
    },
  });
}

/** POST /api/attendance/sessions/{id}/finalize — kunci sesi (non-admin tidak bisa ubah lagi). */
export function useFinalizeAttendanceSession(id: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: () => api.post<AttendanceSessionResult>(`/attendance/sessions/${id}/finalize`),
    onSuccess: (data) => {
      queryClient.setQueryData(attendanceSessionQueryKey(id), data);
      queryClient.invalidateQueries({ queryKey: [ATTENDANCE_CLASSES_KEY] });
    },
  });
}

/** GET /api/attendance/summary?date= — rekap harian per rombel (admin/kepsek). */
export function useAttendanceSummary(date: string, enabled = true) {
  return useQuery({
    queryKey: [ATTENDANCE_SUMMARY_KEY, date],
    queryFn: () => api.get<AttendanceClassSummary[]>(`/attendance/summary?date=${date}`),
    enabled,
  });
}

/** GET /api/me/children — daftar anak untuk akun orang tua. */
export function useChildren(enabled = true) {
  return useQuery({
    queryKey: ['me-children'],
    queryFn: () => api.get<ChildRef[]>('/me/children'),
    enabled,
  });
}

/** GET /api/students/{id}/attendance?from=&to= — riwayat kehadiran satu siswa. */
export function useStudentAttendanceHistory(studentId: string | undefined, from: string, to: string) {
  return useQuery({
    queryKey: [ATTENDANCE_HISTORY_KEY, studentId, from, to],
    queryFn: () => api.get<StudentAttendanceHistory>(`/students/${studentId}/attendance?from=${from}&to=${to}`),
    enabled: Boolean(studentId),
  });
}
