package announcement

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/omanjaya/nouschool/internal/platform/httpx"
	"github.com/omanjaya/nouschool/internal/platform/reqctx"
)

// Handler menerjemahkan HTTP <-> Service. Tidak menyusun SQL, tidak
// memutuskan aturan bisnis — hanya decode/validate bentuk + panggil service.
type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func pathInt64(r *http.Request, name string) (int64, error) {
	return strconv.ParseInt(r.PathValue(name), 10, 64)
}

// isActiveParam menganggap "active" ada tanpa nilai, "1", atau nilai lain
// selain "0"/"false" sebagai truthy (docs/06-teaching.md: `?active=1`).
func isActiveParam(q *http.Request) bool {
	if !q.URL.Query().Has("active") {
		return false
	}
	raw := strings.ToLower(strings.TrimSpace(q.URL.Query().Get("active")))
	return raw != "0" && raw != "false"
}

// List — GET /api/announcements?active=1.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	items, err := h.svc.List(ctx, reqctx.SchoolID(ctx), isActiveParam(r))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, items)
}

type announcementRequest struct {
	Title    string `json:"title"`
	Body     string `json:"body"`
	StartsAt string `json:"starts_at"`
	EndsAt   string `json:"ends_at"`
}

// Create — POST /api/announcements.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var req announcementRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.WriteError(w, err)
		return
	}
	ctx := r.Context()
	view, err := h.svc.Create(ctx, reqctx.UserID(ctx), reqctx.SchoolID(ctx), CreateInput{
		Title: req.Title, Body: req.Body, StartsAt: req.StartsAt, EndsAt: req.EndsAt,
	})
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, view)
}

// Update — PATCH /api/announcements/{id}.
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		httpx.WriteError(w, httpx.Validation("ID pengumuman tidak valid."))
		return
	}
	var req announcementRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.WriteError(w, err)
		return
	}
	ctx := r.Context()
	view, err := h.svc.Update(ctx, reqctx.UserID(ctx), reqctx.SchoolID(ctx), id, UpdateInput{
		Title: req.Title, Body: req.Body, StartsAt: req.StartsAt, EndsAt: req.EndsAt,
	})
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, view)
}

// Delete — DELETE /api/announcements/{id}.
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		httpx.WriteError(w, httpx.Validation("ID pengumuman tidak valid."))
		return
	}
	ctx := r.Context()
	if err := h.svc.Delete(ctx, reqctx.UserID(ctx), reqctx.SchoolID(ctx), id); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true})
}
