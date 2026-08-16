import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { api } from '../../lib/api';
import type {
  AcademicYear,
  AdminAuditLogResult,
  AdminOverview,
  AdminSchoolBillingResult,
  AdminSchoolMember,
  AdminSchoolStats,
  ImpersonateStartResult,
  InterestLead,
  NotificationChannelSettings,
  OutboxListResult,
  OutboxRetryAllResult,
  OutboxStatus,
  Plan,
  PlanUpdateInput,
  ResetPasswordResult,
  School,
} from '../../lib/types';

/** Label peran (kamus existing, lihat `features/profile/ProfilePage.tsx`) — dipakai statistik/anggota sekolah di panel super admin. */
export const ROLE_LABEL: Record<string, string> = {
  admin_sekolah: 'Admin Sekolah',
  kepala_sekolah: 'Kepala Sekolah',
  guru: 'Guru',
  siswa: 'Siswa',
  orang_tua: 'Orang Tua',
  display: 'Display',
};

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

/**
 * POST /api/admin/schools/{id}/impersonate (super admin, host platform) —
 * token sekali pakai dipakai untuk membuka sesi support di host TENANT
 * (`GET /impersonate?token=...`, lihat `features/impersonate`). Tidak ada
 * cache untuk diinvalidasi — hasilnya hanya dipakai sekali untuk membangun URL.
 */
export function useImpersonateSchool(schoolId: string) {
  return useMutation({
    mutationFn: () => api.post<ImpersonateStartResult>(`/admin/schools/${schoolId}/impersonate`),
  });
}

/* ---- Minat sekolah dari landing page (Fase 11) ---- */

export const INTEREST_LEADS_QUERY_KEY = ['admin', 'interest'] as const;

/** GET /api/admin/interest (super admin) — daftar minat sekolah dari form landing page. */
export function useInterestLeads() {
  return useQuery({
    queryKey: INTEREST_LEADS_QUERY_KEY,
    queryFn: () => api.get<InterestLead[]>('/admin/interest'),
  });
}

/* ---- Beranda platform (Fase 13 P1, docs/11-superadmin.md) ---- */

export const ADMIN_OVERVIEW_QUERY_KEY = ['admin', 'overview'] as const;

/** GET /api/admin/overview — satu fetch untuk `/admin` (ringkasan + "perlu perhatian" + aktivitas terakhir). */
export function useAdminOverview() {
  return useQuery({
    queryKey: ADMIN_OVERVIEW_QUERY_KEY,
    queryFn: () => api.get<AdminOverview>('/admin/overview'),
  });
}

/* ---- Statistik & kesehatan per sekolah (Fase 13 P3, docs/11 P3) ---- */

export function schoolStatsQueryKey(schoolId: string) {
  return ['admin', 'schools', schoolId, 'stats'] as const;
}

/** GET /api/admin/schools/{id}/stats — hanya agregat/angka (docs/11: tidak pernah data per-siswa). */
export function useSchoolStats(schoolId: string, enabled = true) {
  return useQuery({
    queryKey: schoolStatsQueryKey(schoolId),
    queryFn: () => api.get<AdminSchoolStats>(`/admin/schools/${schoolId}/stats`),
    enabled: enabled && schoolId.length > 0,
  });
}

/* ---- Anggota & reset password darurat (Fase 13 P4, docs/11 P4) ---- */

export function schoolMembersQueryKey(schoolId: string) {
  return ['admin', 'schools', schoolId, 'members'] as const;
}

/** GET /api/admin/schools/{id}/members. */
export function useSchoolMembers(schoolId: string) {
  return useQuery({
    queryKey: schoolMembersQueryKey(schoolId),
    queryFn: () => api.get<AdminSchoolMember[]>(`/admin/schools/${schoolId}/members`),
    enabled: schoolId.length > 0,
  });
}

/**
 * POST /api/admin/users/{id}/reset-password {school_id} — password sementara,
 * ditampilkan sekali di dialog hasil (lihat `SchoolDetailPage`). Semua sesi
 * user tersebut otomatis keluar di server; tidak ada cache list yang perlu
 * diinvalidasi (daftar anggota tidak berubah akibat reset password).
 */
export function useResetPassword() {
  return useMutation({
    mutationFn: ({ userId, schoolId }: { userId: string; schoolId: string }) =>
      api.post<ResetPasswordResult>(`/admin/users/${userId}/reset-password`, { school_id: schoolId }),
  });
}

/* ---- Audit log per sekolah (Fase 13 P4, docs/11 P4) ---- */

export function schoolAuditQueryKey(schoolId: string, params: { page: number; perPage: number; action: string }) {
  return ['admin', 'schools', schoolId, 'audit', params.page, params.perPage, params.action] as const;
}

/** GET /api/admin/schools/{id}/audit?page=&per_page=&action= */
export function useSchoolAudit(schoolId: string, params: { page: number; perPage: number; action?: string }) {
  const action = params.action ?? '';
  const query = new URLSearchParams({ page: String(params.page), per_page: String(params.perPage) });
  if (action) query.set('action', action);

  return useQuery({
    queryKey: schoolAuditQueryKey(schoolId, { page: params.page, perPage: params.perPage, action }),
    queryFn: () => api.get<AdminAuditLogResult>(`/admin/schools/${schoolId}/audit?${query.toString()}`),
    enabled: schoolId.length > 0,
  });
}

/* ---- Outbox notifikasi global (Fase 13 P4, docs/11 P4) ---- */

export function outboxQueryKey(params: { status: OutboxStatus; schoolId?: string; page: number }) {
  return ['admin', 'outbox', params.status, params.schoolId ?? 'all', params.page] as const;
}

/** GET /api/admin/outbox?status=&school_id=&page= */
export function useOutbox(params: { status: OutboxStatus; schoolId?: string; page: number }) {
  const query = new URLSearchParams({ status: params.status, page: String(params.page) });
  if (params.schoolId) query.set('school_id', params.schoolId);

  return useQuery({
    queryKey: outboxQueryKey(params),
    queryFn: () => api.get<OutboxListResult>(`/admin/outbox?${query.toString()}`),
  });
}

const OUTBOX_LIST_QUERY_KEY = ['admin', 'outbox'] as const;

/** POST /api/admin/outbox/{id}/retry — retry satu pesan (dead/failed). */
export function useRetryOutboxItem() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => api.post(`/admin/outbox/${id}/retry`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: OUTBOX_LIST_QUERY_KEY });
    },
  });
}

/** POST /api/admin/outbox/retry-all {school_id?,status} — retry massal sesuai filter aktif. */
export function useRetryAllOutbox() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ status, schoolId }: { status: OutboxStatus; schoolId?: string }) =>
      api.post<OutboxRetryAllResult>('/admin/outbox/retry-all', {
        status,
        school_id: schoolId,
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: OUTBOX_LIST_QUERY_KEY });
    },
  });
}
