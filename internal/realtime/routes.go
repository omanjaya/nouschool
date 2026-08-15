package realtime

import (
	"net/http"

	"github.com/omanjaya/nouschool/internal/platform/middleware"
)

// RegisterRoutes memasang GET /api/ws di belakang requireAuth SAJA — TANPA
// requirePerm (docs tugas Fase 12: "role display BOLEH", bus event
// read-only, tidak ada aksi untuk digerbang izin lebih lanjut). requireAuth
// disuntik dari main.go (diimplementasikan modul identity) — realtime TIDAK
// mengimpor identity secara langsung (lihat aturan wiring di CLAUDE.md).
func RegisterRoutes(mux *http.ServeMux, h *Handler, requireAuth middleware.Middleware) {
	mux.Handle("GET /api/ws", requireAuth(http.HandlerFunc(h.Serve)))
}
