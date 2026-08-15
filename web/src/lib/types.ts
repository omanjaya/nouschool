/** Bentuk data dari API — cocokkan dengan kontrak backend, jangan menebak field lain. */

export interface SchoolSummary {
  id: string;
  name: string;
  slug: string;
}

export interface School {
  id: string;
  name: string;
  slug: string;
  custom_domain: string | null;
  timezone: string;
  status: 'active' | 'suspended';
  created_at: string;
}

export interface AcademicYear {
  id: string;
  name: string;
  starts_on: string;
  ends_on: string;
  is_active: boolean;
}

export interface Me {
  id: string;
  name: string;
  role: string;
  is_super_admin: boolean;
  school: SchoolSummary | null;
  /**
   * ASUMSI (belum eksplisit di kontrak Fase 3): id baris `students` milik
   * user ini saat role === 'siswa', supaya frontend bisa memanggil
   * `GET /api/students/{id}/attendance` untuk riwayat kehadiran sendiri
   * (mis. orang_tua punya `/api/me/children` untuk kebutuhan yang sama,
   * siswa belum). Opsional supaya aman kalau backend belum mengirimnya.
   */
  student_id?: string | null;
}

export interface Branding {
  app_name: string;
  primary_color: string;
}

/* ---- Student (Fase 2) ---- */

export type StudentStatus = 'active' | 'graduated' | 'moved' | 'dropped';

export interface ClassRef {
  id: string;
  name: string;
}

export interface Student {
  id: string;
  nis: string;
  nisn: string | null;
  name: string;
  gender: string | null;
  birth_date: string | null;
  status: StudentStatus;
  class: ClassRef | null;
  /**
   * Belum eksplisit di kontrak ringkas backend, tapi skema `students.user_id`
   * (docs/03-student.md) dipakai untuk menandai akun sudah aktivasi atau belum.
   * Dibuat opsional supaya tetap aman kalau field ini belum dikirim backend.
   */
  user_id?: string | null;
}

export interface StudentListResult {
  items: Student[];
  total: number;
}

/* ---- Class / Rombel (Fase 2) ---- */

export interface TeacherRef {
  id: string;
  name: string;
}

export interface SchoolClass {
  id: string;
  name: string;
  grade: string;
  major: string | null;
  homeroom_teacher: TeacherRef | null;
  student_count: number;
}

/* ---- Teacher (Fase 2) ---- */

export interface Teacher {
  id: string;
  user_id: string;
  name: string;
  email: string | null;
  nip: string | null;
}

/* ---- Subject (Fase 2) ---- */

export interface Subject {
  id: string;
  code: string;
  name: string;
}

/* ---- Import Excel/CSV (Fase 2) ---- */

export type ImportRowAction = 'create' | 'update' | 'error';

export interface ImportRow {
  row: number;
  data: Record<string, string | null>;
  action: ImportRowAction;
  messages: string[];
}

export interface ImportSummary {
  total: number;
  create: number;
  update: number;
  error: number;
}

export interface ImportPreviewResult {
  upload_id: string;
  summary: ImportSummary;
  rows: ImportRow[];
}

export interface ImportCommitResult {
  created: number;
  updated: number;
  skipped: number;
}

/* ---- Undangan / aktivasi akun (Fase 2) ---- */

export interface GeneratedInvitation {
  student_id: string;
  student_name: string;
  student_code: string | null;
  guardian_code: string | null;
}

export type InvitationRole = 'siswa' | 'orang_tua' | 'guru';

export interface InvitationInfo {
  role: InvitationRole;
  student_name: string;
  school_name: string;
}

export interface ActivateInput {
  code: string;
  name: string;
  username?: string;
  email?: string;
  password: string;
}

/* ---- Attendance / Absensi (Fase 3) ---- */

export type AttendanceStatus = 'hadir' | 'terlambat' | 'izin' | 'sakit' | 'alpa';

export type AttendanceSessionStatus = 'open' | 'finalized';

