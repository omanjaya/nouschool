package identity

import (
	"context"
	"testing"
	"time"
)

// -- effectivePermissionFrom (fungsi murni dipakai RequirePerm) --

func TestEffectivePermissionFrom(t *testing.T) {
	overrides := map[string]map[string]bool{
		// guru default punya discipline:record (true) — override jadi false.
		RoleGuru: {PermDisciplineRecord: false},
		// siswa default TIDAK punya student:read — override jadi true.
		RoleSiswa: {PermStudentRead: true},
	}

	cases := []struct {
		name string
		role string
		perm string
		want bool
	}{
		{"override default-allow jadi deny", RoleGuru, PermDisciplineRecord, false},
		{"override default-deny jadi allow", RoleSiswa, PermStudentRead, true},
		{"role ada di overrides tapi perm lain -> fallback default", RoleGuru, PermLeaveRequest, true},
		{"role tanpa override sama sekali -> fallback default", RoleKepalaSekolah, PermDashboardSchool, true},
		{"role & perm tanpa override -> fallback default (deny)", RoleKepalaSekolah, PermStudentManage, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := effectivePermissionFrom(overrides, c.role, c.perm)
			if got != c.want {
				t.Errorf("effectivePermissionFrom(%q,%q) = %v, want %v", c.role, c.perm, got, c.want)
			}
		})
	}
}

// -- validateRolePermissionChanges --

func TestValidateRolePermissionChanges(t *testing.T) {
	allowTrue := true
	cases := []struct {
		name    string
		changes []RolePermOverrideChange
		wantErr bool
	}{
		{"valid: guru discipline:record false", []RolePermOverrideChange{{Role: RoleGuru, Permission: PermDisciplineRecord, Allowed: &allowTrue}}, false},
		{"valid: hapus override (Allowed nil)", []RolePermOverrideChange{{Role: RoleSiswa, Permission: PermStudentRead, Allowed: nil}}, false},
		{"role tidak dikenal", []RolePermOverrideChange{{Role: "role_ngawur", Permission: PermStudentRead, Allowed: &allowTrue}}, true},
		{"role pegawai bukan editable role", []RolePermOverrideChange{{Role: RolePegawai, Permission: PermStudentRead, Allowed: &allowTrue}}, true},
		{"role admin_sekolah dilarang", []RolePermOverrideChange{{Role: RoleAdminSekolah, Permission: PermStudentRead, Allowed: &allowTrue}}, true},
		{"role admin_sekolah dilarang walau hapus (nil)", []RolePermOverrideChange{{Role: RoleAdminSekolah, Permission: PermStudentRead, Allowed: nil}}, true},
		{"permission tidak dikenal", []RolePermOverrideChange{{Role: RoleGuru, Permission: "ngawur:apapun", Allowed: &allowTrue}}, true},
		{"permission settings:manage dilarang", []RolePermOverrideChange{{Role: RoleGuru, Permission: PermSettingsManage, Allowed: &allowTrue}}, true},
		{"permission settings:manage dilarang walau hapus", []RolePermOverrideChange{{Role: RoleGuru, Permission: PermSettingsManage, Allowed: nil}}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := validateRolePermissionChanges(c.changes)
			if (err != nil) != c.wantErr {
				t.Errorf("validateRolePermissionChanges() err = %v, wantErr %v", err, c.wantErr)
			}
		})
	}
}

// -- fake repo untuk getRolePermissions/putRolePermissions --

type fakeRolePermRepo struct {
	rows []RolePermOverrideRow
}

func (f *fakeRolePermRepo) ListRolePermissionOverrides(ctx context.Context, schoolID int64) ([]RolePermOverrideRow, error) {
	return append([]RolePermOverrideRow{}, f.rows...), nil
}

func (f *fakeRolePermRepo) ReplaceRolePermissionOverrides(ctx context.Context, schoolID int64, changes []RolePermOverrideChange) error {
	for _, c := range changes {
		// hapus baris lama (role,permission) yang sama dulu.
		out := f.rows[:0]
		for _, r := range f.rows {
			if r.Role == c.Role && r.Permission == c.Permission {
				continue
			}
			out = append(out, r)
		}
		f.rows = out
		if c.Allowed != nil {
			f.rows = append(f.rows, RolePermOverrideRow{Role: c.Role, Permission: c.Permission, Allowed: *c.Allowed})
		}
	}
	return nil
}

