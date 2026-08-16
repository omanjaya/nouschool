package identity

import (
	"context"
	"sync"
	"time"

	"github.com/omanjaya/nouschool/internal/platform/httpx"
)

// Fase 15 Gap 2 (docs/12-sion-parity.md "matrix permission per role per
// sekolah, override — TANPA custom role baru"): school_role_permissions
// (migrasi 00022_role_perms_gap.sql) menyimpan PENGECUALIAN dari peta
// rolePermissions statis (rbac.go) per sekolah — bukan pengganti, bukan
// tabel role baru. RequirePerm (middleware.go) mengecek override INI dulu
// (per sekolah, cache in-memory TTL 60 detik — lihat roleOverrideCache di
// bawah, invalidate saat PUT), fallback ke HasPermission statis bila tidak
// ada baris override utk (role,permission) tsb.
//
// **Keterbatasan yang DISENGAJA & didokumentasikan (sesuai instruksi
// tugas)**: HasPermission(role, perm) paket-level (rbac.go) TETAP hanya
// membaca peta statis TANPA context/schoolID — beberapa modul memanggilnya
// LANGSUNG sebagai gerbang kasar tambahan lewat consumer-side interface
// (mis. Service.HasPermission di gateway.go, dipakai modul lain utk
// otorisasi campuran "permission ATAU object-level" di service layer
// mereka sendiri) — panggilan LANGSUNG semacam itu TIDAK ikut kena override
// sekolah (object-level shortcut, bukan lewat middleware). Gerbang UTAMA
// tiap route (RequirePerm) SELALU kena override karena request selalu
// membawa reqctx.SchoolID. Ini keterbatasan yang disengaja, bukan bug —
// dicatat di sini supaya tidak mengejutkan kalau ditemukan lagi nanti.

// overrideEditableRoles — 6 role TENANT yang tampil & bisa diedit lewat
// matrix (BUKAN pegawai: permission-nya SENGAJA selalu kosong by design,
// docs/02-identity.md, bukan kandidat override; BUKAN super_admin: lintas
// sekolah, tidak relevan di sini).
var overrideEditableRoles = []string{
	RoleAdminSekolah, RoleKepalaSekolah, RoleGuru, RoleSiswa, RoleOrangTua, RoleDisplay,
}

func isEditableRole(role string) bool {
	for _, r := range overrideEditableRoles {
		if r == role {
			return true
		}
	}
	return false
}

// PermissionCatalogItem adalah satu baris "permissions" pada
// GET /api/role-permissions — key kanonik + label singkat Indonesia.
type PermissionCatalogItem struct {
	Key   string `json:"key"`
	Label string `json:"label"`
}

// permissionCatalog — SELURUH permission kanonik (rbac.go) + label singkat
// Indonesia dipakai UI matrix. Permission baru = tambah konstanta di
// rbac.go, baris di rolePermissions, DAN baris di sini.
var permissionCatalog = []PermissionCatalogItem{
	{Key: PermStudentManage, Label: "Kelola siswa"},
	{Key: PermStudentRead, Label: "Lihat siswa"},
	{Key: PermScheduleManage, Label: "Kelola jadwal"},
	{Key: PermScheduleRead, Label: "Lihat jadwal"},
	{Key: PermAttendanceWrite, Label: "Input absensi"},
	{Key: PermAttendanceSelfCheck, Label: "Absen mandiri"},
	{Key: PermAttendanceReadOwn, Label: "Lihat absensi sendiri"},
	{Key: PermAttendanceReport, Label: "Lihat rekap absensi"},
	{Key: PermTeachingJournalWrite, Label: "Isi jurnal mengajar"},
	{Key: PermTeachingMonitor, Label: "Monitor mengajar"},
	{Key: PermLeaveRequest, Label: "Ajukan izin"},
	{Key: PermLeaveApprove, Label: "Setujui izin"},
	{Key: PermLeaveManage, Label: "Kelola izin"},
	{Key: PermSettingsManage, Label: "Kelola pengaturan"},
	{Key: PermBillingView, Label: "Lihat tagihan"},
	{Key: PermAnnouncementManage, Label: "Kelola pengumuman"},
	{Key: PermDashboardSchool, Label: "Dashboard sekolah"},
	{Key: PermDisciplineManage, Label: "Kelola kedisiplinan"},
	{Key: PermDisciplineRecord, Label: "Catat pelanggaran"},
	{Key: PermDisciplineRead, Label: "Lihat kedisiplinan"},
	{Key: PermDutyManage, Label: "Kelola tugas tambahan"},
	{Key: PermGradingManage, Label: "Kelola nilai"},
	{Key: PermGradingRead, Label: "Lihat nilai"},
	{Key: PermUserImpersonate, Label: "Impersonate user"},
}

