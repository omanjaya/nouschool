// Package substitution mengurus permintaan guru pengganti per slot jadwal +
// tanggal: request -> accept/reject oleh pengganti, atau cancel oleh pengaju
// selama masih pending (Fase 14 Gelombang D, docs/12-sion-parity.md). Modul
// teaching (scan QR) & monitoring/TV mengonsumsi status "accepted" lewat
// consumer-side interface publik SubstituteName/IsSubstituteToday.
package substitution

import "time"

// Role kanonik dipakai modul substitution (nilai HARUS sama persis dengan
// internal/identity — didefinisikan ulang di sini karena substitution TIDAK
// boleh mengimpor identity, lihat CLAUDE.md).
const (
	RoleAdminSekolah = "admin_sekolah"
	RoleGuru         = "guru"
)

// Permission kanonik dipakai modul substitution (lihat docs/02-identity.md —
// reuse "schedule:manage", TIDAK menambah permission baru: scope=all adalah
// pandangan admin atas jadwal, konsisten dgn gerbang modul schedule sendiri).
const PermScheduleManage = "schedule:manage"

// Status kanonik teacher_substitution_requests.status.
const (
	StatusPending  = "pending"
	StatusAccepted = "accepted"
	StatusRejected = "rejected"
	StatusCanceled = "canceled"
)

func validStatus(s string) bool {
	switch s {
	case StatusPending, StatusAccepted, StatusRejected, StatusCanceled:
		return true
	default:
		return false
	}
}

// -- response shapes --

type SlotRef struct {
	ID          int64  `json:"id"`
	ClassName   string `json:"class_name"`
	SubjectName string `json:"subject_name"`
	DayOfWeek   int    `json:"day_of_week"`
	PeriodStart int    `json:"period_start"`
	PeriodEnd   int    `json:"period_end"`
}

type UserRef struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// RequestView adalah shape satu baris response GET/POST /api/substitutions.
type RequestView struct {
	ID          int64      `json:"id"`
	Slot        SlotRef    `json:"slot"`
	Date        string     `json:"date"`
	RequestedBy UserRef    `json:"requested_by"`
	Substitute  UserRef    `json:"substitute"`
	Reason      string     `json:"reason"`
	Status      string     `json:"status"`
	DecidedAt   *time.Time `json:"decided_at"`
	CreatedAt   time.Time  `json:"created_at"`
}
