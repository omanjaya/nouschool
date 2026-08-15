package announcement

import (
	"net/http"

	"github.com/omanjaya/nouschool/internal/platform/middleware"
)

// RegisterRoutes memasang route modul announcement. Middleware auth/permission
// disuntik dari main.go (diimplementasikan modul identity) — announcement
// TIDAK mengimpor identity secara langsung (lihat aturan wiring di CLAUDE.md).
func RegisterRoutes(
	mux *http.ServeMux,
	h *Handler,
	requireAuth middleware.Middleware,
	requirePerm func(perm string) middleware.Middleware,
) {
	auth := func(hf http.HandlerFunc) http.Handler { return requireAuth(hf) }
	manage := func(hf http.HandlerFunc) http.Handler { return requireAuth(requirePerm(PermAnnouncementManage)(hf)) }

	// List sengaja hanya requireAuth: ?active=1 boleh SEMUA role (termasuk
	// akun display), tanpa "active" butuh announcement:manage — dicek di
	// Service.List (otorisasi campuran, pola sama teaching.ListJournals).
	mux.Handle("GET /api/announcements", auth(h.List))
	mux.Handle("POST /api/announcements", manage(h.Create))
	mux.Handle("PATCH /api/announcements/{id}", manage(h.Update))
	mux.Handle("DELETE /api/announcements/{id}", manage(h.Delete))
}
