package identity

import "testing"

func TestHasPermissionSpotCheck(t *testing.T) {
	cases := []struct {
		role string
		perm string
		want bool
	}{
		// admin_sekolah: kelola semuanya kecuali journal & self-checkin
		{RoleAdminSekolah, PermStudentManage, true},
		{RoleAdminSekolah, PermSettingsManage, true},
		{RoleAdminSekolah, PermTeachingJournalWrite, false},
		{RoleAdminSekolah, PermLeaveApprove, false},

		// kepala_sekolah: baca + approve + monitor, bukan manage
		{RoleKepalaSekolah, PermStudentManage, false},
		{RoleKepalaSekolah, PermStudentRead, true},
		{RoleKepalaSekolah, PermLeaveApprove, true},
		{RoleKepalaSekolah, PermDashboardSchool, true},

		// guru: absen + jurnal + izin, bukan manage siswa/jadwal
		{RoleGuru, PermAttendanceWrite, true},
		{RoleGuru, PermTeachingJournalWrite, true},
		{RoleGuru, PermLeaveRequest, true},
		{RoleGuru, PermLeaveApprove, true},
		{RoleGuru, PermStudentManage, false},
		{RoleGuru, PermTeachingMonitor, false},

		// siswa: self-checkin + baca sendiri, bukan tulis absen orang lain
		{RoleSiswa, PermAttendanceSelfCheck, true},
		{RoleSiswa, PermAttendanceReadOwn, true},
		{RoleSiswa, PermAttendanceWrite, false},
		{RoleSiswa, PermScheduleRead, true},

		// orang_tua: baca sendiri saja
		{RoleOrangTua, PermAttendanceReadOwn, true},
		{RoleOrangTua, PermAttendanceSelfCheck, false},
		{RoleOrangTua, PermStudentRead, false},

		// display: monitor + jadwal, tidak lainnya
		{RoleDisplay, PermTeachingMonitor, true},
		{RoleDisplay, PermScheduleRead, true},
		{RoleDisplay, PermAttendanceReport, false},

		// role tidak dikenal -> selalu false
		{"role_ngawur", PermScheduleRead, false},
	}

	for _, c := range cases {
		got := HasPermission(c.role, c.perm)
		if got != c.want {
			t.Errorf("HasPermission(%q, %q) = %v, want %v", c.role, c.perm, got, c.want)
		}
	}
}

func TestPickActiveRolePriority(t *testing.T) {
	cases := []struct {
		roles []string
		want  string
	}{
		{[]string{RoleGuru, RoleOrangTua}, RoleGuru},
		{[]string{RoleOrangTua, RoleGuru}, RoleGuru},
		{[]string{RoleSiswa, RoleAdminSekolah}, RoleAdminSekolah},
		{[]string{RoleKepalaSekolah, RoleGuru}, RoleKepalaSekolah},
		{[]string{RoleSiswa}, RoleSiswa},
		{nil, ""},
	}
	for _, c := range cases {
		got := PickActiveRole(c.roles)
		if got != c.want {
			t.Errorf("PickActiveRole(%v) = %q, want %q", c.roles, got, c.want)
		}
	}
}
