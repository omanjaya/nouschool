package attendance

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

func queryInt64(r *http.Request, name string) int64 {
	v, _ := strconv.ParseInt(r.URL.Query().Get(name), 10, 64)
	return v
}

// ListClasses — GET /api/attendance/classes?date=.
func (h *Handler) ListClasses(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	items, err := h.svc.ListClassesForDate(ctx, reqctx.SchoolID(ctx), r.URL.Query().Get("date"))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, items)
}

type createSessionRequest struct {
	ClassID        int64  `json:"class_id"`
	ScheduleSlotID int64  `json:"schedule_slot_id"`
	Date           string `json:"date"`
}

// CreateSession — POST /api/attendance/sessions. Terima {class_id} (sesi
// daily) ATAU {schedule_slot_id} (sesi subject dari slot jadwal, fase 6 —
// docs/06-teaching.md).
func (h *Handler) CreateSession(w http.ResponseWriter, r *http.Request) {
	var req createSessionRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.WriteError(w, err)
		return
	}
	ctx := r.Context()
	detail, err := h.svc.CreateSession(ctx, reqctx.UserID(ctx), reqctx.SchoolID(ctx), NewSessionInput{
		ClassID: req.ClassID, ScheduleSlotID: req.ScheduleSlotID, Date: req.Date,
	})
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, detail)
}

// GetSession — GET /api/attendance/sessions/{id}.
func (h *Handler) GetSession(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		httpx.WriteError(w, httpx.Validation("ID sesi tidak valid."))
		return
	}
	ctx := r.Context()
	detail, err := h.svc.GetSession(ctx, reqctx.SchoolID(ctx), id)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, detail)
}

type recordRequest struct {
	StudentID int64  `json:"student_id"`
	Status    string `json:"status"`
	Note      string `json:"note"`
}

type updateRecordsRequest struct {
	Records []recordRequest `json:"records"`
}

// UpdateRecords — PUT /api/attendance/sessions/{id}/records.
func (h *Handler) UpdateRecords(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		httpx.WriteError(w, httpx.Validation("ID sesi tidak valid."))
		return
	}
	var req updateRecordsRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.WriteError(w, err)
		return
	}
	records := make([]RecordInputRequest, 0, len(req.Records))
	for _, rec := range req.Records {
		records = append(records, RecordInputRequest{StudentID: rec.StudentID, Status: rec.Status, Note: rec.Note})
	}
	ctx := r.Context()
	detail, err := h.svc.UpdateRecords(ctx, reqctx.UserID(ctx), reqctx.SchoolID(ctx), id, records)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, detail)
}

// Finalize — POST /api/attendance/sessions/{id}/finalize.
func (h *Handler) Finalize(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		httpx.WriteError(w, httpx.Validation("ID sesi tidak valid."))
		return
	}
	ctx := r.Context()
	result, err := h.svc.Finalize(ctx, reqctx.UserID(ctx), reqctx.SchoolID(ctx), id)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, result)
}

// SlotsToday — GET /api/attendance/slots-today.
func (h *Handler) SlotsToday(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	items, err := h.svc.SlotsToday(ctx, reqctx.SchoolID(ctx), reqctx.UserID(ctx))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, items)
}

// Summary — GET /api/attendance/summary?date=&class_id=.
func (h *Handler) Summary(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	items, err := h.svc.Summary(ctx, reqctx.SchoolID(ctx), r.URL.Query().Get("date"), queryInt64(r, "class_id"))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, items)
}

// StudentAttendance — GET /api/students/{id}/attendance?from=&to=.
func (h *Handler) StudentAttendance(w http.ResponseWriter, r *http.Request) {
	studentID, err := pathInt64(r, "id")
	if err != nil {
		httpx.WriteError(w, httpx.Validation("ID siswa tidak valid."))
		return
	}
	ctx := r.Context()
	q := r.URL.Query()
	result, err := h.svc.StudentHistory(ctx, reqctx.SchoolID(ctx), studentID, q.Get("from"), q.Get("to"))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, result)
}
