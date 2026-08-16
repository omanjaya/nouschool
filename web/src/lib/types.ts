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
  /** Path penyimpanan internal logo — bukan URL yang bisa dipakai langsung; tampilan logo memakai `PublicContextBranding.logo_url`. */
  logo_path?: string | null;
}

/* ---- Konteks publik & branding runtime (Fase 11, docs/01-tenant.md §branding) ---- */

export interface PublicContextSchool {
  name: string;
  slug: string;
}

export interface PublicContextBranding {
  app_name: string;
  primary_color: string;
  logo_url: string | null;
}

/** GET /api/public/context — publik, dipanggil sekali saat app load (`lib/useAppBranding`). */
export type PublicContext =
  | { platform: true }
  | { platform: false; school: PublicContextSchool; branding: PublicContextBranding };

/* ---- Custom domain (Fase 11, docs/01-tenant.md §Custom domain & Caddy) ---- */

/** GET /api/custom-domain (admin). */
export interface CustomDomainStatus {
  domain: string | null;
  pending_domain: string | null;
  verified: boolean;
  server_ip: string;
  instructions: string;
}

/* ---- Minat sekolah / landing page (Fase 11) ---- */

export interface InterestLeadInput {
  school_name: string;
  contact_name: string;
  phone: string;
  email?: string;
  note?: string;
}

/**
 * GET /api/admin/interest — ASUMSI: tiap baris punya `id` (belum eksplisit
 * di kontrak ringkas), dipakai sebagai key list di `InterestLeadsPage`.
 */
