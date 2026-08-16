package studentleave

import (
	"net/http"

	"github.com/omanjaya/nouschool/internal/platform/middleware"
)

// RegisterRoutes memasang route modul studentleave. Otorisasi granular
// (siswa/ortu mengajukan utk diri sendiri/anak, reviewer via flag,
// scope=all butuh student:manage, pemilik/reviewer/admin utk lampiran &
// surat) SELURUHNYA di Service — pola yang sama dengan internal/leave
// (mux hanya requireAuth, kecuali endpoint publik TANPA auth sama sekali).
func RegisterRoutes(
	mux *http.ServeMux,
	h *Handler,
	requireAuth middleware.Middleware,
	requirePerm func(perm string) middleware.Middleware,
) {
	auth := func(hf http.HandlerFunc) http.Handler { return requireAuth(hf) }

	mux.Handle("POST /api/student-leave", auth(h.SubmitRequest))
	mux.Handle("GET /api/student-leave", auth(h.ListRequests))
	mux.Handle("POST /api/student-leave/{id}/review", auth(h.ReviewHomeroom))
	mux.Handle("POST /api/student-leave/{id}/issue", auth(h.IssueBK))
	mux.Handle("POST /api/student-leave/{id}/cancel", auth(h.CancelRequest))
	mux.Handle("GET /api/student-leave/{id}/attachment", auth(h.Attachment))
	mux.Handle("GET /api/student-leave/{id}/letter/pdf", auth(h.LetterPDF))

	// PUBLIK (host tenant, TANPA requireAuth) — verifikasi surat via QR/tautan.
	mux.HandleFunc("GET /api/public/leave-verify", h.PublicVerify)
}