export interface AttendanceClassSession {
  id: string;
  status: AttendanceSessionStatus;
  marked_count: number;
}

/** GET /api/attendance/classes?date= — satu baris per rombel guru/admin. */
export interface AttendanceClassListItem {
  class_id: string;
  name: string;
  student_count: number;
  session: AttendanceClassSession | null;
}

export interface AttendanceRecord {
  status: AttendanceStatus;
  note: string | null;
  method: string;
  marked_at: string;
}

export interface AttendanceSessionStudent {
  student_id: string;
  name: string;
  nis: string;
  record: AttendanceRecord | null;
}

export interface AttendanceSessionInfo {
  id: string;
  class_id: string;
  class_name: string;
  date: string;
  type: string;
  status: AttendanceSessionStatus;
  opened_by_name: string;
}

/** Shape bersama POST /sessions, GET /sessions/{id}, PUT .../records, POST .../finalize. */
export interface AttendanceSessionResult {
  session: AttendanceSessionInfo;
  students: AttendanceSessionStudent[];
}

export interface AttendanceRecordInput {
  student_id: string;
  status: AttendanceStatus;
  note?: string;
}

/** GET /api/attendance/summary?date= — rekap harian per rombel (admin/kepsek). */
export interface AttendanceClassSummary {
  class_id: string;
  class_name: string;
  total: number;
  hadir: number;
  terlambat: number;
  izin: number;
  sakit: number;
  alpa: number;
  unmarked: number;
  session_status: AttendanceSessionStatus | null;
}

/** GET /api/me/children (ortu). */
export interface ChildRef {
  student_id: string;
  name: string;
  class_name: string;
}

export interface StudentAttendanceHistoryItem {
  date: string;
  status: AttendanceStatus;
  note: string | null;
}

/** GET /api/students/{id}/attendance?from=&to= */
export interface StudentAttendanceHistory {
  counts: Record<AttendanceStatus, number>;
  items: StudentAttendanceHistoryItem[];
}

/* ---- Leave / Izin guru (Fase 4, docs/07-leave.md) ---- */

export type LeaveRequestStatus = 'pending' | 'approved' | 'rejected' | 'canceled';
export type LeaveDecision = 'approved' | 'rejected';

/** GET /api/leave/types → { types } */
export interface LeaveType {
  key: string;
  label: string;
}

export interface LeaveRequestTeacherRef {
  id: string;
  name: string;
}

export interface LeaveApprovalStep {
  id: string;
  step_order: number;
  approver_role: string;
  approver_name: string | null;
  decision: LeaveDecision | null;
  decided_at: string | null;
  comment: string | null;
}

/** Shape bersama create/list/cancel/decide — GET .../requests, POST .../cancel, POST .../decide. */
export interface LeaveRequest {
  id: string;
  type: string;
  type_label: string;
  date_start: string;
  date_end: string;
  days: number;
  reason: string;
  status: LeaveRequestStatus;
  created_at: string;
  teacher: LeaveRequestTeacherRef;
  attachment_url: string | null;
  steps: LeaveApprovalStep[];
}

export interface LeaveRequestCreateInput {
  type: string;
  date_start: string;
  date_end: string;
  reason: string;
  attachment?: File | null;
}

/** GET /api/leave/approvals → { items: [{step_id, request}] } — antrian menunggu keputusan user ini. */
export interface LeaveApprovalQueueItem {
  step_id: string;
  request: LeaveRequest;
}

export interface LeaveDecideInput {
  decision: LeaveDecision;
  comment?: string;
}

/** GET /api/leave/summary?from=&to= (kepsek/admin). */
export interface LeaveSummaryRow {
  teacher_id: string;
  name: string;
  per_type: Record<string, number>;
  total_days: number;
}

/* Settings module `leave` (GET/PUT /api/settings/leave). */
export interface LeaveChainStep {
  role: string;
  user_id?: string | null;
}

export interface LeaveSettings {
  types: LeaveType[];
  chain: LeaveChainStep[];
}
