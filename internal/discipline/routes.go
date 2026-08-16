package discipline

import (
	"net/http"

	"github.com/omanjaya/nouschool/internal/platform/middleware"
)

// RegisterRoutes memasang route modul discipline. Middleware auth/permission
// disuntik dari main.go (diimplementasikan modul identity) — discipline
// TIDAK mengimpor identity secara langsung (lihat aturan wiring di CLAUDE.md).
//
// Catatan bentuk URL surat: instruksi tugas menulis
// "GET /api/discipline/letters/{id}.pdf & .html" — Go net/http ServeMux
// (1.22+) MEWAJIBKAN wildcard {id} menempati SATU SEGMEN PENUH ("{id}.pdf"
// bukan pola valid: ekstensi tidak boleh menempel di segmen wildcard).
// Keputusan sendiri (dilaporkan): dipetakan sebagai segmen terpisah
// "/api/discipline/letters/{id}/pdf" & ".../html" — pola YANG SUDAH ADA di
// codebase ini untuk kasus serupa (lihat internal/schedule/routes.go
// "/api/rooms/{id}/qr.png" & internal/billing/routes.go
// "/api/billing/invoices/{id}/pdf").
func RegisterRoutes(
	mux *http.ServeMux,
	h *Handler,
	requireAuth middleware.Middleware,
	requirePerm func(perm string) middleware.Middleware,
) {
	auth := func(hf http.HandlerFunc) http.Handler { return requireAuth(hf) }
	read := func(hf http.HandlerFunc) http.Handler { return requireAuth(requirePerm(PermDisciplineRead)(hf)) }
	record := func(hf http.HandlerFunc) http.Handler { return requireAuth(requirePerm(PermDisciplineRecord)(hf)) }
	manage := func(hf http.HandlerFunc) http.Handler { return requireAuth(requirePerm(PermDisciplineManage)(hf)) }

	mux.Handle("GET /api/discipline/types", read(h.ListTypes))
	mux.Handle("POST /api/discipline/types", manage(h.CreateType))
	mux.Handle("PATCH /api/discipline/types/{id}", manage(h.UpdateType))
	mux.Handle("DELETE /api/discipline/types/{id}", manage(h.DeleteType))

	mux.Handle("GET /api/discipline/sp-settings", manage(h.GetSPSettings))
	mux.Handle("PUT /api/discipline/sp-settings", manage(h.PutSPSettings))

	mux.Handle("POST /api/discipline/records", record(h.RecordViolation))
	mux.Handle("DELETE /api/discipline/records/{id}", manage(h.DeleteRecord))
	mux.Handle("GET /api/discipline/records", read(h.ListRecords))

	// StudentDiscipline sengaja hanya requireAuth: discipline:read ATAU
	// object-level (siswa sendiri/orang tua anaknya) ditegakkan penuh di
	// Service.StudentDiscipline — pola yang sama dengan
	// attendance.StudentHistory.
	mux.Handle("GET /api/students/{id}/discipline", auth(h.StudentDiscipline))

	mux.Handle("GET /api/discipline/letters", read(h.ListLetters))
	// Letter PDF/HTML sengaja hanya requireAuth: otorisasi (discipline:read
	// ATAU siswa/orang tua pemilik) sepenuhnya di Service.getLetterAuthorized
	// — pola yang sama dengan leave.GetAttachment.
	mux.Handle("GET /api/discipline/letters/{id}/pdf", auth(h.LetterPDF))
	mux.Handle("GET /api/discipline/letters/{id}/html", auth(h.LetterHTML))

	mux.Handle("GET /api/discipline/summary", read(h.Summary))
	mux.Handle("GET /api/discipline/export", read(h.ExportXLSX))
}
