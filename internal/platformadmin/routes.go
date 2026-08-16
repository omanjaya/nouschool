package platformadmin

import (
	"net/http"

	"github.com/omanjaya/nouschool/internal/platform/middleware"
)

// RegisterRoutes memasang route modul platformadmin — SEMUA host platform,
// RequireAuth+RequireSuperAdmin (docs/11-superadmin.md aturan #1).
func RegisterRoutes(mux *http.ServeMux, h *Handler, requireAuth, requireSuperAdmin middleware.Middleware) {
	admin := func(hf http.HandlerFunc) http.Handler { return requireAuth(requireSuperAdmin(hf)) }

	mux.Handle("GET /api/admin/overview", admin(h.Overview))
	mux.Handle("GET /api/admin/schools/{id}/stats", admin(h.SchoolStats))
	mux.Handle("GET /api/admin/outbox", admin(h.ListOutbox))
	mux.Handle("POST /api/admin/outbox/{id}/retry", admin(h.RetryOutbox))
	mux.Handle("POST /api/admin/outbox/retry-all", admin(h.RetryAllOutbox))

	// Fase 13 Gelombang 2 (docs/11-superadmin.md P2 onboarding + P5 pengumuman
	// platform).
	mux.Handle("GET /api/admin/schools/{id}/onboarding", admin(h.Onboarding))

	mux.Handle("GET /api/admin/platform-announcements", admin(h.ListPlatformAnnouncements))
	mux.Handle("POST /api/admin/platform-announcements", admin(h.CreatePlatformAnnouncement))
	mux.Handle("PATCH /api/admin/platform-announcements/{id}", admin(h.UpdatePlatformAnnouncement))
	mux.Handle("DELETE /api/admin/platform-announcements/{id}", admin(h.DeletePlatformAnnouncement))
}
