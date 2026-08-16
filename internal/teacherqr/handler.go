package teacherqr

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

// GenerateToken — POST /api/teacher-qr.
func (h *Handler) GenerateToken(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	result, err := h.svc.GenerateToken(ctx, reqctx.UserID(ctx), reqctx.SchoolID(ctx))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, result)
}
