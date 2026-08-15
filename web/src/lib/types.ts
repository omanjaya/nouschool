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
  /**
   * Fase 10 (docs/09-billing.md): ringkasan langganan sekolah host tenant,
   * `null` untuk platform admin (tanpa `school`) atau kalau sekolah belum
   * pernah punya langganan. Dipakai banner status global (App.tsx).
   */
  subscription: MeSubscription | null;
  /**
   * Fase 10: daftar kunci fitur aktif dari snapshot langganan berjalan —
   * dipakai UX gating client-side (`lib/features.ts#hasFeature`), server
   * tetap menegakkan lewat `requireFeature` (docs/09 "Feature gating").
   */
  features: string[];
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

/* ---- Schedule / Jadwal pelajaran (Fase 5, docs/04-schedule.md) ---- */

/** GET/PUT /api/periods — "jam ke-" per sekolah. `label` terisi = non-KBM (mis. "Istirahat"). */
export interface Period {
  id: string;
  number: number;
  starts_at: string;
  ends_at: string;
  label: string | null;
}

/** GET /api/rooms — `qr_token` hanya dikirim untuk admin. */
export interface Room {
  id: string;
  name: string;
  qr_token?: string | null;
}

export interface ScheduleSlotSubjectRef {
  id: string;
  code: string;
  name: string;
}

export interface ScheduleSlotRoomRef {
  id: string;
  name: string;
}

/** 1=Senin ... 6=Sabtu (docs/04-schedule.md). */
export type DayOfWeek = 1 | 2 | 3 | 4 | 5 | 6;

/** GET /api/schedule/slots?class_id=|teacher_id= */
export interface ScheduleSlot {
  id: string;
  class: ClassRef;
  subject: ScheduleSlotSubjectRef;
  teacher: TeacherRef;
  room: ScheduleSlotRoomRef | null;
  day_of_week: DayOfWeek;
  period_start: number;
  period_end: number;
}

export interface ScheduleSlotInput {
  class_id: string;
  subject_id: string;
  teacher_id: string;
  room_id?: string | null;
  day_of_week: DayOfWeek;
  period_start: number;
  period_end: number;
}

/** POST /api/schedule/copy → { copied, skipped: [{reason}] } */
export interface ScheduleCopyResult {
  copied: number;
  skipped: { reason: string }[];
}

/** GET /api/schedule/today?class_id=|teacher_id= */
export interface ScheduleTodaySlot extends ScheduleSlot {
  is_now: boolean;
}

export interface CurrentPeriodInfo {
  number: number;
  starts_at: string;
  ends_at: string;
}

/** GET /api/schedule/current-period */
export interface CurrentPeriodResult {
  period: CurrentPeriodInfo | null;
  next_starts_at: string | null;
}

/* ---- Teaching / jurnal mengajar & monitoring (Fase 6, docs/06-teaching.md) ---- */

/** Flag konteks jurnal — "satu scan tiga hasil" (docs/06 #1). */
export type JournalFlag = 'room_mismatch' | 'unscheduled' | 'manual_pick';

export type JournalStatus = 'ongoing' | 'done';

/**
 * `teaching_journals` — dibuat lewat scan QR ruangan atau entri manual di luar
 * jadwal. `material`/`note` diasumsikan bisa `null` (belum diisi guru), konsisten
 * dengan field catatan lain di kontrak (mis. `AttendanceRecord.note`).
 */
export interface Journal {
  id: string;
  teacher: TeacherRef;
  class: ClassRef;
  subject: ScheduleSlotSubjectRef | null;
  room: ScheduleSlotRoomRef | null;
  /** ASUMSI: id string konsisten dgn `ScheduleSlot.id`, bukan angka murni. */
  schedule_slot_id: string | null;
  date: string;
  started_at: string;
  ended_at: string | null;
  material: string | null;
  note: string | null;
  flags: JournalFlag[];
  status: JournalStatus;
}

/** GET /api/teaching/journals?scope=&date=|month= */
export interface JournalListResult {
  items: Journal[];
}

export interface TeachingJournalCreateInput {
  room_id?: string;
  class_id: string;
  subject_id: string;
}

export interface TeachingJournalUpdateInput {
  material?: string;
  note?: string;
}

