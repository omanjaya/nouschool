package attendance

import (
	"net/http"

	"github.com/omanjaya/nouschool/internal/platform/middleware"
)

// RegisterRoutes memasang route modul attendance. Middleware auth/permission
// disuntik dari main.go (diimplementasikan modul identity) — attendance TIDAK
// mengimpor identity secara langsung (lihat aturan wiring di CLAUDE.md).
func RegisterRoutes(
	mux *http.ServeMux,
	h *Handler,
	requireAuth middleware.Middleware,
	requirePerm func(perm string) middleware.Middleware,
) {
	auth := func(hf http.HandlerFunc) http.Handler { return requireAuth(hf) }
	write := func(hf http.HandlerFunc) http.Handler { return requireAuth(requirePerm(PermAttendanceWrite)(hf)) }
	report := func(hf http.HandlerFunc) http.Handler { return requireAuth(requirePerm(PermAttendanceReport)(hf)) }

	mux.Handle("GET /api/attendance/classes", write(h.ListClasses))
	mux.Handle("GET /api/attendance/slots-today", write(h.SlotsToday))
	mux.Handle("POST /api/attendance/sessions", write(h.CreateSession))
	// GetSession sengaja hanya requireAuth: otorisasi (attendance:write ATAU
	// attendance:report) ditegakkan sepenuhnya di Service.GetSession/checkSessionAccess.
	mux.Handle("GET /api/attendance/sessions/{id}", auth(h.GetSession))
	mux.Handle("PUT /api/attendance/sessions/{id}/records", write(h.UpdateRecords))
	mux.Handle("POST /api/attendance/sessions/{id}/finalize", write(h.Finalize))
	mux.Handle("GET /api/attendance/summary", report(h.Summary))

	// StudentAttendance sengaja hanya requireAuth: otorisasi (attendance:report
	// ATAU object-level siswa/orang tua) ditegakkan di Service.StudentHistory.
	mux.Handle("GET /api/students/{id}/attendance", auth(h.StudentAttendance))
}
