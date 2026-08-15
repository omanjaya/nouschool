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
	// manage — QR kartu siswa (Fase 8) digerbang student:manage (mengelola
	// kartu = mengelola data siswa, docs/05-attendance.md).
	manage := func(hf http.HandlerFunc) http.Handler { return requireAuth(requirePerm(PermStudentManage)(hf)) }
	selfCheckin := func(hf http.HandlerFunc) http.Handler { return requireAuth(requirePerm(PermAttendanceSelfCheck)(hf)) }

	mux.Handle("GET /api/attendance/classes", write(h.ListClasses))
	mux.Handle("GET /api/attendance/slots-today", write(h.SlotsToday))
	mux.Handle("POST /api/attendance/sessions", write(h.CreateSession))
	// GetSession sengaja hanya requireAuth: otorisasi (attendance:write ATAU
	// attendance:report) ditegakkan sepenuhnya di Service.GetSession/checkSessionAccess.
	mux.Handle("GET /api/attendance/sessions/{id}", auth(h.GetSession))
	mux.Handle("PUT /api/attendance/sessions/{id}/records", write(h.UpdateRecords))
	mux.Handle("POST /api/attendance/sessions/{id}/finalize", write(h.Finalize))
	mux.Handle("POST /api/attendance/sessions/{id}/scan", write(h.Scan))
	mux.Handle("GET /api/attendance/summary", report(h.Summary))
	mux.Handle("GET /api/attendance/anomalies", report(h.Anomalies))

	// QR kartu siswa (Fase 8, docs/05-attendance.md "QR kartu siswa").
	mux.Handle("POST /api/attendance/qr-cards/generate", manage(h.GenerateQRCards))
	mux.Handle("GET /api/attendance/qr-cards", manage(h.ListQRCards))
	mux.Handle("POST /api/attendance/qr-cards/{studentId}/revoke", manage(h.RevokeQRCard))
	mux.Handle("GET /api/attendance/qr-cards/{studentId}/qr.png", manage(h.QRCardPNG))

	// Self check-in siswa (Fase 8, docs/05-attendance.md "Self check-in siswa").
	mux.Handle("GET /api/attendance/self-checkin/status", selfCheckin(h.SelfCheckinStatus))
	mux.Handle("POST /api/attendance/self-checkin", selfCheckin(h.SelfCheckin))

	// StudentAttendance sengaja hanya requireAuth: otorisasi (attendance:report
	// ATAU object-level siswa/orang tua) ditegakkan di Service.StudentHistory.
	mux.Handle("GET /api/students/{id}/attendance", auth(h.StudentAttendance))
}