/** POST /api/teaching/scan — hasil "tidak ada jadwal di jam ini di ruangan ini". */
export interface TeachingScanNeedsManualResult {
  needs_manual: true;
  room: { id: string; name: string };
}

/**
 * POST /api/teaching/scan — hasil normal: jurnal dibuka + sesi absensi disiapkan
 * sekaligus. `attendance_session_id` diketik `number` sesuai kontrak literal;
 * dipakai hanya untuk interpolasi path (`/absensi/sesi/${id}`).
 */
export interface TeachingScanJournalResult {
  needs_manual: false;
  journal: Journal;
  attendance_session_id: number;
}

export type TeachingScanResult = TeachingScanJournalResult | TeachingScanNeedsManualResult;

/* ---- Status mengajar / monitoring (kepsek & admin, docs/06 #2) ---- */

export type TeachingStatus = 'mengajar' | 'belum_masuk' | 'izin' | 'belum_mulai' | 'selesai';

/**
 * ASUMSI: `starts_at`/`ends_at` di sini adalah timestamptz penuh untuk
 * kemunculan slot pada tanggal yang difilter (bukan jam "HH:MM" seperti
 * `Period.starts_at`) — supaya frontend bisa menentukan grup "sedang
 * berlangsung" murni dari data ini tanpa fetch tambahan ke `/periods`.
 */
export interface TeachingStatusSlot {
  id: string;
  class: ClassRef;
  subject: ScheduleSlotSubjectRef;
  teacher: TeacherRef;
  room: ScheduleSlotRoomRef | null;
  day_of_week: DayOfWeek;
  period_start: number;
  period_end: number;
  /** Jam dinding "HH:MM" (bukan timestamp) — dari definisi periods sekolah. */
  period_starts_at: string;
  period_ends_at: string;
}

export interface TeachingStatusItem {
  slot: TeachingStatusSlot;
  teacher: TeacherRef;
  status: TeachingStatus;
  journal_id: string | null;
  /** Ruang AKTUAL hasil scan — beda dari `slot.room` kalau guru pindah ruangan. */
  room_actual: ScheduleSlotRoomRef | null;
}

export interface TeachingStatusSummary {
  mengajar: number;
  belum_masuk: number;
  izin: number;
  belum_mulai: number;
  selesai: number;
}

/** GET /api/teaching/status?date= */
export interface TeachingStatusResult {
  items: TeachingStatusItem[];
  current_period: CurrentPeriodInfo | null;
  summary: TeachingStatusSummary;
}

/* ---- Pengumuman / TV / dashboard kepsek (Fase 7, docs/06-teaching.md) ---- */

/** GET /api/announcements(?active=1), POST/PATCH/DELETE (admin & kepsek). */
export interface Announcement {
  id: string;
  title: string;
  body: string;
  starts_at: string;
  ends_at: string;
}

export interface AnnouncementInput {
  title: string;
  body: string;
  starts_at: string;
  ends_at: string;
}

/** Bentuk ringkas pengumuman dalam payload `/api/tv/board` — tanpa rentang tanggal. */
export interface TvAnnouncement {
  id: string;
  title: string;
  body: string;
}

/** `now` di `/api/tv/board` — waktu dinding sekolah saat payload dibuat. */
export interface TvBoardNow {
  date: string;
  day_label: string;
  /** Jam dinding "HH:MM" (bukan timestamp) — dipakai menyinkronkan jam TV yang tick per detik di client. */
  time: string;
  current_period: CurrentPeriodInfo | null;
  next_starts_at: string | null;
}

/**
 * Item status mengajar di papan TV — shape FLAT (bukan nested TeachingStatusItem):
 * payload TV sengaja ramping, satu fetch (docs/06). `room_name` = ruang AKTUAL
 * hasil scan bila ada, selain itu ruang terjadwal; kosong = tanpa ruangan.
 */
export interface TvTeachingItem {
  class_id: string;
  class_name: string;
  subject_code: string;
  subject_name: string;
  teacher_id: string;
  teacher_name: string;
  room_name?: string;
  status: TeachingStatus;
  period_starts_at: string;
  period_ends_at: string;
}

export interface TvBoardTeaching {
  summary: TeachingStatusSummary;
  current: TvTeachingItem[];
  upcoming: TvTeachingItem[];
}

