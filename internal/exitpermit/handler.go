package exitpermit

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

func pathInt64(r *http.Request, name string) (int64, error) {
	return strconv.ParseInt(r.PathValue(name), 10, 64)
}

type submitRequest struct {
	Reason string `json:"reason"`
}

// SubmitRequest — POST /api/exit-permits.
func (h *Handler) SubmitRequest(w http.ResponseWriter, r *http.Request) {
	var req submitRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.WriteError(w, err)
		return
	}
	ctx := r.Context()
	result, err := h.svc.SubmitRequest(ctx, reqctx.UserID(ctx), reqctx.SchoolID(ctx), req.Reason)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, result)
}

// ListRequests — GET /api/exit-permits?scope=mine|active|all&date=.
func (h *Handler) ListRequests(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := r.URL.Query()
	result, err := h.svc.ListRequests(ctx, reqctx.UserID(ctx), reqctx.SchoolID(ctx), q.Get("scope"), q.Get("date"))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, result)
}

type scanRequest struct {
	Token string `json:"token"`
}

// Scan — POST /api/exit-permits/{id}/scan.
func (h *Handler) Scan(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		httpx.WriteError(w, httpx.Validation("ID pengajuan tidak valid."))
		return
	}
	var req scanRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.WriteError(w, err)
		return
	}
	ctx := r.Context()
	result, err := h.svc.Scan(ctx, reqctx.UserID(ctx), reqctx.SchoolID(ctx), id, req.Token)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, result)
}

// CancelRequest — POST /api/exit-permits/{id}/cancel.
func (h *Handler) CancelRequest(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		httpx.WriteError(w, httpx.Validation("ID pengajuan tidak valid."))
		return
	}
	ctx := r.Context()
	if err := h.svc.CancelRequest(ctx, reqctx.UserID(ctx), reqctx.SchoolID(ctx), id); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"status": "canceled"})
}

type rejectRequest struct {
	Comment string `json:"comment"`
}

// RejectRequest — POST /api/exit-permits/{id}/reject.
func (h *Handler) RejectRequest(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		httpx.WriteError(w, httpx.Validation("ID pengajuan tidak valid."))
		return
	}
	var req rejectRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.WriteError(w, err)
		return
	}
	ctx := r.Context()
	result, err := h.svc.RejectRequest(ctx, reqctx.UserID(ctx), reqctx.SchoolID(ctx), id, req.Comment)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, result)
}

type gateScanRequest struct {
	GateToken string `json:"gate_token"`
}

// GateScan — POST /api/exit-permits/gate-scan.
func (h *Handler) GateScan(w http.ResponseWriter, r *http.Request) {
	var req gateScanRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.WriteError(w, err)
		return
	}
	ctx := r.Context()
	result, err := h.svc.GateScan(ctx, reqctx.UserID(ctx), reqctx.SchoolID(ctx), req.GateToken)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, result)
}

// GateHistory — GET /api/exit-permits/gate-history?date=.
func (h *Handler) GateHistory(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	result, err := h.svc.GateHistory(ctx, reqctx.UserID(ctx), reqctx.SchoolID(ctx), r.URL.Query().Get("date"))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": result})
}
