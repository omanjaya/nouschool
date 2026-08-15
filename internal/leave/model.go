// Package leave mengurus izin guru dengan approval engine konfigurable: chain
// approval disimpan sebagai settings per sekolah, tapi DI-SNAPSHOT ke baris
// leave_approval_steps saat pengajuan dibuat — perubahan chain setelahnya
// TIDAK memengaruhi request yang sedang berjalan (lihat docs/07-leave.md).
package leave

import (
	"fmt"
	"strings"
	"time"
)

// Role kanonik dipakai modul leave (nilai HARUS sama persis dengan
// internal/identity — didefinisikan ulang di sini karena leave TIDAK boleh
// mengimpor identity, lihat CLAUDE.md).
const (
	RoleAdminSekolah  = "admin_sekolah"
	RoleKepalaSekolah = "kepala_sekolah"
	RoleGuru          = "guru"
)

// Permission kanonik dipakai modul leave (lihat docs/02-identity.md).
const (
	PermLeaveRequest = "leave:request"
	PermLeaveApprove = "leave:approve"
	PermLeaveManage  = "leave:manage"
)

// Status kanonik leave_requests.status.
const (
	StatusPending  = "pending"
	StatusApproved = "approved"
	StatusRejected = "rejected"
	StatusCanceled = "canceled"
)

// Decision kanonik leave_approval_steps.decision.
const (
	DecisionApproved = "approved"
	DecisionRejected = "rejected"
)

func validChainRole(role string) bool {
	switch role {
	case RoleAdminSekolah, RoleKepalaSekolah, RoleGuru:
		return true
	default:
		return false
	}
}

// Date adalah tanggal murni (tanpa jam), dikodekan JSON sebagai "YYYY-MM-DD".
// Sama seperti internal/attendance.Date — didefinisikan ulang di sini supaya
// leave TIDAK mengimpor package lain untuk tipe ini.
type Date struct{ time.Time }

func NewDate(t time.Time) Date { return Date{t} }

func (d Date) MarshalJSON() ([]byte, error) {
	if d.IsZero() {
		return []byte("null"), nil
	}
	return []byte(fmt.Sprintf("%q", d.Format("2006-01-02"))), nil
}

func (d *Date) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	if s == "" || s == "null" {
		return nil
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return fmt.Errorf("format tanggal harus YYYY-MM-DD: %w", err)
	}
	d.Time = t
	return nil
}

// -- response shapes --

// LeaveTypeView adalah shape response GET /api/leave/types.
type LeaveTypeView struct {
	Key   string `json:"key"`
	Label string `json:"label"`
}

// TeacherView adalah pengaju cuti (bagian dari Request).
type TeacherView struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// StepView adalah satu baris approval step (bagian dari Request).
type StepView struct {
	ID           int64      `json:"id"`
	StepOrder    int        `json:"step_order"`
	ApproverRole string     `json:"approver_role"`
	ApproverName *string    `json:"approver_name"`
	Decision     *string    `json:"decision"`
	DecidedAt    *time.Time `json:"decided_at"`
	Comment      *string    `json:"comment"`
}

// Request adalah shape lengkap satu pengajuan izin — dipakai response
// POST/GET /api/leave/requests dan GET /api/leave/approvals.
type Request struct {
	ID            int64       `json:"id"`
	Type          string      `json:"type"`
	TypeLabel     string      `json:"type_label"`
	DateStart     Date        `json:"date_start"`
	DateEnd       Date        `json:"date_end"`
	Days          int         `json:"days"`
	Reason        string      `json:"reason"`
	Status        string      `json:"status"`
	CreatedAt     time.Time   `json:"created_at"`
	Teacher       TeacherView `json:"teacher"`
	AttachmentURL *string     `json:"attachment_url"`
	Steps         []StepView  `json:"steps"`
}

// QueueItem adalah shape satu baris response GET /api/leave/approvals.
type QueueItem struct {
	StepID  int64   `json:"step_id"`
	Request Request `json:"request"`
}

// SummaryRow adalah shape satu baris response GET /api/leave/summary.
type SummaryRow struct {
	TeacherID int64          `json:"teacher_id"`
	Name      string         `json:"name"`
	PerType   map[string]int `json:"per_type"`
	TotalDays int            `json:"total_days"`
}
