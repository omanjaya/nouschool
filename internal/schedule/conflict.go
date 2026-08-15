package schedule

import "fmt"

// Berkas ini berisi deteksi bentrok (docs/04-schedule.md): fungsi MURNI (tanpa
// I/O) beroperasi di atas []SlotRecord supaya bisa dites tanpa DB (lihat
// service_test.go) dan dipakai ulang oleh repository (create/update satu
// slot, dalam transaksi), Service.CopySchedule, dan parser import.

// overlaps melaporkan apakah rentang [aStart,aEnd] beririsan dengan
// [bStart,bEnd] (inklusif kedua ujung — blok 2 JP "period_start=3,
// period_end=4" beririsan dengan slot lain yang memakai jam ke-4).
func overlaps(aStart, aEnd, bStart, bEnd int) bool {
	return aStart <= bEnd && aEnd >= bStart
}

// findConflicts mengembalikan baris dari `existing` yang bentrok dengan
// kandidat slot baru: hari sama, rentang jam beririsan, dan (guru sama ATAU
// kelas sama ATAU ruang sama & tidak kosong) — persis 3 aturan
// docs/04-schedule.md. excludeSlotID dipakai saat update (slot itu sendiri
// tidak dihitung bentrok dengan dirinya sendiri).
func findConflicts(existing []SlotRecord, excludeSlotID int64, dayOfWeek, periodStart, periodEnd int, teacherID, classID, roomID int64) []SlotRecord {
	var out []SlotRecord
	for _, sl := range existing {
		if sl.ID == excludeSlotID {
			continue
		}
		if sl.DayOfWeek != dayOfWeek {
			continue
		}
		if !overlaps(sl.PeriodStart, sl.PeriodEnd, periodStart, periodEnd) {
			continue
		}
		if sl.TeacherID == teacherID || sl.ClassID == classID || (roomID != 0 && sl.RoomID == roomID) {
			out = append(out, sl)
		}
	}
	return out
}

// conflictMessage merangkai pesan Indonesia yang menyebut SIAPA/APA yang
// bentrok (docs/04: "error menyebutkan slot mana yang bentrok dengan apa"),
// prioritas guru > kelas > ruang (aturan pertama yang cocok yang dilaporkan).
func conflictMessage(dayOfWeek int, conflicts []SlotRecord, teacherID, classID, roomID int64) string {
	dn := dayName(dayOfWeek)
	for _, c := range conflicts {
		if c.TeacherID == teacherID {
			return fmt.Sprintf("Bentrok: %s sudah mengajar %s jam ke-%d–%d %s.", c.TeacherName, c.ClassName, c.PeriodStart, c.PeriodEnd, dn)
		}
	}
	for _, c := range conflicts {
		if c.ClassID == classID {
			return fmt.Sprintf("Bentrok: %s sudah ada jadwal %s jam ke-%d–%d %s.", c.ClassName, c.SubjectName, c.PeriodStart, c.PeriodEnd, dn)
		}
	}
	for _, c := range conflicts {
		if roomID != 0 && c.RoomID == roomID {
			return fmt.Sprintf("Bentrok: Ruang %s sudah dipakai %s jam ke-%d–%d %s.", c.RoomName, c.ClassName, c.PeriodStart, c.PeriodEnd, dn)
		}
	}
	return fmt.Sprintf("Bentrok jadwal pada %s jam ke-%d–%d.", dn, periodStartOf(conflicts), periodEndOf(conflicts))
}

func periodStartOf(cs []SlotRecord) int {
	if len(cs) == 0 {
		return 0
	}
	return cs[0].PeriodStart
}

func periodEndOf(cs []SlotRecord) int {
	if len(cs) == 0 {
		return 0
	}
	return cs[0].PeriodEnd
}
