package attendance

import "github.com/omanjaya/nouschool/internal/platform/httpx"

// Mode kanonik Settings.Mode.
type Mode string

const (
	ModeDaily      Mode = "daily"
	ModePerSubject Mode = "per_subject"
)

// Method kanonik Settings.Methods / attendance_records.method.
type Method string

const (
	MethodManual      Method = "manual"
	MethodQRCard      Method = "qr_card"
	MethodSelfCheckin Method = "self_checkin"
)

// SelfCheckinRule — aturan jendela & radius self check-in (Fase 8, disimpan
// sejak sekarang supaya bentuk settings sudah final — lihat docs/05-attendance.md).
type SelfCheckinRule struct {
	Lat      float64 `json:"lat"`
	Lng      float64 `json:"lng"`
	RadiusM  int     `json:"radius_m"`
	OpenFrom string  `json:"open_from"`
	CloseAt  string  `json:"close_at"`
}

// Settings adalah school_settings module "attendance" (docs/05-attendance.md).
type Settings struct {
	Mode            Mode             `json:"mode"`
	Methods         []Method         `json:"methods"`
	SelfCheckin     *SelfCheckinRule `json:"self_checkin,omitempty"`
	EditWindowHours int              `json:"edit_window_hours"`
	LateAfterMin    int              `json:"late_after_min"`
}

// DefaultSettings adalah nilai default sebelum sekolah menyimpan pengaturannya
// sendiri: mode daily, metode manual saja, jendela edit 24 jam.
func DefaultSettings() Settings {
	return Settings{
		Mode:            ModeDaily,
		Methods:         []Method{MethodManual},
		EditWindowHours: 24,
		LateAfterMin:    15,
	}
}

func validMethod(m Method) bool {
	switch m {
	case MethodManual, MethodQRCard, MethodSelfCheckin:
		return true
	default:
		return false
	}
}

// Validate memenuhi interface tenant.Settings (Validate() error) secara
// struktural — attendance TIDAK mengimpor package tenant untuk ini.
func (s *Settings) Validate() error {
	switch s.Mode {
	case "":
		s.Mode = ModeDaily
	case ModeDaily, ModePerSubject:
	default:
		return httpx.Validation("Mode absensi harus daily atau per_subject.")
	}
	if len(s.Methods) == 0 {
		s.Methods = []Method{MethodManual}
	}
	for _, m := range s.Methods {
		if !validMethod(m) {
			return httpx.Validation("Metode absensi tidak dikenal: " + string(m))
		}
	}
	if s.EditWindowHours <= 0 {
		s.EditWindowHours = 24
	}
	if s.LateAfterMin <= 0 {
		s.LateAfterMin = 15
	}
	return nil
}
