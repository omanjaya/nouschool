package tenant

import (
	"regexp"

	"github.com/omanjaya/nouschool/internal/platform/httpx"
)

// BrandingSettings adalah module "branding" school_settings pertama: nama
// tampilan, warna utama, dan logo PWA per sekolah (lihat docs/01-tenant.md).
//
// LogoPath — path relatif di bawah platform/storage (mis.
// "1/branding/logo.png"), TIDAK PERNAH diserialisasikan langsung sebagai URL
// dapat diakses publik: klien membaca logo lewat GET /api/public/branding/logo
// (Fase 11), yang membaca field ini dari settings tersimpan lalu meng-stream
// isi filenya. Diisi HANYA oleh POST /api/settings/branding/logo (upload
// dedicated, lihat handler.go) — endpoint itu memuat settings existing lalu
// menimpa field ini SAJA supaya app_name/primary_color tidak ikut ter-reset;
// PUT /api/settings/branding biasa (generic, replace-all seperti modul
// settings lain) TETAP bisa menimpanya juga bila klien mengirim field ini.
type BrandingSettings struct {
	AppName      string `json:"app_name"`
	PrimaryColor string `json:"primary_color"`
	LogoPath     string `json:"logo_path,omitempty"`
}

// DefaultBrandingSettings adalah nilai default sebelum sekolah menyimpan
// pengaturannya sendiri.
func DefaultBrandingSettings() BrandingSettings {
	return BrandingSettings{
		AppName:      "NouSchool",
		PrimaryColor: "#0E6B4E", // default = token --primary design system (docs/10)
	}
}

var hexColorRe = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// Validate memenuhi interface Settings.
func (b *BrandingSettings) Validate() error {
	if len(b.AppName) == 0 || len(b.AppName) > 60 {
		return httpx.Validation("Nama aplikasi wajib diisi, maksimal 60 karakter.")
	}
	if !hexColorRe.MatchString(b.PrimaryColor) {
		return httpx.Validation("Warna utama harus format hex 6 digit, contoh #0F6B3A.")
	}
	return nil
}
