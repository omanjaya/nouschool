package identity

// Role adalah nilai kanonik role membership (lihat docs/02-identity.md).
const (
	RoleAdminSekolah  = "admin_sekolah"
	RoleKepalaSekolah = "kepala_sekolah"
	RoleGuru          = "guru"
	RoleSiswa         = "siswa"
	RoleOrangTua      = "orang_tua"
	RoleDisplay       = "display"
	// RolePegawai — staff non-guru (mis. security/tata usaha), Fase 14
	// Gelombang B1 (docs/12-sion-parity.md). TANPA permission modul apa pun
	// selain akses auth-implicit (announcements aktif & notifications sudah
	// auth-only, TIDAK butuh entri rolePermissions) — otorisasi pegawai
	// SELALU lewat capability flags (internal/duty), bukan permission RBAC.
	RolePegawai = "pegawai"
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
	RolePegawai:       3, // Fase 14 Gelombang B1: prioritas setelah guru
	RoleOrangTua:      4,
	RoleSiswa:         5,
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
	PermDisciplineManage     = "discipline:manage"
	PermDisciplineRecord     = "discipline:record"
	PermDisciplineRead       = "discipline:read"
	PermDutyManage           = "duty:manage"
	PermGradingManage        = "grading:manage"
	// PermUserImpersonate — Fase 14 Gelombang D (docs/12-sion-parity.md
	// "Impersonate USER oleh admin sekolah"): admin_sekolah masuk sebagai
	// member lain sekolahnya utk mendukung/debug.
	PermUserImpersonate = "user:impersonate"
	// PermGradingRead — Fase 15 Gap 5 (docs/12-sion-parity.md "akses baca
	// modul nilai untuk kepala sekolah"): SATU baris baru, HANYA
	// kepala_sekolah (lihat rolePermissions di bawah) — modul grading yang
	// memakainya (gate endpoint baca nilai kepsek) dikerjakan agent lain;
	// baris ini HANYA menambah konstanta + entri map sesuai lingkup tugas.
	// docs/02-identity.md perlu ditambah baris tabel permission ini — dicatat
	// di laporan tugas untuk diperbarui orchestrator (bukan diedit di sini).
	PermGradingRead = "grading:read"
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
		PermDisciplineManage:   true,
		PermDisciplineRecord:   true,
		PermDisciplineRead:     true,
		PermDutyManage:         true,
		PermGradingManage:      true,
		PermUserImpersonate:    true,
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
		PermDisciplineRead:     true,
		// PermGradingRead — Fase 15 Gap 5: HANYA kepala_sekolah (bukan
		// admin_sekolah/guru, keduanya sudah punya grading:manage yang lebih
		// luas) — akses baca modul nilai untuk kepsek.
		PermGradingRead: true,
	},
	RoleGuru: {
		PermStudentRead:          true,
		PermScheduleRead:         true,
		PermAttendanceWrite:      true,
		PermAttendanceReport:     true,
		PermTeachingJournalWrite: true,
		PermLeaveRequest:         true,
		PermLeaveApprove:         true,
		PermDisciplineRecord:     true,
		PermDisciplineRead:       true,
		PermGradingManage:        true,
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
	// RolePegawai — SENGAJA KOSONG (Fase 14 Gelombang B1, docs tugas: "TIDAK
	// dapat schedule:read dsb"). Akses pegawai HANYA lewat endpoint
	// auth-only (announcements aktif, notifications, GET /api/me) DAN
	// capability flags modul duty (mis. exit_security Gelombang B2) — bukan
	// permission RBAC.
	RolePegawai: {},
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
