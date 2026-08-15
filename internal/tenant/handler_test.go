package tenant

import "testing"

// -- branding publik / manifest (Fase 11) — fungsi murni, tanpa DB/HTTP --

func TestBrandingLogoURLForEmptyIsNil(t *testing.T) {
	if u := brandingLogoURLFor(BrandingSettings{AppName: "X", PrimaryColor: "#000000"}); u != nil {
		t.Fatalf("logo_url seharusnya nil bila LogoPath kosong, dapat %v", *u)
	}
}

func TestBrandingLogoURLForSetReturnsPublicEndpoint(t *testing.T) {
	u := brandingLogoURLFor(BrandingSettings{AppName: "X", PrimaryColor: "#000000", LogoPath: "1/branding/logo.png"})
	if u == nil || *u != publicLogoURL {
		t.Fatalf("logo_url seharusnya %q, dapat %v", publicLogoURL, u)
	}
}

func TestBrandingPublicViewForDefaultsWhenUnset(t *testing.T) {
	v := brandingPublicViewFor(BrandingSettings{})
	def := DefaultBrandingSettings()
	if v.AppName != def.AppName || v.PrimaryColor != def.PrimaryColor {
		t.Fatalf("branding kosong seharusnya jatuh ke default, dapat %+v", v)
	}
	if v.LogoURL != nil {
		t.Fatalf("branding default seharusnya tanpa logo, dapat %v", v.LogoURL)
	}
}

func TestBrandingPublicViewForCustom(t *testing.T) {
	v := brandingPublicViewFor(BrandingSettings{AppName: "SMKN 2 Malang", PrimaryColor: "#0F6B3A", LogoPath: "1/branding/logo.jpg"})
	if v.AppName != "SMKN 2 Malang" || v.PrimaryColor != "#0F6B3A" {
		t.Fatalf("branding custom salah: %+v", v)
	}
	if v.LogoURL == nil || *v.LogoURL != publicLogoURL {
		t.Fatalf("logo_url seharusnya endpoint publik, dapat %v", v.LogoURL)
	}
}

// TestManifestPlatformVsTenant — docs tugas: "manifest dinamis (tenant vs platform)".
func TestManifestPlatformVsTenant(t *testing.T) {
	platform := defaultManifest()
	if platform.Name != "NouSchool" || platform.ShortName != "NouSchool" {
		t.Fatalf("manifest platform seharusnya nama default NouSchool: %+v", platform)
	}
	if platform.ThemeColor != DefaultBrandingSettings().PrimaryColor {
		t.Fatalf("manifest platform seharusnya theme_color default: %+v", platform)
	}
	if len(platform.Icons) != 1 || platform.Icons[0].Src != defaultManifestIcon {
		t.Fatalf("manifest platform seharusnya ikon default: %+v", platform.Icons)
	}

	tenant := manifestForBranding(BrandingSettings{AppName: "SMKN 2 Malang", PrimaryColor: "#123456"})
	if tenant.Name != "SMKN 2 Malang" || tenant.ShortName != "SMKN 2 Malang" {
		t.Fatalf("manifest tenant seharusnya app_name sekolah: %+v", tenant)
	}
	if tenant.ThemeColor != "#123456" {
		t.Fatalf("manifest tenant seharusnya theme_color sekolah: %+v", tenant)
	}
	// Tanpa logo -> tetap ikon default (belum upload).
	if len(tenant.Icons) != 1 || tenant.Icons[0].Src != defaultManifestIcon {
		t.Fatalf("manifest tenant tanpa logo seharusnya ikon default: %+v", tenant.Icons)
	}
}

func TestManifestTenantWithLogo(t *testing.T) {
	m := manifestForBranding(BrandingSettings{AppName: "SMKN 2 Malang", PrimaryColor: "#123456", LogoPath: "1/branding/logo.png"})
	if len(m.Icons) != 1 {
		t.Fatalf("manifest dengan logo seharusnya satu entri icon, dapat %d", len(m.Icons))
	}
	icon := m.Icons[0]
	if icon.Src != publicLogoURL {
		t.Fatalf("icon.src seharusnya endpoint logo publik, dapat %q", icon.Src)
	}
	if icon.Sizes != "any" {
		t.Fatalf("icon.sizes seharusnya \"any\", dapat %q", icon.Sizes)
	}
	if icon.Type != "image/png" {
		t.Fatalf("icon.type seharusnya image/png, dapat %q", icon.Type)
	}
}

func TestManifestDefaultsWhenBrandingNeverSaved(t *testing.T) {
	// AppName kosong = sekolah belum pernah PUT settings branding — harus
	// tetap menghasilkan manifest valid (default), bukan nama kosong.
	m := manifestForBranding(BrandingSettings{})
	def := DefaultBrandingSettings()
	if m.Name != def.AppName || m.ThemeColor != def.PrimaryColor {
		t.Fatalf("manifest tenant tanpa branding tersimpan seharusnya default: %+v", m)
	}
}

func TestContentTypeForExt(t *testing.T) {
	cases := map[string]string{
		"1/branding/logo.png":  "image/png",
		"1/branding/logo.jpg":  "image/jpeg",
		"1/branding/logo.jpeg": "image/jpeg",
		"1/branding/logo.gif":  "application/octet-stream",
	}
	for path, want := range cases {
		if got := contentTypeForExt(path); got != want {
			t.Fatalf("contentTypeForExt(%q) = %q, ingin %q", path, got, want)
		}
	}
}
