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

		// Fase 14 Gelombang D: sesi impersonasi USER (impersonator_user_id
		// terisi) TIDAK PERNAH diperpanjang sliding — TTL 1 jam keras sejak
		// ImpersonateUser membuatnya, apa pun aktivitasnya (lihat
		// impersonation_user.go). Beda dari sesi biasa (termasuk sesi
		// impersonasi sekolah fase 13, yang memakai sentinel role
		// impersonationSessionRole -> sessionRenewWindowForRole sudah
		// mengembalikan 0 utk sentinel itu juga, jadi kondisi ini konsisten
		// dgn keduanya tanpa perlu cek terpisah).
		if sess.ImpersonatorUserID == nil && sess.ExpiresAt.Sub(now) < sessionRenewWindowForRole(sess.Role) {
			newExpiry := now.Add(sessionTTLForRole(sess.Role))
			if err := s.repo.ExtendSession(ctx, sess.ID, newExpiry); err == nil {
				setSessionCookie(w, cookie.Value, newExpiry, s.cookieSecure)
			}
		}

		// effectiveRole: sess.Role di DB bisa berupa sentinel
		// impersonationSessionRole (fase 13, lihat impersonation.go) —
		// diterjemahkan KEMBALI ke RoleAdminSekolah di sini supaya
		// RequirePerm/HasPermission (rbac.go) tetap jalan normal seolah
		// admin_sekolah biasa. Sentinel HANYA dipakai untuk mengecualikan
		// sesi ini dari sliding renewal generik di atas (bukan role RBAC
		// sungguhan, sengaja tidak pernah masuk reqctx/rolePermissions).
		effectiveRole := sess.Role
		if effectiveRole == impersonationSessionRole {
			effectiveRole = RoleAdminSekolah
		}
		ctx = reqctx.WithUser(ctx, sess.UserID, effectiveRole, sess.IsSuperAdmin)
		// Fase 14 Gelombang D: tandai konteks sebagai sesi impersonasi USER
		// supaya Service.Me bisa menyusun field impersonated_by (docs tugas).
		if sess.ImpersonatorUserID != nil {
			ctx = reqctx.WithImpersonator(ctx, *sess.ImpersonatorUserID)
		}
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequirePerm menolak request (403) bila role sesi tidak punya permission
// tersebut. Super admin di host platform lolos semua permission. Object-level
// check TETAP di service layer masing-masing modul.
//
// Fase 15 Gap 2 (docs/12-sion-parity.md "matrix permission per role per
// sekolah, override"): di host TENANT (reqctx.SchoolID != 0), gerbang ini
// mengecek dulu school_role_permissions (override per sekolah, cache
// in-memory TTL 60 detik — lihat effectivePermission/roleOverrideCache di
// permoverride.go), baru fallback ke peta statis rolePermissions (rbac.go)
// bila sekolah itu tidak punya baris override utk (role,permission) ini.
// Host platform (tanpa sekolah) TIDAK PERNAH kena override — selalu peta
// statis, sesuai desain (override adalah pengaturan PER SEKOLAH).
func (s *Service) RequirePerm(perm string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			if reqctx.IsPlatform(ctx) && reqctx.IsSuperAdmin(ctx) {
				next.ServeHTTP(w, r)
				return
			}
			role := reqctx.Role(ctx)
			allowed := HasPermission(role, perm)
			if schoolID := reqctx.SchoolID(ctx); schoolID != 0 {
				allowed = s.effectivePermission(ctx, schoolID, role, perm)
			}
			if !allowed {
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
