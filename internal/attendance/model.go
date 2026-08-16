// Package attendance mengurus absensi siswa mode daily: sesi per rombel per
// hari, record per siswa, bulk input manual, jendela edit, finalisasi,
// rekap harian, dan riwayat per siswa (lihat docs/05-attendance.md). Mode
// per_subject (sesi dari slot jadwal) dibangun di Fase 6.
package attendance

import (
	"fmt"
	"strings"
	"time"
)

// Status kanonik attendance_records.status.
const (
	StatusHadir     = "hadir"
	StatusTerlambat = "terlambat"
	StatusIzin      = "izin"
	StatusSakit     = "sakit"
	StatusAlpa      = "alpa"
)

func validStatus(s string) bool {
	switch s {
	case StatusHadir, StatusTerlambat, StatusIzin, StatusSakit, StatusAlpa:
		return true
	default:
		return false
	}
}

// Type kanonik attendance_sessions.type. Hanya TypeDaily dipakai Fase 3.
const (
	TypeDaily   = "daily"
	TypeSubject = "subject"
)

// Status kanonik attendance_sessions.status.
const (
	SessionOpen      = "open"
	SessionFinalized = "finalized"
)

// RoleAdminSekolah — nilai kanonik role admin sekolah (harus sama persis
// dengan internal/identity.RoleAdminSekolah — didefinisikan ulang di sini
// karena attendance TIDAK boleh mengimpor identity, lihat CLAUDE.md).
const RoleAdminSekolah = "admin_sekolah"

// RoleKepalaSekolah / RoleDisplay — nilai kanonik (harus sama persis dengan
// internal/identity) dipakai targeting event realtime "attendance.summary"
// (Fase 12, lihat SetRealtime di service.go).
const (
	RoleKepalaSekolah = "kepala_sekolah"
	RoleDisplay       = "display"
)

// Date adalah tanggal murni (tanpa jam), dikodekan JSON sebagai "YYYY-MM-DD".
// Sama seperti internal/student.Date / internal/tenant.Date — didefinisikan
// ulang di sini supaya attendance TIDAK mengimpor package lain untuk tipe ini.
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

// ClassAttendanceStatus adalah potongan status sesi disematkan pada
// ClassForDate (null bila belum ada sesi dibuka untuk tanggal itu).
type ClassAttendanceStatus struct {
	ID          int64  `json:"id"`
	Status      string `json:"status"`
	MarkedCount int64  `json:"marked_count"`
}

// ClassForDate adalah shape response GET /api/attendance/classes.
type ClassForDate struct {
	ClassID      int64                  `json:"class_id"`
	Name         string                 `json:"name"`
	StudentCount int64                  `json:"student_count"`
	Session      *ClassAttendanceStatus `json:"session"`
}

// RecordView adalah shape record disematkan pada StudentInSession.
type RecordView struct {
	Status   string    `json:"status"`
	Note     string    `json:"note"`
	Method   string    `json:"method"`
	MarkedAt time.Time `json:"marked_at"`
}

// StudentInSession adalah satu baris siswa di dalam sesi (roster + record bila ada).
type StudentInSession struct {
	StudentID int64       `json:"student_id"`
	Name      string      `json:"name"`
	NIS       string      `json:"nis"`
	Record    *RecordView `json:"record"`
}

// SessionView adalah shape sesi (bagian dari SessionDetail).
type SessionView struct {
	ID           int64  `json:"id"`
	ClassID      int64  `json:"class_id"`
	ClassName    string `json:"class_name"`
	Date         Date   `json:"date"`
	Type         string `json:"type"`
	Status       string `json:"status"`
	OpenedByName string `json:"opened_by_name"`
}

// SessionDetail adalah shape response POST /api/attendance/sessions dan
// GET /api/attendance/sessions/{id}.
type SessionDetail struct {
	Session  SessionView        `json:"session"`
	Students []StudentInSession `json:"students"`
}

// FinalizeResult adalah shape response POST /api/attendance/sessions/{id}/finalize.
type FinalizeResult struct {
	Session       SessionView `json:"session"`
	UnmarkedCount int64       `json:"unmarked_count"`
}

// ClassSummary adalah shape satu baris response GET /api/attendance/summary.
type ClassSummary struct {
	ClassID       int64  `json:"class_id"`
	ClassName     string `json:"class_name"`
	Total         int64  `json:"total"`
	Hadir         int64  `json:"hadir"`
	Terlambat     int64  `json:"terlambat"`
	Izin          int64  `json:"izin"`
	Sakit         int64  `json:"sakit"`
	Alpa          int64  `json:"alpa"`
	Unmarked      int64  `json:"unmarked"`
	SessionStatus string `json:"session_status"` // "" bila belum ada sesi dibuka
}

// HistoryItem adalah satu baris riwayat absensi siswa.
type HistoryItem struct {
	Date   Date   `json:"date"`
	Status string `json:"status"`
	Note   string `json:"note"`
}

