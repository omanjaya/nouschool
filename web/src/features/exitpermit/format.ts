import type { ExitPermit, ExitPermitStatus } from '../../lib/types';

export const EXIT_PERMIT_STATUS_LABEL: Record<ExitPermitStatus, string> = {
  pending_duty_teacher: 'Menunggu Piket',
  pending_class_teacher: 'Menunggu Guru Pengajar',
  pending_bk: 'Menunggu BK',
  pending_leadership: 'Menunggu Pimpinan',
  issued: 'Siap Keluar',
  exited: 'Selesai',
  rejected: 'Ditolak',
  canceled: 'Dibatalkan',
};

export const EXIT_PERMIT_STATUS_TAG_VARIANT: Record<ExitPermitStatus, 'warning' | 'success' | 'danger' | 'neutral' | 'info'> = {
  pending_duty_teacher: 'warning',
  pending_class_teacher: 'warning',
  pending_bk: 'warning',
  pending_leadership: 'warning',
  issued: 'info',
  exited: 'success',
  rejected: 'danger',
  canceled: 'neutral',
};

export type ExitPermitStageField = 'duty' | 'class' | 'bk' | 'leadership';

export const EXIT_PERMIT_STAGE_LABEL: Record<ExitPermitStageField, string> = {
  duty: 'Piket',
  class: 'Guru Pengajar',
  bk: 'BK',
  leadership: 'Pimpinan',
};

export const EXIT_PERMIT_STAGE_ORDER: ExitPermitStageField[] = ['duty', 'class', 'bk', 'leadership'];

export type StepDecision = 'approved' | 'rejected' | null;

/**
 * ASUMSI: sama seperti `studentleave/format.ts` — kontrak tidak mengirim
 * field keputusan eksplisit per tahap, jadi status tiap tahap diturunkan dari
 * kombinasi field `{duty,class,bk,leadership}` (terisi = tahap itu SUDAH
 * dilalui/disetujui) + status akhir `rejected`/`canceled`: kalau permit
 * ditolak/dibatalkan, TAHAP PERTAMA yang field-nya masih kosong (urutan
 * `EXIT_PERMIT_STAGE_ORDER`) adalah tahap penyebabnya — ditandai "rejected"
 * di UI; tahap setelahnya tetap "belum diputuskan" (tidak pernah tercapai).
 */
export function exitPermitStageDecision(permit: ExitPermit, field: ExitPermitStageField): StepDecision {
  if (permit[field]) return 'approved';
  if (permit.status === 'rejected' || permit.status === 'canceled') {
    const firstEmpty = EXIT_PERMIT_STAGE_ORDER.find((f) => !permit[f]);
    if (firstEmpty === field) return 'rejected';
  }
  return null;
}
