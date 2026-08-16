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
