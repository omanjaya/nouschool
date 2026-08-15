/**
 * Util warna brand sekolah (docs/10-design-system.md §1): `--primary` di-inject
 * dari `school_settings.branding.primary_color`; `--primary-soft` (tint ke
 * putih) dan `--primary-ink` (putih atau `--ink`, dipilih dari cek kontras
 * WCAG AA) dihitung dari sini — dipakai baik oleh runtime branding
 * (`lib/useAppBranding`) maupun preview saat admin memilih warna di
 * `features/settings/SettingsPage`.
 */

interface Rgb {
  r: number;
  g: number;
  b: number;
}

const HEX_RE = /^#?([0-9a-fA-F]{6}|[0-9a-fA-F]{3})$/;

export function isValidHexColor(value: string): boolean {
  return HEX_RE.test(value.trim());
}

function hexToRgb(hex: string): Rgb | null {
  const match = HEX_RE.exec(hex.trim());
  if (!match) return null;
  let h = match[1];
  if (h.length === 3) {
    h = h
      .split('')
      .map((c) => c + c)
      .join('');
  }
  const num = parseInt(h, 16);
  return { r: (num >> 16) & 255, g: (num >> 8) & 255, b: num & 255 };
}

function rgbToHex({ r, g, b }: Rgb): string {
  const toHex = (v: number) =>
    Math.round(Math.min(255, Math.max(0, v)))
      .toString(16)
      .padStart(2, '0');
  return `#${toHex(r)}${toHex(g)}${toHex(b)}`.toUpperCase();
}

/** Campur warna ke putih sebesar `whiteRatio` (0..1) — 0.88 = tint ~12% (docs/10 §1). */
export function tintToWhite(hex: string, whiteRatio: number): string {
  const rgb = hexToRgb(hex);
  if (!rgb) return hex;
  const mix = (channel: number) => channel + (255 - channel) * whiteRatio;
  return rgbToHex({ r: mix(rgb.r), g: mix(rgb.g), b: mix(rgb.b) });
}

function channelLuminance(c: number): number {
  const v = c / 255;
  return v <= 0.03928 ? v / 12.92 : Math.pow((v + 0.055) / 1.055, 2.4);
}

function relativeLuminance(rgb: Rgb): number {
  return 0.2126 * channelLuminance(rgb.r) + 0.7152 * channelLuminance(rgb.g) + 0.0722 * channelLuminance(rgb.b);
}

/** Rasio kontras WCAG antara dua warna hex (1..21). */
export function contrastRatio(hexA: string, hexB: string): number {
  const a = hexToRgb(hexA);
  const b = hexToRgb(hexB);
  if (!a || !b) return 1;
  const lumA = relativeLuminance(a);
  const lumB = relativeLuminance(b);
  const lighter = Math.max(lumA, lumB);
  const darker = Math.min(lumA, lumB);
  return (lighter + 0.05) / (darker + 0.05);
}

const WHITE = '#FFFFFF';
const DEFAULT_INK = '#17233A';
/** Ambang WCAG AA untuk teks normal. */
const AA_CONTRAST_MIN = 4.5;

/**
 * Warna teks di atas `primaryHex`: putih kalau kontrasnya cukup (>=4.5:1),
 * kalau tidak jatuh ke `inkHex` (default token `--ink`) — supaya warna brand
 * terang (mis. kuning) tetap terbaca (docs/10 §1).
 */
export function getPrimaryInk(primaryHex: string, inkHex: string = DEFAULT_INK): string {
  return contrastRatio(primaryHex, WHITE) >= AA_CONTRAST_MIN ? WHITE : inkHex;
}

/** Tint latar dari warna primary — dipakai untuk `--primary-soft`. */
export function getPrimarySoft(primaryHex: string): string {
  return tintToWhite(primaryHex, 0.88);
}

/**
 * Terapkan warna brand sekolah ke halaman: set `--primary`, lalu hitung &
 * set `--primary-soft`/`--primary-ink`. Tidak melakukan apa-apa kalau
 * `primaryHex` bukan hex warna valid (data branding rusak/kosong).
 */
export function applyBrandColor(primaryHex: string): void {
  if (typeof document === 'undefined' || !isValidHexColor(primaryHex)) return;
  const root = document.documentElement;
  const currentInk = getComputedStyle(root).getPropertyValue('--ink').trim() || DEFAULT_INK;
  root.style.setProperty('--primary', primaryHex);
  root.style.setProperty('--primary-soft', getPrimarySoft(primaryHex));
  root.style.setProperty('--primary-ink', getPrimaryInk(primaryHex, currentInk));
}
