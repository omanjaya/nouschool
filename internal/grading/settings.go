package grading

import (
	"strings"

	"github.com/omanjaya/nouschool/internal/platform/httpx"
)

// GradeRange adalah satu baris rentang label rapor (mis. 0-74 = "C").
type GradeRange struct {
	Min   int    `json:"min"`
	Max   int    `json:"max"`
	Label string `json:"label"`
}

// Settings adalah school_settings module "grading" (docs/12-sion-parity.md
// Gelombang C "Toggle grading_enabled via school_settings"). Enabled=false
// (default) menyembunyikan SELURUH endpoint grading lain (404
// grading_disabled, digerbang di Service, lihat requireEnabled di
// service.go) — HANYA GET /api/grading/status (semua role) & GET/PUT
// /api/settings/grading (pola generik tenant, admin_sekolah) tetap hidup.
type Settings struct {
	Enabled bool         `json:"enabled"`
	Ranges  []GradeRange `json:"ranges"`
}

// DefaultSettings — grading OFF secara default (sekolah harus mengaktifkan
// eksplisit), rentang label default meniru rapor umum (C/B/A).
func DefaultSettings() Settings {
	return Settings{
		Enabled: false,
		Ranges: []GradeRange{
			{Min: 0, Max: 74, Label: "C"},
			{Min: 75, Max: 84, Label: "B"},
			{Min: 85, Max: 100, Label: "A"},
		},
	}
}

// Validate memenuhi interface tenant.Settings (Validate() error) secara
// struktural — grading TIDAK mengimpor package tenant untuk ini (pola sama
// attendance.Settings/leave.Settings). Ranges harus MENUTUP 0..100 PERSIS
// (tanpa celah/tumpang tindih) dan berurutan naik (docs tugas): baris
// pertama mulai dari 0, baris terakhir berakhir di 100, dan
// ranges[i].Max+1 == ranges[i+1].Min untuk setiap i berurutan.
func (s *Settings) Validate() error {
	if len(s.Ranges) == 0 {
		return httpx.Validation("Rentang nilai (ranges) wajib diisi minimal satu baris.")
	}
	for i, r := range s.Ranges {
		if r.Min < 0 || r.Max > 100 || r.Min > r.Max {
			return httpx.Validation("Rentang nilai tidak valid: setiap baris harus 0 <= min <= max <= 100.")
		}
		if strings.TrimSpace(r.Label) == "" {
			return httpx.Validation("Setiap rentang nilai wajib punya label.")
		}
		if i == 0 && r.Min != 0 {
			return httpx.Validation("Rentang nilai harus dimulai dari 0.")
		}
		if i == len(s.Ranges)-1 && r.Max != 100 {
			return httpx.Validation("Rentang nilai harus berakhir di 100.")
		}
		if i > 0 && s.Ranges[i-1].Max+1 != r.Min {
			return httpx.Validation("Rentang nilai harus berurutan tanpa celah/tumpang tindih (mis. 0-74, 75-84, 85-100).")
		}
	}
	return nil
}

// labelForScore mengembalikan label rentang yang cocok utk nilai (SUDAH
// dibulatkan half-up ke int oleh pemanggil, lihat roundHalfUp di service.go)
// — nil bila tidak ada rentang yang cocok (seharusnya tidak terjadi bila
// Validate sudah menjamin cakupan 0..100 penuh, tapi dijaga untuk keamanan).
func labelForScore(ranges []GradeRange, rounded int) *string {
	for _, r := range ranges {
		if rounded >= r.Min && rounded <= r.Max {
			label := r.Label
			return &label
		}
	}
	return nil
}
