// Package billing mengurus langganan tahunan per sekolah: tier (Basic/Pro) x
// bracket jumlah siswa, invoice, dua channel pembayaran (transfer manual +
// payment gateway Midtrans), lifecycle (active/grace/readonly), dan feature
// gating (lihat docs/09-billing.md).
package billing

import "time"

// Status kanonik subscriptions.status (docs/09-billing.md "Lifecycle langganan").
const (
	StatusActive   = "active"
	StatusGrace    = "grace"
	StatusReadonly = "readonly"
	StatusCanceled = "canceled"
)

// Status kanonik invoices.status.
const (
	InvoiceUnpaid               = "unpaid"
	InvoiceAwaitingVerification = "awaiting_verification"
	InvoicePaid                 = "paid"
	InvoiceVoid                 = "void"
	InvoiceExpired              = "expired"
)

// Metode pembayaran (payments.method).
const (
	MethodManualTransfer = "manual_transfer"
	MethodGateway        = "gateway"
)

// PermBillingView — NILAI HARUS SAMA PERSIS dengan identity.PermBillingView
// (didefinisikan ulang di sini karena billing TIDAK boleh mengimpor identity
// untuk tipe/konstanta apa pun — lihat CLAUDE.md; pola yang sama dengan
// internal/leave.EventLeaveSubmitted vs notification.EventLeaveSubmitted).
const PermBillingView = "billing:view"

// GracePeriodDays — masa tenggang setelah ends_on lewat sebelum readonly
// (docs/09-billing.md: "grace (14 hari, banner peringatan)").
const GracePeriodDays = 14

// SubscriptionPeriodYears — satu periode langganan = 1 tahun ("Model langganan TAHUNAN").
const SubscriptionPeriodYears = 1

// FeatureKeys kanonik — kunci baru = tambah di sini + default per plan di
// seeding migrations/00011_billing.sql (docs/09-billing.md "Feature gating").
const (
	FeatureTVDashboard   = "tv_dashboard"
	FeatureWhatsApp      = "whatsapp"
	FeatureDapodikImport = "dapodik_import"
	FeatureQRCard        = "qr_card"
	FeatureSelfCheckin   = "self_checkin"
)

// -- domain records (representasi baris DB, dipetakan repository.go) --

// PlanRecord adalah satu baris plans + daftar bracket harganya.
type PlanRecord struct {
	Code     string
	Name     string
	Features map[string]bool
	Active   bool
	Prices   []PriceBracket
}

// PriceBracket adalah satu baris plan_prices.
type PriceBracket struct {
	MaxStudents int   `json:"max_students"`
	PriceYearly int64 `json:"price_yearly"`
}

// SubscriptionRecord adalah representasi domain satu baris subscriptions —
// SNAPSHOT plan_code/features/max_students saat subscribe/renew (docs/09).
// FeatureOverrides (fase 13 Gelombang 2, docs/11-superadmin.md P6.1) — kill
// switch/unlock fitur per sekolah TANPA ganti plan, di-MERGE dengan Features
// (override MENANG) saat resolusi fitur efektif — lihat mergeFeatures &
// snapshotFor.
type SubscriptionRecord struct {
	ID               int64
	SchoolID         int64
	PlanCode         string
	Features         map[string]bool
	FeatureOverrides map[string]bool
	MaxStudents      int
	Price            int64
	StartsOn         time.Time
	EndsOn           time.Time
	Status           string
}

// GraceUntil deriva dari EndsOn + GracePeriodDays (TIDAK disimpan sebagai
// kolom — docs/09-billing.md skema subscriptions tidak punya grace_until).
func (s SubscriptionRecord) GraceUntil() time.Time {
	return s.EndsOn.AddDate(0, 0, GracePeriodDays)
}

// InvoiceRecord adalah representasi domain satu baris invoices.
type InvoiceRecord struct {
	ID             int64
	SchoolID       int64
	Number         string
	SubscriptionID int64 // 0 = belum ada subscription (invoice pertama)
	Amount         int64
	Status         string
	DueAt          time.Time
	PaidAt         *time.Time
	PlanCode       string
	MaxStudents    int
	CreatedBy      int64
	CreatedAt      time.Time
}

// PaymentRecord adalah representasi domain satu baris payments.
type PaymentRecord struct {
	ID          int64
	InvoiceID   int64
	Method      string
	Amount      int64
	ProofURL    string
	VerifiedBy  int64
	VerifiedAt  *time.Time
	Provider    string
	ProviderRef string
	RawWebhook  []byte
	CreatedAt   time.Time
}

// -- response views (JSON) --