func isKnownPermission(perm string) bool {
	for _, p := range permissionCatalog {
		if p.Key == perm {
			return true
		}
	}
	return false
}

// RolePermissionsView adalah shape response GET/PUT /api/role-permissions.
type RolePermissionsView struct {
	Roles       []string                   `json:"roles"`
	Permissions []PermissionCatalogItem    `json:"permissions"`
	Defaults    map[string]map[string]bool `json:"defaults"`
	Overrides   map[string]map[string]bool `json:"overrides"`
}

// rolePermOverrideRepo adalah kontrak minimal dibutuhkan getRolePermissions/
// putRolePermissions — dipenuhi *Repository secara struktural, dideklarasikan
// supaya keduanya bisa dites dengan fake in-memory TANPA DB (pola sama
// adminResetRepo di admin.go).
type rolePermOverrideRepo interface {
	ListRolePermissionOverrides(ctx context.Context, schoolID int64) ([]RolePermOverrideRow, error)
	ReplaceRolePermissionOverrides(ctx context.Context, schoolID int64, changes []RolePermOverrideChange) error
}

func overridesToView(rows []RolePermOverrideRow) map[string]map[string]bool {
	overrides := map[string]map[string]bool{}
	for _, row := range rows {
		if !isEditableRole(row.Role) {
			// Baris "yatim" dari role yang sekarang tidak lagi diedit (jaga-jaga,
			// seharusnya tidak pernah terjadi lewat PUT ini) — jangan bocor ke UI.
			continue
		}
		if overrides[row.Role] == nil {
			overrides[row.Role] = map[string]bool{}
		}
		overrides[row.Role][row.Permission] = row.Allowed
	}
	return overrides
}

// getRolePermissions implementasi MURNI (testable) dari
// Service.GetRolePermissions.
func getRolePermissions(ctx context.Context, repo rolePermOverrideRepo, schoolID int64) (RolePermissionsView, error) {
	rows, err := repo.ListRolePermissionOverrides(ctx, schoolID)
	if err != nil {
		return RolePermissionsView{}, err
	}
	defaults := map[string]map[string]bool{}
	for _, role := range overrideEditableRoles {
		perms := map[string]bool{}
		for _, p := range permissionCatalog {
			perms[p.Key] = HasPermission(role, p.Key)
		}
		defaults[role] = perms
	}
	return RolePermissionsView{
		Roles:       append([]string{}, overrideEditableRoles...),
		Permissions: append([]PermissionCatalogItem{}, permissionCatalog...),
		Defaults:    defaults,
		Overrides:   overridesToView(rows),
	}, nil
}

var (
	errOverrideAdminSekolahLocked   = httpx.Validation("Permission role admin_sekolah tidak bisa diubah lewat matrix ini (mencegah admin mengunci dirinya sendiri).")
	errOverrideSettingsManageLocked = httpx.Validation("Permission settings:manage tidak bisa diubah lewat matrix ini.")
)

// validateRolePermissionChanges menegakkan larangan tugas: role admin_sekolah
// & permission settings:manage TIDAK BOLEH disentuh sama sekali (berlaku
// baik untuk upsert maupun hapus override), plus role/permission harus
// dikenal.
func validateRolePermissionChanges(changes []RolePermOverrideChange) error {
	for _, c := range changes {
		if !isEditableRole(c.Role) {
			return httpx.Validation("Role tidak dikenal: " + c.Role)
		}
		if c.Role == RoleAdminSekolah {
			return errOverrideAdminSekolahLocked
		}
		if !isKnownPermission(c.Permission) {
			return httpx.Validation("Permission tidak dikenal: " + c.Permission)
		}
		if c.Permission == PermSettingsManage {
			return errOverrideSettingsManageLocked
		}
	}
	return nil
}

// flattenRoleOverrideInput meratakan body PUT {role: {perm: bool|null}}
// menjadi []RolePermOverrideChange (null -> Allowed nil = hapus).
func flattenRoleOverrideInput(overrides map[string]map[string]*bool) []RolePermOverrideChange {
	changes := make([]RolePermOverrideChange, 0)
	for role, perms := range overrides {
		for perm, allowed := range perms {
			changes = append(changes, RolePermOverrideChange{Role: role, Permission: perm, Allowed: allowed})
		}
	}
	return changes
}

// putRolePermissions implementasi MURNI (testable) dari
// Service.PutRolePermissions — TIDAK memanggil audit di sini (audit butuh
// *Service konkret sebagai AuditLogger, dipanggil pemanggil/Service wrapper
// SETELAH ini sukses, pola sama listAuditLogPage/adminResetPassword).
func putRolePermissions(ctx context.Context, repo rolePermOverrideRepo, schoolID int64, overrides map[string]map[string]*bool) (RolePermissionsView, error) {
	changes := flattenRoleOverrideInput(overrides)
	if err := validateRolePermissionChanges(changes); err != nil {
		return RolePermissionsView{}, err
	}
	if err := repo.ReplaceRolePermissionOverrides(ctx, schoolID, changes); err != nil {
		return RolePermissionsView{}, err
	}
	return getRolePermissions(ctx, repo, schoolID)
}

