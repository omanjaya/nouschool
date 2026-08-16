package exitpermit

import (
	"net/http"

	"github.com/omanjaya/nouschool/internal/platform/middleware"
)

// RegisterRoutes memasang route modul exitpermit. Otorisasi granular (siswa
// pemilik/scope/flag duty/admin) SELURUHNYA di Service — mux hanya
// requireAuth, pola sama internal/studentleave.
func RegisterRoutes(mux *http.ServeMux, h *Handler, requireAuth middleware.Middleware) {
	auth := func(hf http.HandlerFunc) http.Handler { return requireAuth(hf) }

	mux.Handle("POST /api/exit-permits", auth(h.SubmitRequest))
	mux.Handle("GET /api/exit-permits", auth(h.ListRequests))
	mux.Handle("POST /api/exit-permits/gate-scan", auth(h.GateScan))
	mux.Handle("GET /api/exit-permits/gate-history", auth(h.GateHistory))
	mux.Handle("POST /api/exit-permits/{id}/scan", auth(h.Scan))
	mux.Handle("POST /api/exit-permits/{id}/cancel", auth(h.CancelRequest))
	mux.Handle("POST /api/exit-permits/{id}/reject", auth(h.RejectRequest))
}
