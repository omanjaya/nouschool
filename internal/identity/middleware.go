package identity

import (
	"net/http"
	"time"

	"github.com/omanjaya/nouschool/internal/platform/httpx"
	"github.com/omanjaya/nouschool/internal/platform/reqctx"
)

// RequireAuth memuat sesi dari cookie ns_session ke request context
// (reqctx.UserID/Role/IsSuperAdmin). Sesi HARUS cocok dengan konteks host
// saat ini (hasil ResolveTenant yang berjalan lebih dulu di chain):
//   - host platform -> hanya sesi super admin (school_id NULL) yang lolos.
//   - host tenant -> school_id sesi harus sama dengan sekolah host ini.
//
// Sliding expiry: bila sisa masa berlaku < 15 hari, sesi diperpanjang +30 hari.
func (s *Service) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(sessionCookieName)
		if err != nil || cookie.Value == "" {
			httpx.WriteError(w, httpx.ErrUnauthorized)
			return
		}
		hash, err := hashToken(cookie.Value)
		if err != nil {
			httpx.WriteError(w, httpx.ErrUnauthorized)
			return
		}

		ctx := r.Context()
		sess, err := s.repo.SessionByTokenHash(ctx, hash)
		if err != nil {
			httpx.WriteError(w, httpx.ErrUnauthorized)
			return
		}

		now := time.Now()
		if now.After(sess.ExpiresAt) {
			_ = s.repo.DeleteSessionByTokenHash(ctx, hash)
			httpx.WriteError(w, httpx.ErrUnauthorized)
			return
		}

		if reqctx.IsPlatform(ctx) {
			if sess.SchoolID != nil || !sess.IsSuperAdmin {
				httpx.WriteError(w, httpx.ErrUnauthorized)
				return
			}
		} else {
			curSchoolID := reqctx.SchoolID(ctx)
			if sess.SchoolID == nil || *sess.SchoolID != curSchoolID {
				httpx.WriteError(w, httpx.ErrUnauthorized)
				return
			}
		}

		if sess.ExpiresAt.Sub(now) < sessionRenewWindow {
			newExpiry := now.Add(sessionTTL)
			if err := s.repo.ExtendSession(ctx, sess.ID, newExpiry); err == nil {
				setSessionCookie(w, cookie.Value, newExpiry, s.cookieSecure)
			}
		}

		ctx = reqctx.WithUser(ctx, sess.UserID, sess.Role, sess.IsSuperAdmin)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequirePerm menolak request (403) bila role sesi tidak punya permission
// tersebut (peta statis rolePermissions — lihat rbac.go). Super admin di
// host platform lolos semua permission. Object-level check TETAP di service
// layer masing-masing modul.
func (s *Service) RequirePerm(perm string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			if reqctx.IsPlatform(ctx) && reqctx.IsSuperAdmin(ctx) {
				next.ServeHTTP(w, r)
				return
			}
			if !HasPermission(reqctx.Role(ctx), perm) {
				httpx.WriteError(w, httpx.ErrForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequireSuperAdmin menolak request (403) kecuali konteks platform +
// is_super_admin. Dipasang SETELAH RequireAuth di chain (butuh reqctx terisi).
func (s *Service) RequireSuperAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		if !reqctx.IsPlatform(ctx) || !reqctx.IsSuperAdmin(ctx) {
			httpx.WriteError(w, httpx.ErrForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}
