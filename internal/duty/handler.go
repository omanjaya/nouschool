package duty

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

// -- GET /api/duties/flags --

func (h *Handler) ListFlags(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	items, err := h.svc.ListFlags(ctx)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

// -- GET/POST/PATCH/DELETE /api/duties --

func (h *Handler) ListDuties(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	items, err := h.svc.ListDuties(ctx, reqctx.SchoolID(ctx))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

type dutyRequest struct {
	Name    string   `json:"name"`
	ForRole string   `json:"for_role"`
	Flags   []string `json:"flags"`
}

func (h *Handler) CreateDuty(w http.ResponseWriter, r *http.Request) {
	var req dutyRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.WriteError(w, err)
		return
	}
	ctx := r.Context()
	result, err := h.svc.CreateDuty(ctx, reqctx.UserID(ctx), reqctx.SchoolID(ctx), CreateDutyInput{
		Name: req.Name, ForRole: req.ForRole, Flags: req.Flags,
	})
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, result)
}

type patchDutyRequest struct {
	Name    *string  `json:"name"`
	ForRole *string  `json:"for_role"`
	Flags   []string `json:"flags"`
	Active  *bool    `json:"active"`
}

func (h *Handler) UpdateDuty(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		httpx.WriteError(w, httpx.Validation("ID tugas tidak valid."))
		return
	}
	var req patchDutyRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.WriteError(w, err)
		return
	}
	ctx := r.Context()
	result, err := h.svc.UpdateDuty(ctx, reqctx.UserID(ctx), reqctx.SchoolID(ctx), id, PatchDutyInput{
		Name: req.Name, ForRole: req.ForRole, Flags: req.Flags, Active: req.Active,
	})
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, result)
}

func (h *Handler) DeleteDuty(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		httpx.WriteError(w, httpx.Validation("ID tugas tidak valid."))
		return
	}
	ctx := r.Context()
	if err := h.svc.DeleteDuty(ctx, reqctx.UserID(ctx), reqctx.SchoolID(ctx), id); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"status": "deleted"})
}

// -- GET/PUT /api/duties/{id}/assignments --

func (h *Handler) GetAssignments(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		httpx.WriteError(w, httpx.Validation("ID tugas tidak valid."))
		return
	}
	ctx := r.Context()
	items, err := h.svc.GetAssignments(ctx, reqctx.SchoolID(ctx), id)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

type putAssignmentsRequest struct {
	UserIDs []int64 `json:"user_ids"`
}

func (h *Handler) PutAssignments(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		httpx.WriteError(w, httpx.Validation("ID tugas tidak valid."))
		return
	}
	var req putAssignmentsRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.WriteError(w, err)
		return
	}
	ctx := r.Context()
	items, err := h.svc.PutAssignments(ctx, reqctx.UserID(ctx), reqctx.SchoolID(ctx), id, req.UserIDs)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}
