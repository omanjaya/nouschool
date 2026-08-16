// dashboardadapter.go — HANYA wiring (sama seperti scheduleadapter.go):
// menjembatani *teaching.Service, *attendance.Service, *announcement.Service,
// dan *schedule.Service ke consumer-side interface yang dideklarasikan modul
// dashboard (fase 7, docs/06-teaching.md "Dashboard TV ruang guru" &
// "Dashboard kepala sekolah").
//
// Kenapa perlu adapter (bukan struct producer langsung memenuhi interface
// secara struktural)? Sama seperti catatan di scheduleadapter.go: signature
// interface consumer-side HANYA bisa dipenuhi struktural tanpa adapter kalau
// tipe kembaliannya primitif MURNI (int64/string/bool/error) atau slice
// primitif. Di sini producer mengembalikan struct BERNAMA milik package
// masing-masing (teaching.StatusResult, attendance.ClassSummary,
// announcement.BoardItem, schedule.CurrentPeriodView) — walau field-nya
// sama persis dengan tipe lokal dashboard, Go TIDAK menganggap dua tipe
// struct bernama berbeda sebagai sama untuk kepuasan interface. Satu-satunya
// tempat yang boleh mengimpor dashboard SEKALIGUS teaching/attendance/
// announcement/schedule untuk mengonversi adalah cmd/server (CLAUDE.md
// "wiring eksplisit di main.go").
package main

import (
	"context"
	"time"

	"github.com/omanjaya/nouschool/internal/announcement"
	"github.com/omanjaya/nouschool/internal/attendance"
	"github.com/omanjaya/nouschool/internal/dashboard"
	"github.com/omanjaya/nouschool/internal/schedule"
	"github.com/omanjaya/nouschool/internal/teaching"
)

// -- teachingForDashboard: memenuhi dashboard.TeachingGateway --

type teachingForDashboard struct {
	svc *teaching.Service
}

func (a teachingForDashboard) Status(ctx context.Context, schoolID int64, dateStr string) (dashboard.TeachingStatusResult, error) {
	result, err := a.svc.Status(ctx, schoolID, dateStr)
	if err != nil {
		return dashboard.TeachingStatusResult{}, err
	}
	rows := make([]dashboard.TeachingSlot, 0, len(result.Rows))
	for _, row := range result.Rows {
		roomName := ""
		if row.RoomActual != nil {
			roomName = row.RoomActual.Name
		} else if row.Slot.Room != nil {
			roomName = row.Slot.Room.Name
		}
		rows = append(rows, dashboard.TeachingSlot{
			ClassID: row.Slot.Class.ID, ClassName: row.Slot.Class.Name,
			SubjectCode: row.Slot.Subject.Code, SubjectName: row.Slot.Subject.Name,
			TeacherID: row.Teacher.ID, TeacherName: row.Teacher.Name, RoomName: roomName,
			Status: row.Status, PeriodStartsAt: row.Slot.PeriodStartsAt, PeriodEndsAt: row.Slot.PeriodEndsAt,
		})
	}
	return dashboard.TeachingStatusResult{
		Summary: dashboard.TeachingSummary{
			Mengajar: result.Summary.Mengajar, BelumMasuk: result.Summary.BelumMasuk, Izin: result.Summary.Izin,
			BelumMulai: result.Summary.BelumMulai, Selesai: result.Summary.Selesai,
		},
		Rows: rows,
	}, nil
}

// -- attendanceForDashboard: memenuhi dashboard.AttendanceGateway --

type attendanceForDashboard struct {
	svc *attendance.Service
}

// Summary — classID=0: SEMUA kelas (docs/06 "Rekap absensi siswa hari ini"
// per kelas, bukan satu kelas tertentu).
func (a attendanceForDashboard) Summary(ctx context.Context, schoolID int64, dateStr string) ([]dashboard.ClassAttendanceSummary, error) {
	rows, err := a.svc.Summary(ctx, schoolID, dateStr, 0)
	if err != nil {
		return nil, err
	}
	out := make([]dashboard.ClassAttendanceSummary, 0, len(rows))
	for _, r := range rows {
		out = append(out, dashboard.ClassAttendanceSummary{
			ClassID: r.ClassID, ClassName: r.ClassName, Total: r.Total, Hadir: r.Hadir,
			Terlambat: r.Terlambat, Izin: r.Izin, Sakit: r.Sakit, Alpa: r.Alpa,
			Unmarked: r.Unmarked, SessionStatus: r.SessionStatus,
		})
	}
	return out, nil
}

// -- announcementForDashboard: memenuhi dashboard.AnnouncementGateway --

type announcementForDashboard struct {
	svc *announcement.Service
}

func (a announcementForDashboard) ActiveOn(ctx context.Context, schoolID int64, dateStr string) ([]dashboard.AnnouncementItem, error) {
	items, err := a.svc.ActiveOn(ctx, schoolID, dateStr)
	if err != nil {
		return nil, err
	}
	out := make([]dashboard.AnnouncementItem, 0, len(items))
	for _, it := range items {
		out = append(out, dashboard.AnnouncementItem{ID: it.ID, Title: it.Title, Body: it.Body, IsPlatform: it.IsPlatform})
	}
	return out, nil
}

// -- scheduleForDashboard: memenuhi dashboard.ScheduleGateway --

type scheduleForDashboard struct {
	svc *schedule.Service
}

func (a scheduleForDashboard) CurrentPeriod(ctx context.Context, schoolID int64, at time.Time) (int, string, string, bool, string, bool, error) {
	cp, err := a.svc.CurrentPeriod(ctx, schoolID, at)
	if err != nil {
		return 0, "", "", false, "", false, err
	}
	if cp.Period != nil {
		return cp.Period.Number, cp.Period.StartsAt, cp.Period.EndsAt, true, "", false, nil
	}
	if cp.NextStartsAt != nil {
		return 0, "", "", false, *cp.NextStartsAt, true, nil
	}
	return 0, "", "", false, "", false, nil
}
