import { type ClassValue, clsx } from 'clsx'
import { twMerge } from 'tailwind-merge'

// Util standar shadcn `cn()` — dipakai HANYA oleh block/komponen yang
// diambil dari registry (`src/components/blocks/`, lihat README di sana).
// Komponen bersama Rapor asli (`src/components/ui/`) tidak butuh ini —
// classlist-nya sudah statis lewat token, lihat docs/10-design-system.md.
export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}
