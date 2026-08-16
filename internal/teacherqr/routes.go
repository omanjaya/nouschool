package teacherqr

import (
	"net/http"

	"github.com/omanjaya/nouschool/internal/platform/middleware"
)

// RegisterRoutes memasang route modul teacherqr. Otorisasi (role guru saja)
// di Service, bukan di sini — pola sama internal/studentleave.
func RegisterRoutes(mux *http.ServeMux, h *Handler, requireAuth middleware.Middleware) {
	auth := func(hf http.HandlerFunc) http.Handler { return requireAuth(hf) }
	mux.Handle("POST /api/teacher-qr", auth(h.GenerateToken))
}
