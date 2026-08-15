// Package clock mengabstraksi waktu agar logika tanggal (absensi, jadwal)
// bisa dites deterministik. Modul TIDAK memanggil time.Now langsung.
package clock

import "time"

type Clock interface {
	Now() time.Time
}

type System struct{}

func (System) Now() time.Time { return time.Now() }

// InZone mengonversi waktu ke zona sekolah (Asia/Jakarta | Asia/Makassar | Asia/Jayapura).
// Tanggal absensi SELALU dihitung dari waktu lokal sekolah, bukan UTC.
func InZone(t time.Time, tz string) time.Time {
	loc, err := time.LoadLocation(tz)
	if err != nil {
		loc = time.FixedZone("WIB", 7*3600)
	}
	return t.In(loc)
}

// Fixed adalah clock untuk test.
type Fixed struct{ T time.Time }

func (f Fixed) Now() time.Time { return f.T }
