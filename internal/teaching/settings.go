package teaching

// Settings adalah school_settings module "teaching" (docs/06-teaching.md
// "Status mengajar (derivasi, bukan tabel)": toleransi menit sebelum slot
// yang berjalan tanpa journal dianggap "belum masuk kelas" di monitoring).
type Settings struct {
	NotStartedAfterMin int `json:"not_started_after_min"`
}

// DefaultSettings — toleransi 10 menit (docs/06-teaching.md default X=10).
func DefaultSettings() Settings {
	return Settings{NotStartedAfterMin: 10}
}

// Validate memenuhi interface tenant.Settings (Validate() error) secara
// struktural — teaching TIDAK mengimpor package tenant untuk ini (pola sama
// dengan internal/attendance/settings.go & internal/leave).
func (s *Settings) Validate() error {
	if s.NotStartedAfterMin <= 0 {
		s.NotStartedAfterMin = 10
	}
	return nil
}
