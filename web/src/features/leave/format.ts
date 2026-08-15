import type { LeaveApprovalStep, LeaveRequestStatus } from '../../lib/types';

/** Kamus label role approver (docs/07-leave.md) — 'guru' berarti approver spesifik (pakai nama). */
export function stepApproverLabel(step: LeaveApprovalStep): string {
  if (step.approver_role === 'kepala_sekolah') return 'Kepala Sekolah';
  if (step.approver_role === 'admin_sekolah') return 'Admin';
  if (step.approver_role === 'guru') return step.approver_name ?? 'Guru';
  return step.approver_role;
}

export const LEAVE_STATUS_LABEL: Record<LeaveRequestStatus, string> = {
  pending: 'Menunggu',
  approved: 'Disetujui',
  rejected: 'Ditolak',
  canceled: 'Dibatalkan',
};

/** Varian `Tag` per status permohonan izin — pending→st-terlambat, approved→st-hadir, rejected→st-alpa, canceled→netral. */
export const LEAVE_STATUS_TAG_VARIANT: Record<LeaveRequestStatus, 'warning' | 'success' | 'danger' | 'neutral'> = {
  pending: 'warning',
  approved: 'success',
  rejected: 'danger',
  canceled: 'neutral',
};
