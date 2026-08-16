import type { StudentLeaveRequest, StudentLeaveStatus, StudentLeaveType } from '../../lib/types';

export const STUDENT_LEAVE_TYPE_LABEL: Record<StudentLeaveType, string> = {
  sakit: 'Sakit',
  izin: 'Izin',
};

export const STUDENT_LEAVE_STATUS_LABEL: Record<StudentLeaveStatus, string> = {
  pending_homeroom: 'Menunggu Wali Kelas',
  pending_bk: 'Menunggu BK',
  issued: 'Terbit',
  rejected: 'Ditolak',
  canceled: 'Dibatalkan',
};

/** Varian `Tag` per status — pending_*→warning, issued→success, rejected→danger, canceled→netral. */
export const STUDENT_LEAVE_STATUS_TAG_VARIANT: Record<StudentLeaveStatus, 'warning' | 'success' | 'danger' | 'neutral'> = {
  pending_homeroom: 'warning',
  pending_bk: 'warning',
  issued: 'success',
  rejected: 'danger',
  canceled: 'neutral',
};

export type StepDecision = 'approved' | 'rejected' | null;

/**
 * ASUMSI: kontrak `GET /api/student-leave` TIDAK mengirim field keputusan
 * eksplisit per tahap (`homeroom`/`bk` hanya `{decided_by_name,decided_at,comment}`),
 * jadi disetujui/ditolak per tahap diturunkan dari kombinasi status + tahap mana
 * yang sudah terisi — bukan ditebak dari field yang tidak ada di kontrak:
 * - Wali kelas: belum ada `homeroom` → belum diputuskan; kalau `homeroom` ada TAPI
 *   `bk` masih kosong DAN status `rejected` → wali kelas yang menolak; selain itu
 *   (bk terisi, atau status `pending_bk`/`issued`) → wali kelas menyetujui (alur lanjut).
 * - BK: belum ada `bk` → belum diputuskan; status `issued` → BK menyetujui (surat
 *   terbit); status `rejected` (dengan `bk` terisi) → BK menolak.
 */
export function homeroomStepDecision(request: StudentLeaveRequest): StepDecision {
  if (!request.homeroom) return null;
  if (request.status === 'rejected' && !request.bk) return 'rejected';
  return 'approved';
}

export function bkStepDecision(request: StudentLeaveRequest): StepDecision {
  if (!request.bk) return null;
  if (request.status === 'issued') return 'approved';
  if (request.status === 'rejected') return 'rejected';
  return null;
}
