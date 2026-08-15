import { useQuery } from '@tanstack/react-query';
import { api } from '../../lib/api';
import type { TvBoard } from '../../lib/types';

export const TV_BOARD_QUERY_KEY = ['tv-board'] as const;

/** Polling dasar (docs/06 "Teknis TV") — dipakai selama realtime WS TIDAK tersambung. */
export const TV_BOARD_POLL_MS = 30_000;
/**
 * Fase 12: polling dilonggarkan ke ini saat WS tersambung (WS jadi jalur
 * update utama, polling hanya jaring pengaman) — lihat pemanggilan di `TvPage`.
 */
export const TV_BOARD_POLL_WS_CONNECTED_MS = 120_000;

/**
 * GET /api/tv/board (display/kepsek/admin) — payload gabungan satu fetch.
 * `refetchIntervalMs` dikontrol pemanggil (`TvPage`): 30 dtk normal, 120 dtk
 * saat realtime WS tersambung (WS memicu refetch langsung lewat event, lihat
 * `TV_BOARD_POLL_WS_CONNECTED_MS`). Kalau fetch gagal, TanStack Query TETAP
 * mempertahankan `data` hasil sukses terakhir (hanya `isError` yang berubah)
 * — dipakai layar TV untuk tidak mengosongkan tampilan saat koneksi terputus
 * sementara, cukup menampilkan indikator kecil.
 */
export function useTvBoard(refetchIntervalMs: number = TV_BOARD_POLL_MS) {
  return useQuery({
    queryKey: TV_BOARD_QUERY_KEY,
    queryFn: () => api.get<TvBoard>('/tv/board'),
    refetchInterval: refetchIntervalMs,
  });
}
