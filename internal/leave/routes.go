package leave

import (
	"net/http"

	"github.com/omanjaya/nouschool/internal/platform/middleware"
)

// RegisterRoutes memasang route modul leave. Middleware auth/permission
// disuntik dari main.go (diimplementasikan modul identity) — leave TIDAK
// mengimpor identity secara langsung (lihat aturan wiring di CLAUDE.md).
func RegisterRoutes(
	mux *http.ServeMux,
	h *Handler,
	requireAuth middleware.Middleware,
	requirePerm func(perm string) middleware.Middleware,
) {
	auth := func(hf http.HandlerFunc) http.Handler { return requireAuth(hf) }
	request := func(hf http.HandlerFunc) http.Handler { return requireAuth(requirePerm(PermLeaveRequest)(hf)) }
	approve := func(hf http.HandlerFunc) http.Handler { return requireAuth(requirePerm(PermLeaveApprove)(hf)) }

	mux.Handle("GET /api/leave/types", auth(h.Types))
	mux.Handle("POST /api/leave/requests", request(h.SubmitRequest))
	// ListRequests sengaja hanya requireAuth: scope=mine selalu boleh (milik
	// sendiri), scope=all butuh leave:manage — ditegakkan penuh di
	// Service.ListRequests (sama pola dengan attendance.GetSession).
	mux.Handle("GET /api/leave/requests", auth(h.ListRequests))
	mux.Handle("POST /api/leave/requests/{id}/cancel", auth(h.CancelRequest))
	mux.Handle("GET /api/leave/approvals", approve(h.ListApprovals))
	mux.Handle("POST /api/leave/approvals/{step_id}/decide", approve(h.DecideStep))
	// Summary sengaja hanya requireAuth: leave:manage ATAU role kepala_sekolah
	// — ditegakkan di Service.Summary.
	mux.Handle("GET /api/leave/summary", auth(h.Summary))
	// File lampiran: otorisasi (pengaju/approver-di-chain/leave:manage) penuh
	// di Service.GetAttachment — sama pola dengan attendance.StudentAttendance.
	mux.Handle("GET /api/files/leave/{id}/attachment", auth(h.Attachment))
}
