package tenant

import "testing"

func TestBrandingSettingsDefaultIsValid(t *testing.T) {
	b := DefaultBrandingSettings()
	if err := b.Validate(); err != nil {
		t.Fatalf("default branding settings seharusnya valid: %v", err)
	}
}

func TestBrandingSettingsValidate(t *testing.T) {
	cases := []struct {
		name    string
		b       BrandingSettings
		wantErr bool
	}{
		{"valid", BrandingSettings{AppName: "SMKN 2 Malang", PrimaryColor: "#0F6B3A"}, false},
		{"valid lowercase hex", BrandingSettings{AppName: "X", PrimaryColor: "#abcdef"}, false},
		{"nama kosong", BrandingSettings{AppName: "", PrimaryColor: "#0F6B3A"}, true},
		{"warna tanpa pagar", BrandingSettings{AppName: "X", PrimaryColor: "0F6B3A"}, true},
		{"warna 3 digit", BrandingSettings{AppName: "X", PrimaryColor: "#0F6"}, true},
		{"warna bukan hex", BrandingSettings{AppName: "X", PrimaryColor: "#GGGGGG"}, true},
		{"nama terlalu panjang", BrandingSettings{AppName: string(make([]byte, 61)), PrimaryColor: "#0F6B3A"}, true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := c.b.Validate()
			if c.wantErr && err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !c.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestModuleSettingsRegistry(t *testing.T) {
	if _, ok := NewModuleSettings("branding"); !ok {
		t.Fatal("module 'branding' seharusnya dikenal")
	}
	if _, ok := NewModuleSettings("tidak-ada"); ok {
		t.Fatal("module tidak dikenal seharusnya ok=false")
	}
}
