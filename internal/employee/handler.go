package employee

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

func (h *Handler) ListEmployees(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	items, err := h.svc.ListEmployees(ctx, reqctx.SchoolID(ctx))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

type createEmployeeRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Username string `json:"username"`
	NIP      string `json:"nip"`
}

func (h *Handler) CreateEmployee(w http.ResponseWriter, r *http.Request) {
	var req createEmployeeRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.WriteError(w, err)
		return
	}
	ctx := r.Context()
	result, err := h.svc.CreateEmployee(ctx, reqctx.UserID(ctx), reqctx.SchoolID(ctx), CreateEmployeeInput{
		Name: req.Name, Email: req.Email, Username: req.Username, NIP: req.NIP,
	})
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, result)
}

type patchEmployeeRequest struct {
	NIP string `json:"nip"`
}

func (h *Handler) UpdateEmployee(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		httpx.WriteError(w, httpx.Validation("ID pegawai tidak valid."))
		return
	}
	var req patchEmployeeRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.WriteError(w, err)
		return
	}
	ctx := r.Context()
	result, err := h.svc.UpdateEmployee(ctx, reqctx.UserID(ctx), reqctx.SchoolID(ctx), id, req.NIP)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, result)
}
