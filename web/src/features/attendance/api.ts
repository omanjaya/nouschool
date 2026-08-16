import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { api } from '../../lib/api';
import type {
  AttendanceAnomaly,
  AttendanceClassListItem,
  AttendanceClassSummary,
  AttendanceRecordInput,
  AttendanceScanResult,
  AttendanceSessionResult,
  AttendanceSettings,
  AttendanceSlotToday,
  ChildRef,
  SelfCheckinInput,
  SelfCheckinResult,
  SelfCheckinStatus,
  StudentAttendanceCalendarResult,
  StudentAttendanceHistory,
  StudentQrCard,
} from '../../lib/types';

export const ATTENDANCE_CLASSES_KEY = 'attendance-classes' as const;
export const ATTENDANCE_SESSION_KEY = 'attendance-session' as const;
export const ATTENDANCE_SUMMARY_KEY = 'attendance-summary' as const;
export const ATTENDANCE_HISTORY_KEY = 'attendance-history' as const;
export const ATTENDANCE_SLOTS_TODAY_KEY = 'attendance-slots-today' as const;

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

/**
 * GET /api/attendance/slots-today (guru) — jadwal mengajar hari ini mode
 * per-mapel, satu baris per slot dengan status sesi absensinya. Di sekolah
 * mode daily-only ini mengembalikan array kosong (bukan error) — heuristik
 * dipakai `AttendanceClassesPage` untuk memutuskan menampilkan seksi
 * "Jadwal Mengajar Hari Ini" atau tidak (lihat docs/05-attendance.md).
 */
export function useAttendanceSlotsToday(enabled = true) {
  return useQuery({
    queryKey: [ATTENDANCE_SLOTS_TODAY_KEY],
    queryFn: () => api.get<AttendanceSlotToday[]>('/attendance/slots-today'),
    enabled,
  });
}

/** POST /api/attendance/sessions {schedule_slot_id} — buka sesi absensi mode per-mapel dari slot jadwal. */
export function useOpenAttendanceSessionForSlot() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (scheduleSlotId: string) =>
      api.post<AttendanceSessionResult>('/attendance/sessions', { schedule_slot_id: scheduleSlotId }),
    onSuccess: (data) => {
      queryClient.setQueryData(attendanceSessionQueryKey(data.session.id), data);
      queryClient.invalidateQueries({ queryKey: [ATTENDANCE_CLASSES_KEY] });
      queryClient.invalidateQueries({ queryKey: [ATTENDANCE_SLOTS_TODAY_KEY] });
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

export const ATTENDANCE_CALENDAR_KEY = 'attendance-calendar' as const;

/** GET /api/students/{id}/attendance/calendar?month=YYYY-MM — kalender bulanan (Fase 14 Gelombang D). */
export function useStudentAttendanceCalendar(studentId: string | undefined, month: string) {
  return useQuery({
    queryKey: [ATTENDANCE_CALENDAR_KEY, studentId, month],
    queryFn: () =>
      api.get<StudentAttendanceCalendarResult>(`/students/${studentId}/attendance/calendar?month=${month}`),
    enabled: Boolean(studentId),
  });
}

/* ---- Kartu QR siswa (Fase 8, admin `student:manage`) ---- */

export const QR_CARDS_QUERY_KEY = 'attendance-qr-cards' as const;

/**
 * POST /api/attendance/qr-cards/generate {class_id} — buat/ambil kartu QR
 * untuk seluruh anggota rombel sekaligus (idempotent: siswa yang sudah punya
 * kartu aktif tidak diberi token baru).
 */
export function useGenerateQrCards() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (classId: string) =>
      api.post<StudentQrCard[]>('/attendance/qr-cards/generate', { class_id: classId }),
    onSuccess: (data, classId) => {
      queryClient.setQueryData([QR_CARDS_QUERY_KEY, classId], data);
    },
  });
}

