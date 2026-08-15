package dashboard

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

// Board — GET /api/tv/board.
func (h *Handler) Board(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	board, err := h.svc.Board(ctx, reqctx.SchoolID(ctx))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, board)
}
