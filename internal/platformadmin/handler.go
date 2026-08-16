package platformadmin

import (
	"net/http"
	"strconv"

	"github.com/omanjaya/nouschool/internal/platform/httpx"
	"github.com/omanjaya/nouschool/internal/platform/reqctx"
)

// Handler menerjemahkan HTTP <-> Service. Semua route host platform,
// RequireAuth+RequireSuperAdmin (dipasang di routes.go).
type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func pathInt64(r *http.Request, name string) (int64, error) {
	return strconv.ParseInt(r.PathValue(name), 10, 64)
}

// Overview — GET /api/admin/overview (P1).
func (h *Handler) Overview(w http.ResponseWriter, r *http.Request) {
	view, err := h.svc.Overview(r.Context())
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, view)
}

// SchoolStats — GET /api/admin/schools/{id}/stats (P3).
func (h *Handler) SchoolStats(w http.ResponseWriter, r *http.Request) {
	schoolID, err := pathInt64(r, "id")
	if err != nil {
		httpx.WriteError(w, httpx.Validation("ID sekolah tidak valid."))
		return
	}
	view, err := h.svc.SchoolStats(r.Context(), schoolID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, view)
}

// ListOutbox — GET /api/admin/outbox?status=&school_id=&page= (P4.4).
func (h *Handler) ListOutbox(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	status := q.Get("status")
	var schoolID int64
	if v := q.Get("school_id"); v != "" {
		schoolID, _ = strconv.ParseInt(v, 10, 64)
	}
	page, _ := strconv.Atoi(q.Get("page"))
	result, err := h.svc.ListOutbox(r.Context(), status, schoolID, page)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, result)
}

// RetryOutbox — POST /api/admin/outbox/{id}/retry (P4.4).
func (h *Handler) RetryOutbox(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		httpx.WriteError(w, httpx.Validation("ID outbox tidak valid."))
		return
	}
	if err := h.svc.RetryOutbox(r.Context(), id); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]bool{"ok": true})
}

type retryAllOutboxRequest struct {
	SchoolID int64  `json:"school_id"`
	Status   string `json:"status"`
}

// RetryAllOutbox — POST /api/admin/outbox/retry-all (P4.4).
func (h *Handler) RetryAllOutbox(w http.ResponseWriter, r *http.Request) {
	var req retryAllOutboxRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.WriteError(w, err)
		return
	}
	ctx := r.Context()
	n, err := h.svc.RetryAllOutbox(ctx, reqctx.UserID(ctx), req.SchoolID, req.Status)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]int{"retried": n})
}

// -- P2.2 (fase 13 Gelombang 2) --

// Onboarding — GET /api/admin/schools/{id}/onboarding.
func (h *Handler) Onboarding(w http.ResponseWriter, r *http.Request) {
	schoolID, err := pathInt64(r, "id")
	if err != nil {
		httpx.WriteError(w, httpx.Validation("ID sekolah tidak valid."))
		return
	}
	status, err := h.svc.Onboarding(r.Context(), schoolID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, status)
}

// -- P5 (fase 13 Gelombang 2) --

// ListPlatformAnnouncements — GET /api/admin/platform-announcements.
func (h *Handler) ListPlatformAnnouncements(w http.ResponseWriter, r *http.Request) {
	items, err := h.svc.ListPlatformAnnouncements(r.Context())
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, items)
}

type platformAnnouncementRequest struct {
	Title    string `json:"title"`
	Body     string `json:"body"`
	StartsAt string `json:"starts_at"`
	EndsAt   string `json:"ends_at"`
}

// CreatePlatformAnnouncement — POST /api/admin/platform-announcements.
func (h *Handler) CreatePlatformAnnouncement(w http.ResponseWriter, r *http.Request) {
	var req platformAnnouncementRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.WriteError(w, err)
		return
	}
	ctx := r.Context()
	view, err := h.svc.CreatePlatformAnnouncement(ctx, reqctx.UserID(ctx), CreatePlatformAnnouncementInput{
		Title: req.Title, Body: req.Body, StartsAt: req.StartsAt, EndsAt: req.EndsAt,
	})
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, view)
}

// UpdatePlatformAnnouncement — PATCH /api/admin/platform-announcements/{id}.
func (h *Handler) UpdatePlatformAnnouncement(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		httpx.WriteError(w, httpx.Validation("ID pengumuman tidak valid."))
		return
	}
	var req platformAnnouncementRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.WriteError(w, err)
		return
	}
	ctx := r.Context()
	view, err := h.svc.UpdatePlatformAnnouncement(ctx, reqctx.UserID(ctx), id, UpdatePlatformAnnouncementInput{
		Title: req.Title, Body: req.Body, StartsAt: req.StartsAt, EndsAt: req.EndsAt,
	})
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, view)
}

// DeletePlatformAnnouncement — DELETE /api/admin/platform-announcements/{id}.
func (h *Handler) DeletePlatformAnnouncement(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		httpx.WriteError(w, httpx.Validation("ID pengumuman tidak valid."))
		return
	}
	ctx := r.Context()
	if err := h.svc.DeletePlatformAnnouncement(ctx, reqctx.UserID(ctx), id); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]bool{"ok": true})
}
