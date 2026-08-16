import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { api } from '../../lib/api';
import type {
  CurrentPeriodResult,
  DayOfWeek,
  Period,
  PeriodOverridesInput,
  PeriodOverridesResult,
  Room,
  ScheduleCopyResult,
  ScheduleSlot,
  ScheduleSlotInput,
  ScheduleTodaySlot,
} from '../../lib/types';

/* ---- Periods / jam pelajaran ---- */

export const PERIODS_QUERY_KEY = ['periods'] as const;

/** GET /api/periods */
export function usePeriods() {
  return useQuery({
    queryKey: PERIODS_QUERY_KEY,
    queryFn: () => api.get<Period[]>('/periods'),
  });
}

export interface PeriodInput {
  number: number;
  starts_at: string;
  ends_at: string;
  label: string | null;
}

/** PUT /api/periods (admin) — replace-all: body array penuh menggantikan seluruh daftar jam. */
export function useSavePeriods() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (periods: PeriodInput[]) => api.put<Period[]>('/periods', periods),
    onSuccess: (data) => {
      queryClient.setQueryData(PERIODS_QUERY_KEY, data);
    },
  });
}

/* ---- Jam khusus per hari / period overrides (Fase 14 Gelombang D) ---- */

export function periodOverridesQueryKey(day: DayOfWeek) {
  return [...PERIODS_QUERY_KEY, 'overrides', day] as const;
}

/** GET /api/periods/overrides?day=1..6 — `periods: []` berarti hari ini ikut jam default. */
export function usePeriodOverrides(day: DayOfWeek, enabled = true) {
  return useQuery({
    queryKey: periodOverridesQueryKey(day),
    queryFn: () => api.get<PeriodOverridesResult>(`/periods/overrides?day=${day}`),
    enabled,
  });
}

/**
 * PUT /api/periods/overrides {day_of_week,periods} — `periods: []` menghapus
 * jadwal khusus. ASUMSI bentuk respons: sama `PeriodOverridesResult` seperti
 * GET (kontrak ringkas tidak merincinya, tapi konsisten dengan pola endpoint
 * lain di modul ini yang mengembalikan snapshot terbaru daripada `undefined`).
 */
export function useSavePeriodOverrides() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (input: PeriodOverridesInput) => api.put<PeriodOverridesResult>('/periods/overrides', input),
    onSuccess: (data, variables) => {
      queryClient.setQueryData(periodOverridesQueryKey(variables.day_of_week), data);
    },
  });
}

/* ---- Rooms / ruangan ---- */

export const ROOMS_QUERY_KEY = ['rooms'] as const;

/** GET /api/rooms */
export function useRooms() {
  return useQuery({
    queryKey: ROOMS_QUERY_KEY,
    queryFn: () => api.get<Room[]>('/rooms'),
  });
}

export interface RoomInput {
  name: string;
}

export function useCreateRoom() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (input: RoomInput) => api.post<Room>('/rooms', input),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ROOMS_QUERY_KEY }),
  });
}

export function useUpdateRoom(id: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (input: RoomInput) => api.patch<Room>(`/rooms/${id}`, input),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ROOMS_QUERY_KEY }),
  });
}

export function useDeleteRoom() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => api.delete<undefined>(`/rooms/${id}`),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ROOMS_QUERY_KEY }),
  });
}

/** POST /api/rooms/{id}/regenerate-qr — QR lama tidak berlaku lagi setelah ini. */
export function useRegenerateRoomQr() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => api.post<Room>(`/rooms/${id}/regenerate-qr`),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ROOMS_QUERY_KEY }),
  });
}

/** GET /api/rooms/{id}/qr.png (admin) — dipakai langsung sebagai <img src>, bukan fetch. */
export function roomQrImageUrl(id: string): string {
  return `/api/rooms/${id}/qr.png`;
}

/* ---- Schedule slots (builder admin) ---- */

export interface ScheduleFilter {
  classId?: string;
  teacherId?: string;
}

function scheduleFilterQuery(filter: ScheduleFilter): string {
  const params = new URLSearchParams();
  if (filter.classId) params.set('class_id', filter.classId);
  if (filter.teacherId) params.set('teacher_id', filter.teacherId);
  const qs = params.toString();
  return qs ? `?${qs}` : '';
}

export const SCHEDULE_SLOTS_QUERY_KEY = 'schedule-slots' as const;

export function scheduleSlotsQueryKey(filter: ScheduleFilter) {
  return [SCHEDULE_SLOTS_QUERY_KEY, filter.classId ?? '', filter.teacherId ?? ''] as const;
}

/**
 * GET /api/schedule/slots?class_id=|teacher_id= — filter kosong dipakai untuk
 * guru/siswa melihat jadwal seminggu penuh milik sendiri (lihat catatan asumsi
 * di `useScheduleToday`). Builder admin selalu mengisi salah satu id.
 */
export function useScheduleSlots(filter: ScheduleFilter, enabled = true) {
  return useQuery({
    queryKey: scheduleSlotsQueryKey(filter),
    queryFn: () => api.get<ScheduleSlot[]>(`/schedule/slots${scheduleFilterQuery(filter)}`),
    enabled,
  });
}

export function useCreateSlot() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (input: ScheduleSlotInput) => api.post<ScheduleSlot>('/schedule/slots', input),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: [SCHEDULE_SLOTS_QUERY_KEY] }),
  });
}

export function useUpdateSlot(id: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (input: ScheduleSlotInput) => api.patch<ScheduleSlot>(`/schedule/slots/${id}`, input),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: [SCHEDULE_SLOTS_QUERY_KEY] }),
  });
}

export function useDeleteSlot() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => api.delete<undefined>(`/schedule/slots/${id}`),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: [SCHEDULE_SLOTS_QUERY_KEY] }),
  });
}

/** POST /api/schedule/copy — salin seluruh slot kelas sumber ke kelas tujuan. */
export function useCopySchedule() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (input: { from_class_id: string; to_class_id: string }) =>
      api.post<ScheduleCopyResult>('/schedule/copy', input),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: [SCHEDULE_SLOTS_QUERY_KEY] }),
  });
}

/* ---- Jadwal hari ini & jam berjalan (guru/siswa, dan kartu Beranda) ---- */

/**
 * GET /api/schedule/today?class_id=|teacher_id= — dipanggil TANPA parameter untuk
 * guru/siswa yang melihat jadwal sendiri.
 *
 * ASUMSI (kontrak Fase 5 tidak menyebut sumber teacher_id/class_id "diri sendiri"
 * secara eksplisit): backend menginfer dari sesi login, sama seperti pola yang
 * sudah dipakai `GET /api/attendance/classes?date=` (Fase 3) untuk guru — endpoint
 * itu juga tidak menerima teacher_id, backend mengambil dari membership sesi.
 * Kalau asumsi ini salah, layar akan menampilkan ErrorState (lihat SchedulePage).
 */
export function useScheduleToday(filter: ScheduleFilter, enabled = true) {
  return useQuery({
    queryKey: ['schedule-today', filter.classId ?? '', filter.teacherId ?? ''],
    queryFn: () => api.get<ScheduleTodaySlot[]>(`/schedule/today${scheduleFilterQuery(filter)}`),
    enabled,
  });
}

/** GET /api/schedule/current-period — dipoll ringan supaya tag "Sekarang" tetap akurat. */
export function useCurrentPeriod(enabled = true) {
  return useQuery({
    queryKey: ['schedule-current-period'],
    queryFn: () => api.get<CurrentPeriodResult>('/schedule/current-period'),
    enabled,
    refetchInterval: 60_000,
  });
}
