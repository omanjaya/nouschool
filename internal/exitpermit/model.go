// Package exitpermit mengurus izin dispensasi keluar (alur 2 dari 3 alur
// izin siswa SION, docs/12-sion-parity.md Gelombang B2): rantai QR 4 tahap
// (piket -> guru pengajar jam berjalan/berikutnya & BEDA ORANG dari piket ->
// BK -> pimpinan) -> issued (gate_token kedaluwarsa otomatis akhir jam
// terakhir hari itu) -> scan gerbang security -> exited. Otorisasi tahap
// membaca CAPABILITY FLAGS (modul duty) + jadwal (modul schedule), BUKAN
// role langsung — pola sama internal/studentleave.
package exitpermit

import "time"

// Role kanonik dipakai modul exitpermit (nilai HARUS sama persis dengan
// internal/identity — didefinisikan ulang di sini karena exitpermit TIDAK
// boleh mengimpor identity, lihat CLAUDE.md).
const (
	RoleAdminSekolah  = "admin_sekolah"
	RoleKepalaSekolah = "kepala_sekolah"
	RoleSiswa         = "siswa"
	RoleOrangTua      = "orang_tua"
)

// Capability flags kanonik — nilai HARUS sama persis dengan
// internal/duty.Flag* (didefinisikan ulang di sini, exitpermit TIDAK boleh
// mengimpor duty).
const (
	FlagLateArrivalDuty        = "late_arrival_duty" // tahap 1 "piket" — flag DIBAGI dengan internal/latearrival (docs tugas)
	FlagExitBKApproval         = "exit_bk_approval"
	FlagExitLeadershipApproval = "exit_leadership_approval"
	FlagExitSecurity           = "exit_security"
)

// Permission kanonik dipakai modul exitpermit (lihat docs/02-identity.md) —
// TIDAK ada permission baru, reuse student:read/student:manage (pola sama
// internal/studentleave).
const (
	PermStudentRead   = "student:read"
	PermStudentManage = "student:manage"
)

// Status kanonik student_exit_permits.status — state machine:
// pending_duty_teacher -> (scan piket)          -> pending_class_teacher
//
//	-> (scan guru pengajar, BEDA ORANG)  -> pending_bk
//	-> (scan BK)                         -> pending_leadership
//	-> (scan pimpinan)                   -> issued (gate_token+gate_expires_at)
//	-> (scan gerbang security)           -> exited
//
// Reject (admin, tahap mana pun sebelum exited) -> rejected. Cancel (pemilik,
// tahap mana pun sebelum issued) -> canceled.
const (
	StatusPendingDutyTeacher  = "pending_duty_teacher"
	StatusPendingClassTeacher = "pending_class_teacher"
	StatusPendingBK           = "pending_bk"
	StatusPendingLeadership   = "pending_leadership"
	StatusIssued              = "issued"
	StatusExited              = "exited"
	StatusRejected            = "rejected"
	StatusCanceled            = "canceled"
)

func isPending(status string) bool {
	switch status {
	case StatusPendingDutyTeacher, StatusPendingClassTeacher, StatusPendingBK, StatusPendingLeadership:
		return true
	default:
		return false
	}
}

// isFinal — status akhir, tidak bisa diproses lagi (dipakai gerbang
// reject/scope=active).
func isFinal(status string) bool {
	return status == StatusExited || status == StatusRejected || status == StatusCanceled
}

// isActivePermit — dipakai gerbang "maks 1 permit aktif per siswa" (docs
// tugas): pending_* ATAU issued (belum keluar gerbang).
func isActivePermit(status string) bool {
	return isPending(status) || status == StatusIssued
}

// Event kanonik — nilai HARUS sama persis dengan
// notification.EventExitPermitIssued/Exited/Rejected (didefinisikan ulang di
// sini karena exitpermit TIDAK boleh mengimpor notification).
const (
	EventExitPermitIssued   = "exitpermit.issued"
	EventExitPermitExited   = "exitpermit.exited"
	EventExitPermitRejected = "exitpermit.rejected" // Fase 15 GAP 3
)

// -- response shapes --

// StudentRef adalah potongan info siswa disematkan pada PermitView.
type StudentRef struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	NIS       string `json:"nis"`
	ClassName string `json:"class_name"`
}

// StageInfo adalah shape satu tahap yang SUDAH dilalui — nil bila belum.
type StageInfo struct {
	ByName string     `json:"by_name"`
	At     *time.Time `json:"at"`
}

// RejectInfo adalah shape penolakan admin — nil bila tidak ditolak.
type RejectInfo struct {
	ByName  string     `json:"by_name"`
	At      *time.Time `json:"at"`
	Comment string     `json:"comment"`
}

// PermitView adalah shape lengkap satu pengajuan dispensasi keluar (trail
// SEMUA tahap by name + at, docs tugas: "shape lengkap trail per tahap").
type PermitView struct {
	ID              int64       `json:"id"`
	Student         StudentRef  `json:"student"`
	Reason          string      `json:"reason"`
	Status          string      `json:"status"`
	Duty            *StageInfo  `json:"duty"`
	Class           *StageInfo  `json:"class"`
	BK              *StageInfo  `json:"bk"`
	Leadership      *StageInfo  `json:"leadership"`
	Rejected        *RejectInfo `json:"rejected"`
	GateToken       *string     `json:"gate_token,omitempty"`
	GateExpiresAt   *time.Time  `json:"gate_expires_at,omitempty"`
	ExitedAt        *time.Time  `json:"exited_at,omitempty"`
	CreatedAt       time.Time   `json:"created_at"`
	StageAdvancedTo string      `json:"stage_advanced_to,omitempty"`
}

// PermitPage adalah shape response GET /api/exit-permits.
type PermitPage struct {
	Items []PermitView `json:"items"`
	Total int64        `json:"total"`
}

// GateExitView adalah shape response POST /api/exit-permits/gate-scan.
type GateExitView struct {
	Student       StudentRef `json:"student"`
	Reason        string     `json:"reason"`
	IssuedAt      time.Time  `json:"issued_at"`
	GateExpiresAt time.Time  `json:"gate_expires_at"`
	ExitedAt      time.Time  `json:"exited_at"`
}

// GateHistoryRow adalah shape satu baris GET /api/exit-permits/gate-history.
type GateHistoryRow struct {
	ID       int64      `json:"id"`
	Student  StudentRef `json:"student"`
	Reason   string     `json:"reason"`
	ExitedAt time.Time  `json:"exited_at"`
}
