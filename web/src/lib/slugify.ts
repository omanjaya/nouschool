/** Ubah nama sekolah jadi slug: huruf kecil, angka, tanda hubung saja. */
export function slugify(value: string): string {
  return value
    .toLowerCase()
    .trim()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '');
}
