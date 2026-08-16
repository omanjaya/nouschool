package identity

import (
	"net/http"

	"github.com/omanjaya/nouschool/internal/platform/middleware"
)

// RegisterRoutes memasang route modul identity.
func RegisterRoutes(mux *http.ServeMux, h *Handler, requireAuth, requireSuperAdmin middleware.Middleware) {
	mux.HandleFunc("POST /api/auth/login", h.Login)
	mux.HandleFunc("POST /api/auth/logout", h.Logout)
	mux.Handle("GET /api/me", requireAuth(http.HandlerFunc(h.Me)))

	// Impersonation (fase 13, docs/11-superadmin.md "Support"):
	//   - issue: host platform, super admin saja.
	//   - exchange: PUBLIK (belum ada sesi sekolah saat dipanggil), host
	//     tenant — validasi token/schoolID/expiry ada di ExchangeImpersonation,
	//     BUKAN di sini.
	mux.Handle("POST /api/admin/schools/{id}/impersonate", requireAuth(requireSuperAdmin(http.HandlerFunc(h.AdminIssueImpersonation))))
	mux.HandleFunc("POST /api/auth/impersonate", h.ImpersonateExchange)

	// Fase 13, docs/11-superadmin.md P4 "Operasional" — host platform, super
	// admin saja. Ditempatkan di modul identity (bukan modul agregator baru
	// platformadmin) karena hanya menyentuh tabel identity sendiri (lihat
	// catatan desain di admin.go).
	admin := func(hf http.HandlerFunc) http.Handler { return requireAuth(requireSuperAdmin(hf)) }
	mux.Handle("GET /api/admin/schools/{id}/members", admin(h.AdminListMembers))
	mux.Handle("POST /api/admin/users/{id}/reset-password", admin(h.AdminResetPassword))
	mux.Handle("GET /api/admin/schools/{id}/audit", admin(h.AdminAuditLog))
}
