package tenant

import "github.com/omanjaya/nouschool/internal/platform/httpx"

// LettersSettings (Fase 14 Gelombang D, docs/12-sion-parity.md "template
// surat", school_settings module "letters"): catatan footer ditambahkan di
// BAGIAN BAWAH PDF surat SP (internal/discipline/pdf.go) & surat izin siswa
// (internal/studentleave/pdf.go) BILA NON-KOSONG — dibaca kedua modul lewat
// consumer-side interface SettingsGateway.GetSetting (RAW json, pola sama
// internal/grading.SettingsGateway), BUKAN lewat SettingsService (tenant
// TIDAK punya alasan mengimpor discipline/studentleave untuk tipe apa pun).
type LettersSettings struct {
	SPFooterNote    string `json:"sp_footer_note"`
	LeaveFooterNote string `json:"leave_footer_note"`
}

// DefaultLettersSettings — "" (kosong) utk keduanya, sesuai docs tugas.
func DefaultLettersSettings() LettersSettings {
	return LettersSettings{}
}

// lettersFooterMaxLen — batas wajar catatan footer (bukan esai) supaya PDF
// satu halaman tetap muat.
const lettersFooterMaxLen = 1000

// Validate memenuhi interface Settings.
func (l *LettersSettings) Validate() error {
	if len(l.SPFooterNote) > lettersFooterMaxLen {
		return httpx.Validation("Catatan footer surat SP maksimal 1000 karakter.")
	}
	if len(l.LeaveFooterNote) > lettersFooterMaxLen {
		return httpx.Validation("Catatan footer surat izin maksimal 1000 karakter.")
	}
	return nil
}
