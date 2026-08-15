import { useQuery } from '@tanstack/react-query';
import { api } from '../../lib/api';
import type { TvBoard } from '../../lib/types';

export const TV_BOARD_QUERY_KEY = ['tv-board'] as const;

/**
 * GET /api/tv/board (display/kepsek/admin) — payload gabungan satu fetch,
 * dipoll tiap 30 detik (docs/06 "Teknis TV"). Kalau fetch gagal, TanStack
 * Query TETAP mempertahankan `data` hasil sukses terakhir (hanya `isError`
 * yang berubah) — dipakai layar TV untuk tidak mengosongkan tampilan saat
 * koneksi terputus sementara, cukup menampilkan indikator kecil.
 */
export function useTvBoard() {
  return useQuery({
    queryKey: TV_BOARD_QUERY_KEY,
    queryFn: () => api.get<TvBoard>('/tv/board'),
    refetchInterval: 30_000,
  });
}
