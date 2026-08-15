package notification

import (
	"net/http"

	"github.com/omanjaya/nouschool/internal/platform/middleware"
)

// RegisterRoutes memasang route modul notification. Semua endpoint HANYA
// butuh requireAuth (siapa pun yang login, apa pun rolenya — docs fase 9
// "Endpoint tenant (auth semua role)") — TIDAK ada requirePerm karena inbox
// & push subscription selalu milik diri sendiri (reqctx.UserID), tidak ada
// akses lintas user. Middleware auth disuntik dari main.go (diimplementasikan
// modul identity) — notification TIDAK mengimpor identity secara langsung
// (lihat aturan wiring di CLAUDE.md).
func RegisterRoutes(mux *http.ServeMux, h *Handler, requireAuth middleware.Middleware) {
	auth := func(hf http.HandlerFunc) http.Handler { return requireAuth(hf) }

	mux.Handle("GET /api/notifications", auth(h.ListNotifications))
	mux.Handle("POST /api/notifications/read", auth(h.MarkRead))
	mux.Handle("GET /api/push/public-key", auth(h.PublicKey))
	mux.Handle("POST /api/push/subscribe", auth(h.Subscribe))
	mux.Handle("POST /api/push/unsubscribe", auth(h.Unsubscribe))
}
