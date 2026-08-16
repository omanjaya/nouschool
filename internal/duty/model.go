// Package duty mengurus tugas tambahan (Wali Kelas, Guru BK, Guru Piket,
// Pimpinan, Security, dst) + capability flags per TA — fondasi otorisasi
// dipakai modul lain (mis. studentleave) via consumer-side interface publik
// UserHasFlag/UserIDsWithFlag. Adopsi pola SION (docs/12-sion-parity.md
// Gelombang B): alur izin/dispensasi membaca FLAGS yang dibawa tugas
// seseorang, BUKAN mengecek role secara langsung.
package duty

// Role kanonik dipakai modul duty (nilai HARUS sama persis dengan
// internal/identity — didefinisikan ulang di sini karena duty TIDAK boleh
// mengimpor identity, lihat CLAUDE.md).
const (
	RoleGuru    = "guru"
	RolePegawai = "pegawai"
)

// ForRole kanonik duties.for_role (CHECK constraint migrasi) — nilainya
// SAMA dengan Role di atas (satu tugas "untuk" role guru atau pegawai).
const (
	ForRoleGuru    = RoleGuru
	ForRolePegawai = RolePegawai
)

func validForRole(v string) bool {
	return v == ForRoleGuru || v == ForRolePegawai
}

// Permission kanonik dipakai modul duty (lihat docs/02-identity.md).
const PermDutyManage = "duty:manage"

// Capability flags kanonik (docs/12-sion-parity.md Gelombang B). Nilai-nilai
// ini dibaca modul lain (studentleave Gelombang B1; exit/late_arrival
// Gelombang B2 — DIDEFINISIKAN SEKARANG sesuai instruksi tugas, belum dipakai
// endpoint apa pun) lewat Service.UserHasFlag/UserIDsWithFlag.
const (
	FlagLeaveHomeroomReview    = "leave_homeroom_review"
	FlagLeaveIssuance          = "leave_issuance"
	FlagExitBKApproval         = "exit_bk_approval"
	FlagExitLeadershipApproval = "exit_leadership_approval"
	FlagExitSecurity           = "exit_security"
	FlagLateArrivalDuty        = "late_arrival_duty"
	FlagLateArrivalLeadership  = "late_arrival_leadership"
	FlagAllAttendanceReports   = "all_attendance_reports"
)

// flagLabels — label Indonesia dipakai GET /api/duties/flags (UI memilih
// flags saat membuat/mengubah tugas).
var flagLabels = map[string]string{
	FlagLeaveHomeroomReview:    "Review izin (wali kelas)",
	FlagLeaveIssuance:          "Terbitkan surat izin (BK)",
	FlagExitBKApproval:         "Setujui dispensasi keluar (BK)",
	FlagExitLeadershipApproval: "Setujui dispensasi keluar (pimpinan)",
	FlagExitSecurity:           "Scan gate keluar (security)",
	FlagLateArrivalDuty:        "Tangani keterlambatan (guru piket)",
	FlagLateArrivalLeadership:  "Tangani keterlambatan (pimpinan)",
	FlagAllAttendanceReports:   "Lihat semua laporan presensi",
}

// flagForRoleHint — dipakai UI sebagai petunjuk (BUKAN validasi keras): flag
// ini "biasanya" dipasang pada tugas for_role apa. "" berarti keduanya wajar.
var flagForRoleHint = map[string]string{
	FlagLeaveHomeroomReview:    ForRoleGuru,
	FlagLeaveIssuance:          ForRoleGuru,
	FlagExitBKApproval:         ForRoleGuru,
	FlagExitLeadershipApproval: ForRoleGuru,
	FlagExitSecurity:           ForRolePegawai,
	FlagLateArrivalDuty:        ForRoleGuru,
	FlagLateArrivalLeadership:  ForRoleGuru,
	FlagAllAttendanceReports:   "",
}

// allFlags — urutan tampil GET /api/duties/flags.
var allFlags = []string{
	FlagLeaveHomeroomReview, FlagLeaveIssuance, FlagExitBKApproval, FlagExitLeadershipApproval,
	FlagExitSecurity, FlagLateArrivalDuty, FlagLateArrivalLeadership, FlagAllAttendanceReports,
}

func validFlag(f string) bool {
	_, ok := flagLabels[f]
	return ok
}

// -- response shapes --

// DutyView adalah shape satu baris response GET /api/duties.
type DutyView struct {
	ID            int64    `json:"id"`
	Name          string   `json:"name"`
	ForRole       string   `json:"for_role"`
	Flags         []string `json:"flags"`
	Active        bool     `json:"active"`
	AssigneeCount int64    `json:"assignee_count"`
}

// FlagInfo adalah shape satu baris response GET /api/duties/flags.
type FlagInfo struct {
	Key         string `json:"key"`
	Label       string `json:"label"`
	ForRoleHint string `json:"for_role_hint"`
}

// AssignmentUserView adalah shape satu baris response GET .../assignments.
type AssignmentUserView struct {
	UserID int64  `json:"user_id"`
	Name   string `json:"name"`
	Role   string `json:"role"`
}
