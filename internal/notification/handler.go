package notification

import (
	"net/http"
	"strconv"

	"github.com/omanjaya/nouschool/internal/platform/httpx"
	"github.com/omanjaya/nouschool/internal/platform/reqctx"
)

// Handler menerjemahkan HTTP <-> Service. Tidak menyusun SQL, tidak
// memutuskan aturan bisnis — hanya decode/validate bentuk + panggil service.
type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// ListNotifications — GET /api/notifications?page=.
func (h *Handler) ListNotifications(w http.ResponseWriter, r *http.Request) {
	page := 1
	if raw := r.URL.Query().Get("page"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 {
			page = v
		}
	}
	ctx := r.Context()
	result, err := h.svc.ListNotifications(ctx, reqctx.SchoolID(ctx), reqctx.UserID(ctx), page)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, result)
}

type markReadRequest struct {
	IDs []int64 `json:"ids"`
	All bool    `json:"all"`
}

// MarkRead — POST /api/notifications/read {ids:[]} atau {all:true}.
func (h *Handler) MarkRead(w http.ResponseWriter, r *http.Request) {
	var req markReadRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.WriteError(w, err)
		return
	}
	ctx := r.Context()
	if err := h.svc.MarkRead(ctx, reqctx.SchoolID(ctx), reqctx.UserID(ctx), req.IDs, req.All); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

// PublicKey — GET /api/push/public-key.
func (h *Handler) PublicKey(w http.ResponseWriter, r *http.Request) {
	httpx.JSON(w, http.StatusOK, map[string]string{"key": h.svc.PublicKey()})
}

type subscribeRequest struct {
	Endpoint string `json:"endpoint"`
	P256dh   string `json:"p256dh"`
	Auth     string `json:"auth"`
}

// Subscribe — POST /api/push/subscribe {endpoint,p256dh,auth}.
func (h *Handler) Subscribe(w http.ResponseWriter, r *http.Request) {
	var req subscribeRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.WriteError(w, err)
		return
	}
	ctx := r.Context()
	if err := h.svc.Subscribe(ctx, reqctx.SchoolID(ctx), reqctx.UserID(ctx), req.Endpoint, req.P256dh, req.Auth); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

type unsubscribeRequest struct {
	Endpoint string `json:"endpoint"`
}

// Unsubscribe — POST /api/push/unsubscribe {endpoint}.
func (h *Handler) Unsubscribe(w http.ResponseWriter, r *http.Request) {
	var req unsubscribeRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.WriteError(w, err)
		return
	}
	ctx := r.Context()
	if err := h.svc.Unsubscribe(ctx, reqctx.SchoolID(ctx), reqctx.UserID(ctx), req.Endpoint); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"status": "ok"})
}
