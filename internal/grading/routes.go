package grading

import (
	"net/http"

	"github.com/omanjaya/nouschool/internal/platform/middleware"
)

// RegisterRoutes memasang route modul grading. Middleware auth/permission
// disuntik dari main.go (diimplementasikan modul identity) — grading TIDAK
// mengimpor identity secara langsung (lihat aturan wiring di CLAUDE.md).
//
// Fase 15 GAP 5 (docs tugas, MENGGANTI keputusan Fase 14 di atas): endpoint
// READ (components GET, grades GET, recap GET, report/analysis GET)
// SEKARANG hanya digerbang requireAuth di sini — pengecekan permission
// (grading:manage ATAU grading:read) DIPINDAH ke Service.requireReadAccess
// supaya kepala_sekolah (grading:read SAJA, TANPA grading:manage) tetap bisa
// lolos middleware dan sampai ke service. Endpoint MUTASI (POST/PATCH/PUT/
// DELETE) TETAP digerbang requirePerm(grading:manage) di sini seperti
// sebelumnya — stars GET & report/tp-mappings & report/manual-scores &
// report/export SENGAJA TETAP manage-only (docs tugas: "stars GET? TIDAK").
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

	mux.Handle("GET /api/grading/components", auth(h.ListComponents))
	mux.Handle("POST /api/grading/components", manage(h.CreateComponent))
	mux.Handle("PATCH /api/grading/components/{id}", manage(h.UpdateComponent))
	mux.Handle("DELETE /api/grading/components/{id}", manage(h.DeleteComponent))

	mux.Handle("GET /api/grading/components/{id}/grades", auth(h.GetGrades))
	mux.Handle("PUT /api/grading/components/{id}/grades", manage(h.PutGrades))

	mux.Handle("GET /api/grading/recap", auth(h.Recap))
	mux.Handle("PUT /api/grading/publication", manage(h.PutPublication))
	mux.Handle("GET /api/grading/export", manage(h.ExportXLSX))

	// report — Fase 15 GAP 1 (rapor lanjutan). tp-mappings & manual-scores
	// (GET+PUT) & report/export TETAP manage-only + object-level (docs tugas
	// tidak memasukkannya ke daftar grading:read); report/analysis GET
	// MENGIKUTI guard components/recap (grading:manage ATAU grading:read).
	mux.Handle("GET /api/grading/report/tp-mappings", manage(h.GetTPMappings))
	mux.Handle("PUT /api/grading/report/tp-mappings", manage(h.PutTPMappings))
	mux.Handle("GET /api/grading/report/manual-scores", manage(h.GetManualScores))
	mux.Handle("PUT /api/grading/report/manual-scores", manage(h.PutManualScores))
	mux.Handle("GET /api/grading/report/analysis", auth(h.ReportAnalysis))
	mux.Handle("GET /api/grading/report/export", manage(h.ReportExportXLSX))

	// my-grades — siswa sendiri/orang tua anaknya, object-level PENUH di
	// Service.MyGrades (TANPA requirePerm — pola yang sama dengan
	// attendance.StudentHistory/discipline.StudentDiscipline).
	mux.Handle("GET /api/my-grades", auth(h.MyGrades))

	mux.Handle("POST /api/grading/stars", manage(h.CreateStar))
	mux.Handle("DELETE /api/grading/stars/{id}", manage(h.DeleteStar))
	mux.Handle("GET /api/grading/stars", manage(h.ListStars))
	mux.Handle("GET /api/my-stars", auth(h.MyStars))
}
