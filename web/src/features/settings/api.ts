import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { api } from '../../lib/api';
import { PUBLIC_CONTEXT_QUERY_KEY } from '../branding/api';
import type { Branding, CustomDomainStatus, LettersSettings } from '../../lib/types';

export const BRANDING_QUERY_KEY = ['settings', 'branding'] as const;

export function useBranding() {
  return useQuery({
    queryKey: BRANDING_QUERY_KEY,
    queryFn: () => api.get<Branding>('/settings/branding'),
  });
}

export function useUpdateBranding() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (input: Branding) => api.put<Branding>('/settings/branding', input),
    onSuccess: (data) => {
      queryClient.setQueryData(BRANDING_QUERY_KEY, data);
      // Nama & warna baru harus langsung terlihat di AppShell/LoginPage lain
      // (dan tab lain) — bukan hanya di form ini.
      queryClient.invalidateQueries({ queryKey: PUBLIC_CONTEXT_QUERY_KEY });
    },
  });
}

/** POST /api/settings/branding/logo — multipart `file` (png/jpg, maks 2MB, admin). */
export function useUploadBrandingLogo() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (file: File) => {
      const fd = new FormData();
      fd.append('file', file);
      return api.upload<Branding>('/settings/branding/logo', fd);
    },
    onSuccess: (data) => {
      queryClient.setQueryData(BRANDING_QUERY_KEY, data);
      queryClient.invalidateQueries({ queryKey: PUBLIC_CONTEXT_QUERY_KEY });
    },
  });
}

/* ---- Custom domain (Fase 11, docs/01-tenant.md §Custom domain & Caddy) ---- */

export const CUSTOM_DOMAIN_QUERY_KEY = ['settings', 'custom-domain'] as const;

/** GET /api/custom-domain (admin). */
export function useCustomDomain() {
  return useQuery({
    queryKey: CUSTOM_DOMAIN_QUERY_KEY,
    queryFn: () => api.get<CustomDomainStatus>('/custom-domain'),
  });
}

/** PUT /api/custom-domain {domain} — mengajukan domain baru (belum verified). */
export function useSetCustomDomain() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (domain: string) => api.put<CustomDomainStatus>('/custom-domain', { domain }),
    onSuccess: (data) => {
      queryClient.setQueryData(CUSTOM_DOMAIN_QUERY_KEY, data);
    },
  });
}

/** POST /api/custom-domain/verify — cek DNS sekarang; error server (mis. A record belum mengarah) diteruskan apa adanya. */
export function useVerifyCustomDomain() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: () => api.post<CustomDomainStatus>('/custom-domain/verify'),
    onSuccess: (data) => {
      queryClient.setQueryData(CUSTOM_DOMAIN_QUERY_KEY, data);
    },
  });
}

/** DELETE /api/custom-domain — lepaskan domain sendiri, kembali ke subdomain default. */
export function useDeleteCustomDomain() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: () => api.delete<undefined>('/custom-domain'),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: CUSTOM_DOMAIN_QUERY_KEY });
    },
  });
}

/* ---- Template surat / catatan kaki (Fase 14 Gelombang D) ---- */

export const LETTERS_SETTINGS_QUERY_KEY = ['settings', 'letters'] as const;

/** GET /api/settings/letters (admin). */
export function useLettersSettings() {
  return useQuery({
    queryKey: LETTERS_SETTINGS_QUERY_KEY,
    queryFn: () => api.get<LettersSettings>('/settings/letters'),
  });
}

/** PUT /api/settings/letters (admin). */
export function useUpdateLettersSettings() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (input: LettersSettings) => api.put<LettersSettings>('/settings/letters', input),
    onSuccess: (data) => {
      queryClient.setQueryData(LETTERS_SETTINGS_QUERY_KEY, data);
    },
  });
}
