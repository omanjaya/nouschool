package identity

// Role adalah nilai kanonik role membership (lihat docs/02-identity.md).
const (
	RoleAdminSekolah  = "admin_sekolah"
	RoleKepalaSekolah = "kepala_sekolah"
	RoleGuru          = "guru"
	RoleSiswa         = "siswa"
	RoleOrangTua      = "orang_tua"
	RoleDisplay       = "display"
	// RoleSuperAdmin bukan membership — flag terpisah users.is_super_admin,
	// dipakai sebagai nilai sessions.role saat super admin login di host platform.
	RoleSuperAdmin = "super_admin"
)

// rolePriority menentukan role aktif saat user punya >1 role di sekolah yang
// sama (mis. guru + orang_tua) — urutan sesuai docs/02-identity.md.
var rolePriority = map[string]int{
	RoleAdminSekolah:  0,
	RoleKepalaSekolah: 1,
	RoleGuru:          2,
	RoleOrangTua:      3,
	RoleSiswa:         4,
}

// Permission kanonik — lihat tabel di docs/02-identity.md. Permission baru =
// tambah konstanta + baris di rolePermissions + baris di tabel dokumen itu.
const (
	PermStudentManage        = "student:manage"
	PermStudentRead          = "student:read"
	PermScheduleManage       = "schedule:manage"
	PermScheduleRead         = "schedule:read"
	PermAttendanceWrite      = "attendance:write"
	PermAttendanceSelfCheck  = "attendance:self_checkin"
	PermAttendanceReadOwn    = "attendance:read_own"
	PermAttendanceReport     = "attendance:report"
	PermTeachingJournalWrite = "teaching:journal_write"
	PermTeachingMonitor      = "teaching:monitor"
	PermLeaveRequest         = "leave:request"
	PermLeaveApprove         = "leave:approve"
	PermLeaveManage          = "leave:manage"
	PermSettingsManage       = "settings:manage"
	PermBillingView          = "billing:view"
	PermAnnouncementManage   = "announcement:manage"
	PermDashboardSchool      = "dashboard:school"
)

// rolePermissions adalah map role->permission statis (hardcode, bukan DB —
// lihat CLAUDE.md aturan RBAC).
var rolePermissions = map[string]map[string]bool{
	RoleAdminSekolah: {
		PermStudentManage:      true,
		PermStudentRead:        true,
		PermScheduleManage:     true,
		PermScheduleRead:       true,
		PermAttendanceWrite:    true,
		PermAttendanceReport:   true,
		PermTeachingMonitor:    true,
		PermLeaveManage:        true,
		PermSettingsManage:     true,
		PermBillingView:        true,
		PermAnnouncementManage: true,
		PermDashboardSchool:    true,
	},
	RoleKepalaSekolah: {
		PermStudentRead:        true,
		PermScheduleRead:       true,
		PermAttendanceReport:   true,
		PermTeachingMonitor:    true,
		PermLeaveApprove:       true,
		PermBillingView:        true,
		PermAnnouncementManage: true,
		PermDashboardSchool:    true,
	},
	RoleGuru: {
		PermStudentRead:          true,
		PermScheduleRead:         true,
		PermAttendanceWrite:      true,
		PermAttendanceReport:     true,
		PermTeachingJournalWrite: true,
		PermLeaveRequest:         true,
		PermLeaveApprove:         true,
	},
	RoleSiswa: {
		PermScheduleRead:        true,
		PermAttendanceSelfCheck: true,
		PermAttendanceReadOwn:   true,
	},
	RoleOrangTua: {
		PermScheduleRead:      true,
		PermAttendanceReadOwn: true,
	},
	RoleDisplay: {
		PermScheduleRead:    true,
		PermTeachingMonitor: true,
	},
}

// HasPermission mengecek map role->permission statis. Object-level check
// (mis. ortu hanya anaknya sendiri) TETAP dilakukan di service layer modul
// masing-masing — middleware ini hanya gerbang kasar.
func HasPermission(role, perm string) bool {
	perms, ok := rolePermissions[role]
	if !ok {
		return false
	}
	return perms[perm]
}

// PickActiveRole memilih role aktif dari daftar role membership user di satu
// sekolah, sesuai prioritas docs/02: admin_sekolah > kepala_sekolah > guru >
// orang_tua > siswa.
func PickActiveRole(roles []string) string {
	best := ""
	bestPriority := int(^uint(0) >> 1) // max int
	for _, r := range roles {
		p, ok := rolePriority[r]
		if !ok {
			continue
		}
		if p < bestPriority {
			bestPriority = p
			best = r
		}
	}
	if best == "" && len(roles) > 0 {
		return roles[0]
	}
	return best
}
