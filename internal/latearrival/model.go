// Package latearrival mengurus izin terlambat (alur 3 dari 3 alur izin
// siswa SION, docs/12-sion-parity.md Gelombang B2): siswa scan QR guru ->
// piket -> pimpinan -> guru kelas -> completed, dengan aksi otomatis
// berdasarkan hitungan telat TA berjalan (ke-2/5 = panggil ortu, ke-3/6 =
// pulangkan). Maks SATU record per siswa per hari (tanggal lokal sekolah) —
// ditegakkan service layer (bukan CHECK/index DB, lihat catatan
// service.go).
package latearrival

import "time"

// Role kanonik dipakai modul latearrival (nilai HARUS sama persis dengan
// internal/identity — didefinisikan ulang di sini karena latearrival TIDAK
// boleh mengimpor identity, lihat CLAUDE.md).
const (
	RoleAdminSekolah  = "admin_sekolah"
	RoleKepalaSekolah = "kepala_sekolah"
	RoleSiswa         = "siswa"
	RoleOrangTua      = "orang_tua"
)

// Capability flags kanonik — nilai HARUS sama persis dengan
// internal/duty.Flag* (didefinisikan ulang di sini, latearrival TIDAK boleh
// mengimpor duty).
const (
	FlagLateArrivalDuty       = "late_arrival_duty" // tahap 1 "piket" — DIBAGI dengan internal/exitpermit
	FlagLateArrivalLeadership = "late_arrival_leadership"
)

// Permission kanonik dipakai modul latearrival — TIDAK ada permission baru,
// reuse student:read/student:manage (pola sama internal/studentleave &
// internal/exitpermit).
const (
	PermStudentRead   = "student:read"
	PermStudentManage = "student:manage"
)

// Action kanonik student_late_arrivals.action.
const (
	ActionNone       = "none"
	ActionCallParent = "call_parent"
	ActionSendHome   = "send_home"
)

// lateArrivalAction — fungsi MURNI (docs tugas: test count 1..7): telat
// ke-2 & ke-5 = panggil ortu, ke-3 & ke-6 = pulangkan, selainnya tanpa aksi.
func lateArrivalAction(count int) string {
	switch count {
	case 2, 5:
		return ActionCallParent
	case 3, 6:
		return ActionSendHome
	default:
		return ActionNone
	}
}

// actionLabel — label Indonesia dipakai notifikasi ortu (docs tugas: "sebut
// hitungan + action BILA ADA") — "" untuk ActionNone supaya template notif
// bisa {{if .action}} menyembunyikan kalimat aksi sama sekali.
func actionLabel(action string) string {
	switch action {
	case ActionCallParent:
		return "hubungi orang tua"
	case ActionSendHome:
		return "pulangkan siswa"
	default:
		return ""
	}
}

// Status kanonik student_late_arrivals.status — state machine SATU
// endpoint: belum ada record hari ini -> (scan piket) CREATE langsung
// pending_leadership -> (scan pimpinan) pending_class_teacher -> (scan guru
// kelas) completed. StatusPendingDutyTeacher TIDAK PERNAH tersimpan di DB
// (hanya nilai konseptual "belum ada record" — kolom CHECK migrasi tetap
// mengizinkannya untuk konsistensi skema, docs tugas).
const (
	StatusPendingDutyTeacher  = "pending_duty_teacher"
	StatusPendingLeadership   = "pending_leadership"
	StatusPendingClassTeacher = "pending_class_teacher"
	StatusCompleted           = "completed"
)

// Event kanonik — nilai HARUS sama persis dengan
// notification.EventLateArrivalRecorded (didefinisikan ulang di sini karena
// latearrival TIDAK boleh mengimpor notification).
const EventLateArrivalRecorded = "latearrival.recorded"

// -- response shapes --

type StudentRef struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	NIS       string `json:"nis"`
	ClassName string `json:"class_name"`
}

type StageInfo struct {
	ByName string     `json:"by_name"`
	At     *time.Time `json:"at"`
}

// RecordView adalah shape lengkap satu catatan keterlambatan.
type RecordView struct {
	ID         int64      `json:"id"`
	Student    StudentRef `json:"student"`
	ArrivedAt  time.Time  `json:"arrived_at"`
	LateCount  int        `json:"late_count"`
	Action     string     `json:"action"`
	Status     string     `json:"status"`
	Duty       *StageInfo `json:"duty"`
	Leadership *StageInfo `json:"leadership"`
	Class      *StageInfo `json:"class"`
	CreatedAt  time.Time  `json:"created_at"`
}

// RecordPage adalah shape response GET /api/late-arrivals.
type RecordPage struct {
	Items []RecordView `json:"items"`
	Total int64        `json:"total"`
}

// ScanResult adalah shape response POST /api/late-arrivals/scan.
type ScanResult struct {
	Record    RecordView `json:"record"`
	Action    string     `json:"action"`
	LateCount int        `json:"late_count"`
}

// SummaryRow adalah shape satu baris GET /api/late-arrivals/summary.
type SummaryRow struct {
	Student StudentRef `json:"student"`
	Count   int64      `json:"count"`
}
