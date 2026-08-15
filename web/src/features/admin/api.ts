import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { api } from '../../lib/api';
import type {
  AcademicYear,
  AdminSchoolBillingResult,
  NotificationChannelSettings,
  Plan,
  PlanUpdateInput,
  School,
} from '../../lib/types';

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

/** Pengaturan channel notifikasi per sekolah — hanya super admin (docs/08-notification.md). */
export function notificationSettingsQueryKey(schoolId: string) {
  return ['admin', 'schools', schoolId, 'settings', 'notification'] as const;
}

export function useNotificationChannelSettings(schoolId: string) {
  return useQuery({
    queryKey: notificationSettingsQueryKey(schoolId),
    queryFn: () => api.get<NotificationChannelSettings>(`/admin/schools/${schoolId}/settings/notification`),
    enabled: schoolId.length > 0,
  });
}

export function useUpdateNotificationChannelSettings(schoolId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (input: NotificationChannelSettings) =>
      api.put<NotificationChannelSettings>(`/admin/schools/${schoolId}/settings/notification`, input),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: notificationSettingsQueryKey(schoolId) });
    },
  });
}

/* ---- Billing / Langganan — panel super admin (Fase 10, docs/09-billing.md) ---- */

export const PLANS_QUERY_KEY = ['admin', 'plans'] as const;

/** GET /api/admin/plans */
export function usePlans() {
  return useQuery({
    queryKey: PLANS_QUERY_KEY,
    queryFn: () => api.get<Plan[]>('/admin/plans'),
  });
}

/** PUT /api/admin/plans/{code} */
export function useUpdatePlan() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ code, input }: { code: string; input: PlanUpdateInput }) =>
      api.put<Plan>(`/admin/plans/${code}`, input),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: PLANS_QUERY_KEY });
    },
  });
}

export function schoolBillingQueryKey(schoolId: string) {
  return ['admin', 'schools', schoolId, 'billing'] as const;
}

/** GET /api/admin/schools/{id}/billing — shape sama `GET /api/billing` + `proof_url` per invoice. */
export function useSchoolBilling(schoolId: string) {
  return useQuery({
    queryKey: schoolBillingQueryKey(schoolId),
    queryFn: () => api.get<AdminSchoolBillingResult>(`/admin/schools/${schoolId}/billing`),
    enabled: schoolId.length > 0,
  });
}

/** POST /api/admin/schools/{id}/subscriptions {plan_code} — buat/perpanjang langganan (bracket & harga dihitung server dari jumlah siswa aktif). */
export function useCreateSubscription(schoolId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (planCode: string) =>
      api.post(`/admin/schools/${schoolId}/subscriptions`, { plan_code: planCode }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: schoolBillingQueryKey(schoolId) });
    },
  });
}

/** POST /api/admin/schools/{id}/subscriptions/extend {days} — perpanjangan manual (goodwill). */
export function useExtendSubscription(schoolId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (days: number) => api.post(`/admin/schools/${schoolId}/subscriptions/extend`, { days }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: schoolBillingQueryKey(schoolId) });
    },
  });
}

/**
 * POST /api/admin/invoices/{id}/verify — verifikasi transfer manual.
 * `schoolId` hanya dipakai untuk invalidasi cache halaman detail sekolah
 * (endpoint sendiri tidak butuh school_id di path/body).
 */
export function useVerifyInvoice(schoolId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (invoiceId: string) => api.post(`/admin/invoices/${invoiceId}/verify`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: schoolBillingQueryKey(schoolId) });
    },
  });
}

/** POST /api/admin/invoices/{id}/void */
export function useVoidInvoice(schoolId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (invoiceId: string) => api.post(`/admin/invoices/${invoiceId}/void`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: schoolBillingQueryKey(schoolId) });
    },
  });
}
