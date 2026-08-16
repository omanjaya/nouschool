package attendance

import (
	"net/http"

	"github.com/omanjaya/nouschool/internal/platform/middleware"
)

// RegisterRoutes memasang route modul attendance. Middleware auth/permission
// disuntik dari main.go (diimplementasikan modul identity) — attendance TIDAK
// mengimpor identity secara langsung (lihat aturan wiring di CLAUDE.md).
// requireQRCardFeature/requireSelfCheckinFeature (fase 10, docs/09-billing.md
// "Feature gate ... terapkan sekarang ke ... self-checkin endpoints
// (self_checkin), qr-cards endpoints (qr_card)") — dibangun main.go lewat
// billingSvc.RequireFeature(billing.FeatureQRCard/FeatureSelfCheckin) dan
// disuntik di sini sebagai middleware.Middleware SIAP PAKAI supaya
// attendance TIDAK perlu mengimpor billing untuk konstanta kunci fitur.
func RegisterRoutes(
	mux *http.ServeMux,
	h *Handler,
	requireAuth middleware.Middleware,
	requirePerm func(perm string) middleware.Middleware,
	requireQRCardFeature middleware.Middleware,
	requireSelfCheckinFeature middleware.Middleware,
) {
	auth := func(hf http.HandlerFunc) http.Handler { return requireAuth(hf) }
	write := func(hf http.HandlerFunc) http.Handler { return requireAuth(requirePerm(PermAttendanceWrite)(hf)) }
	report := func(hf http.HandlerFunc) http.Handler { return requireAuth(requirePerm(PermAttendanceReport)(hf)) }
	// manage — QR kartu siswa (Fase 8) digerbang student:manage (mengelola
	// kartu = mengelola data siswa, docs/05-attendance.md) + fitur qr_card
	// (Fase 10, docs/09-billing.md).
	manage := func(hf http.HandlerFunc) http.Handler {
		return requireAuth(requirePerm(PermStudentManage)(requireQRCardFeature(hf)))
	}
	selfCheckin := func(hf http.HandlerFunc) http.Handler {
		return requireAuth(requirePerm(PermAttendanceSelfCheck)(requireSelfCheckinFeature(hf)))
	}

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
	// Export Excel rekap bulanan (Fase 11).
	mux.Handle("GET /api/attendance/export", report(h.ExportMonthlyXLSX))

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
	// Fase 14 Gelombang D (docs/12-sion-parity.md) — object-level SAMA dengan
	// StudentAttendance di atas (attendance:report ATAU siswa/orang tua sendiri).
	mux.Handle("GET /api/students/{id}/attendance/calendar", auth(h.StudentAttendanceCalendar))
}
