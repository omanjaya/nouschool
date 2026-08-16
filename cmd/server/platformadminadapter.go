// platformadminadapter.go — HANYA wiring (sama seperti dashboardadapter.go &
// scheduleadapter.go): menjembatani *platformadmin.Service ke consumer-side
// interface primitif yang dideklarasikan modul announcement (fase 13
// Gelombang 2, P5 "Pengumuman platform" — docs/11-superadmin.md).
//
// Kenapa perlu adapter (bukan *platformadmin.Service memenuhi
// announcement.PlatformAnnouncements secara struktural langsung)? Sama
// seperti catatan di dashboardadapter.go: platformadmin.Service.
// ActivePlatformAnnouncements mengembalikan []platformadmin.
// PlatformAnnouncementItem (struct BERNAMA milik package platformadmin),
// sedangkan announcement.PlatformAnnouncements butuh
// []announcement.PlatformAnnouncementItem — dua tipe struct bernama berbeda
// TIDAK dianggap sama oleh Go untuk kepuasan interface walau field-nya
// identik. cmd/server adalah satu-satunya tempat yang boleh mengimpor
// announcement SEKALIGUS platformadmin untuk mengonversi (CLAUDE.md "wiring
// eksplisit di main.go").
package main

import (
	"context"

	"github.com/omanjaya/nouschool/internal/announcement"
	"github.com/omanjaya/nouschool/internal/platformadmin"
)

// platformAdminForAnnouncement memenuhi announcement.PlatformAnnouncements.
type platformAdminForAnnouncement struct {
	svc *platformadmin.Service
}

func (a platformAdminForAnnouncement) ActiveOn(ctx context.Context, dateStr string) ([]announcement.PlatformAnnouncementItem, error) {
	items, err := a.svc.ActivePlatformAnnouncements(ctx, dateStr)
	if err != nil {
		return nil, err
	}
	out := make([]announcement.PlatformAnnouncementItem, 0, len(items))
	for _, it := range items {
		out = append(out, announcement.PlatformAnnouncementItem{
			ID: it.ID, Title: it.Title, Body: it.Body, StartsAt: it.StartsAt, EndsAt: it.EndsAt, CreatedAt: it.CreatedAt,
		})
	}
	return out, nil
}
