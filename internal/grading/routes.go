package grading

import (
	"net/http"

	"github.com/omanjaya/nouschool/internal/platform/middleware"
)

// RegisterRoutes memasang route modul grading. Middleware auth/permission
// disuntik dari main.go (diimplementasikan modul identity) — grading TIDAK
// mengimpor identity secara langsung (lihat aturan wiring di CLAUDE.md).
//
// Keputusan dilaporkan (docs tugas: "grading:manage saja + kepsek TIDAK —
// bisa nyusul"): GET /api/grading/components/recap/stars/export SEMUA
// digerbang requirePerm(grading:manage) SAJA — kepala_sekolah TIDAK diberi
// akses baca pada sesi ini (bisa ditambah permission read terpisah nanti
// bila dibutuhkan).
func RegisterRoutes(
	mux *http.ServeMux,
	h *Handler,
	requireAuth middleware.Middleware,
	requirePerm func(perm string) middleware.Middleware,
) {
	auth := func(hf http.HandlerFunc) http.Handler { return requireAuth(hf) }
	manage := func(hf http.HandlerFunc) http.Handler { return requireAuth(requirePerm(PermGradingManage)(hf)) }

	// Status — SEMUA role login (dipakai klien mengecek nyala/tidaknya modul
	// SEBELUM menampilkan menu), TANPA requirePerm & TANPA guard
	// requireEnabled (Service.Status justru dipakai mengecek status itu).
	mux.Handle("GET /api/grading/status", auth(h.Status))

	mux.Handle("GET /api/grading/components", manage(h.ListComponents))
	mux.Handle("POST /api/grading/components", manage(h.CreateComponent))
	mux.Handle("PATCH /api/grading/components/{id}", manage(h.UpdateComponent))
	mux.Handle("DELETE /api/grading/components/{id}", manage(h.DeleteComponent))

	mux.Handle("GET /api/grading/components/{id}/grades", manage(h.GetGrades))
	mux.Handle("PUT /api/grading/components/{id}/grades", manage(h.PutGrades))

	mux.Handle("GET /api/grading/recap", manage(h.Recap))
	mux.Handle("PUT /api/grading/publication", manage(h.PutPublication))
	mux.Handle("GET /api/grading/export", manage(h.ExportXLSX))

	// my-grades — siswa sendiri/orang tua anaknya, object-level PENUH di
	// Service.MyGrades (TANPA requirePerm — pola yang sama dengan
	// attendance.StudentHistory/discipline.StudentDiscipline).
	mux.Handle("GET /api/my-grades", auth(h.MyGrades))

	mux.Handle("POST /api/grading/stars", manage(h.CreateStar))
	mux.Handle("DELETE /api/grading/stars/{id}", manage(h.DeleteStar))
	mux.Handle("GET /api/grading/stars", manage(h.ListStars))
	mux.Handle("GET /api/my-stars", auth(h.MyStars))
}
