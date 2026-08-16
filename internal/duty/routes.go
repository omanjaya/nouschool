package duty

import (
	"net/http"

	"github.com/omanjaya/nouschool/internal/platform/middleware"
)

// RegisterRoutes memasang route modul duty. SEMUA endpoint pakai gerbang
// duty:manage (admin_sekolah) — kepsek TIDAK butuh kelola tugas tambahan,
// keputusan eksplisit sesuai instruksi tugas ("kepsek tidak butuh").
func RegisterRoutes(
	mux *http.ServeMux,
	h *Handler,
	requireAuth middleware.Middleware,
	requirePerm func(perm string) middleware.Middleware,
) {
	manage := func(hf http.HandlerFunc) http.Handler { return requireAuth(requirePerm(PermDutyManage)(hf)) }

	mux.Handle("GET /api/duties/flags", manage(h.ListFlags))
	mux.Handle("GET /api/duties", manage(h.ListDuties))
	mux.Handle("POST /api/duties", manage(h.CreateDuty))
	mux.Handle("PATCH /api/duties/{id}", manage(h.UpdateDuty))
	mux.Handle("DELETE /api/duties/{id}", manage(h.DeleteDuty))
	mux.Handle("GET /api/duties/{id}/assignments", manage(h.GetAssignments))
	mux.Handle("PUT /api/duties/{id}/assignments", manage(h.PutAssignments))
}
