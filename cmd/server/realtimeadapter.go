// realtimeadapter.go — HANYA wiring (sama seperti scheduleadapter.go &
// dashboardadapter.go): menjembatani *realtime.Hub ke consumer-side
// interface primitif yang dideklarasikan tiap modul pemakai (attendance.
// Realtime, teaching.Realtime, notification.Realtime, leave.Realtime,
// announcement.Realtime, schedule.Realtime, billing.Realtime,
// student.Realtime — Fase 12, docs/ROADMAP.md).
//
// Kenapa perlu adapter di sini (bukan Hub langsung memenuhi interface secara
// struktural)? Hub.Publish sengaja punya SATU signature kaya
// (Publish(schoolID int64, ev realtime.Event)) supaya realtime_test.go bisa
// menguji targeting Roles/UserIDs lewat satu tipe Event, sedangkan tiap
// modul pemakai mendeklarasikan interface primitif dangkal
// (Publish(schoolID int64, eventType string, data map[string]any) +
// PublishTo(..., roles []string, userIDs []int64)) sesuai CLAUDE.md "interface
// kecil di sisi pemakai". Adapter tipis ini menyusun realtime.Event dari
// parameter primitif itu — satu instance (stateless) dipakai bersama oleh
// SEMUA modul pemakai karena method set-nya identik di semua consumer-side
// interface tsb (Go: kepuasan interface struktural, bukan perlu satu adapter
// per modul).
package main

import "github.com/omanjaya/nouschool/internal/realtime"

type realtimeForModules struct{ hub *realtime.Hub }

// Publish — broadcast satu sekolah (Roles & UserIDs kosong, lihat
// realtime.Event).
func (a realtimeForModules) Publish(schoolID int64, eventType string, data map[string]any) {
	a.hub.Publish(schoolID, realtime.Event{Type: eventType, Data: data})
}

// PublishTo — bertarget role/user tertentu (union bila keduanya terisi).
func (a realtimeForModules) PublishTo(schoolID int64, eventType string, data map[string]any, roles []string, userIDs []int64) {
	a.hub.Publish(schoolID, realtime.Event{Type: eventType, Data: data, Roles: roles, UserIDs: userIDs})
}

// PublishAll — broadcast LINTAS SEMUA sekolah (fase 13 Gelombang 2,
// docs/11-superadmin.md P5 "Pengumuman platform") — memenuhi
// platformadmin.PlatformRealtime secara struktural. Lihat catatan
// Hub.PublishAll untuk kenapa hanya sekolah dengan koneksi aktif yang
// dijangkau.
func (a realtimeForModules) PublishAll(eventType string, data map[string]any) {
	a.hub.PublishAll(realtime.Event{Type: eventType, Data: data})
}
