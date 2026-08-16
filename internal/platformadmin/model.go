// Package platformadmin adalah modul AGREGATOR untuk panel super admin fase
// 13 (docs/11-superadmin.md): P1 dashboard platform (GET /api/admin/overview),
// P3 statistik kesehatan per sekolah (GET /api/admin/schools/{id}/stats), dan
// P4.4 outbox notifikasi global.
//
// Keputusan desain (dilaporkan di ROADMAP): modul BARU, bukan perluasan
// modul existing manapun, karena SEMUA endpoint di sini butuh menggabungkan
// data lintas BANYAK modul sekaligus (schools/subscriptions/invoices dari
// tenant+billing, students/teachers/classes dari student, attendance_sessions
// dari attendance, teaching_journals dari teaching, notification_outbox dari
// notification, sessions dari identity) — tidak ada satu modul existing yang
// jadi "pemilik alami" gabungan ini. Ini SAMA PERSIS alasan modul dashboard
// (fase 7, tv-board) dibuat: menyusun ulang data lintas modul untuk satu
// layar gabungan.
//
// BEDA dengan dashboard: dashboard TIDAK punya repository sendiri (murni
// consumer-side interface + adapter cmd/server, lihat dashboardadapter.go)
// karena kebutuhannya sedikit & method producer sudah ada. Modul ini
// sebaliknya PUNYA repository sendiri dengan SQL JOIN lintas tabel READ-ONLY
// langsung (lihat queries.sql) — pola consumer-side interface akan berarti
// puluhan primitif kecil + N+1 query per sekolah untuk overview lintas-SEMUA-
// sekolah (P1) & rekap 7/30 hari (P3), jauh lebih mahal & rumit daripada satu
// query agregat. Instruksi tugas fase 13 EKSPLISIT mengizinkan ini: "SQL
// agregasi lintas tabel read-only DIPERBOLEHKAN di queries modul agregator
// (preseden dashboard/tv-board)". Mutasi (retry outbox) TETAP dibatasi HANYA
// pada satu tabel (notification_outbox, status flag sederhana yang sudah
// dipahami worker existing di internal/notification/worker.go) — TIDAK
// menduplikasi logika bisnis modul lain.
//
// P4.1 (daftar anggota), P4.2 (reset password), P4.3 (audit log viewer)
// SENGAJA TIDAK di sini — ketiganya hanya menyentuh tabel milik modul
// identity sendiri (users/memberships/sessions/audit_log), jadi ditempatkan
// sebagai perluasan internal/identity (lihat internal/identity/admin.go).
//
// docs/11-superadmin.md aturan #3: "Statistik (P3) hanya agregat/angka,
// bukan data per-siswa" — SETIAP query di sini mengembalikan COUNT/SUM/MAX,
// TIDAK PERNAH baris data siswa individual.
package platformadmin

import "time"

// -- P1: GET /api/admin/overview --

// OverviewStats adalah bagian "stats" response.
type OverviewStats struct {
	SchoolsActive    int   `json:"schools_active"`
	SchoolsGrace     int   `json:"schools_grace"`
	SchoolsReadonly  int   `json:"schools_readonly"`
	SchoolsSuspended int   `json:"schools_suspended"`
	TotalStudents    int   `json:"total_students"`
	RevenueYear      int64 `json:"revenue_year"`
	Leads7d          int   `json:"leads_7d"`
}

// InvoiceAwaiting adalah satu baris attention.invoices_awaiting.
type InvoiceAwaiting struct {
	InvoiceID  int64  `json:"invoice_id"`
	Number     string `json:"number"`
	SchoolID   int64  `json:"school_id"`
	SchoolName string `json:"school_name"`
	Amount     int64  `json:"amount"`
}

// SchoolGrace adalah satu baris attention.schools_grace.
type SchoolGrace struct {
	SchoolID   int64  `json:"school_id"`
	Name       string `json:"name"`
	EndsOn     string `json:"ends_on"`
	GraceUntil string `json:"grace_until"`
}

// SchoolRef adalah satu baris attention.schools_readonly /
// attention.schools_no_active_year.
type SchoolRef struct {
	SchoolID int64  `json:"school_id"`
	Name     string `json:"name"`
}

// OutboxDeadBySchool adalah satu baris attention.outbox_dead.
type OutboxDeadBySchool struct {
	SchoolID   int64  `json:"school_id"`
	SchoolName string `json:"school_name"`
	DeadCount  int64  `json:"dead_count"`
}

// Attention adalah bagian "attention" response (inti dashboard P1).
type Attention struct {
	InvoicesAwaiting    []InvoiceAwaiting    `json:"invoices_awaiting"`
	SchoolsGrace        []SchoolGrace        `json:"schools_grace"`
	SchoolsReadonly     []SchoolRef          `json:"schools_readonly"`
	SchoolsNoActiveYear []SchoolRef          `json:"schools_no_active_year"`
	OutboxDead          []OutboxDeadBySchool `json:"outbox_dead"`
}

// LastActivity adalah satu baris "last_activity" response.
type LastActivity struct {
	SchoolID              int64      `json:"school_id"`
	Name                  string     `json:"name"`
	LastLogin             *time.Time `json:"last_login"`
	LastAttendanceSession *time.Time `json:"last_attendance_session"`
}

