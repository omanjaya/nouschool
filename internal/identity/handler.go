package identity

import (
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/omanjaya/nouschool/internal/platform/httpx"
	"github.com/omanjaya/nouschool/internal/platform/reqctx"
)

// Handler menerjemahkan HTTP <-> Service.
type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

type loginRequest struct {
	Identifier string `json:"identifier"`
	Password   string `json:"password"`
}

// clientIP mengambil IP klien dari RemoteAddr (host:port) untuk rate limit.
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// Login — POST /api/auth/login.
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.WriteError(w, err)
		return
	}
	result, err := h.svc.Login(r.Context(), LoginInput{
		Identifier: req.Identifier,
		Password:   req.Password,
		IP:         clientIP(r),
		UserAgent:  r.UserAgent(),
	})
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	setSessionCookie(w, result.Token, result.ExpiresAt, h.svc.CookieSecure())
	httpx.JSON(w, http.StatusOK, map[string]any{"user": result.View})
}

// Logout — POST /api/auth/logout.
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(sessionCookieName); err == nil {
		_ = h.svc.Logout(r.Context(), cookie.Value)
	}
	clearSessionCookie(w, h.svc.CookieSecure())
	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

// Me — GET /api/me.
func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	view, err := h.svc.Me(r.Context())
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"user": view})
}

// -- impersonation (fase 13, docs/11-superadmin.md "Support") --

type impersonateIssueResponse struct {
	Token     string `json:"token"`
	Slug      string `json:"slug"`
	ExpiresAt string `json:"expires_at"`
}

// AdminIssueImpersonation — POST /api/admin/schools/{id}/impersonate (host
// platform, RequireAuth+RequireSuperAdmin dipasang di routes.go). Membuat
// token sekali-pakai (2 menit) untuk super admin loncat ke sesi
// admin_sekolah sekolah tsb di tab baru (domain sekolah).
func (h *Handler) AdminIssueImpersonation(w http.ResponseWriter, r *http.Request) {
	schoolID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		httpx.WriteError(w, httpx.Validation("ID sekolah tidak valid."))
		return
	}
	ctx := r.Context()
	token, slug, expiresAt, err := h.svc.IssueImpersonation(ctx, schoolID, reqctx.UserID(ctx))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, impersonateIssueResponse{
		Token: token, Slug: slug, ExpiresAt: expiresAt.UTC().Format(time.RFC3339),
	})
}

type impersonateExchangeRequest struct {
	Token string `json:"token"`
}

// ImpersonateExchange — POST /api/auth/impersonate (PUBLIK — tidak butuh
// sesi, host TENANT). Menukar token sekali-pakai menjadi sesi admin_sekolah
// atas nama super admin penerbit token, di sekolah host saat ini.
func (h *Handler) ImpersonateExchange(w http.ResponseWriter, r *http.Request) {
	var req impersonateExchangeRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.WriteError(w, err)
		return
	}
	result, err := h.svc.ExchangeImpersonation(r.Context(), req.Token, clientIP(r), r.UserAgent())
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	setSessionCookie(w, result.Token, result.ExpiresAt, h.svc.CookieSecure())
	httpx.JSON(w, http.StatusOK, map[string]any{"user": result.View})
}