/** GET /api/tv/board (display/kepsek/admin) — payload gabungan satu fetch untuk dashboard TV. */
export interface TvBoard {
  school: { name: string };
  now: TvBoardNow;
  teaching: TvBoardTeaching;
  attendance: AttendanceClassSummary[];
  announcements: TvAnnouncement[];
  generated_at: string;
}

/** GET /api/teaching/compliance?from=&to= (kepsek/admin) — "ketertiban mengajar" (docs/06 #dashboard kepsek). */
export interface TeachingComplianceRow {
  teacher: TeacherRef;
  scheduled: number;
  taught: number;
  pct: number;
  unscheduled: number;
  material_filled: number;
  material_pct: number;
}

/* ---- Kartu QR siswa & scan kartu (Fase 8, docs/05-attendance.md) ---- */

/** POST /api/attendance/qr-cards/generate, GET /api/attendance/qr-cards?class_id= */
export interface StudentQrCard {
  student_id: string;
  name: string;
  nis: string;
  token: string;
}

/** POST /api/attendance/sessions/{id}/scan {token} */
export interface AttendanceScanResult {
  student_id: string;
  name: string;
  nis: string;
  status: AttendanceStatus;
  already_marked: boolean;
}

/* ---- Self check-in siswa (Fase 8) ---- */

export interface SelfCheckinWindow {
  open_from: string;
  close_at: string;
}

export interface SelfCheckinToday {
  status: AttendanceStatus;
  checked_at: string;
}

/** GET /api/attendance/self-checkin/status (siswa). */
export interface SelfCheckinStatus {
  enabled: boolean;
  window: SelfCheckinWindow | null;
  /**
   * ASUMSI: jam "HH:MM" waktu lokal sekolah (batas toleransi terlambat),
   * konsisten dengan format `window.open_from`/`close_at` — bukan menit
   * (beda dari `AttendanceSettings.late_after_min` yang dipakai admin
   * mengonfigurasinya sebagai durasi).
   */
  late_after: string | null;
  today: SelfCheckinToday | null;
}

export interface SelfCheckinInput {
  lat: number;
  lng: number;
  accuracy: number;
}

/** POST /api/attendance/self-checkin */
export interface SelfCheckinResult {
  status: AttendanceStatus;
  checked_at: string;
}

/* ---- Anomali check-in (Fase 8, admin/kepsek) ---- */

/** GET /api/attendance/anomalies?date=&class_id= */
export interface AttendanceAnomaly {
  student_id: string;
  name: string;
  class_name: string;
  issue: 'same_location' | 'low_accuracy';
  detail: string;
}

/* ---- Pengaturan absensi (Fase 8, admin — GET/PUT /api/settings/attendance) ---- */

export type AttendanceMode = 'daily' | 'per_subject';
export type AttendanceMethod = 'manual' | 'qr_card' | 'self_checkin';

export interface AttendanceSelfCheckinRule {
  lat: number;
  lng: number;
  radius_m: number;
  open_from: string;
  close_at: string;
}

export interface AttendanceSettings {
  mode: AttendanceMode;
  methods: AttendanceMethod[];
  self_checkin: AttendanceSelfCheckinRule | null;
  edit_window_hours: number;
  late_after_min: number;
}

/* ---- Notifikasi in-app & Web Push (Fase 9, docs/08-notification.md) ---- */

/** GET /api/notifications?page= — inbox notifikasi in-app. */
export interface NotificationItem {
  id: string;
  event: string;
  title: string;
  body: string;
  link: string | null;
  read_at: string | null;
  created_at: string;
}

export interface NotificationListResult {
  items: NotificationItem[];
  unread_count: number;
}

/** POST /api/notifications/read — `{ids}` (baris terpilih) atau `{all: true}` (semua). */
export type NotificationsReadInput = { ids: string[] } | { all: true };

/** GET /api/push/public-key — kunci VAPID publik (base64url), dipakai `applicationServerKey`. */
export interface PushPublicKey {
  key: string;
}

/** POST /api/push/subscribe — dari `PushSubscription.toJSON()`. */
export interface PushSubscribeInput {
  endpoint: string;
  p256dh: string;
  auth: string;
}

