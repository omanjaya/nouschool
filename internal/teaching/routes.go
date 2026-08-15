package teaching

import (
	"net/http"

	"github.com/omanjaya/nouschool/internal/platform/middleware"
)

// RegisterRoutes memasang route modul teaching. Middleware auth/permission
// disuntik dari main.go (diimplementasikan modul identity) — teaching TIDAK
// mengimpor identity secara langsung (lihat aturan wiring di CLAUDE.md).
func RegisterRoutes(
	mux *http.ServeMux,
	h *Handler,
	requireAuth middleware.Middleware,
	requirePerm func(perm string) middleware.Middleware,
) {
	auth := func(hf http.HandlerFunc) http.Handler { return requireAuth(hf) }
	write := func(hf http.HandlerFunc) http.Handler { return requireAuth(requirePerm(PermTeachingJournalWrite)(hf)) }
	monitor := func(hf http.HandlerFunc) http.Handler { return requireAuth(requirePerm(PermTeachingMonitor)(hf)) }

	mux.Handle("POST /api/teaching/scan", write(h.Scan))
	mux.Handle("POST /api/teaching/journals", write(h.CreateUnscheduled))
	mux.Handle("PATCH /api/teaching/journals/{id}", write(h.UpdateJournal))
	mux.Handle("POST /api/teaching/journals/{id}/end", write(h.EndJournal))
	// ListJournals sengaja hanya requireAuth: scope=mine butuh profil guru
	// (dicek Service.ListJournals via MyTeacherID), scope=all butuh
	// teaching:monitor (dicek Service.ListJournals) — otorisasi campuran,
	// pola yang sama dengan attendance.GetSession/StudentAttendance.
	mux.Handle("GET /api/teaching/journals", auth(h.ListJournals))
	mux.Handle("GET /api/teaching/status", monitor(h.Status))
}
