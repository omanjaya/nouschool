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

// -- fase 13 Gelombang 2, docs/11-superadmin.md P2 "Onboarding" --

func pathInt64(r *http.Request, name string) (int64, error) {
	return strconv.ParseInt(r.PathValue(name), 10, 64)
}

type createSchoolAdminRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Username string `json:"username"`
}

// AdminCreateSchoolAdmin — POST /api/admin/schools/{id}/admins.
func (h *Handler) AdminCreateSchoolAdmin(w http.ResponseWriter, r *http.Request) {
	schoolID, err := pathInt64(r, "id")
	if err != nil {
		httpx.WriteError(w, httpx.Validation("ID sekolah tidak valid."))
		return
	}
	var req createSchoolAdminRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.WriteError(w, err)
		return
	}
	ctx := r.Context()
	userID, tempPassword, err := h.svc.AdminCreateSchoolAdmin(ctx, reqctx.UserID(ctx), schoolID, req.Name, req.Email, req.Username)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, map[string]any{"user_id": userID, "temp_password": tempPassword})
}

// -- fase 13, docs/11-superadmin.md P4 "Operasional" --

// AdminListMembers — GET /api/admin/schools/{id}/members.
func (h *Handler) AdminListMembers(w http.ResponseWriter, r *http.Request) {
	schoolID, err := pathInt64(r, "id")
	if err != nil {
		httpx.WriteError(w, httpx.Validation("ID sekolah tidak valid."))
		return
	}
	items, err := h.svc.ListMembers(r.Context(), schoolID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, items)
}

type adminResetPasswordRequest struct {
	SchoolID int64 `json:"school_id"`
}

// AdminResetPassword — POST /api/admin/users/{id}/reset-password.
func (h *Handler) AdminResetPassword(w http.ResponseWriter, r *http.Request) {
	userID, err := pathInt64(r, "id")
	if err != nil {
		httpx.WriteError(w, httpx.Validation("ID user tidak valid."))
		return
	}
	var req adminResetPasswordRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.WriteError(w, err)
		return
	}
	if req.SchoolID == 0 {
		httpx.WriteError(w, httpx.Validation("school_id wajib diisi."))
		return
	}
	ctx := r.Context()
	tempPassword, err := h.svc.AdminResetPassword(ctx, reqctx.UserID(ctx), req.SchoolID, userID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"temp_password": tempPassword})
}

// AdminAuditLog — GET /api/admin/schools/{id}/audit?page=&per_page=&action=.
func (h *Handler) AdminAuditLog(w http.ResponseWriter, r *http.Request) {
	schoolID, err := pathInt64(r, "id")
	if err != nil {
		httpx.WriteError(w, httpx.Validation("ID sekolah tidak valid."))
		return
	}
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	perPage, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
	action := r.URL.Query().Get("action")
	result, err := h.svc.ListAuditLog(r.Context(), schoolID, page, perPage, action)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, result)
}

// -- Fase 14 Gelombang D, docs/12-sion-parity.md "Impersonate USER oleh
// admin sekolah" (beda dari impersonation.go fase 13 "masuk sebagai
// sekolah") --

// ImpersonateUser — POST /api/users/{id}/impersonate (host tenant, role
// admin_sekolah — RequirePerm dipasang di routes.go). Cookie diganti
// LANGSUNG (admin & target sama-sama di host tenant ini).
func (h *Handler) ImpersonateUser(w http.ResponseWriter, r *http.Request) {
	targetID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		httpx.WriteError(w, httpx.Validation("ID user tidak valid."))
		return
	}
	ctx := r.Context()
	result, err := h.svc.ImpersonateUser(ctx, reqctx.UserID(ctx), reqctx.SchoolID(ctx), targetID, clientIP(r), r.UserAgent())
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	setSessionCookie(w, result.Token, result.ExpiresAt, h.svc.CookieSecure())
	httpx.JSON(w, http.StatusOK, map[string]any{"user": result.View})
}

// StopImpersonation — POST /api/auth/impersonation/stop (host tenant, sesi
// HARUS sesi impersonasi USER — Service.StopImpersonation menolak selainnya).
func (h *Handler) StopImpersonation(w http.ResponseWriter, r *http.Request) {
	var token string
	if cookie, err := r.Cookie(sessionCookieName); err == nil {
		token = cookie.Value
	}
	ctx := r.Context()
	result, err := h.svc.StopImpersonation(ctx, token, clientIP(r), r.UserAgent())
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	setSessionCookie(w, result.Token, result.ExpiresAt, h.svc.CookieSecure())
	httpx.JSON(w, http.StatusOK, map[string]any{"user": result.View})
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

// -- Fase 15 Gap 2, docs/12-sion-parity.md "matrix permission per role per
// sekolah" --

// GetRolePermissions — GET /api/role-permissions (host tenant, perm
// settings:manage ditegakkan di routes.go).
func (h *Handler) GetRolePermissions(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	view, err := h.svc.GetRolePermissions(ctx, reqctx.SchoolID(ctx))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, view)
}

type putRolePermissionsRequest struct {
	Overrides map[string]map[string]*bool `json:"overrides"`
}

// PutRolePermissions — PUT /api/role-permissions. Body {"overrides":
// {"role": {"perm": true|false|null}}} — null menghapus override (kembali
// ke default statis).
func (h *Handler) PutRolePermissions(w http.ResponseWriter, r *http.Request) {
	var req putRolePermissionsRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.WriteError(w, err)
		return
	}
	ctx := r.Context()
	view, err := h.svc.PutRolePermissions(ctx, reqctx.SchoolID(ctx), reqctx.UserID(ctx), req.Overrides)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, view)
}

// -- Fase 15 Gap 6, docs/12-sion-parity.md "nonaktifkan/aktifkan user" --

type setMemberStatusRequest struct {
	Role   string `json:"role"`
	Status string `json:"status"`
}

// SetMemberStatus — PATCH /api/members/{userId}/status (host tenant, perm
// student:manage ditegakkan di routes.go).
func (h *Handler) SetMemberStatus(w http.ResponseWriter, r *http.Request) {
	targetID, err := pathInt64(r, "userId")
	if err != nil {
		httpx.WriteError(w, httpx.Validation("ID user tidak valid."))
		return
	}
	var req setMemberStatusRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.WriteError(w, err)
		return
	}
	ctx := r.Context()
	m, err := h.svc.SetMemberStatus(ctx, reqctx.SchoolID(ctx), reqctx.UserID(ctx), targetID, req.Role, req.Status)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"user_id": m.UserID, "role": m.Role, "status": m.Status})
}
