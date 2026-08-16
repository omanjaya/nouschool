// Package identity mengurus user, membership, RBAC, dan sesi login
// (lihat docs/02-identity.md).
package identity

import "time"

// User adalah representasi domain dari baris tabel users.
type User struct {
	ID           int64
	Email        string
	Username     string
	PasswordHash string
	Name         string
	Phone        string
	IsSuperAdmin bool
	CreatedAt    time.Time
}

// Membership adalah representasi domain dari baris tabel memberships.
type Membership struct {
	ID       int64
	UserID   int64
	SchoolID int64
	Role     string
	Status   string
}

// SchoolView adalah potongan info sekolah yang disertakan pada response
// login/me — cukup id/name/slug, diambil dari reqctx (hasil resolusi Host).
type SchoolView struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
}

// UserView adalah shape response user untuk /api/auth/login dan /api/me.
type UserView struct {
	ID           int64       `json:"id"`
	Name         string      `json:"name"`
	Role         string      `json:"role"`
	Roles        []string    `json:"roles,omitempty"`
	IsSuperAdmin bool        `json:"is_super_admin"`
	School       *SchoolView `json:"school"`
	// StudentID terisi hanya untuk role siswa: id baris students miliknya —
	// dipakai UI untuk memanggil endpoint per-siswa (mis. riwayat kehadiran).
	StudentID int64 `json:"student_id,omitempty"`
	// Subscription & Features (fase 10, docs/09-billing.md "Frontend menerima
	// daftar fitur aktif di payload /api/me") — hanya terisi di host tenant
	// (School != nil) DAN bila BillingGateway di-inject (lihat service.go
	// SetBillingGateway). Sekolah TANPA subscription -> Subscription nil.
	Subscription *SubscriptionView `json:"subscription,omitempty"`
	Features     []string          `json:"features,omitempty"`
	// ImpersonatedBy (Fase 14 Gelombang D, docs/12-sion-parity.md) — terisi
	// HANYA bila sesi ini adalah sesi impersonasi USER (lihat
	// impersonation_user.go) — dipakai frontend menampilkan banner "sedang
	// masuk sebagai ... oleh {name}".
	ImpersonatedBy *ImpersonatorView `json:"impersonated_by,omitempty"`
}

// ImpersonatorView adalah potongan info admin yang sedang mengimpersonasi
// sesi ini (Fase 14 Gelombang D).
type ImpersonatorView struct {
	Name string `json:"name"`
}

// SubscriptionView adalah ringkasan status langganan sekolah pada payload
// /api/me (fase 10, docs/09-billing.md).
type SubscriptionView struct {
	Status     string  `json:"status"`
	PlanCode   string  `json:"plan_code"`
	EndsOn     string  `json:"ends_on"`
	GraceUntil *string `json:"grace_until,omitempty"`
}
