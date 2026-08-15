package identity

import "testing"

// TestSessionTTLForRole — docs/02-identity.md "Cookie: ... umur 30 hari
// (sliding), role display 1 tahun".
func TestSessionTTLForRole(t *testing.T) {
	cases := []struct {
		role string
		want int // hari
	}{
		{RoleAdminSekolah, 30},
		{RoleKepalaSekolah, 30},
		{RoleGuru, 30},
		{RoleSiswa, 30},
		{RoleOrangTua, 30},
		{RoleDisplay, 365},
		{RoleSuperAdmin, 30},
	}
	for _, c := range cases {
		got := sessionTTLForRole(c.role)
		wantHours := float64(c.want * 24)
		if got.Hours() != wantHours {
			t.Errorf("role %s: expected TTL %d hari, got %v", c.role, c.want, got)
		}
	}
}

func TestSessionRenewWindowForRole(t *testing.T) {
	if sessionRenewWindowForRole(RoleGuru) != sessionRenewWindow {
		t.Errorf("expected renew window role selain display = sessionRenewWindow biasa")
	}
	if sessionRenewWindowForRole(RoleDisplay) != displaySessionRenewWindow {
		t.Errorf("expected renew window role display = displaySessionRenewWindow")
	}
	// Renew window HARUS lebih kecil dari TTL-nya sendiri (kalau tidak,
	// sesi tidak pernah diperpanjang tepat waktu / selalu diperpanjang).
	if sessionRenewWindow >= sessionTTL {
		t.Fatal("sessionRenewWindow harus < sessionTTL")
	}
	if displaySessionRenewWindow >= displaySessionTTL {
		t.Fatal("displaySessionRenewWindow harus < displaySessionTTL")
	}
}