// SubscriptionView adalah shape "subscription" pada GET /api/billing &
// GET /api/admin/schools/{id}/billing (docs/09-billing.md scope Fase 10).
// Features adalah DAFTAR kunci yang AKTIF (bukan map) — sama makna/bentuk
// dengan UserView.Features di /api/me (identity/model.go), supaya frontend
// memakai satu bentuk konsisten `features.includes(key)` di mana pun.
//
// FeatureOverrides & FeaturesEffective (fase 13 Gelombang 2, docs/11-
// superadmin.md P6.1/P6.2) — field TAMBAHAN SAJA, tidak mengubah Features
// (frontend existing tetap jalan tanpa perubahan): FeatureOverrides adalah
// override MENTAH tersimpan (key hanya ada bila di-set eksplisit true/false
// oleh super admin), FeaturesEffective adalah Features di-MERGE dengan
// FeatureOverrides (override menang) — sama persis dengan yang dipakai
// /api/me, RequireFeature, notification whatsapp gate (lihat snapshotFor).
type SubscriptionView struct {
	PlanCode          string          `json:"plan_code"`
	PlanName          string          `json:"plan_name"`
	Status            string          `json:"status"`
	StartsOn          string          `json:"starts_on"`
	EndsOn            string          `json:"ends_on"`
	GraceUntil        *string         `json:"grace_until"`
	MaxStudents       int             `json:"max_students"`
	StudentCount      int             `json:"student_count"`
	Price             int64           `json:"price"`
	Features          []string        `json:"features"`
	FeatureOverrides  map[string]bool `json:"feature_overrides"`
	FeaturesEffective []string        `json:"features_effective"`
}

// PaymentView adalah shape "payment" tersemat pada InvoiceView.
type PaymentView struct {
	Method     string     `json:"method"`
	Status     string     `json:"status"` // pending|awaiting_verification|paid
	ProofURL   *string    `json:"proof_url,omitempty"`
	Provider   *string    `json:"provider,omitempty"`
	VerifiedAt *time.Time `json:"verified_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}

// InvoiceView adalah satu baris pada "invoices" GET /api/billing & GET
// /api/admin/schools/{id}/billing. ProofURL (bukti transfer, nil bila
// belum ada) sengaja SELALU disertakan (bukan hanya di endpoint admin) —
// tenant sendiri boleh melihat bukti transfernya (docs/09-billing.md
// "Bukti transfer file: ... tenant sendiri boleh lihat").
type InvoiceView struct {
	ID       int64        `json:"id"`
	Number   string       `json:"number"`
	Amount   int64        `json:"amount"`
	Status   string       `json:"status"`
	DueAt    time.Time    `json:"due_at"`
	PaidAt   *time.Time   `json:"paid_at"`
	PDFURL   string       `json:"pdf_url"`
	ProofURL *string      `json:"proof_url"`
	Payment  *PaymentView `json:"payment"`
}

// BillingView adalah shape lengkap GET /api/billing.
type BillingView struct {
	Subscription *SubscriptionView `json:"subscription"`
	Invoices     []InvoiceView     `json:"invoices"`
}

// PlanView adalah shape GET /api/admin/plans.
type PlanView struct {
	Code     string          `json:"code"`
	Name     string          `json:"name"`
	Active   bool            `json:"active"`
	Features map[string]bool `json:"features"`
	Prices   []PriceBracket  `json:"prices"`
}

// AdminSubscriptionRow adalah satu baris GET (daftar internal, dipakai
// panel super admin bila dibutuhkan) — status semua sekolah.
type AdminSubscriptionRow struct {
	SchoolID   int64  `json:"school_id"`
	SchoolName string `json:"school_name"`
	SchoolSlug string `json:"school_slug"`
	PlanCode   string `json:"plan_code"`
	Status     string `json:"status"`
	EndsOn     string `json:"ends_on"`
}

// -- P6.3/P6.4 (fase 13 Gelombang 2): GET /api/admin/revenue & /export --

// RevenueInvoiceRow adalah satu baris invoice PAID (dipakai buildRevenueReport
// & sheet "Invoice" export xlsx).
type RevenueInvoiceRow struct {
	Number     string
	SchoolName string
	PlanCode   string
	Amount     int64
	PaidAt     time.Time
}

// RevenueMonth adalah satu baris "months" GET /api/admin/revenue — SELALU 12
// baris (Januari..Desember), termasuk bulan tanpa invoice paid (paid_total=0).
type RevenueMonth struct {
	Month        int   `json:"month"`
	PaidTotal    int64 `json:"paid_total"`
	InvoiceCount int   `json:"invoice_count"`
}

// RevenueByPlan adalah satu baris "by_plan" GET /api/admin/revenue — HANYA
// plan yang muncul di invoice paid tahun itu, terurut plan_code.
type RevenueByPlan struct {
	PlanCode     string `json:"plan_code"`
	PaidTotal    int64  `json:"paid_total"`
	InvoiceCount int    `json:"invoice_count"`
}

// RevenueReport adalah shape lengkap GET /api/admin/revenue.
type RevenueReport struct {
	Year   int             `json:"year"`
	Total  int64           `json:"total"`
	Months []RevenueMonth  `json:"months"`
	ByPlan []RevenueByPlan `json:"by_plan"`
}
