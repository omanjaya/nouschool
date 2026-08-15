// Package schedule mengurus jadwal pelajaran: periods (definisi jam ke-),
// rooms (ruang fisik + QR), dan schedule_slots (kelas x mapel x guru x hari x
// jam) — termasuk deteksi bentrok, copy jadwal antar kelas, import Excel/CSV,
// dan query kunci dipakai modul lain (SlotNow/SlotsToday/CurrentPeriod).
// Lihat docs/04-schedule.md.
package schedule

import (
	"fmt"
	"strings"
	"time"
)

// Role kanonik dipakai modul schedule (nilai HARUS sama persis dengan
// internal/identity — didefinisikan ulang di sini karena schedule TIDAK
// boleh mengimpor identity, lihat CLAUDE.md).
const (
	RoleSiswa = "siswa"
	RoleGuru  = "guru"
)

// Permission kanonik dipakai modul schedule (lihat docs/02-identity.md).
const (
	PermScheduleRead   = "schedule:read"
	PermScheduleManage = "schedule:manage"
)

// dayNames — 1=Senin ... 6=Sabtu (docs/04-schedule.md). 0=Minggu DITAMBAHKAN
// fase 6 (docs/06-teaching.md verifikasi e2e): beberapa sekolah punya kelas
// akselerasi/ekstrakurikuler Minggu, dan modul teaching butuh kemampuan
// menyeed slot pada HARI INI apa pun harinya untuk verifikasi live —
// keputusan diotorisasi eksplisit oleh scope kerja fase 6 (bukan penyimpangan
// diam-diam). Nilai selaras dengan Go time.Weekday (Minggu=0 ... Sabtu=6).
var dayNames = map[int]string{
	0: "Minggu", 1: "Senin", 2: "Selasa", 3: "Rabu", 4: "Kamis", 5: "Jumat", 6: "Sabtu",
}

func dayName(d int) string {
	if n, ok := dayNames[d]; ok {
		return n
	}
	return fmt.Sprintf("hari-%d", d)
}

// dayNameToNumber menerima nama hari (case-insensitive, "Senin".."Sabtu")
// atau angka "1".."6" — dipakai parser import (docs/04: kolom "hari").
func dayNameToNumber(raw string) (int, bool) {
	raw = strings.TrimSpace(raw)
	for n, name := range dayNames {
		if strings.EqualFold(name, raw) {
			return n, true
		}
	}
	switch raw {
	case "0", "1", "2", "3", "4", "5", "6":
		return int(raw[0] - '0'), true
	default:
		return 0, false
	}
}

// clockMinutes mengonversi "HH:MM" -> menit sejak tengah malam. Dipakai
// domain & repository (bukan cuma parser import) supaya perbandingan overlap
// periode sederhana (bilangan bulat, bukan string).
func clockMinutes(s string) (int, error) {
	t, err := time.Parse("15:04", strings.TrimSpace(s))
	if err != nil {
		return 0, fmt.Errorf("format jam harus HH:MM: %q", s)
	}
	return t.Hour()*60 + t.Minute(), nil
}

func minutesToClock(m int) string {
	if m < 0 {
		m = 0
	}
	return fmt.Sprintf("%02d:%02d", m/60, m%60)
}

// -- response shapes --

// PeriodView adalah shape satu baris response GET/PUT /api/periods.
type PeriodView struct {
	ID       int64  `json:"id"`
	Number   int    `json:"number"`
	StartsAt string `json:"starts_at"`
	EndsAt   string `json:"ends_at"`
	Label    string `json:"label,omitempty"`
}

// RoomView adalah shape satu baris response GET /api/rooms. QRToken hanya
// diisi untuk requester dengan permission schedule:manage (lihat service.go).
type RoomView struct {
	ID      int64   `json:"id"`
	Name    string  `json:"name"`
	QRToken *string `json:"qr_token,omitempty"`
}

// ClassRef / SubjectRef / TeacherRef / RoomRef — potongan info disematkan
// pada SlotView.
type ClassRef struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type SubjectRef struct {
	ID   int64  `json:"id"`
	Code string `json:"code"`
	Name string `json:"name"`
}

type TeacherRef struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type RoomRef struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// SlotView adalah shape satu baris response GET /api/schedule/slots dan hasil
// POST/PATCH satu slot.
type SlotView struct {
	ID          int64      `json:"id"`
	Class       ClassRef   `json:"class"`
	Subject     SubjectRef `json:"subject"`
	Teacher     TeacherRef `json:"teacher"`
	Room        *RoomRef   `json:"room"`
	DayOfWeek   int        `json:"day_of_week"`
	PeriodStart int        `json:"period_start"`
	PeriodEnd   int        `json:"period_end"`
}

// TodaySlotView adalah SlotView + penanda "sedang berlangsung sekarang" —
// GET /api/schedule/today (docs/04: dipakai jurnal mengajar & TV fase 6/7).
type TodaySlotView struct {
	SlotView
	IsNow bool `json:"is_now"`
}

// CurrentPeriodView adalah shape response GET /api/schedule/current-period.
type CurrentPeriodView struct {
	Period       *CurrentPeriodInfo `json:"period"`
	NextStartsAt *string            `json:"next_starts_at"`
}

type CurrentPeriodInfo struct {
	Number   int    `json:"number"`
	StartsAt string `json:"starts_at"`
	EndsAt   string `json:"ends_at"`
}

// CopyResult adalah shape response POST /api/schedule/copy.
type CopyResult struct {
	Copied  int            `json:"copied"`
	Skipped []CopySkipItem `json:"skipped"`
}

type CopySkipItem struct {
	Subject string `json:"subject"`
	Reason  string `json:"reason"`
}