func TestGetRolePermissions_DefaultsAndOverrides(t *testing.T) {
	repo := &fakeRolePermRepo{rows: []RolePermOverrideRow{
		{Role: RoleGuru, Permission: PermDisciplineRecord, Allowed: false},
	}}
	view, err := getRolePermissions(context.Background(), repo, 1)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(view.Roles) != 6 {
		t.Fatalf("expected 6 editable roles, got %d: %v", len(view.Roles), view.Roles)
	}
	for _, r := range view.Roles {
		if r == RolePegawai || r == RoleSuperAdmin {
			t.Fatalf("pegawai/super_admin seharusnya TIDAK ada di daftar roles: %v", view.Roles)
		}
	}
	if !view.Defaults[RoleGuru][PermDisciplineRecord] {
		t.Fatalf("defaults guru discipline:record seharusnya true (peta statis)")
	}
	if view.Overrides[RoleGuru][PermDisciplineRecord] != false {
		t.Fatalf("overrides guru discipline:record seharusnya false (override tersimpan)")
	}
	if _, ok := view.Overrides[RoleSiswa]; ok {
		t.Fatalf("siswa tidak seharusnya punya entri overrides (tidak ada baris)")
	}
}

func TestPutRolePermissions_UpsertAndDelete(t *testing.T) {
	repo := &fakeRolePermRepo{}
	allowFalse := false

	// upsert: cabut discipline:record dari guru.
	view, err := putRolePermissions(context.Background(), repo, 1, map[string]map[string]*bool{
		RoleGuru: {PermDisciplineRecord: &allowFalse},
	})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if view.Overrides[RoleGuru][PermDisciplineRecord] != false {
		t.Fatalf("override belum tersimpan: %+v", view.Overrides)
	}

	// hapus override (null) -> kembali ke default statis (true), tidak ada
	// lagi di overrides.
	view, err = putRolePermissions(context.Background(), repo, 1, map[string]map[string]*bool{
		RoleGuru: {PermDisciplineRecord: nil},
	})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if _, ok := view.Overrides[RoleGuru][PermDisciplineRecord]; ok {
		t.Fatalf("override seharusnya sudah terhapus: %+v", view.Overrides)
	}
	if !view.Defaults[RoleGuru][PermDisciplineRecord] {
		t.Fatalf("default guru discipline:record seharusnya tetap true")
	}
}

func TestPutRolePermissions_RejectsLockedRoleAndPermission(t *testing.T) {
	repo := &fakeRolePermRepo{}
	allowTrue := true

	if _, err := putRolePermissions(context.Background(), repo, 1, map[string]map[string]*bool{
		RoleAdminSekolah: {PermStudentManage: &allowTrue},
	}); err == nil {
		t.Fatal("expected error mengubah role admin_sekolah")
	}
	if _, err := putRolePermissions(context.Background(), repo, 1, map[string]map[string]*bool{
		RoleGuru: {PermSettingsManage: &allowTrue},
	}); err == nil {
		t.Fatal("expected error mengubah permission settings:manage")
	}
	if len(repo.rows) != 0 {
		t.Fatalf("tidak ada perubahan yang seharusnya tersimpan setelah validasi gagal: %+v", repo.rows)
	}
}

// -- roleOverrideCache (TTL + invalidate) --

func TestRoleOverrideCache_TTLAndInvalidate(t *testing.T) {
	c := newRoleOverrideCache(60 * time.Second)
	if _, ok := c.get(1); ok {
		t.Fatal("cache kosong seharusnya miss")
	}

	c.set(1, map[string]map[string]bool{RoleGuru: {PermDisciplineRecord: false}})
	data, ok := c.get(1)
	if !ok {
		t.Fatal("seharusnya hit sebelum TTL habis")
	}
	if data[RoleGuru][PermDisciplineRecord] != false {
		t.Fatalf("data cache salah: %+v", data)
	}

	// White-box: paksa entri kedaluwarsa (hindari flaky test tergantung
	// resolusi jam OS pada TTL sangat kecil) — simulasikan waktu sudah lewat
	// TTL tanpa perlu clock injection tambahan di cache.
	c.mu.Lock()
	e := c.entries[1]
	e.expiresAt = time.Now().Add(-time.Second)
	c.entries[1] = e
	c.mu.Unlock()
	if _, ok := c.get(1); ok {
		t.Fatal("entri kedaluwarsa seharusnya miss")
	}

	c.set(2, map[string]map[string]bool{RoleGuru: {PermDisciplineRecord: false}})
	c.invalidate(2)
	if _, ok := c.get(2); ok {
		t.Fatal("seharusnya miss setelah invalidate")
	}
}
