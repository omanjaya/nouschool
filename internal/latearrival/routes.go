package latearrival

import (
	"net/http"

	"github.com/omanjaya/nouschool/internal/platform/middleware"
)

// RegisterRoutes memasang route modul latearrival. Otorisasi granular
// (siswa/scope/perm) SELURUHNYA di Service — mux hanya requireAuth, pola
// sama internal/studentleave & internal/exitpermit.
func RegisterRoutes(mux *http.ServeMux, h *Handler, requireAuth middleware.Middleware) {
	auth := func(hf http.HandlerFunc) http.Handler { return requireAuth(hf) }

	mux.Handle("POST /api/late-arrivals/scan", auth(h.Scan))
	mux.Handle("GET /api/late-arrivals", auth(h.ListRecords))
	mux.Handle("GET /api/late-arrivals/summary", auth(h.Summary))
}
