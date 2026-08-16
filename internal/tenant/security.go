package tenant

// SecuritySettings (Fase 15 Gap 4, docs/12-sion-parity.md "single-device
// login"): toggle per sekolah, school_settings module "security". Saat
// aktif, login baru (host TENANT saja) menghapus SEMUA sesi LAIN user itu DI
// SEKOLAH INI — dibaca modul identity lewat consumer-side interface
// SecuritySettingsGateway.GetSetting (RAW json, pola sama
// internal/tenant/letters.go LettersSettings), BUKAN lewat SettingsService
// (identity TIDAK mengimpor tenant untuk tipe apa pun, CLAUDE.md).
type SecuritySettings struct {
	SingleDevice bool `json:"single_device"`
}

// DefaultSecuritySettings — false (sekolah baru TIDAK memaksa single-device
// secara default, sesuai instruksi tugas).
func DefaultSecuritySettings() SecuritySettings {
	return SecuritySettings{SingleDevice: false}
}

// Validate memenuhi interface Settings — bool tidak butuh validasi bentuk.
func (s *SecuritySettings) Validate() error {
	return nil
}
