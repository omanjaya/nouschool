package notification

import "github.com/omanjaya/nouschool/internal/platform/httpx"

// Settings adalah school_settings module "notification" (docs/08-notification.md
// "Konfigurasi": "Per sekolah (oleh super admin)... {"channels": [...]}").
// DIKELOLA SUPER ADMIN SAJA — lihat tenant.IsSuperAdminOnlyModule &
// GET/PUT /api/admin/schools/{id}/settings/{module} di internal/tenant.
type Settings struct {
	Channels []string `json:"channels"`
}

// DefaultSettings — in_app + web_push aktif (docs/08-notification.md).
func DefaultSettings() Settings {
	return Settings{Channels: []string{ChannelInApp, ChannelWebPush}}
}

// Validate memenuhi interface tenant.Settings (Validate() error) secara
// struktural — notification TIDAK mengimpor package tenant untuk ini (pola
// yang sama dengan internal/leave/settings.go).
func (s *Settings) Validate() error {
	if len(s.Channels) == 0 {
		return httpx.Validation("Minimal satu channel notifikasi harus aktif.")
	}
	seen := make(map[string]bool, len(s.Channels))
	for _, c := range s.Channels {
		if !validChannel(c) {
			return httpx.Validation("Channel notifikasi tidak dikenal: " + c)
		}
		if seen[c] {
			return httpx.Validation("Channel notifikasi duplikat: " + c)
		}
		seen[c] = true
	}
	return nil
}

func channelsSet(channels []string) map[string]bool {
	out := make(map[string]bool, len(channels))
	for _, c := range channels {
		out[c] = true
	}
	return out
}
