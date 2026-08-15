// scheduleadapter.go — HANYA wiring (sama seperti main.go): menjembatani
// *schedule.Service ke consumer-side interface yang dideklarasikan modul
// teaching & attendance (fase 6, docs/06-teaching.md).
//
// Kenapa perlu adapter di sini (bukan struct *schedule.Service langsung
// memenuhi interface secara struktural seperti modul lain)? Aturan CLAUDE.md
// "interface kecil primitif di sisi pemakai" bekerja tanpa adapter SELAMA
// signature-nya murni primitif (int64/string/bool/error) — itu sebabnya
// schedule.StudentClassLookup, leave.ApprovedOn, dst TIDAK butuh adapter.
// teaching & attendance di fase 6 butuh data lebih kaya (nama kelas/mapel/
// guru/ruang, jam mulai-selesai) yang di schedule direpresentasikan sebagai
// SlotView/TodaySlotView/PeriodView — tipe MILIK PACKAGE schedule. Consumer
// (teaching/attendance) mendeklarasikan tipe LOKAL mereka sendiri (SlotInfo/
// SlotToday) supaya TIDAK mengimpor schedule sama sekali; satu-satunya
// tempat yang boleh mengimpor kedua sisi sekaligus untuk mengonversi adalah
// main.go — makanya adapter ini tinggal di cmd/server, bukan di internal/.
package main

import (
	"context"
	"time"

	"github.com/omanjaya/nouschool/internal/attendance"
	"github.com/omanjaya/nouschool/internal/schedule"
	"github.com/omanjaya/nouschool/internal/teaching"
)

// -- scheduleForTeaching: memenuhi teaching.ScheduleGateway --

type scheduleForTeaching struct {
	svc *schedule.Service
}

// periodClocks mengembalikan map "jam ke-" -> jam lokal (StartsAt/EndsAt),
// dipakai mengisi SlotInfo.PeriodStartsAt/PeriodEndsAt (SlotView schedule
// hanya punya nomor period, bukan jamnya — lihat internal/schedule/model.go).
func periodClocks(ctx context.Context, svc *schedule.Service, schoolID int64) (map[int]schedule.PeriodView, error) {
	periods, err := svc.ListPeriods(ctx, schoolID)
	if err != nil {
		return nil, err
	}
	m := make(map[int]schedule.PeriodView, len(periods))
	for _, p := range periods {
		m[p.Number] = p
	}
	return m, nil
}

func toSlotInfo(sv schedule.SlotView, periods map[int]schedule.PeriodView) teaching.SlotInfo {
	info := teaching.SlotInfo{
		ID: sv.ID, ClassID: sv.Class.ID, ClassName: sv.Class.Name,
		SubjectID: sv.Subject.ID, SubjectCode: sv.Subject.Code, SubjectName: sv.Subject.Name,
		TeacherID: sv.Teacher.ID, TeacherName: sv.Teacher.Name,
		DayOfWeek: sv.DayOfWeek, PeriodStart: sv.PeriodStart, PeriodEnd: sv.PeriodEnd,
		PeriodStartsAt: periods[sv.PeriodStart].StartsAt, PeriodEndsAt: periods[sv.PeriodEnd].EndsAt,
	}
	if sv.Room != nil {
		info.RoomID = sv.Room.ID
		info.RoomName = sv.Room.Name
	}
	return info
}

func (a scheduleForTeaching) SlotNow(ctx context.Context, schoolID, teacherID int64, at time.Time) (teaching.SlotInfo, bool, error) {
	sv, err := a.svc.SlotNow(ctx, schoolID, teacherID, at)
	if err != nil {
		return teaching.SlotInfo{}, false, err
	}
	if sv == nil {
		return teaching.SlotInfo{}, false, nil
	}
	periods, err := periodClocks(ctx, a.svc, schoolID)
	if err != nil {
		return teaching.SlotInfo{}, false, err
	}
	return toSlotInfo(*sv, periods), true, nil
}

func (a scheduleForTeaching) SlotByID(ctx context.Context, schoolID, slotID int64) (teaching.SlotInfo, bool, error) {
	sv, ok, err := a.svc.SlotByID(ctx, schoolID, slotID)
	if err != nil || !ok {
		return teaching.SlotInfo{}, ok, err
	}
	periods, err := periodClocks(ctx, a.svc, schoolID)
	if err != nil {
		return teaching.SlotInfo{}, false, err
	}
	return toSlotInfo(sv, periods), true, nil
}

func (a scheduleForTeaching) SlotsForDayOfWeek(ctx context.Context, schoolID int64, dayOfWeek int) ([]teaching.SlotInfo, error) {
	svs, err := a.svc.SlotsForDayOfWeek(ctx, schoolID, dayOfWeek)
	if err != nil {
		return nil, err
	}
	periods, err := periodClocks(ctx, a.svc, schoolID)
	if err != nil {
		return nil, err
	}
	out := make([]teaching.SlotInfo, 0, len(svs))
	for _, sv := range svs {
		out = append(out, toSlotInfo(sv, periods))
	}
	return out, nil
}

func (a scheduleForTeaching) CurrentPeriod(ctx context.Context, schoolID int64, at time.Time) (int, string, string, bool, error) {
	cp, err := a.svc.CurrentPeriod(ctx, schoolID, at)
	if err != nil {
		return 0, "", "", false, err
	}
	if cp.Period == nil {
		return 0, "", "", false, nil
	}
	return cp.Period.Number, cp.Period.StartsAt, cp.Period.EndsAt, true, nil
}

// -- scheduleForAttendance: memenuhi attendance.ScheduleSlotLookup --

type scheduleForAttendance struct {
	svc *schedule.Service
}

// SlotOwnership sudah primitif murni di schedule.Service — diteruskan
// langsung TANPA konversi (satu-satunya method di adapter ini yang bukan
// "penerjemah tipe", disatukan di struct yang sama demi kesederhanaan
// wiring: satu adapter per modul target, bukan per method).
func (a scheduleForAttendance) SlotOwnership(ctx context.Context, schoolID, slotID int64) (int64, int64, bool, error) {
	return a.svc.SlotOwnership(ctx, schoolID, slotID)
}

func (a scheduleForAttendance) SlotsTodayForTeacher(ctx context.Context, schoolID, teacherID int64, at time.Time) ([]attendance.SlotToday, error) {
	items, err := a.svc.SlotsToday(ctx, schoolID, schedule.TodayQuery{TeacherID: teacherID}, at)
	if err != nil {
		return nil, err
	}
	periods, err := periodClocks(ctx, a.svc, schoolID)
	if err != nil {
		return nil, err
	}
	out := make([]attendance.SlotToday, 0, len(items))
	for _, it := range items {
		out = append(out, attendance.SlotToday{
			ID: it.ID, ClassID: it.Class.ID, ClassName: it.Class.Name,
			SubjectID: it.Subject.ID, SubjectCode: it.Subject.Code, SubjectName: it.Subject.Name,
			PeriodStart: it.PeriodStart, PeriodEnd: it.PeriodEnd,
			StartsAt: periods[it.PeriodStart].StartsAt, EndsAt: periods[it.PeriodEnd].EndsAt,
			IsNow: it.IsNow,
		})
	}
	return out, nil
}
