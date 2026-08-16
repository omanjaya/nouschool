// Package studentleave mengurus izin terencana siswa (alur 1 dari 3 alur
// izin siswa SION, docs/12-sion-parity.md Gelombang B): siswa/orang tua
// ajukan (+dokumen opsional) -> wali kelas review -> BK terbitkan surat
// bernomor -> verifikasi surat publik via QR/token. Otorisasi reviewer
// membaca CAPABILITY FLAGS (modul duty), BUKAN role langsung.
package studentleave

import (
	"fmt"
	"strings"
	"time"
)

// Role kanonik dipakai modul studentleave (nilai HARUS sama persis dengan
// internal/identity — didefinisikan ulang di sini karena studentleave TIDAK
// boleh mengimpor identity, lihat CLAUDE.md).
const (
	RoleAdminSekolah = "admin_sekolah"
	RoleGuru         = "guru"
	RoleSiswa        = "siswa"
	RoleOrangTua     = "orang_tua"
)

// Capability flags kanonik — nilai HARUS sama persis dengan
// internal/duty.FlagLeaveHomeroomReview / FlagLeaveIssuance (didefinisikan
// ulang di sini karena studentleave TIDAK boleh mengimpor duty).
const (
	FlagLeaveHomeroomReview = "leave_homeroom_review"
	FlagLeaveIssuance       = "leave_issuance"
)

// Permission kanonik dipakai modul studentleave (scope=all admin, lihat
// docs/02-identity.md) — SENGAJA `student:manage` (BUKAN permission baru),
// sama pola dengan internal/employee.
const PermManageAll = "student:manage"

// Type kanonik student_leave_requests.type.
const (
	TypeSakit = "sakit"
	TypeIzin  = "izin"
)

func validType(t string) bool { return t == TypeSakit || t == TypeIzin }

// Status kanonik student_leave_requests.status — state machine:
// pending_homeroom -> (approve) pending_bk -> (approve) issued
//
//	\-> (reject di tahap mana pun) -> rejected
//
// pemilik boleh cancel selama pending_*.
const (
	StatusPendingHomeroom = "pending_homeroom"
	StatusPendingBK       = "pending_bk"
	StatusIssued          = "issued"
	StatusRejected        = "rejected"
	StatusCanceled        = "canceled"
)

func isPending(status string) bool {
	return status == StatusPendingHomeroom || status == StatusPendingBK
}

// Decision kanonik — nilai LITERAL sesuai instruksi tugas ('approve'/
// 'reject', BEDA dari internal/leave yang memakai 'approved'/'rejected').
const (
	DecisionApprove = "approve"
	DecisionReject  = "reject"
)

func validDecision(d string) bool { return d == DecisionApprove || d == DecisionReject }

// Event kanonik — nilai HARUS sama persis dengan
// notification.EventStudentLeaveSubmitted/Forwarded/Decided (didefinisikan
// ulang di sini karena studentleave TIDAK boleh mengimpor notification).
const (
	EventStudentLeaveSubmitted = "studentleave.submitted"
	EventStudentLeaveForwarded = "studentleave.forwarded"
	EventStudentLeaveDecided   = "studentleave.decided"
)

// maxAttachmentSize/allowedAttachmentTypes — pola IDENTIK internal/leave
// (pdf/jpg/png maks 5MB).
const maxAttachmentSize = 5 << 20

var allowedAttachmentTypes = map[string]string{
	"application/pdf": "pdf",
	"image/jpeg":      "jpg",
	"image/png":       "png",
}

// Date adalah tanggal murni (tanpa jam), dikodekan JSON sebagai "YYYY-MM-DD"
// — didefinisikan ulang di sini (sama seperti internal/leave.Date/
// internal/discipline.Date) supaya studentleave tidak mengimpor modul lain.
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

// StudentRef adalah potongan info siswa disematkan pada RequestView.
type StudentRef struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	NIS       string `json:"nis"`
	ClassName string `json:"class_name"`
}

// DecisionInfo adalah shape satu tahap keputusan (homeroom/bk) — nil bila
// belum diputuskan.
type DecisionInfo struct {
	DecidedByName string     `json:"decided_by_name"`
	DecidedAt     *time.Time `json:"decided_at"`
	Comment       string     `json:"comment"`
}

// RequestView adalah shape lengkap satu pengajuan izin siswa.
type RequestView struct {
	ID            int64         `json:"id"`
	Student       StudentRef    `json:"student"`
	Type          string        `json:"type"`
	DateStart     Date          `json:"date_start"`
	DateEnd       Date          `json:"date_end"`
	Reason        string        `json:"reason"`
	Status        string        `json:"status"`
	AttachmentURL *string       `json:"attachment_url"`
	LetterNumber  *string       `json:"letter_number"`
	VerifyToken   *string       `json:"verify_token,omitempty"`
	Homeroom      *DecisionInfo `json:"homeroom"`
	BK            *DecisionInfo `json:"bk"`
	CreatedAt     time.Time     `json:"created_at"`
}

// RequestPage adalah shape response GET /api/student-leave.
type RequestPage struct {
	Items []RequestView `json:"items"`
	Total int64         `json:"total"`
}

// VerifyView adalah shape response GET /api/public/leave-verify.
type VerifyView struct {
	Valid        bool   `json:"valid"`
	LetterNumber string `json:"letter_number,omitempty"`
	StudentName  string `json:"student_name,omitempty"`
	ClassName    string `json:"class_name,omitempty"`
	Type         string `json:"type,omitempty"`
	DateStart    string `json:"date_start,omitempty"`
	DateEnd      string `json:"date_end,omitempty"`
	IssuedAt     string `json:"issued_at,omitempty"`
}
