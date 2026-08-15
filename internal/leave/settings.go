package leave

import (
	"regexp"

	"github.com/omanjaya/nouschool/internal/platform/httpx"
)

// LeaveType adalah satu tipe izin yang bisa dipilih guru saat mengajukan
// (docs/07-leave.md) — editable per sekolah.
type LeaveType struct {
	Key   string `json:"key"`
	Label string `json:"label"`
}

// ChainStep adalah satu tingkat approval. UserID kosong (0) berarti "siapa
// pun yang sedang memegang role ini" — WAJIB diisi bila Role == "guru"
// (guru bukan approver default, hanya bisa jadi approver spesifik lewat
// penunjukan langsung di chain).
type ChainStep struct {
	Role   string `json:"role"`
	UserID int64  `json:"user_id,omitempty"`
}

// Settings adalah school_settings module "leave" (docs/07-leave.md).
type Settings struct {
	Types []LeaveType `json:"types"`
	Chain []ChainStep `json:"chain"`
}

// DefaultSettings — Types [izin, sakit, dinas_luar], Chain [{role: kepala_sekolah}]
// (satu tingkat, tanpa approver spesifik) — nilai default sebelum sekolah
// menyimpan pengaturannya sendiri.
func DefaultSettings() Settings {
	return Settings{
		Types: []LeaveType{
			{Key: "izin", Label: "Izin"},
			{Key: "sakit", Label: "Sakit"},
			{Key: "dinas_luar", Label: "Dinas Luar"},
		},
		Chain: []ChainStep{
			{Role: RoleKepalaSekolah},
		},
	}
}

var snakeCaseKey = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// Validate memenuhi interface tenant.Settings (Validate() error) secara
// struktural — leave TIDAK mengimpor package tenant untuk ini (lihat
// internal/attendance/settings.go untuk pola yang sama).
func (s *Settings) Validate() error {
	if len(s.Types) == 0 {
		return httpx.Validation("Tipe izin wajib diisi minimal satu.")
	}
	seen := make(map[string]bool, len(s.Types))
	for _, t := range s.Types {
		if !snakeCaseKey.MatchString(t.Key) {
			return httpx.Validation("Key tipe izin harus snake_case (huruf kecil, angka, underscore): " + t.Key)
		}
		if seen[t.Key] {
			return httpx.Validation("Key tipe izin duplikat: " + t.Key)
		}
		seen[t.Key] = true
		if t.Label == "" {
			return httpx.Validation("Label tipe izin (" + t.Key + ") wajib diisi.")
		}
	}
	for _, c := range s.Chain {
		if !validChainRole(c.Role) {
			return httpx.Validation("Role chain approval tidak dikenal: " + c.Role)
		}
		if c.Role == RoleGuru && c.UserID == 0 {
			return httpx.Validation("Chain dengan role guru wajib menunjuk approver spesifik (user_id).")
		}
	}
	return nil
}
