// Package dashboard menyusun payload dashboard TV ruang guru & kepala
// sekolah (fase 7, docs/06-teaching.md "Dashboard TV ruang guru" & "Dashboard
// kepala sekolah"). SATU endpoint (`GET /api/tv/board`) mengembalikan payload
// gabungan supaya klien TV cukup satu fetch (polling 15-30 dtk) — modul ini
// TIDAK punya tabel/repository sendiri, murni menyusun ULANG data dari
// service modul lain (teaching, attendance, announcement, schedule) lewat
// consumer-side interface — TIDAK menduplikasi logika derivasi status/rekap.
package dashboard

import "time"

// SchoolInfo adalah potongan info sekolah pada payload board.
type SchoolInfo struct {
	Name string `json:"name"`
}

// CurrentPeriodInfo — jam pelajaran ke berapa sekarang (docs/06 "Jam &
// pergantian").
type CurrentPeriodInfo struct {
	Number   int    `json:"number"`
	StartsAt string `json:"starts_at"`
	EndsAt   string `json:"ends_at"`
}

// NowInfo adalah bagian "now" payload board.
type NowInfo struct {
	Date          string             `json:"date"`
	DayLabel      string             `json:"day_label"`
	Time          string             `json:"time"`
	CurrentPeriod *CurrentPeriodInfo `json:"current_period"`
	NextStartsAt  *string            `json:"next_starts_at"`
}

// TeachingSummary — ringkasan jumlah per status mengajar (docs/06-teaching.md
// "5 status": mengajar/belum_masuk/izin/belum_mulai/selesai).
type TeachingSummary struct {
	Mengajar   int `json:"mengajar"`
	BelumMasuk int `json:"belum_masuk"`
	Izin       int `json:"izin"`
	BelumMulai int `json:"belum_mulai"`
	Selesai    int `json:"selesai"`
}

// TeachingItem adalah satu baris status mengajar guru pada "current"/"upcoming".
type TeachingItem struct {
	ClassID        int64  `json:"class_id"`
	ClassName      string `json:"class_name"`
	SubjectCode    string `json:"subject_code"`
	SubjectName    string `json:"subject_name"`
	TeacherID      int64  `json:"teacher_id"`
	TeacherName    string `json:"teacher_name"`
	RoomName       string `json:"room_name,omitempty"`
	Status         string `json:"status"`
	PeriodStartsAt string `json:"period_starts_at"`
	PeriodEndsAt   string `json:"period_ends_at"`
}

// TeachingBoard adalah bagian "teaching" payload board.
type TeachingBoard struct {
	Summary  TeachingSummary `json:"summary"`
	Current  []TeachingItem  `json:"current"`
	Upcoming []TeachingItem  `json:"upcoming"`
}

// AttendanceRow adalah satu baris rekap absensi siswa hari ini per kelas.
type AttendanceRow struct {
	ClassID       int64  `json:"class_id"`
	ClassName     string `json:"class_name"`
	Total         int64  `json:"total"`
	Hadir         int64  `json:"hadir"`
	Terlambat     int64  `json:"terlambat"`
	Izin          int64  `json:"izin"`
	Sakit         int64  `json:"sakit"`
	Alpa          int64  `json:"alpa"`
	Unmarked      int64  `json:"unmarked"`
	SessionStatus string `json:"session_status"`
}

// AnnouncementItem — bentuk ringkas pengumuman aktif (docs/06: cukup
// id/title/body). IsPlatform (fase 13 Gelombang 2, docs/11-superadmin.md P5)
// — true untuk pengumuman platform ("NouSchool", tampil di semua sekolah,
// platform dulu lalu sekolah di Board.Announcements), false untuk pengumuman
// sekolah sendiri.
type AnnouncementItem struct {
	ID         int64  `json:"id"`
	Title      string `json:"title"`
	Body       string `json:"body"`
	IsPlatform bool   `json:"is_platform"`
}

// Board adalah shape response GET /api/tv/board — payload gabungan SATU
// fetch (docs/06-teaching.md "Endpoint TV mengembalikan payload gabungan
// satu kali fetch").
type Board struct {
	School        SchoolInfo         `json:"school"`
	Now           NowInfo            `json:"now"`
	Teaching      TeachingBoard      `json:"teaching"`
	Attendance    []AttendanceRow    `json:"attendance"`
	Announcements []AnnouncementItem `json:"announcements"`
	GeneratedAt   time.Time          `json:"generated_at"`
}
