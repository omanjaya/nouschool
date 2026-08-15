package identity

import (
	"net"
	"net/http"

	"github.com/omanjaya/nouschool/internal/platform/httpx"
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