// GetRolePermissions — GET /api/role-permissions (host tenant, perm
// settings:manage — RequirePerm dipasang di routes.go).
func (s *Service) GetRolePermissions(ctx context.Context, schoolID int64) (RolePermissionsView, error) {
	return getRolePermissions(ctx, s.repo, schoolID)
}

// PutRolePermissions — PUT /api/role-permissions. Meng-invalidate cache
// RequirePerm (s.overrides) untuk sekolah ini SETELAH sukses supaya
// perubahan langsung berlaku tanpa menunggu TTL 60 detik.
func (s *Service) PutRolePermissions(ctx context.Context, schoolID, actorUserID int64, overrides map[string]map[string]*bool) (RolePermissionsView, error) {
	view, err := putRolePermissions(ctx, s.repo, schoolID, overrides)
	if err != nil {
		return RolePermissionsView{}, err
	}
	s.overrides.invalidate(schoolID)
	sid, uid := schoolID, actorUserID
	_ = s.Log(ctx, &sid, &uid, "admin.role_permissions_update", "school_role_permissions", nil, nil, overrides)
	return view, nil
}

// -- cache in-memory TTL 60 detik dipakai RequirePerm (middleware.go) --
// Pola sama tenant.HostResolver (resolver.go): peta per-key + RWMutex + TTL,
// now injectable untuk test (walau tidak dites langsung di sini — perilaku
// akhir dites lewat e2e curl, lihat laporan tugas; hanya
// effectivePermissionFrom murni di bawah yang dites unit).

type roleOverrideCacheEntry struct {
	data      map[string]map[string]bool // role -> permission -> allowed
	expiresAt time.Time
}

type roleOverrideCache struct {
	mu      sync.RWMutex
	ttl     time.Duration
	now     func() time.Time
	entries map[int64]roleOverrideCacheEntry
}

func newRoleOverrideCache(ttl time.Duration) *roleOverrideCache {
	return &roleOverrideCache{ttl: ttl, now: time.Now, entries: map[int64]roleOverrideCacheEntry{}}
}

func (c *roleOverrideCache) get(schoolID int64) (map[string]map[string]bool, bool) {
	c.mu.RLock()
	e, ok := c.entries[schoolID]
	c.mu.RUnlock()
	if !ok || c.now().After(e.expiresAt) {
		return nil, false
	}
	return e.data, true
}

func (c *roleOverrideCache) set(schoolID int64, data map[string]map[string]bool) {
	c.mu.Lock()
	c.entries[schoolID] = roleOverrideCacheEntry{data: data, expiresAt: c.now().Add(c.ttl)}
	c.mu.Unlock()
}

// invalidate menghapus entri cache sekolah ini — dipanggil PutRolePermissions
// setelah PUT sukses supaya perubahan langsung berlaku.
func (c *roleOverrideCache) invalidate(schoolID int64) {
	c.mu.Lock()
	delete(c.entries, schoolID)
	c.mu.Unlock()
}

// effectivePermissionFrom adalah fungsi MURNI (testable tanpa DB/cache) yang
// dipakai RequirePerm: cek override tri-state dulu (role ada di map DAN
// permission itu ada = override menang, apa pun nilainya, true ATAU false),
// baru fallback ke HasPermission statis (rbac.go) bila tidak ada baris.
func effectivePermissionFrom(overrides map[string]map[string]bool, role, perm string) bool {
	if perms, ok := overrides[role]; ok {
		if v, ok := perms[perm]; ok {
			return v
		}
	}
	return HasPermission(role, perm)
}

// effectivePermission mengecek cache (isi ulang dari repo bila kosong/basi)
// lalu delegasi ke effectivePermissionFrom murni di atas — dipakai
// RequirePerm (middleware.go). Kegagalan load override (mis. DB flapping)
// DIABAIKAN dan jatuh ke default statis — gerbang RBAC tidak boleh berhenti
// total hanya karena override gagal termuat (fail-safe ke perilaku SEBELUM
// Gap 2 ada).
func (s *Service) effectivePermission(ctx context.Context, schoolID int64, role, perm string) bool {
	data, ok := s.overrides.get(schoolID)
	if !ok {
		rows, err := s.repo.ListRolePermissionOverrides(ctx, schoolID)
		if err != nil {
			return HasPermission(role, perm)
		}
		data = overridesToView(rows)
		s.overrides.set(schoolID, data)
	}
	return effectivePermissionFrom(data, role, perm)
}
