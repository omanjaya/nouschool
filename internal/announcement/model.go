// Package announcement mengurus pengumuman sekolah (ditampilkan di dashboard
// TV ruang guru & dashboard kepsek — lihat docs/06-teaching.md "Pengumuman").
// Dikelola admin/kepala sekolah (perm announcement:manage), dibaca SEMUA role
// termasuk akun display saat ?active=1 (docs/02-identity.md).
package announcement

import (
	"fmt"
	"strings"
	"time"
)

// Permission kanonik dipakai modul announcement (nilai HARUS sama persis
// dengan docs/02-identity.md — didefinisikan ulang di sini karena
// announcement TIDAK boleh mengimpor identity, lihat CLAUDE.md).
const PermAnnouncementManage = "announcement:manage"

// Date adalah tanggal murni (tanpa jam), dikodekan JSON sebagai "YYYY-MM-DD"
// — pola yang sama dengan internal/attendance.Date / internal/teaching.Date
// (didefinisikan ulang di sini supaya announcement tidak mengimpor modul lain
// untuk tipe ini, lihat CLAUDE.md).
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

// AnnouncementView adalah shape response GET/POST/PATCH /api/announcements.
// IsPlatform (fase 13 Gelombang 2, docs/11-superadmin.md P5 "Pengumuman
// platform"): true untuk pengumuman platform (dari internal/platformadmin,
// tampil di SEMUA sekolah), false untuk pengumuman sekolah sendiri — lihat
// Service.List/platformViews.
type AnnouncementView struct {
	ID         int64     `json:"id"`
	Title      string    `json:"title"`
	Body       string    `json:"body"`
	StartsAt   Date      `json:"starts_at"`
	EndsAt     Date      `json:"ends_at"`
	CreatedAt  time.Time `json:"created_at"`
	IsPlatform bool      `json:"is_platform"`
}

// BoardItem adalah shape ringkas pengumuman aktif dipakai modul dashboard TV
// (docs/06-teaching.md: payload gabungan `announcements: [{"id","title","body"}]`).
// IsPlatform — lihat catatan AnnouncementView di atas.
type BoardItem struct {
	ID         int64  `json:"id"`
	Title      string `json:"title"`
	Body       string `json:"body"`
	IsPlatform bool   `json:"is_platform"`
}
