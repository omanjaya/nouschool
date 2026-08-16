import { useMutation } from '@tanstack/react-query';
import { api } from '../../lib/api';
import type { TeacherQrToken } from '../../lib/types';

/**
 * POST /api/teacher-qr (guru) — buat token QR baru, TTL 60 detik, SEKALI
 * pakai. Sengaja BUKAN `useQuery` (tidak ada state "daftar token" untuk
 * di-cache/invalidate — tiap pemanggilan adalah aksi "buat token baru", baik
 * dipicu otomatis oleh `TeacherQrPage` (habis waktu / token terpakai lewat
 * event realtime `teacherqr`) maupun retry manual saat gagal.
 */
export function useGenerateTeacherQr() {
  return useMutation({
    mutationFn: () => api.post<TeacherQrToken>('/teacher-qr'),
  });
}

/** Prefix konten QR token guru (dipindai siswa) — kontrak Fase 14 Gelombang B2. */
export const TEACHER_QR_PREFIX = 'nouschool:tqr:';
