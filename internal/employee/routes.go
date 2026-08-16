package employee

import (
	"net/http"

	"github.com/omanjaya/nouschool/internal/platform/middleware"
)

// RegisterRoutes memasang route modul employee. Perm SENGAJA `student:manage`
// (BUKAN permission baru) — lihat model.go PermManage & instruksi tugas.
func RegisterRoutes(
	mux *http.ServeMux,
	h *Handler,
	requireAuth middleware.Middleware,
	requirePerm func(perm string) middleware.Middleware,
) {
	manage := func(hf http.HandlerFunc) http.Handler { return requireAuth(requirePerm(PermManage)(hf)) }

	mux.Handle("GET /api/employees", manage(h.ListEmployees))
	mux.Handle("POST /api/employees", manage(h.CreateEmployee))
	mux.Handle("PATCH /api/employees/{id}", manage(h.UpdateEmployee))
}
