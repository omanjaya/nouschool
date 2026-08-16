package substitution

import (
	"net/http"

	"github.com/omanjaya/nouschool/internal/platform/middleware"
)

// RegisterRoutes memasang route modul substitution. Otorisasi granular
// (guru pemilik slot / pengganti diminta / scope=all butuh schedule:manage)
// SELURUHNYA di Service — mux hanya requireAuth (pola sama internal/studentleave).
func RegisterRoutes(mux *http.ServeMux, h *Handler, requireAuth middleware.Middleware) {
	auth := func(hf http.HandlerFunc) http.Handler { return requireAuth(hf) }

	mux.Handle("POST /api/substitutions", auth(h.Request))
	mux.Handle("GET /api/substitutions", auth(h.List))
	mux.Handle("POST /api/substitutions/{id}/accept", auth(h.Accept))
	mux.Handle("POST /api/substitutions/{id}/reject", auth(h.Reject))
	mux.Handle("POST /api/substitutions/{id}/cancel", auth(h.Cancel))
}
