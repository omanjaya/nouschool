// b2adapter.go — HANYA wiring (sama seperti main.go, pola SAMA
// scheduleadapter.go): menjembatani *schedule.Service + *student.Service ke
// consumer-side interface exitpermit.ScheduleGateway &
// latearrival.ScheduleGateway (Fase 14 Gelombang B2, docs/12-sion-parity.md).
//
// Kenapa perlu adapter: exitpermit/latearrival butuh tahu APAKAH seorang
// guru (diidentifikasi lewat user_id, hasil ConsumeToken teacherqr) sedang/
// akan mengajar kelas siswa — tapi schedule.Service hanya mengenal "teacher
// profil ID" (SlotView.Teacher.ID, FK ke tabel teachers), bukan user_id.
// Pemetaan teacher profil ID <-> user_id SUDAH ADA (student.Service.
// MyTeacherID, dipakai modul schedule sejak fase 5) — TIDAK perlu method
// baru di modul student. Karena butuh MENGGABUNGKAN dua modul (schedule +
// student) untuk satu jawaban primitif (bool/time.Time), adapter ini tinggal
// di cmd/server, bukan di internal/ — exitpermit/latearrival sendiri TIDAK
// mengimpor schedule maupun student sama sekali.
package main

import (
	"context"
	"time"

	"github.com/omanjaya/nouschool/internal/schedule"
	"github.com/omanjaya/nouschool/internal/student"
)

type b2ScheduleGateway struct {
	schedule *schedule.Service
	students *student.Service
}

// TeacherMatchesClassNow memenuhi exitpermit.ScheduleGateway — tahap 2
// rantai dispensasi keluar ("guru pengajar jam berjalan/berikutnya").
func (a b2ScheduleGateway) TeacherMatchesClassNow(ctx context.Context, schoolID, classID, teacherUserID int64, at time.Time) (matches, hasSlot bool, err error) {
	slot, err := a.schedule.ClassSlotNowOrNext(ctx, schoolID, classID, at)
	if err != nil {
		return false, false, err
	}
	if slot == nil {
		return false, false, nil
	}
	teacherID, ok, err := a.students.MyTeacherID(ctx, schoolID, teacherUserID)
	if err != nil {
		return false, false, err
	}
	if !ok {
		return false, true, nil
	}
	return slot.Teacher.ID == teacherID, true, nil
}

// GateExpiryToday memenuhi exitpermit.ScheduleGateway — akhir period
// terakhir hari ini, fallback +6 jam bila sekolah belum punya period.
func (a b2ScheduleGateway) GateExpiryToday(ctx context.Context, schoolID int64, at time.Time) (time.Time, error) {
	t, ok, err := a.schedule.LastPeriodEndToday(ctx, schoolID, at)
	if err != nil {
		return time.Time{}, err
	}
	if !ok {
		return at.Add(6 * time.Hour), nil
	}
	return t, nil
}

// TeacherTeachesClassToday memenuhi latearrival.ScheduleGateway — tahap
// akhir izin terlambat ("guru kelas", cukup punya slot hari itu, tanpa
// syarat jam berjalan/berikutnya).
func (a b2ScheduleGateway) TeacherTeachesClassToday(ctx context.Context, schoolID, classID, teacherUserID int64, at time.Time) (bool, error) {
	teacherID, ok, err := a.students.MyTeacherID(ctx, schoolID, teacherUserID)
	if err != nil {
		return false, err
	}
	if !ok {
		return false, nil
	}
	return a.schedule.TeachesClassToday(ctx, schoolID, classID, teacherID, at)
}
