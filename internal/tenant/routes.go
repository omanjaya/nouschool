package tenant

import (
	"net/http"

	"github.com/omanjaya/nouschool/internal/platform/middleware"
)

// RegisterRoutes memasang route modul tenant. Middleware auth/permission
// disuntik dari main.go (diimplementasikan modul identity) — tenant TIDAK
// mengimpor identity secara langsung (lihat aturan wiring di CLAUDE.md).
func RegisterRoutes(
	mux *http.ServeMux,
	h *Handler,
	requireAuth middleware.Middleware,
	requireSuperAdmin middleware.Middleware,
	requirePerm func(perm string) middleware.Middleware,
) {
	// Dipanggil Caddy (On-Demand TLS) — lihat cmd/server/main.go untuk catatan bind localhost.
	mux.HandleFunc("GET /internal/check-domain", h.CheckDomain)

	// Panel super admin — konteks platform + is_super_admin.
	adminOnly := func(hf http.HandlerFunc) http.Handler {
		return requireAuth(requireSuperAdmin(hf))
	}
	mux.Handle("GET /api/admin/schools", adminOnly(h.ListSchools))
	mux.Handle("POST /api/admin/schools", adminOnly(h.CreateSchool))
	mux.Handle("PATCH /api/admin/schools/{id}", adminOnly(h.UpdateSchool))
	mux.Handle("GET /api/admin/schools/{id}/academic-years", adminOnly(h.ListAcademicYears))
	mux.Handle("POST /api/admin/schools/{id}/academic-years", adminOnly(h.CreateAcademicYear))
	mux.Handle("POST /api/admin/schools/{id}/academic-years/{ayID}/activate", adminOnly(h.ActivateAcademicYear))
	// Settings sekolah lintas-tenant, super admin saja — dipakai module
	// "superadmin-only" (mis. "notification", lihat IsSuperAdminOnlyModule di
	// settings.go) yang TIDAK bisa diubah admin sekolah lewat
	// PUT /api/settings/{module} biasa (docs/08-notification.md).
	mux.Handle("GET /api/admin/schools/{id}/settings/{module}", adminOnly(h.AdminGetSettings))
	mux.Handle("PUT /api/admin/schools/{id}/settings/{module}", adminOnly(h.AdminPutSettings))

	// Daftar tahun ajaran sekolah sendiri (host tenant, siapa saja yang login) — untuk filter UI.
	mux.Handle("GET /api/academic-years", requireAuth(http.HandlerFunc(h.ListAcademicYearsForSchool)))

	// Settings tenant — GET siapa saja yang login di sekolah itu, PUT butuh settings:manage.
	mux.Handle("GET /api/settings/{module}", requireAuth(http.HandlerFunc(h.GetSettings)))
	mux.Handle("PUT /api/settings/{module}", requireAuth(requirePerm("settings:manage")(http.HandlerFunc(h.PutSettings))))
}