// HistoryCounts adalah ringkasan jumlah per status pada rentang riwayat.
type HistoryCounts struct {
	Hadir     int64 `json:"hadir"`
	Terlambat int64 `json:"terlambat"`
	Izin      int64 `json:"izin"`
	Sakit     int64 `json:"sakit"`
	Alpa      int64 `json:"alpa"`
}

// HistoryResult adalah shape response GET /api/students/{id}/attendance.
type HistoryResult struct {
	Counts HistoryCounts `json:"counts"`
	Items  []HistoryItem `json:"items"`
}

// -- kalender presensi siswa (Fase 14 Gelombang D, docs/12-sion-parity.md) --

// CalendarDay adalah satu baris response GET
// /api/students/{id}/attendance/calendar — Status/Note nil bila TIDAK ADA
// record sama sekali pada tanggal itu (hari libur/belum diabsen).
type CalendarDay struct {
	Date         Date    `json:"date"`
	Status       *string `json:"status"`
	Note         *string `json:"note"`
	SessionCount int64   `json:"session_count"`
}

// CalendarResult adalah shape response GET
// /api/students/{id}/attendance/calendar.
type CalendarResult struct {
	Days   []CalendarDay `json:"days"`
	Counts HistoryCounts `json:"counts"`
}

// -- mode per_subject (fase 6, docs/05-attendance.md & docs/06-teaching.md) --

// SlotRef adalah potongan info kelas/mapel disematkan pada SlotTodayView.
type SlotClassRef struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type SlotSubjectRef struct {
	ID   int64  `json:"id"`
	Code string `json:"code"`
	Name string `json:"name"`
}

// SlotTodaySlot adalah potongan info slot jadwal disematkan pada
// SlotTodayView (docs/06: dipakai GET /api/attendance/slots-today).
type SlotTodaySlot struct {
	ID          int64          `json:"id"`
	Class       SlotClassRef   `json:"class"`
	Subject     SlotSubjectRef `json:"subject"`
	PeriodStart int            `json:"period_start"`
	PeriodEnd   int            `json:"period_end"`
	StartsAt    string         `json:"starts_at"`
	EndsAt      string         `json:"ends_at"`
	IsNow       bool           `json:"is_now"`
}

// SlotTodaySession adalah status sesi absen subject utk satu slot (null bila
// belum dibuka).
type SlotTodaySession struct {
	ID          int64  `json:"id"`
	Status      string `json:"status"`
	MarkedCount int64  `json:"marked_count"`
	Total       int64  `json:"total"`
}

// SlotTodayView adalah satu baris response GET /api/attendance/slots-today.
type SlotTodayView struct {
	Slot    SlotTodaySlot     `json:"slot"`
	Session *SlotTodaySession `json:"session"`
}

// -- QR kartu siswa (Fase 8, docs/05-attendance.md "QR kartu siswa") --

// QRCardView adalah satu baris response GET/POST /api/attendance/qr-cards*.
type QRCardView struct {
	StudentID int64  `json:"student_id"`
	Name      string `json:"name"`
	NIS       string `json:"nis"`
	Token     string `json:"token"`
}

// ScanResult adalah shape response POST /api/attendance/sessions/{id}/scan.
type ScanResult struct {
	StudentID     int64  `json:"student_id"`
	Name          string `json:"name"`
	NIS           string `json:"nis"`
	Status        string `json:"status"`
	AlreadyMarked bool   `json:"already_marked"`
}

// -- Self check-in siswa (Fase 8, docs/05-attendance.md "Self check-in siswa") --

// SelfCheckinWindow adalah potongan jendela waktu disematkan pada
// SelfCheckinStatusView.
type SelfCheckinWindow struct {
	OpenFrom string `json:"open_from"`
	CloseAt  string `json:"close_at"`
}

// SelfCheckinToday adalah status check-in siswa HARI INI (sesi daily
// rombelnya), null bila belum ada record apa pun untuk hari ini.
type SelfCheckinToday struct {
	Status    string    `json:"status"`
	CheckedAt time.Time `json:"checked_at"`
}

// SelfCheckinStatusView adalah shape response GET /api/attendance/self-checkin/status.
type SelfCheckinStatusView struct {
	Enabled   bool               `json:"enabled"`
	Window    *SelfCheckinWindow `json:"window"`
	LateAfter *string            `json:"late_after"`
	Today     *SelfCheckinToday  `json:"today"`
}

// SelfCheckinResult adalah shape response POST /api/attendance/self-checkin.
type SelfCheckinResult struct {
	Status    string    `json:"status"`
	CheckedAt time.Time `json:"checked_at"`
}

// AnomalyItem adalah satu baris response GET /api/attendance/anomalies —
// alat bantu deteksi (docs/05: "BUKAN anti-curang"), keputusan akhir tetap
// guru/wali kelas.
type AnomalyItem struct {
	StudentID int64  `json:"student_id"`
	Name      string `json:"name"`
	ClassName string `json:"class_name"`
	Issue     string `json:"issue"` // "same_location" | "low_accuracy"
	Detail    string `json:"detail"`
}
