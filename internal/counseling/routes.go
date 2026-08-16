package counseling

import (
	"net/http"

	"github.com/omanjaya/nouschool/internal/platform/middleware"
)

// RegisterRoutes memasang route modul counseling. Otorisasi granular (flag
// duty leave_issuance / admin_sekolah / kepala_sekolah read-only / pemilik
// utk PATCH-DELETE) SELURUHNYA di Service — mux hanya requireAuth (pola sama
// internal/studentleave).
func RegisterRoutes(mux *http.ServeMux, h *Handler, requireAuth middleware.Middleware) {
	auth := func(hf http.HandlerFunc) http.Handler { return requireAuth(hf) }

	mux.Handle("GET /api/counselings", auth(h.ListCounselings))
	mux.Handle("POST /api/counselings", auth(h.CreateCounseling))
	mux.Handle("PATCH /api/counselings/{id}", auth(h.UpdateCounseling))
	mux.Handle("DELETE /api/counselings/{id}", auth(h.DeleteCounseling))
	mux.Handle("GET /api/counselings/{id}/evidence", auth(h.Evidence))
	mux.Handle("GET /api/counselings/{id}/report/html", auth(h.Report))
}
