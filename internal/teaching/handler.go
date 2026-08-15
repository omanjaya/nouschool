package teaching

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

type scanRequest struct {
	RoomToken  string `json:"room_token"`
	RoomID     int64  `json:"room_id"`
	ManualPick bool   `json:"manual_pick"`
}

// Scan — POST /api/teaching/scan.
func (h *Handler) Scan(w http.ResponseWriter, r *http.Request) {
	var req scanRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.WriteError(w, err)
		return
	}
	ctx := r.Context()
	result, err := h.svc.Scan(ctx, reqctx.UserID(ctx), reqctx.SchoolID(ctx), ScanInput{
		RoomToken: req.RoomToken, RoomID: req.RoomID, ManualPick: req.ManualPick,
	})
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, result)
}

type unscheduledRequest struct {
	RoomID    int64 `json:"room_id"`
	ClassID   int64 `json:"class_id"`
	SubjectID int64 `json:"subject_id"`
}

// CreateUnscheduled — POST /api/teaching/journals.
func (h *Handler) CreateUnscheduled(w http.ResponseWriter, r *http.Request) {
	var req unscheduledRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.WriteError(w, err)
		return
	}
	ctx := r.Context()
	view, err := h.svc.CreateUnscheduled(ctx, reqctx.UserID(ctx), reqctx.SchoolID(ctx), UnscheduledInput{
		RoomID: req.RoomID, ClassID: req.ClassID, SubjectID: req.SubjectID,
	})
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, view)
}

type patchJournalRequest struct {
	Material string `json:"material"`
	Note     string `json:"note"`
}

// UpdateJournal — PATCH /api/teaching/journals/{id}.
func (h *Handler) UpdateJournal(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		httpx.WriteError(w, httpx.Validation("ID jurnal tidak valid."))
		return
	}
	var req patchJournalRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.WriteError(w, err)
		return
	}
	ctx := r.Context()
	view, err := h.svc.UpdateJournal(ctx, reqctx.UserID(ctx), reqctx.SchoolID(ctx), id, PatchInput{
		Material: req.Material, Note: req.Note,
	})
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, view)
}

// EndJournal — POST /api/teaching/journals/{id}/end.
func (h *Handler) EndJournal(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		httpx.WriteError(w, httpx.Validation("ID jurnal tidak valid."))
		return
	}
	ctx := r.Context()
	view, err := h.svc.EndJournal(ctx, reqctx.UserID(ctx), reqctx.SchoolID(ctx), id)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, view)
}

// ListJournals — GET /api/teaching/journals?scope=&date=&month=.
func (h *Handler) ListJournals(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := r.URL.Query()
	items, err := h.svc.ListJournals(ctx, reqctx.SchoolID(ctx), ListQuery{
		Scope: q.Get("scope"), Date: q.Get("date"), Month: q.Get("month"),
	})
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, items)
}

// Status — GET /api/teaching/status?date=.
func (h *Handler) Status(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	result, err := h.svc.Status(ctx, reqctx.SchoolID(ctx), r.URL.Query().Get("date"))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, result)
}

// Compliance — GET /api/teaching/compliance?from=&to=.
func (h *Handler) Compliance(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := r.URL.Query()
	result, err := h.svc.Compliance(ctx, reqctx.SchoolID(ctx), q.Get("from"), q.Get("to"))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, result)
}