/** GET /api/attendance/qr-cards?class_id= */
export function useQrCards(classId: string | undefined, enabled = true) {
  return useQuery({
    queryKey: [QR_CARDS_QUERY_KEY, classId ?? ''],
    queryFn: () => api.get<StudentQrCard[]>(`/attendance/qr-cards?class_id=${classId}`),
    enabled: enabled && Boolean(classId),
  });
}

/**
 * POST /api/attendance/qr-cards/{studentId}/revoke — "Kartu lama tidak
 * berlaku." ASUMSI: respons berisi kartu pengganti (token baru) supaya
 * halaman cetak bisa langsung menampilkan QR baru tanpa fetch ulang seluruh
 * rombel — konsisten dengan tombol aksinya "Cabut & ganti", bukan sekadar
 * "Cabut".
 */
export function useRevokeQrCard() {
  return useMutation({
    mutationFn: (studentId: string) => api.post<StudentQrCard>(`/attendance/qr-cards/${studentId}/revoke`),
  });
}

/** GET /api/attendance/qr-cards/{studentId}/qr.png — dipakai langsung sebagai <img src>. */
export function qrCardImageUrl(studentId: string, cacheBust?: number): string {
  return `/api/attendance/qr-cards/${studentId}/qr.png${cacheBust ? `?v=${cacheBust}` : ''}`;
}

/* ---- Scan kartu QR di sesi absensi (guru, Fase 8) ---- */

/** POST /api/attendance/sessions/{id}/scan {token} — kirim token QR apa adanya (backend toleran prefix). */
export function useAttendanceScan(sessionId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (token: string) => api.post<AttendanceScanResult>(`/attendance/sessions/${sessionId}/scan`, { token }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: [ATTENDANCE_CLASSES_KEY] });
    },
  });
}

/* ---- Self check-in siswa (Fase 8) ---- */

export const SELF_CHECKIN_STATUS_KEY = 'attendance-self-checkin-status' as const;

/** GET /api/attendance/self-checkin/status (siswa). */
export function useSelfCheckinStatus(enabled = true) {
  return useQuery({
    queryKey: [SELF_CHECKIN_STATUS_KEY],
    queryFn: () => api.get<SelfCheckinStatus>('/attendance/self-checkin/status'),
    enabled,
  });
}

/** POST /api/attendance/self-checkin {lat,lng,accuracy} */
export function useSelfCheckin() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (input: SelfCheckinInput) => api.post<SelfCheckinResult>('/attendance/self-checkin', input),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: [SELF_CHECKIN_STATUS_KEY] });
    },
  });
}

/* ---- Anomali check-in (admin/kepsek, Fase 8) ---- */

export const ATTENDANCE_ANOMALIES_KEY = 'attendance-anomalies' as const;

/** GET /api/attendance/anomalies?date=&class_id= */
export function useAttendanceAnomalies(date: string, enabled = true) {
  return useQuery({
    queryKey: [ATTENDANCE_ANOMALIES_KEY, date],
    queryFn: () => api.get<AttendanceAnomaly[]>(`/attendance/anomalies?date=${date}`),
    enabled,
  });
}

/* ---- Pengaturan absensi (admin, Fase 8) ---- */

export const ATTENDANCE_SETTINGS_KEY = ['settings', 'attendance'] as const;

/** GET /api/settings/attendance (admin). */
export function useAttendanceSettings(enabled = true) {
  return useQuery({
    queryKey: ATTENDANCE_SETTINGS_KEY,
    queryFn: () => api.get<AttendanceSettings>('/settings/attendance'),
    enabled,
  });
}

/** PUT /api/settings/attendance (admin). */
export function useUpdateAttendanceSettings() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (input: AttendanceSettings) => api.put<AttendanceSettings>('/settings/attendance', input),
    onSuccess: (data) => {
      queryClient.setQueryData(ATTENDANCE_SETTINGS_KEY, data);
    },
  });
}
