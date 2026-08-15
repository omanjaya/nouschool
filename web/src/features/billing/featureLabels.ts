/**
 * Kamus label Indonesia untuk kunci fitur plan (docs/09-billing.md contoh:
 * `{"tv_dashboard": true, "whatsapp": true, "dapodik_import": true, ...}`).
 * Dipakai daftar fitur di kartu langganan (/tagihan) & editor plan super
 * admin (/admin/plans). Kunci yang belum terdaftar di sini tetap tampil
 * (fallback ke kunci mentah) supaya fitur baru tidak membuat UI pecah.
 */
export const FEATURE_LABELS: Record<string, string> = {
  tv_dashboard: 'Dashboard TV',
  qr_card: 'Kartu QR Siswa',
  self_checkin: 'Check-in Mandiri (GPS)',
  whatsapp: 'Notifikasi WhatsApp',
  dapodik_import: 'Import Dapodik',
};

export function featureLabel(key: string): string {
  return FEATURE_LABELS[key] ?? key;
}
