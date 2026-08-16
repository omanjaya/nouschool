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
}
