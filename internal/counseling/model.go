// Package counseling mengurus sesi konseling BK: tujuan karir, deskripsi
// masalah, rencana tindak lanjut, bukti foto/dokumen opsional, dan laporan
// HTML per sesi (Fase 14 Gelombang D, docs/12-sion-parity.md). PRIVAT BK:
// siswa/ortu TIDAK punya akses sama sekali (beda dari discipline yang
// siswa/ortu boleh lihat poin sendiri) — otorisasi dibaca dari duty flag
// leave_issuance (Guru BK) ATAU role admin_sekolah; kepala_sekolah read-only.
package counseling

import "time"

// Role kanonik dipakai modul counseling (nilai HARUS sama persis dengan
// internal/identity — didefinisikan ulang di sini karena counseling TIDAK
// boleh mengimpor identity, lihat CLAUDE.md).
const (
	RoleAdminSekolah  = "admin_sekolah"
	RoleKepalaSekolah = "kepala_sekolah"
)

// FlagLeaveIssuance — nilai HARUS sama persis dengan duty.FlagLeaveIssuance
// (didefinisikan ulang di sini karena counseling TIDAK boleh mengimpor duty,
// lihat CLAUDE.md). Pemegang flag ini = Guru BK (docs/12-sion-parity.md
// Gelombang B "adopsi pola SION").
const FlagLeaveIssuance = "leave_issuance"

// -- response shapes --

// StudentRef adalah potongan info siswa disematkan pada CounselingView.
type StudentRef struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	NIS       string `json:"nis"`
	ClassName string `json:"class_name"`
}

// CounselingView adalah shape satu baris response GET/POST/PATCH
// /api/counselings.
type CounselingView struct {
	ID                 int64      `json:"id"`
	Student            StudentRef `json:"student"`
	CounselorID        int64      `json:"counselor_id"`
	CounselorName      string     `json:"counselor_name"`
	CareerGoals        string     `json:"career_goals"`
	ProblemDescription string     `json:"problem_description"`
	FollowUpPlan       string     `json:"follow_up_plan"`
	HasEvidence        bool       `json:"has_evidence"`
	EvidenceName       string     `json:"evidence_name,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

// ListResult adalah shape response GET /api/counselings.
type ListResult struct {
	Items      []CounselingView `json:"items"`
	Page       int              `json:"page"`
	PerPage    int              `json:"per_page"`
	TotalCount int64            `json:"total_count"`
}

// EvidenceInfo adalah hasil Service.GetEvidence — dipakai handler menyusun
// response file (pola yang sama dengan studentleave.AttachmentInfo).
type EvidenceInfo struct {
	Path        string
	Name        string
	ContentType string
}
