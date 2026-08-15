/**
 * Format rupiah — "Rp 4.000.000" (docs/09-billing.md, harga langganan/tagihan).
 * `toLocaleString('id-ID')` sudah memakai titik sebagai pemisah ribuan.
 */
export function formatRupiah(amount: number): string {
  return `Rp ${amount.toLocaleString('id-ID')}`;
}