// Overview adalah shape lengkap GET /api/admin/overview.
type Overview struct {
	Stats        OverviewStats  `json:"stats"`
	Attention    Attention      `json:"attention"`
	LastActivity []LastActivity `json:"last_activity"`
}

// -- P3: GET /api/admin/schools/{id}/stats --

// OutboxCounts adalah bagian notifications_30d.
type OutboxCounts struct {
	Sent   int64 `json:"sent"`
	Failed int64 `json:"failed"`
	Dead   int64 `json:"dead"`
}

// LastLoginByRole adalah satu baris last_logins.
type LastLoginByRole struct {
	Role string    `json:"role"`
	At   time.Time `json:"at"`
}

// SchoolStats adalah shape lengkap GET /api/admin/schools/{id}/stats.
type SchoolStats struct {
	Teachers             int               `json:"teachers"`
	Students             int               `json:"students"`
	Classes              int               `json:"classes"`
	AttendanceSessions7d int               `json:"attendance_sessions_7d"`
	Journals7d           int               `json:"journals_7d"`
	Notifications30d     OutboxCounts      `json:"notifications_30d"`
	LastLogins           []LastLoginByRole `json:"last_logins"`
	UploadsBytes         int64             `json:"uploads_bytes"`
}

// -- P4.4: outbox global --

// OutboxItem adalah satu baris response GET /api/admin/outbox.
type OutboxItem struct {
	ID          int64      `json:"id"`
	SchoolID    int64      `json:"school_id"`
	SchoolName  string     `json:"school_name"`
	Event       string     `json:"event"`
	Channel     string     `json:"channel"`
	UserName    string     `json:"user_name"`
	Status      string     `json:"status"`
	Attempts    int        `json:"attempts"`
	NextRetryAt *time.Time `json:"next_retry_at"`
	CreatedAt   time.Time  `json:"created_at"`
}

// OutboxPage adalah shape response GET /api/admin/outbox.
type OutboxPage struct {
	Items []OutboxItem `json:"items"`
	Total int64        `json:"total"`
}

// -- repository row shapes (dipetakan repository.go dari sqlc) --

// SchoolOverviewRow adalah satu baris mentah dari ListSchoolsOverview —
// dipetakan Service.Overview (effectiveStatus) menjadi bucket status +
// daftar perlu-perhatian. Diekspos (bukan unexported) supaya fake repository
// test bisa membangunnya langsung (lihat service_test.go).
type SchoolOverviewRow struct {
	SchoolID     int64
	Name         string
	SchoolStatus string
	SubStatus    *string
	SubEndsOn    *time.Time
}

// -- P2.2 (fase 13 Gelombang 2): GET /api/admin/schools/{id}/onboarding --

// OnboardingRow adalah baris mentah dari SchoolOnboardingStatus (booleans
// murni, TANPA field Ready — diderivasi Service.Onboarding).
type OnboardingRow struct {
	HasActiveYear         bool
	HasAdmin              bool
	HasSubscriptionActive bool
	HasStudents           bool
	HasSchedule           bool
}

// OnboardingStatus adalah shape response GET /api/admin/schools/{id}/onboarding.
type OnboardingStatus struct {
	HasActiveYear         bool `json:"has_active_year"`
	HasAdmin              bool `json:"has_admin"`
	HasSubscriptionActive bool `json:"has_subscription_active"`
	HasStudents           bool `json:"has_students"`
	HasSchedule           bool `json:"has_schedule"`
	Ready                 bool `json:"ready"`
}

// -- P5 (fase 13 Gelombang 2): platform_announcements,
// docs/11-superadmin.md "Pengumuman platform" --

// PlatformAnnouncementRecord adalah representasi mentah satu baris
// platform_announcements (dipetakan repository.go dari sqlc).
type PlatformAnnouncementRecord struct {
	ID        int64
	Title     string
	Body      string
	StartsAt  time.Time
	EndsAt    time.Time
	CreatedBy int64
	CreatedAt time.Time
}

// PlatformAnnouncementView adalah shape response CRUD
// /api/admin/platform-announcements[/{id}].
type PlatformAnnouncementView struct {
	ID        int64     `json:"id"`
	Title     string    `json:"title"`
	Body      string    `json:"body"`
	StartsAt  string    `json:"starts_at"`
	EndsAt    string    `json:"ends_at"`
	CreatedBy int64     `json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
}

// PlatformAnnouncementItem — bentuk ringkas (primitif) dipakai konsumen
// lintas modul (internal/announcement, via adapter cmd/server) untuk
// menyisipkan pengumuman platform ke GET /api/announcements?active=1 dan
// GET /api/tv/board (docs/11-superadmin.md P5 "tampil sebagai announcement
// berlabel NouSchool di beranda/TV sekolah").
type PlatformAnnouncementItem struct {
	ID        int64
	Title     string
	Body      string
	StartsAt  string // YYYY-MM-DD
	EndsAt    string // YYYY-MM-DD
	CreatedAt time.Time
}
