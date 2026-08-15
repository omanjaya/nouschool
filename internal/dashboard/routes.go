package dashboard

import (
	"net/http"

	"github.com/omanjaya/nouschool/internal/platform/middleware"
)

// RegisterRoutes memasang route modul dashboard. Middleware auth/permission
// disuntik dari main.go (diimplementasikan modul identity) — dashboard TIDAK
// mengimpor identity secara langsung (lihat aturan wiring di CLAUDE.md).
// requireTVFeature (fase 10, docs/09-billing.md "Feature gate ... terapkan
// sekarang ke /api/tv/board (tv_dashboard)") — dibangun main.go lewat
// billingSvc.RequireFeature(billing.FeatureTVDashboard) dan disuntik di sini
// sebagai middleware.Middleware SIAP PAKAI supaya dashboard TIDAK perlu
// mengimpor billing untuk konstanta kunci fitur.
func RegisterRoutes(
	mux *http.ServeMux,
	h *Handler,
	requireAuth middleware.Middleware,
	requirePerm func(perm string) middleware.Middleware,
	requireTVFeature middleware.Middleware,
) {
	monitor := func(hf http.HandlerFunc) http.Handler {
		return requireAuth(requirePerm(PermTeachingMonitor)(requireTVFeature(hf)))
	}

	// teaching:monitor — display/kepsek/admin (docs/02-identity.md).
	mux.Handle("GET /api/tv/board", monitor(h.Board))
}
