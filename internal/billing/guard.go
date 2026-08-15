package billing

import (
	"net/http"
	"strings"

	"github.com/omanjaya/nouschool/internal/platform/httpx"
	"github.com/omanjaya/nouschool/internal/platform/middleware"
	"github.com/omanjaya/nouschool/internal/platform/reqctx"
)

// ErrReadonly — 402, langganan sekolah readonly (docs/09-billing.md
// "readonly ... semua mutasi non-billing ditolak").
var ErrReadonly = &httpx.Error{
	Status:  http.StatusPaymentRequired,
	Code:    "subscription_readonly",
	Message: "Langganan sekolah berakhir. Perpanjang untuk kembali menginput data.",
}

// ErrFeatureUnavailable — 402, fitur tidak aktif di snapshot langganan.
var ErrFeatureUnavailable = &httpx.Error{
	Status:  http.StatusPaymentRequired,
	Code:    "feature_unavailable",
	Message: "Fitur ini tidak tersedia di paket langganan sekolah Anda.",
}

// exemptPrefixes — path yang TETAP boleh diakses walau status readonly
// (docs/01-tenant.md "readonly -> tolak semua mutasi non-billing"):
//   - /api/auth/  — login/logout tetap harus bisa (mis. admin login utk bayar).
//   - /api/billing/ — endpoint billing sendiri (upload bukti, bayar) TIDAK
//     boleh ikut terkunci, atau sekolah readonly tidak akan pernah bisa
//     membayar lagi.
//   - /api/webhooks/ — webhook gateway TIDAK terikat mutasi sekolah tertentu
//     via reqctx (school_id diresolusi dari invoice) dan HARUS tetap bisa
//     mengaktivasi langganan sekolah yang justru sedang readonly (keputusan
//     implementasi, konsisten dengan tujuan endpoint /api/billing di atas —
//     lihat laporan akhir tugas).
var exemptPrefixes = []string{"/api/auth/", "/api/billing/", "/api/webhooks/"}

func isExemptPath(path string) bool {
	for _, p := range exemptPrefixes {
		if strings.HasPrefix(path, p) {
			return true
		}
	}
	return false
}

// Middleware adalah SubscriptionGuard (docs/09-billing.md & docs/01-tenant.md
// "Kaitan subscription"): dipasang di chain SETELAH ResolveTenant, SEBELUM
// routing. Hanya menggerbang request MUTASI (non-GET/HEAD/OPTIONS) di host
// tenant; host platform (panel super admin) tidak pernah digerbang di sini.
func (s *Service) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		if reqctx.IsPlatform(ctx) {
			next.ServeHTTP(w, r)
			return
		}
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			next.ServeHTTP(w, r)
			return
		}
		if isExemptPath(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}
		sch, ok := reqctx.SchoolFromContext(ctx)
		if !ok {
			next.ServeHTTP(w, r)
			return
		}
		snap := s.snapshotFor(ctx, sch.ID)
		if snap.status == StatusReadonly {
			httpx.WriteError(w, ErrReadonly)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RequireFeature adalah middleware per-route (docs/09-billing.md "Feature
// gating"): menolak (402) bila fitur key TIDAK aktif di snapshot langganan
// sekolah host ini. Dipasang di routes.go modul lain (tv/board, self-checkin,
// qr-cards) lewat instance *billing.Service yang di-inject dari main.go.
func (s *Service) RequireFeature(key string) middleware.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			if reqctx.IsPlatform(ctx) {
				next.ServeHTTP(w, r)
				return
			}
			sch, ok := reqctx.SchoolFromContext(ctx)
			if !ok {
				next.ServeHTTP(w, r)
				return
			}
			snap := s.snapshotFor(ctx, sch.ID)
			if !snap.features[key] {
				httpx.WriteError(w, ErrFeatureUnavailable)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
