import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { api } from '../../lib/api';
import type { BillingResult, Invoice, PayInvoiceResult } from '../../lib/types';

export const BILLING_QUERY_KEY = ['billing'] as const;

/** GET /api/billing (admin & kepsek, host tenant) — langganan berjalan + daftar invoice. */
export function useBilling() {
  return useQuery({
    queryKey: BILLING_QUERY_KEY,
    queryFn: () => api.get<BillingResult>('/billing'),
  });
}

/** POST /api/billing/invoices/{id}/proof — multipart, field `file` (pdf/jpg/png ≤5MB). */
export function useUploadInvoiceProof() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ invoiceId, file }: { invoiceId: string; file: File }) => {
      const fd = new FormData();
      fd.append('file', file);
      return api.upload<Invoice>(`/billing/invoices/${invoiceId}/proof`, fd);
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: BILLING_QUERY_KEY });
    },
  });
}

/**
 * POST /api/billing/invoices/{id}/pay — mengembalikan `redirect_url` ke
 * checkout gateway. 422 (gateway belum dikonfigurasi) dilempar sebagai
 * `ApiError` biasa — pemanggil menampilkan `err.message` lewat Toast.
 */
export function usePayInvoice() {
  return useMutation({
    mutationFn: (invoiceId: string) => api.post<PayInvoiceResult>(`/billing/invoices/${invoiceId}/pay`),
  });
}
