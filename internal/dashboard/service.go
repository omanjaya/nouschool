package dashboard

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/omanjaya/nouschool/internal/platform/clock"
	"github.com/omanjaya/nouschool/internal/platform/reqctx"
)

// Permission kanonik dipakai modul dashboard (nilai HARUS sama persis dengan
// docs/02-identity.md — didefinisikan ulang di sini karena dashboard TIDAK
// boleh mengimpor identity, lihat CLAUDE.md).
const PermTeachingMonitor = "teaching:monitor"

// -- consumer-side interfaces (lihat CLAUDE.md: antar-modul lewat interface
// kecil dideklarasikan di sisi PEMAKAI, di-inject lewat constructor di
// main.go). Signature primitif kecuali beberapa struct BUTUH data lebih kaya
// (mirip catatan di internal/teaching.ScheduleGateway) — tipe struct itu
// didefinisikan LOKAL di modul ini (bukan diimpor dari teaching/attendance/
// announcement/schedule) supaya dashboard TIDAK mengimpor package-package
// itu; kepuasan interface oleh *teaching.Service / *attendance.Service /
// *announcement.Service / *schedule.Service dijembatani lewat adapter kecil
// di cmd/server/dashboardadapter.go (satu-satunya tempat yang boleh
// mengimpor keempatnya sekaligus).

// TeachingSlot adalah potongan data satu baris status mengajar (dari
// teaching.StatusResult.Rows) yang dibutuhkan dashboard — primitif saja.
type TeachingSlot struct {
	ClassID        int64
	ClassName      string
	SubjectCode    string
	SubjectName    string
	TeacherID      int64
	TeacherName    string
	RoomName       string // "" = belum ada ruang aktual
	Status         string
	PeriodStartsAt string
	PeriodEndsAt   string
}

// TeachingStatusResult adalah kebutuhan dashboard dari GET /api/teaching/status
// (docs/06-teaching.md "Status guru mengajar"): summary + SEMUA baris hari
// itu (dashboard sendiri yang memfilah current/upcoming dari PeriodStartsAt/
// PeriodEndsAt masing-masing baris — TIDAK menduplikasi derivasi status,
// hanya menyaring apa yang sudah diderivasi teaching.Service.Status).
type TeachingStatusResult struct {
	Summary TeachingSummary
	Rows    []TeachingSlot
}

// TeachingGateway adalah kebutuhan modul dashboard dari modul teaching.
type TeachingGateway interface {
	Status(ctx context.Context, schoolID int64, dateStr string) (TeachingStatusResult, error)
}

// ClassAttendanceSummary adalah kebutuhan dashboard dari GET /api/attendance/summary.
type ClassAttendanceSummary struct {
	ClassID       int64
	ClassName     string
	Total         int64
	Hadir         int64
	Terlambat     int64
	Izin          int64
	Sakit         int64
	Alpa          int64
	Unmarked      int64
	SessionStatus string
}

// AttendanceGateway adalah kebutuhan modul dashboard dari modul attendance:
// rekap absensi siswa HARI INI per kelas (docs/06 "Rekap absensi siswa hari ini").
type AttendanceGateway interface {
	Summary(ctx context.Context, schoolID int64, dateStr string) ([]ClassAttendanceSummary, error)
}

// AnnouncementGateway adalah kebutuhan modul dashboard dari modul
// announcement: pengumuman aktif pada tanggal tertentu (docs/06 "Pengumuman").
type AnnouncementGateway interface {
	ActiveOn(ctx context.Context, schoolID int64, dateStr string) ([]AnnouncementItem, error)
}

// ScheduleGateway adalah kebutuhan modul dashboard dari modul schedule: jam
// pelajaran ke berapa sekarang + jam mulai berikutnya (docs/06 "Jam &
// pergantian", dipakai bareng CurrentPeriod dari internal/schedule).
type ScheduleGateway interface {
	CurrentPeriod(ctx context.Context, schoolID int64, at time.Time) (periodNumber int, startsAt, endsAt string, hasCurrent bool, nextStartsAt string, hasNext bool, err error)
}

// Service menyusun payload dashboard TV/kepsek dari service modul lain.
type Service struct {
	teaching      TeachingGateway
	attendance    AttendanceGateway
	announcements AnnouncementGateway
	schedule      ScheduleGateway
	clock         clock.Clock
}

func NewService(teaching TeachingGateway, attendance AttendanceGateway, announcements AnnouncementGateway, schedule ScheduleGateway, clk clock.Clock) *Service {
	if clk == nil {
		clk = clock.System{}
	}
	return &Service{teaching: teaching, attendance: attendance, announcements: announcements, schedule: schedule, clock: clk}
}

// newServiceForTest membangun Service dengan gateway FAKE — dipakai test di
// package ini saja (service_test.go).
func newServiceForTest(teaching TeachingGateway, attendance AttendanceGateway, announcements AnnouncementGateway, schedule ScheduleGateway, clk clock.Clock) *Service {
	return &Service{teaching: teaching, attendance: attendance, announcements: announcements, schedule: schedule, clock: clk}
}

func schoolTimezone(ctx context.Context) string {
	sch, _ := reqctx.SchoolFromContext(ctx)
	return sch.Timezone
}

// indoDayNames & indoMonthNames — nama hari/bulan Indonesia dipakai day_label
// (mis. "Minggu, 16 Agu 2026"). Redefinisi lokal (bukan impor dari
// internal/schedule) — pola yang sama dengan Date/clockMinutes di modul lain
// (CLAUDE.md: modul tidak saling impor untuk hal sekecil ini).
var indoDayNames = map[time.Weekday]string{
	time.Sunday: "Minggu", time.Monday: "Senin", time.Tuesday: "Selasa", time.Wednesday: "Rabu",
	time.Thursday: "Kamis", time.Friday: "Jumat", time.Saturday: "Sabtu",
}

