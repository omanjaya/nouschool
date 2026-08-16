package substitution

import (
	"net/http"
	"strconv"

	"github.com/omanjaya/nouschool/internal/platform/httpx"
	"github.com/omanjaya/nouschool/internal/platform/reqctx"
)

// Handler menerjemahkan HTTP <-> Service.
type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func pathInt64(r *http.Request, name string) (int64, error) {
	return strconv.ParseInt(r.PathValue(name), 10, 64)
}

type requestBody struct {
	ScheduleSlotID   int64  `json:"schedule_slot_id"`
	Date             string `json:"date"`
	SubstituteUserID int64  `json:"substitute_user_id"`
	Reason           string `json:"reason"`
}

// Request — POST /api/substitutions.
func (h *Handler) Request(w http.ResponseWriter, r *http.Request) {
	var req requestBody
	if err := httpx.Decode(r, &req); err != nil {
		httpx.WriteError(w, err)
		return
	}
	ctx := r.Context()
	result, err := h.svc.Request(ctx, reqctx.UserID(ctx), reqctx.SchoolID(ctx), RequestInput{
		ScheduleSlotID: req.ScheduleSlotID, Date: req.Date, SubstituteUserID: req.SubstituteUserID, Reason: req.Reason,
	})
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, result)
}

// List — GET /api/substitutions?scope=mine|for-me|all&date=.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	ctx := r.Context()
	result, err := h.svc.List(ctx, reqctx.UserID(ctx), reqctx.SchoolID(ctx), ListQuery{Scope: q.Get("scope"), Date: q.Get("date")})
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, result)
}

// Accept — POST /api/substitutions/{id}/accept.
func (h *Handler) Accept(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		httpx.WriteError(w, httpx.Validation("ID permintaan tidak valid."))
		return
	}
	ctx := r.Context()
	result, err := h.svc.Accept(ctx, reqctx.UserID(ctx), reqctx.SchoolID(ctx), id)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, result)
}

// Reject — POST /api/substitutions/{id}/reject.
func (h *Handler) Reject(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		httpx.WriteError(w, httpx.Validation("ID permintaan tidak valid."))
		return
	}
	ctx := r.Context()
	result, err := h.svc.Reject(ctx, reqctx.UserID(ctx), reqctx.SchoolID(ctx), id)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, result)
}

// Cancel — POST /api/substitutions/{id}/cancel.
func (h *Handler) Cancel(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		httpx.WriteError(w, httpx.Validation("ID permintaan tidak valid."))
		return
	}
	ctx := r.Context()
	result, err := h.svc.Cancel(ctx, reqctx.UserID(ctx), reqctx.SchoolID(ctx), id)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, result)
}
