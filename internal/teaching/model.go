// Package teaching mengurus jurnal mengajar guru (scan QR ruangan atau entry
// manual/unscheduled) dan derivasi status monitoring "guru sedang mengajar di
// mana" (docs/06-teaching.md). Modul ini TIDAK menyimpan status sebagai
// tabel — status selalu DIHITUNG saat baca dari journal + jadwal + izin.
package teaching

import (
	"fmt"
	"strings"
	"time"
)

// Role kanonik dipakai modul teaching (nilai HARUS sama persis dengan
// internal/identity — didefinisikan ulang di sini karena teaching TIDAK
// boleh mengimpor identity, lihat CLAUDE.md).
const (
	RoleAdminSekolah  = "admin_sekolah"
	RoleGuru          = "guru"
	RoleKepalaSekolah = "kepala_sekolah"
	RoleDisplay       = "display"
)

// Permission kanonik dipakai modul teaching (lihat docs/02-identity.md).
const (
	PermTeachingJournalWrite = "teaching:journal_write"
	PermTeachingMonitor      = "teaching:monitor"
)

// Flag kanonik teaching_journals.flags.
const (
	FlagRoomMismatch = "room_mismatch"
	FlagUnscheduled  = "unscheduled"
	// FlagSubstitute — jurnal dibuka oleh guru PENGGANTI (accepted lewat
	// internal/substitution), bukan pemilik asli slot (Fase 14 Gelombang D,
	// docs/12-sion-parity.md).
	FlagSubstitute = "substitute"
)

// Status jurnal (DERIVASI, bukan kolom — docs/06-teaching.md).
const (
	JournalOngoing = "ongoing"
	JournalDone    = "done"
)

// Status monitoring per slot (DERIVASI — docs/06-teaching.md "Status mengajar").
const (
	StatusMengajar   = "mengajar"
	StatusBelumMasuk = "belum_masuk"
	StatusIzin       = "izin"
	StatusBelumMulai = "belum_mulai"
	StatusSelesai    = "selesai"
)

// Date adalah tanggal murni (tanpa jam), dikodekan JSON sebagai "YYYY-MM-DD"
// — pola yang sama dengan internal/attendance.Date / internal/student.Date
// (didefinisikan ulang di sini supaya teaching tidak mengimpor modul lain
// untuk tipe ini, lihat CLAUDE.md).
type Date struct{ time.Time }

func NewDate(t time.Time) Date { return Date{t} }

func (d Date) MarshalJSON() ([]byte, error) {
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

type TeacherRef struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type ClassRef struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type SubjectRef struct {
	ID   int64  `json:"id"`
	Code string `json:"code"`
	Name string `json:"name"`
}

type RoomRef struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// JournalView adalah shape response jurnal mengajar (docs/06-teaching.md).
// Status DIHITUNG saat baca (lihat service.go deriveJournalStatus) — bukan
// kolom tersimpan.
type JournalView struct {
	ID             int64       `json:"id"`
	Teacher        TeacherRef  `json:"teacher"`
	Class          ClassRef    `json:"class"`
	Subject        *SubjectRef `json:"subject"`
	Room           *RoomRef    `json:"room"`
	ScheduleSlotID *int64      `json:"schedule_slot_id"`
	Date           Date        `json:"date"`
	StartedAt      time.Time   `json:"started_at"`
	EndedAt        *time.Time  `json:"ended_at"`
	Material       string      `json:"material"`
	Note           string      `json:"note"`
	Flags          []string    `json:"flags"`
	Status         string      `json:"status"`
}

// ScanResult adalah shape response POST /api/teaching/scan.
type ScanResult struct {
	Journal             *JournalView `json:"journal"`
	AttendanceSessionID *int64       `json:"attendance_session_id"`
	NeedsManual         bool         `json:"needs_manual"`
	Room                *RoomRef     `json:"room,omitempty"`
}

// StatusRow adalah satu baris response GET /api/teaching/status.
type StatusRow struct {
	Slot       StatusSlot `json:"slot"`
	Teacher    TeacherRef `json:"teacher"`
	Status     string     `json:"status"`
	JournalID  *int64     `json:"journal_id"`
	RoomActual *RoomRef   `json:"room_actual"`
}

// StatusSlot adalah potongan info slot jadwal disematkan pada StatusRow
// (field schedule "...schedule fields" per docs/06 — class/subject/room
// terjadwal, hari, jam ke-, jam lokal).
type StatusSlot struct {
	ID             int64      `json:"id"`
	Class          ClassRef   `json:"class"`
	Subject        SubjectRef `json:"subject"`
	Room           *RoomRef   `json:"room"`
	DayOfWeek      int        `json:"day_of_week"`
	PeriodStart    int        `json:"period_start"`
	PeriodEnd      int        `json:"period_end"`
	PeriodStartsAt string     `json:"period_starts_at"`
	PeriodEndsAt   string     `json:"period_ends_at"`
}

// CurrentPeriodInfo — potongan "jam ke berapa sekarang" disematkan pada
// StatusResult (docs/06: dipakai dashboard TV/kepsek).
type CurrentPeriodInfo struct {
	Number   int    `json:"number"`
	StartsAt string `json:"starts_at"`
	EndsAt   string `json:"ends_at"`
}

// StatusSummary adalah ringkasan jumlah per status (docs/06 "Sertakan juga
// summary").
type StatusSummary struct {
	Mengajar   int `json:"mengajar"`
	BelumMasuk int `json:"belum_masuk"`
	Izin       int `json:"izin"`
	BelumMulai int `json:"belum_mulai"`
	Selesai    int `json:"selesai"`
}

// StatusResult adalah shape response GET /api/teaching/status.
type StatusResult struct {
	CurrentPeriod *CurrentPeriodInfo `json:"current_period"`
	Summary       StatusSummary      `json:"summary"`
	Rows          []StatusRow        `json:"rows"`
}

// ComplianceView adalah satu baris response GET /api/teaching/compliance
// (fase 7, docs/06-teaching.md "Dashboard kepala sekolah": rekap ketertiban
// mengajar — % slot terlaksana per guru pada rentang tanggal).
type ComplianceView struct {
	Teacher        TeacherRef `json:"teacher"`
	Scheduled      int        `json:"scheduled"`
	Taught         int        `json:"taught"`
	Pct            float64    `json:"pct"`
	Unscheduled    int        `json:"unscheduled"`
	MaterialFilled int        `json:"material_filled"`
	MaterialPct    float64    `json:"material_pct"`
}