var indoMonthNames = [...]string{"Jan", "Feb", "Mar", "Apr", "Mei", "Jun", "Jul", "Agu", "Sep", "Okt", "Nov", "Des"}

func dayLabel(t time.Time) string {
	return fmt.Sprintf("%s, %d %s %d", indoDayNames[t.Weekday()], t.Day(), indoMonthNames[t.Month()-1], t.Year())
}

// clockMinutes mengonversi "HH:MM" -> menit sejak tengah malam (redefinisi
// lokal — sama seperti internal/teaching, dashboard TIDAK mengimpor modul lain).
func clockMinutes(s string) int {
	t, err := time.Parse("15:04", strings.TrimSpace(s))
	if err != nil {
		return -1
	}
	return t.Hour()*60 + t.Minute()
}

func toTeachingItem(row TeachingSlot) TeachingItem {
	return TeachingItem{
		ClassID: row.ClassID, ClassName: row.ClassName, SubjectCode: row.SubjectCode, SubjectName: row.SubjectName,
		TeacherID: row.TeacherID, TeacherName: row.TeacherName, RoomName: row.RoomName, Status: row.Status,
		PeriodStartsAt: row.PeriodStartsAt, PeriodEndsAt: row.PeriodEndsAt,
	}
}

// splitCurrentUpcoming memilah baris status mengajar hari ini menjadi
// "current" (slot jam BERJALAN — now di antara period_starts_at..ends_at) dan
// "upcoming" (SEMUA baris pada slot jam BERIKUTNYA — starts_at minimal di
// antara yang belum mulai, docs/06: "current":[items slot jam BERJALAN
// saja],"upcoming":[items slot jam berikutnya]).
func splitCurrentUpcoming(rows []TeachingSlot, nowMin int) (current, upcoming []TeachingItem) {
	nextStart := -1
	for _, row := range rows {
		start := clockMinutes(row.PeriodStartsAt)
		end := clockMinutes(row.PeriodEndsAt)
		if start < 0 || end < 0 {
			continue
		}
		if nowMin >= start && nowMin <= end {
			current = append(current, toTeachingItem(row))
			continue
		}
		if start > nowMin && (nextStart == -1 || start < nextStart) {
			nextStart = start
		}
	}
	if nextStart != -1 {
		for _, row := range rows {
			if clockMinutes(row.PeriodStartsAt) == nextStart {
				upcoming = append(upcoming, toTeachingItem(row))
			}
		}
	}
	if current == nil {
		current = []TeachingItem{}
	}
	if upcoming == nil {
		upcoming = []TeachingItem{}
	}
	return current, upcoming
}

// Board — GET /api/tv/board: payload gabungan SATU fetch (docs/06-teaching.md).
// Otorisasi (teaching:monitor — display/kepsek/admin) ditegakkan di route
// level (requirePerm), lihat routes.go.
func (s *Service) Board(ctx context.Context, schoolID int64) (Board, error) {
	now := s.clock.Now()
	tz := schoolTimezone(ctx)
	local := clock.InZone(now, tz)
	dateStr := local.Format("2006-01-02")
	nowMin := local.Hour()*60 + local.Minute()

	sch, _ := reqctx.SchoolFromContext(ctx)

	num, startsAt, endsAt, hasCurrent, nextStartsAt, hasNext, err := s.schedule.CurrentPeriod(ctx, schoolID, now)
	if err != nil {
		return Board{}, err
	}
	var cp *CurrentPeriodInfo
	if hasCurrent {
		cp = &CurrentPeriodInfo{Number: num, StartsAt: startsAt, EndsAt: endsAt}
	}
	var next *string
	if hasNext {
		v := nextStartsAt
		next = &v
	}

	teachingStatus, err := s.teaching.Status(ctx, schoolID, dateStr)
	if err != nil {
		return Board{}, err
	}
	current, upcoming := splitCurrentUpcoming(teachingStatus.Rows, nowMin)

	attendanceRows, err := s.attendance.Summary(ctx, schoolID, dateStr)
	if err != nil {
		return Board{}, err
	}
	attendance := make([]AttendanceRow, 0, len(attendanceRows))
	for _, r := range attendanceRows {
		attendance = append(attendance, AttendanceRow{
			ClassID: r.ClassID, ClassName: r.ClassName, Total: r.Total, Hadir: r.Hadir,
			Terlambat: r.Terlambat, Izin: r.Izin, Sakit: r.Sakit, Alpa: r.Alpa,
			Unmarked: r.Unmarked, SessionStatus: r.SessionStatus,
		})
	}

	announcements, err := s.announcements.ActiveOn(ctx, schoolID, dateStr)
	if err != nil {
		return Board{}, err
	}
	if announcements == nil {
		announcements = []AnnouncementItem{}
	}

	return Board{
		School: SchoolInfo{Name: sch.Name},
		Now: NowInfo{
			Date: dateStr, DayLabel: dayLabel(local), Time: local.Format("15:04"),
			CurrentPeriod: cp, NextStartsAt: next,
		},
		Teaching:      TeachingBoard{Summary: teachingStatus.Summary, Current: current, Upcoming: upcoming},
		Attendance:    attendance,
		Announcements: announcements,
		GeneratedAt:   now,
	}, nil
}
