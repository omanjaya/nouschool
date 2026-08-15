package dashboard

import (
	"net/http"

	"github.com/omanjaya/nouschool/internal/platform/middleware"
)

// RegisterRoutes memasang route modul dashboard. Middleware auth/permission
// disuntik dari main.go (diimplementasikan modul identity) — dashboard TIDAK
// mengimpor identity secara langsung (lihat aturan wiring di CLAUDE.md).
func RegisterRoutes(
	mux *http.ServeMux,
	h *Handler,
	requireAuth middleware.Middleware,
	requirePerm func(perm string) middleware.Middleware,
) {
	monitor := func(hf http.HandlerFunc) http.Handler { return requireAuth(requirePerm(PermTeachingMonitor)(hf)) }

	// teaching:monitor — display/kepsek/admin (docs/02-identity.md).
	mux.Handle("GET /api/tv/board", monitor(h.Board))
}