export interface InterestLead extends InterestLeadInput {
  id: string;
  created_at: string;
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

/**
 * GET /api/announcements(?active=1), POST/PATCH/DELETE (admin & kepsek).
 * Fase 13 Gelombang 2 (P5, docs/11 P5): `is_platform` menandai pengumuman yang
 * disuntikkan dari `/admin/platform-announcements` (super admin) — item ini
 * tampil di sisi sekolah tapi tidak bisa diubah/dihapus dari sana.
 */
export interface Announcement {
  id: string;
  title: string;
  body: string;
  starts_at: string;
  ends_at: string;
  is_platform: boolean;
}

export interface AnnouncementInput {
  title: string;
  body: string;
  starts_at: string;
  ends_at: string;
}

/**
 * Bentuk ringkas pengumuman dalam payload `/api/tv/board` — tanpa rentang tanggal.
 * `is_platform` (Fase 13 Gelombang 2 P5) — sama makna dengan `Announcement.is_platform`.
 */
export interface TvAnnouncement {
  id: string;
  title: string;
  body: string;
  is_platform: boolean;
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
  /**
   * Fase 13 Gelombang 2 (P6, docs/11 P6) — hanya terisi di panel super admin
   * (`GET /api/admin/schools/{id}/billing`): override manual per kunci fitur
   * (kill switch/unlock tanpa ganti plan) + union akhir kunci fitur aktif
   * setelah merge override × fitur plan. Opsional karena `GET /api/billing`
   * (tenant) belum tentu mengirim field ini.
   */
  feature_overrides?: Record<string, boolean>;
  features_effective?: string[];
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

/* ---- Impersonation / sesi support super admin (Fase 12) ---- */

/** POST /api/admin/schools/{id}/impersonate (super admin, host platform) — token sekali pakai untuk masuk sebagai sekolah ini di host tenant. */
export interface ImpersonateStartResult {
  token: string;
  slug: string;
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

/* ---- Panel super admin: beranda platform, statistik & operasional (Fase 13, docs/11 P1/P3/P4) ---- */

export interface AdminOverviewStats {
  schools_active: number;
  schools_grace: number;
  schools_readonly: number;
  schools_suspended: number;
  total_students: number;
  revenue_year: number;
  leads_7d: number;
}

export interface AdminAttentionInvoice {
  invoice_id: string;
  number: string;
  school_id: string;
  school_name: string;
  amount: number;
}

export interface AdminAttentionSchoolGrace {
  school_id: string;
  name: string;
  ends_on: string;
  grace_until: string;
}

/** Bentuk minimal sekolah pada beberapa daftar "perlu perhatian" (readonly, tanpa TA aktif). */
export interface AdminAttentionSchoolRef {
  school_id: string;
  name: string;
}

export interface AdminAttentionOutboxDead {
  school_id: string;
  school_name: string;
  dead_count: number;
}

export interface AdminOverviewAttention {
  invoices_awaiting: AdminAttentionInvoice[];
  schools_grace: AdminAttentionSchoolGrace[];
  schools_readonly: AdminAttentionSchoolRef[];
  schools_no_active_year: AdminAttentionSchoolRef[];
  outbox_dead: AdminAttentionOutboxDead[];
}

export interface AdminLastActivity {
  school_id: string;
  name: string;
  last_login: string | null;
  last_attendance_session: string | null;
}

/** GET /api/admin/overview (super admin, docs/11 P1) — satu fetch untuk beranda platform. */
export interface AdminOverview {
  stats: AdminOverviewStats;
  attention: AdminOverviewAttention;
  last_activity: AdminLastActivity[];
}

/** GET /api/admin/schools/{id}/stats (super admin, docs/11 P3). */
export interface AdminSchoolStats {
  teachers: number;
  students: number;
  classes: number;
  attendance_sessions_7d: number;
  journals_7d: number;
  notifications_30d: {
    sent: number;
    failed: number;
    dead: number;
  };
  last_logins: { role: string; at: string }[];
  uploads_bytes: number;
}

/** GET /api/admin/schools/{id}/members (super admin, docs/11 P4). */
export interface AdminSchoolMember {
  user_id: string;
  name: string;
  email: string | null;
  username: string | null;
  role: string;
  status: string;
  last_login: string | null;
}

/** POST /api/admin/users/{id}/reset-password (super admin) — password sementara, tampil sekali. */
export interface ResetPasswordResult {
  temp_password: string;
}

/** Item `GET /api/admin/schools/{id}/audit` (super admin, docs/11 P4). */
export interface AdminAuditLogItem {
  id: string;
  user_name: string | null;
  action: string;
  entity: string;
  entity_id: string;
  at: string;
}

export interface AdminAuditLogResult {
  items: AdminAuditLogItem[];
  total: number;
}

/** Status antrean pesan keluar (docs/08-notification.md — outbox pluggable). */
export type OutboxStatus = 'pending' | 'sent' | 'failed' | 'dead';

/** Item `GET /api/admin/outbox` (super admin, docs/11 P4) — antrean notifikasi lintas sekolah. */
export interface OutboxItem {
  id: string;
  school_id: string;
  school_name: string;
  event: string;
  channel: NotificationChannel;
  user_name: string | null;
  status: OutboxStatus;
  attempts: number;
  next_retry_at: string | null;
  created_at: string;
}

export interface OutboxListResult {
  items: OutboxItem[];
  total: number;
}

export interface OutboxRetryAllResult {
  retried: number;
}

/* ---- Onboarding wizard sekolah baru & admin sekolah (Fase 13 Gelombang 2 P2, docs/11 P2) ---- */

/** GET /api/admin/schools/{id}/onboarding (super admin) — checklist status siap pakai. */
export interface AdminSchoolOnboarding {
  has_active_year: boolean;
  has_admin: boolean;
  has_subscription_active: boolean;
  has_students: boolean;
  has_schedule: boolean;
  ready: boolean;
}

/** POST /api/admin/schools/{id}/admins {name, email?, username?} (super admin) — akun admin sekolah pertama. */
export interface CreateSchoolAdminInput {
  name: string;
  email?: string;
  username?: string;
}

/** Password sementara ditampilkan sekali (pola sama `ResetPasswordResult`). */
export interface CreateSchoolAdminResult {
  user_id: string;
  temp_password: string;
}

/* ---- Pengumuman platform (Fase 13 Gelombang 2 P5, docs/11 P5) ---- */

/** GET/POST/PATCH/DELETE /api/admin/platform-announcements[/{id}] (super admin) — shape sama `Announcement` sekolah, tanpa scope sekolah. */
export interface PlatformAnnouncement {
  id: string;
  title: string;
  body: string;
  starts_at: string;
  ends_at: string;
}

export interface PlatformAnnouncementInput {
  title: string;
  body: string;
  starts_at: string;
  ends_at: string;
}

/* ---- Feature override per sekolah (Fase 13 Gelombang 2 P6, docs/11 P6) ---- */

/** PUT /api/admin/schools/{id}/feature-overrides body — `null` = ikut plan (hapus override). */
export type FeatureOverridesInput = Record<string, boolean | null>;

/**
 * PUT /api/admin/schools/{id}/feature-overrides (super admin) → features efektif hasil merge
 * override + plan. ASUMSI bentuk respons: echo `feature_overrides` (map key aktif override, key
 * "ikut plan" tidak muncul) + `features_effective` (union akhir), konsisten dengan field yang
 * sama pada `BillingSubscription` di bawah.
 */
export interface FeatureOverridesResult {
  feature_overrides: Record<string, boolean>;
  features_effective: string[];
}

/* ---- Laporan pendapatan platform (Fase 13 Gelombang 2 P6, docs/11 P6) ---- */

export interface AdminRevenueMonth {
  month: number;
  paid_total: number;
  invoice_count: number;
}

export interface AdminRevenueByPlan {
  plan_code: string;
  paid_total: number;
  invoice_count: number;
}

/** GET /api/admin/revenue?year= (super admin). Export Excel: `GET /api/admin/revenue/export?year=`. */
export interface AdminRevenueResult {
  year: number;
  total: number;
  months: AdminRevenueMonth[];
  by_plan: AdminRevenueByPlan[];
}

/* ---- Kedisiplinan / Pelanggaran (Fase 14 Gelombang A, docs/12-sion-parity.md) ---- */

/**
 * GET/POST/PATCH/DELETE /api/discipline/types (admin) — master jenis pelanggaran per sekolah.
 * ASUMSI: GET daftar mengembalikan array langsung (bukan dibungkus `{items}`),
 * konsisten dengan master data sejenis (`GET /api/subjects` → `Subject[]`).
 */
export interface DisciplineType {
  id: string;
  name: string;
  points: number;
  category: string;
  active: boolean;
}

export interface DisciplineTypeInput {
  name: string;
  points: number;
  category: string;
  active: boolean;
}

/** GET/PUT /api/discipline/sp-settings (admin) — ambang poin Surat Peringatan per TA aktif. */
export interface DisciplineSpSettings {
  sp1: number;
  sp2: number;
  sp3: number;
}

export interface DisciplineStudentRef {
  id: string;
  name: string;
  nis: string;
  class_name: string;
}

export interface DisciplineViolationRef {
  id: string;
  name: string;
  points: number;
}

/** Satu baris `GET /api/discipline/records` → `{items,total}`. */
export interface DisciplineRecord {
  id: string;
  student: DisciplineStudentRef;
  violation: DisciplineViolationRef;
  noted_by_name: string;
  /** String kosong (bukan `null`) kalau tidak diisi — backend Go `Note string` non-pointer, default `''`. */
  note: string;
  occurred_on: string;
  attendance_session_id: string | null;
}

/** GET /api/discipline/records?student_id=&class_id=&from=&to=&page= */
export interface DisciplineRecordListResult {
  items: DisciplineRecord[];
  total: number;
}

/** POST /api/discipline/records body. */
export interface DisciplineRecordCreateInput {
  student_id: string;
  violation_type_id: string;
  attendance_session_id?: string;
  note?: string;
  occurred_on?: string;
}

export type DisciplineLetterLevel = 1 | 2 | 3;

export interface DisciplineIssuedLetter {
  id: string;
  level: DisciplineLetterLevel;
  number: string;
}

/** POST /api/discipline/records response — `issued_letter` terisi kalau ambang SP tercapai (surat otomatis terbit). */
export interface DisciplineRecordCreateResult {
  record: DisciplineRecord;
  total_points: number;
  issued_letter: DisciplineIssuedLetter | null;
}

export interface StudentDisciplineLetter {
  id: string;
  level: DisciplineLetterLevel;
  number: string;
  points_snapshot: number;
  created_at: string;
}

/** GET /api/students/{id}/discipline (siswa/ortu: milik sendiri; staf: sesuai akses). */
export interface StudentDisciplineSummary {
  total_points: number;
  sp_level_reached: number;
  records: DisciplineRecord[];
  letters: StudentDisciplineLetter[];
}

/**
 * Satu baris `GET /api/discipline/letters` → `{items,total}`. `student`
 * ABSEN (bukan `null`) di bagian "letters" `GET /api/students/{id}/discipline`
 * (jsonb `omitempty`) — di situ pakai `StudentDisciplineLetter`, bukan tipe ini.
 */
export interface DisciplineLetter {
  id: string;
  level: DisciplineLetterLevel;
  number: string;
  student?: DisciplineStudentRef;
  points_snapshot: number;
  created_at: string;
}

/** GET /api/discipline/letters?page=&level= */
export interface DisciplineLetterListResult {
  items: DisciplineLetter[];
  total: number;
}

/** GET /api/discipline/summary?class_id=&from=&to= — array langsung (bukan dibungkus). Export Excel: `GET /api/discipline/export?...`. */
export interface DisciplineSummaryRow {
  student: DisciplineStudentRef;
  total_points: number;
  record_count: number;
  last_at: string | null;
}

/* ---- Tugas Tambahan & Pegawai (Fase 14 Gelombang B1, docs/12-sion-parity.md Gelombang B — fondasi otorisasi) ---- */

export type DutyForRole = 'guru' | 'pegawai';

/** GET /api/duties/flags — daftar capability flag yang bisa dipilih untuk sebuah tugas. */
export interface DutyFlagOption {
  key: string;
  label: string;
}

/** GET/POST/PATCH /api/duties (admin) — tugas tambahan per TA aktif. */
export interface Duty {
  id: string;
  name: string;
  for_role: DutyForRole;
  flags: string[];
  active: boolean;
  assignee_count: number;
}

export interface DutyInput {
  name: string;
  for_role: DutyForRole;
  flags: string[];
  active: boolean;
}

/** Satu baris `GET /api/duties/{id}/assignments` — array langsung (bukan dibungkus). */
export interface DutyAssignee {
  user_id: string;
  name: string;
  role: string;
}

/** GET/POST/PATCH /api/employees (admin) — profil pegawai non-guru (role `pegawai`). */
export interface Employee {
  id: string;
  user_id: string;
  name: string;
  email: string | null;
  username: string | null;
  nip: string | null;
}

export interface EmployeeInput {
  name: string;
  email?: string;
  username?: string;
  nip?: string;
}

/** POST /api/employees response — password sementara ditampilkan SEKALI (pola sama `ResetPasswordResult`). */
export interface EmployeeCreateResult extends Employee {
  temp_password: string;
}

/* ---- Izin Siswa / "izin terencana" (Fase 14 Gelombang B1, docs/12-sion-parity.md Gelombang B alur 1) ---- */

export type StudentLeaveType = 'sakit' | 'izin';

/** Dua tahap approval tetap: wali kelas lalu BK (berbeda dari `leave` guru yang punya rantai step dinamis). */
export type StudentLeaveStatus = 'pending_homeroom' | 'pending_bk' | 'issued' | 'rejected' | 'canceled';

export interface StudentLeaveStudentRef {
  id: string;
  name: string;
  nis: string;
  class_name: string;
}

/** Rincian satu tahap keputusan (wali kelas ATAU BK) — `null` selama tahap itu belum diputuskan. */
export interface StudentLeaveStepInfo {
  decided_by_name: string;
  decided_at: string;
  comment: string | null;
}

/** Satu baris `GET /api/student-leave?scope=mine|queue|all&status=` → `{items,total}`. */
export interface StudentLeaveRequest {
  id: string;
  student: StudentLeaveStudentRef;
  type: StudentLeaveType;
  date_start: string;
  date_end: string;
  reason: string;
  status: StudentLeaveStatus;
  attachment_url: string | null;
  letter_number: string | null;
  verify_token: string | null;
  homeroom: StudentLeaveStepInfo | null;
  bk: StudentLeaveStepInfo | null;
  created_at: string;
}

export interface StudentLeaveListResult {
  items: StudentLeaveRequest[];
  total: number;
}

/** POST /api/student-leave (multipart) — siswa untuk diri sendiri; ortu untuk anak (sertakan `student_id`). */
export interface StudentLeaveCreateInput {
  type: StudentLeaveType;
  date_start: string;
  date_end: string;
  reason: string;
  attachment?: File | null;
  student_id?: string;
}

/** POST /api/student-leave/{id}/review (tahap wali kelas) & /issue (tahap BK) body. */
export interface StudentLeaveDecideInput {
  decision: 'approve' | 'reject';
  comment?: string;
}

/** GET /api/public/leave-verify?token= — PUBLIK, tanpa auth (dibuka dari scan QR di surat PDF). */
export interface StudentLeaveVerifyResult {
  valid: boolean;
  letter_number?: string;
  student_name?: string;
  class_name?: string;
  type?: StudentLeaveType;
  date_start?: string;
  date_end?: string;
  issued_at?: string;
}

/* ---- QR Token Guru / Dispensasi Keluar / Terlambat (Fase 14 Gelombang B2, docs/12-sion-parity.md Gelombang B alur 2-3) ---- */

/** POST /api/teacher-qr (guru) — TTL 60 detik, sekali pakai. Konten QR: `nouschool:tqr:{token}`. */
export interface TeacherQrToken {
  token: string;
  expires_at: string;
}

export type ExitPermitStatus =
  | 'pending_duty_teacher'
  | 'pending_class_teacher'
  | 'pending_bk'
  | 'pending_leadership'
  | 'issued'
  | 'exited'
  | 'rejected'
  | 'canceled';

export interface ExitPermitStudentRef {
  id: string;
  name: string;
  nis: string;
  class_name: string;
}

/** Rincian satu tahap keputusan rantai dispensasi keluar (piket/guru pengajar/BK/pimpinan) — `null` selama tahap itu belum dilalui. */
export interface ExitPermitStepInfo {
  by_name: string;
  at: string;
}

/**
 * Satu baris `GET /api/exit-permits?scope=mine|active|all&date=` → `{items,total}`.
 * `gate_token` HANYA terisi untuk pemilik (siswa yang mengajukan) saat `status === 'issued'`
 * (kontrak Fase 14 Gelombang B2) — di baris lain/peran lain selalu `null`/absen.
 */
export interface ExitPermit {
  id: string;
  student: ExitPermitStudentRef;
  reason: string;
  status: ExitPermitStatus;
  duty: ExitPermitStepInfo | null;
  class: ExitPermitStepInfo | null;
  bk: ExitPermitStepInfo | null;
  leadership: ExitPermitStepInfo | null;
  gate_token?: string | null;
  gate_expires_at: string | null;
  exited_at: string | null;
  created_at: string;
}

export interface ExitPermitListResult {
  items: ExitPermit[];
  total: number;
}

/** POST /api/exit-permits (siswa) — 409 kalau masih ada dispensasi aktif. */
export interface ExitPermitCreateInput {
  reason: string;
}

/**
 * POST /api/exit-permits/{id}/scan (siswa) `{token}` → izin + tahap baru.
 * ASUMSI bentuk respons: mengikuti pola `DisciplineRecordCreateResult`/late-arrival
 * scan yang sudah dikonfirmasi kontrak (entitas + field ekstra sebagai saudara,
 * bukan digabung rata) — `stage_advanced_to` memakai nilai `ExitPermitStatus`
 * yang baru dicapai setelah scan ini (mis. `pending_bk` setelah tahap guru
 * pengajar, atau `issued` setelah tahap terakhir/pimpinan).
 */
export interface ExitPermitScanResult {
  permit: ExitPermit;
  stage_advanced_to: ExitPermitStatus;
}

/** POST /api/exit-permits/gate-scan (security/pegawai) `{gate_token}` — 410 kalau kedaluwarsa. */
export interface ExitPermitGateScanResult {
  student: { name: string; nis: string; class_name: string };
  reason: string;
  issued_at: string;
  gate_expires_at: string;
}

/**
 * Satu baris `GET /api/exit-permits/gate-history?date=` → `{items,total}`.
 * ASUMSI: bentuk mengikuti pola daftar lain di modul ini (`{items,total}`,
 * `student` sebagai ref bersarang) — riwayat siswa yang SUDAH discan gate
 * (`exited`) pada tanggal terkait.
 */
export interface ExitPermitGateHistoryItem {
  id: string;
  student: ExitPermitStudentRef;
  reason: string;
  exited_at: string;
}

export interface ExitPermitGateHistoryResult {
  items: ExitPermitGateHistoryItem[];
  total: number;
}

/** POST /api/exit-permits/{id}/reject (admin only) — hanya kalau status masih pending_*. */
export interface ExitPermitRejectInput {
  comment: string;
}

export type LateArrivalAction = 'none' | 'call_parent' | 'send_home';

export type LateArrivalStatus = 'pending_duty_teacher' | 'pending_leadership' | 'pending_class_teacher' | 'completed';

/** Rincian satu tahap keputusan rantai terlambat (piket/pimpinan/guru kelas) — `null` selama tahap itu belum dilalui. */
export interface LateArrivalStepInfo {
  by_name: string;
  at: string;
}

/**
 * `record` dalam `POST /api/late-arrivals/scan` & satu baris `GET /api/late-arrivals?scope=&month=`.
 * ASUMSI: `student` HANYA terisi di scope `all` (staf melihat lintas siswa) —
 * sama pola `DisciplineRecord`/`ExitPermit` (siswa sendiri tidak perlu ref ke dirinya di scope `mine`).
 */
export interface LateArrivalRecord {
  id: string;
  student?: ExitPermitStudentRef;
  arrived_at: string;
  late_count: number;
  action: LateArrivalAction;
  status: LateArrivalStatus;
  duty: LateArrivalStepInfo | null;
  leadership: LateArrivalStepInfo | null;
  class_teacher: LateArrivalStepInfo | null;
}

export interface LateArrivalListResult {
  items: LateArrivalRecord[];
  total: number;
}

/** POST /api/late-arrivals/scan (siswa) `{token}` — kontrak eksplisit Fase 14 Gelombang B2. */
export interface LateArrivalScanResult {
  record: LateArrivalRecord;
  action: LateArrivalAction;
  late_count: number;
}

/** GET /api/late-arrivals/summary?month= — array langsung (pola sama `DisciplineSummaryRow`), rekap per siswa. */
export interface LateArrivalSummaryRow {
  student: ExitPermitStudentRef;
  count: number;
  last_action: LateArrivalAction;
  last_at: string | null;
}
