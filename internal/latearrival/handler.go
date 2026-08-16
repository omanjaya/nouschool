package latearrival

import (
	"net/http"

	"github.com/omanjaya/nouschool/internal/platform/httpx"
	"github.com/omanjaya/nouschool/internal/platform/reqctx"
)

// Handler menerjemahkan HTTP <-> Service.
type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

type scanRequest struct {
	Token string `json:"token"`
}

// Scan — POST /api/late-arrivals/scan.
func (h *Handler) Scan(w http.ResponseWriter, r *http.Request) {
	var req scanRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.WriteError(w, err)
		return
	}
	ctx := r.Context()
	result, err := h.svc.Scan(ctx, reqctx.UserID(ctx), reqctx.SchoolID(ctx), req.Token)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, result)
}

// ListRecords — GET /api/late-arrivals?scope=mine|today|all&month=.
func (h *Handler) ListRecords(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := r.URL.Query()
	result, err := h.svc.ListRecords(ctx, reqctx.UserID(ctx), reqctx.SchoolID(ctx), q.Get("scope"), q.Get("month"))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, result)
}

// Summary — GET /api/late-arrivals/summary?month=.
func (h *Handler) Summary(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	result, err := h.svc.Summary(ctx, reqctx.SchoolID(ctx), r.URL.Query().Get("month"))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": result})
}