/** Channel notifikasi pluggable (docs/08 — in_app selalu aktif, sisanya diatur super admin per sekolah). */
export type NotificationChannel = 'in_app' | 'web_push' | 'whatsapp' | 'email';

/** GET/PUT /api/admin/schools/{id}/settings/notification (super admin). */
export interface NotificationChannelSettings {
  channels: NotificationChannel[];
}

/* ---- Absensi mode per-mapel dari jadwal (guru, Fase 6) ---- */

/**
 * Sesi absensi pada `slots-today` — beda dari `AttendanceClassSession` (dipakai
 * `attendance/classes`) karena sudah menyertakan `total` langsung (di daftar
 * rombel harian, total datang dari `student_count` pada item induk, bukan
 * dari objek sesi itu sendiri).
 */
export interface AttendanceSlotSession {
  id: string;
  status: AttendanceSessionStatus;
  marked_count: number;
  total: number;
}

/** GET /api/attendance/slots-today (guru) — satu baris per slot jadwal hari ini. */
export interface AttendanceSlotToday {
  slot: {
    id: string;
    class: ClassRef;
    subject: ScheduleSlotSubjectRef;
    period_start: number;
    period_end: number;
    starts_at: string;
    ends_at: string;
    is_now: boolean;
  };
  session: AttendanceSlotSession | null;
}

/* ---- Billing / Langganan tahunan (Fase 10, docs/09-billing.md) ---- */

export type SubscriptionStatus = 'active' | 'grace' | 'readonly' | 'canceled';

/** Ringkasan di `/api/me` — dipakai banner status global. */
export interface MeSubscription {
  status: SubscriptionStatus;
  plan_code: string;
  ends_on: string;
  grace_until: string | null;
}

export type InvoiceStatus = 'unpaid' | 'awaiting_verification' | 'paid' | 'void' | 'expired';

export interface InvoicePaymentInfo {
  method: string;
  verified_at?: string | null;
}

/** GET /api/billing — satu baris invoice. */
export interface Invoice {
  id: string;
  number: string;
  amount: number;
  status: InvoiceStatus;
  due_at: string;
  paid_at: string | null;
  pdf_url: string;
  payment: InvoicePaymentInfo | null;
}

/**
 * Invoice versi panel super admin (`GET /api/admin/schools/{id}/billing`) —
 * shape sama + `proof_url` (bukti transfer yang diunggah sekolah, `null`
 * kalau belum ada/bukan transfer manual).
 */
export interface AdminInvoice extends Invoice {
  proof_url: string | null;
}

/** Bentuk `subscription` di `GET /api/billing` & `GET /api/admin/schools/{id}/billing`. */
export interface BillingSubscription {
  plan_code: string;
  plan_name: string;
  status: SubscriptionStatus;
  starts_on: string;
  ends_on: string;
  grace_until: string | null;
  max_students: number;
  student_count: number;
  price: number;
  /** Snapshot kunci fitur aktif langganan ini — sama makna dengan `Me.features`. */
  features: string[];
}

/** GET /api/billing (admin & kepsek, host tenant). */
export interface BillingResult {
  subscription: BillingSubscription | null;
  invoices: Invoice[];
}

/** GET /api/admin/schools/{id}/billing (super admin). */
export interface AdminSchoolBillingResult {
  subscription: BillingSubscription | null;
  invoices: AdminInvoice[];
}

/** POST /api/billing/invoices/{id}/pay → redirect ke checkout gateway. */
export interface PayInvoiceResult {
  redirect_url: string;
}

/* ---- Panel super admin: plans & harga (Fase 10) ---- */

/** Bracket harga per jumlah siswa maksimal. */
export interface PlanPrice {
  max_students: number;
  price_yearly: number;
}

/** GET /api/admin/plans — item tunggal. `features` adalah map kunci→aktif (beda dari `Me.features`/`BillingSubscription.features` yang array). */
export interface Plan {
  code: string;
  name: string;
  features: Record<string, boolean>;
  prices: PlanPrice[];
}

/** PUT /api/admin/plans/{code} — body. */
export interface PlanUpdateInput {
  name: string;
  features: Record<string, boolean>;
  prices: PlanPrice[];
}
