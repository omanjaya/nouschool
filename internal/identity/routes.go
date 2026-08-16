package identity

import (
	"net/http"

	"github.com/omanjaya/nouschool/internal/platform/middleware"
)

// RegisterRoutes memasang route modul identity.
func RegisterRoutes(mux *http.ServeMux, h *Handler, requireAuth, requireSuperAdmin middleware.Middleware, requirePerm func(perm string) middleware.Middleware) {
	mux.HandleFunc("POST /api/auth/login", h.Login)
	mux.HandleFunc("POST /api/auth/logout", h.Logout)
	mux.Handle("GET /api/me", requireAuth(http.HandlerFunc(h.Me)))

	// Fase 14 Gelombang D, docs/12-sion-parity.md "Impersonate USER oleh
	// admin sekolah" — host tenant, admin_sekolah saja (permission
	// user:impersonate). Stop HANYA butuh requireAuth: Service.StopImpersonation
	// sendiri yang menolak sesi yang BUKAN sesi impersonasi (cek
	// impersonator_user_id), bukan gerbang permission role.
	mux.Handle("POST /api/users/{id}/impersonate", requireAuth(requirePerm(PermUserImpersonate)(http.HandlerFunc(h.ImpersonateUser))))
	mux.Handle("POST /api/auth/impersonation/stop", requireAuth(http.HandlerFunc(h.StopImpersonation)))

	// Impersonation (fase 13, docs/11-superadmin.md "Support"):
	//   - issue: host platform, super admin saja.
	//   - exchange: PUBLIK (belum ada sesi sekolah saat dipanggil), host
	//     tenant — validasi token/schoolID/expiry ada di ExchangeImpersonation,
	//     BUKAN di sini.
	mux.Handle("POST /api/admin/schools/{id}/impersonate", requireAuth(requireSuperAdmin(http.HandlerFunc(h.AdminIssueImpersonation))))
	mux.HandleFunc("POST /api/auth/impersonate", h.ImpersonateExchange)

	// Fase 15 Gap 2, docs/12-sion-parity.md "matrix permission per role per
	// sekolah" — host tenant, admin_sekolah (perm settings:manage, SATU
	// permission yang TIDAK BISA dioverride lewat endpoint ini sendiri,
	// lihat permoverride.go — mencegah admin mengunci diri dari editor ini).
	mux.Handle("GET /api/role-permissions", requireAuth(requirePerm(PermSettingsManage)(http.HandlerFunc(h.GetRolePermissions))))
	mux.Handle("PUT /api/role-permissions", requireAuth(requirePerm(PermSettingsManage)(http.HandlerFunc(h.PutRolePermissions))))

	// Fase 15 Gap 6, docs/12-sion-parity.md "nonaktifkan/aktifkan user" —
	// host tenant, gerbang perm SAMA dgn CRUD siswa/pegawai (student:manage,
	// lihat catatan di memberstatus.go); larangan diri sendiri/admin_sekolah/
	// super admin ditegakkan di Service.SetMemberStatus.
	mux.Handle("PATCH /api/members/{userId}/status", requireAuth(requirePerm(PermStudentManage)(http.HandlerFunc(h.SetMemberStatus))))

	// Fase 13, docs/11-superadmin.md P4 "Operasional" — host platform, super
	// admin saja. Ditempatkan di modul identity (bukan modul agregator baru
	// platformadmin) karena hanya menyentuh tabel identity sendiri (lihat
	// catatan desain di admin.go).
	admin := func(hf http.HandlerFunc) http.Handler { return requireAuth(requireSuperAdmin(hf)) }
	mux.Handle("GET /api/admin/schools/{id}/members", admin(h.AdminListMembers))
	mux.Handle("POST /api/admin/users/{id}/reset-password", admin(h.AdminResetPassword))
	mux.Handle("GET /api/admin/schools/{id}/audit", admin(h.AdminAuditLog))

	// Fase 13 Gelombang 2, docs/11-superadmin.md P2 "Onboarding" — akun admin
	// sekolah pertama. Ditempatkan di modul identity (bukan platformadmin)
	// dengan alasan yang sama dengan P4.1/4.2/4.3 (lihat catatan admin.go).
	mux.Handle("POST /api/admin/schools/{id}/admins", admin(h.AdminCreateSchoolAdmin))
}
