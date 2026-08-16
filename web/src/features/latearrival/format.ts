import type { LateArrivalAction, LateArrivalRecord, LateArrivalStatus } from '../../lib/types';

export const LATE_ARRIVAL_STATUS_LABEL: Record<LateArrivalStatus, string> = {
  pending_duty_teacher: 'Menunggu Piket',
  pending_leadership: 'Menunggu Pimpinan',
  pending_class_teacher: 'Menunggu Guru Kelas',
  completed: 'Selesai',
};

export const LATE_ARRIVAL_STATUS_TAG_VARIANT: Record<LateArrivalStatus, 'warning' | 'success'> = {
  pending_duty_teacher: 'warning',
  pending_leadership: 'warning',
  pending_class_teacher: 'warning',
  completed: 'success',
};

export const LATE_ARRIVAL_ACTION_LABEL: Record<LateArrivalAction, string> = {
  none: 'Tidak ada tindakan tambahan',
  call_parent: 'Orang tua dihubungi',
  send_home: 'Dipulangkan',
};

export type LateArrivalStageField = 'duty' | 'leadership' | 'class_teacher';

/** Urutan rantai terlambat: piket → pimpinan → guru kelas (docs/12-sion-parity.md Gelombang B alur 3 — berbeda urutan dari dispensasi keluar). */
export const LATE_ARRIVAL_STAGE_ORDER: LateArrivalStageField[] = ['duty', 'leadership', 'class_teacher'];

export const LATE_ARRIVAL_STAGE_LABEL: Record<LateArrivalStageField, string> = {
  duty: 'Piket',
  leadership: 'Pimpinan',
  class_teacher: 'Guru Kelas',
};

/**
 * Status di luar rantai (`completed`) — tidak ada lagi "tahap aktif". Beda
 * dengan exit-permit, kontrak Fase 14 Gelombang B2 alur terlambat tidak
 * menyebut jalur penolakan (aksi otomatis by hitungan, bukan approve/reject
 * manual) — jadi tahap yang belum tercapai selalu `null` ("belum", bukan pernah "ditolak").
 */
export function currentLateArrivalStage(record: LateArrivalRecord): LateArrivalStageField | null {
  if (record.status === 'completed') return null;
  return LATE_ARRIVAL_STAGE_ORDER.find((f) => !record[f]) ?? null;
}
