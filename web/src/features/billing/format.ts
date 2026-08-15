import type { InvoiceStatus, SubscriptionStatus } from '../../lib/types';

export const SUBSCRIPTION_STATUS_LABEL: Record<SubscriptionStatus, string> = {
  active: 'Aktif',
  grace: 'Masa Tenggang',
  readonly: 'Baca Saja',
  canceled: 'Dibatalkan',
};

/** Varian `Tag` per status langganan — active→hijau, grace→warning, readonly→danger. */
export const SUBSCRIPTION_STATUS_TAG_VARIANT: Record<SubscriptionStatus, 'success' | 'warning' | 'danger' | 'neutral'> = {
  active: 'success',
  grace: 'warning',
  readonly: 'danger',
  canceled: 'neutral',
};

export const INVOICE_STATUS_LABEL: Record<InvoiceStatus, string> = {
  unpaid: 'Belum Dibayar',
  awaiting_verification: 'Menunggu Verifikasi',
  paid: 'Lunas',
  void: 'Dibatalkan',
  expired: 'Kedaluwarsa',
};

/** Varian `Tag` per status invoice (docs/09-billing.md #Skema `invoices.status`). */
export const INVOICE_STATUS_TAG_VARIANT: Record<InvoiceStatus, 'warning' | 'info' | 'success' | 'done' | 'danger'> = {
  unpaid: 'warning',
  awaiting_verification: 'info',
  paid: 'success',
  void: 'done',
  expired: 'danger',
};
