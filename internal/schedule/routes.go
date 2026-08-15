package schedule

import (
	"net/http"

	"github.com/omanjaya/nouschool/internal/platform/middleware"
)

// RegisterRoutes memasang route modul schedule. Middleware auth/permission
// disuntik dari main.go (diimplementasikan modul identity) — schedule TIDAK
// mengimpor identity secara langsung (lihat aturan wiring di CLAUDE.md).
func RegisterRoutes(
	mux *http.ServeMux,
	h *Handler,
	requireAuth middleware.Middleware,
	requirePerm func(perm string) middleware.Middleware,
) {
	read := func(hf http.HandlerFunc) http.Handler { return requireAuth(requirePerm(PermScheduleRead)(hf)) }
	manage := func(hf http.HandlerFunc) http.Handler { return requireAuth(requirePerm(PermScheduleManage)(hf)) }

	// -- periods --
	mux.Handle("GET /api/periods", read(h.ListPeriods))
	mux.Handle("PUT /api/periods", manage(h.ReplacePeriods))

	// -- rooms -- (qr_token di response GET hanya utk schedule:manage — lihat
	// Service.ListRooms; endpoint QR/regenerate/tulis tetap butuh schedule:manage)
	mux.Handle("GET /api/rooms", read(h.ListRooms))
	mux.Handle("POST /api/rooms", manage(h.CreateRoom))
	mux.Handle("PATCH /api/rooms/{id}", manage(h.UpdateRoom))
	mux.Handle("DELETE /api/rooms/{id}", manage(h.DeleteRoom))
	mux.Handle("POST /api/rooms/{id}/regenerate-qr", manage(h.RegenerateRoomQR))
	mux.Handle("GET /api/rooms/{id}/qr.png", manage(h.RoomQRPNG))

	// -- slots -- (GET sengaja hanya requirePerm schedule:read: guru/siswa
	// boleh baca; object-level siswa hanya rombelnya sendiri ditegakkan di
	// Service.ListSlots — docs/04-schedule.md)
	mux.Handle("GET /api/schedule/slots", read(h.ListSlots))
	mux.Handle("POST /api/schedule/slots", manage(h.CreateSlot))
	mux.Handle("PATCH /api/schedule/slots/{id}", manage(h.UpdateSlot))
	mux.Handle("DELETE /api/schedule/slots/{id}", manage(h.DeleteSlot))

	// -- copy --
	mux.Handle("POST /api/schedule/copy", manage(h.CopySchedule))

	// -- query kunci (dipakai fase 6/7) --
	mux.Handle("GET /api/schedule/today", read(h.Today))
	mux.Handle("GET /api/schedule/current-period", read(h.CurrentPeriod))

	// -- import --
	mux.Handle("GET /api/import/schedule/template", manage(h.ScheduleImportTemplate))
	mux.Handle("POST /api/import/schedule", manage(h.PreviewScheduleImport))
	mux.Handle("POST /api/import/schedule/commit", manage(h.CommitScheduleImport))
}
