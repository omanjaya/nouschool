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

// ExportMonthlyXLSX — GET /api/attendance/export?month=YYYY-MM&class_id=
// (Fase 11, perm attendance:report).
func (h *Handler) ExportMonthlyXLSX(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	filename, data, err := h.svc.ExportMonthlyXLSX(ctx, reqctx.SchoolID(ctx), r.URL.Query().Get("month"), queryInt64(r, "class_id"))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", `attachment; filename="`+sanitizeExportFilename(filename)+`"`)
	_, _ = w.Write(data)
}

// sanitizeExportFilename membuang tanda kutip/kontrol karakter supaya header
// Content-Disposition tidak bisa disuntik lewat nama file (sama pola dengan
// internal/leave.sanitizeFilename).
func sanitizeExportFilename(name string) string {
	out := make([]rune, 0, len(name))
	for _, r := range name {
		if r == '"' || r == '\\' || r < 0x20 {
			continue
		}
		out = append(out, r)
	}
	if len(out) == 0 {
		return "export.xlsx"
	}
	return string(out)
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

// StudentAttendanceCalendar — GET /api/students/{id}/attendance/calendar?month=YYYY-MM
// (Fase 14 Gelombang D, docs/12-sion-parity.md).
func (h *Handler) StudentAttendanceCalendar(w http.ResponseWriter, r *http.Request) {
	studentID, err := pathInt64(r, "id")
	if err != nil {
		httpx.WriteError(w, httpx.Validation("ID siswa tidak valid."))
		return
	}
	ctx := r.Context()
	result, err := h.svc.StudentCalendar(ctx, reqctx.SchoolID(ctx), studentID, r.URL.Query().Get("month"))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, result)
}

// -- QR kartu siswa (Fase 8) --

type generateQRCardsRequest struct {
	ClassID int64 `json:"class_id"`
}

// GenerateQRCards — POST /api/attendance/qr-cards/generate.
func (h *Handler) GenerateQRCards(w http.ResponseWriter, r *http.Request) {
	var req generateQRCardsRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.WriteError(w, err)
		return
	}
	ctx := r.Context()
	items, err := h.svc.GenerateQRCards(ctx, reqctx.UserID(ctx), reqctx.SchoolID(ctx), req.ClassID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, items)
}

// ListQRCards — GET /api/attendance/qr-cards?class_id=.
func (h *Handler) ListQRCards(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	items, err := h.svc.ListQRCards(ctx, reqctx.SchoolID(ctx), queryInt64(r, "class_id"))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, items)
}

// RevokeQRCard — POST /api/attendance/qr-cards/{studentId}/revoke.
func (h *Handler) RevokeQRCard(w http.ResponseWriter, r *http.Request) {
	studentID, err := pathInt64(r, "studentId")
	if err != nil {
		httpx.WriteError(w, httpx.Validation("ID siswa tidak valid."))
		return
	}
	ctx := r.Context()
	view, err := h.svc.RevokeQRCard(ctx, reqctx.UserID(ctx), reqctx.SchoolID(ctx), studentID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, view)
}

// QRCardPNG — GET /api/attendance/qr-cards/{studentId}/qr.png.
func (h *Handler) QRCardPNG(w http.ResponseWriter, r *http.Request) {
	studentID, err := pathInt64(r, "studentId")
	if err != nil {
		httpx.WriteError(w, httpx.Validation("ID siswa tidak valid."))
		return
	}
	png, err := h.svc.QRCardPNG(r.Context(), reqctx.SchoolID(r.Context()), studentID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(png)
}

type scanRequest struct {
	Token string `json:"token"`
}

// Scan — POST /api/attendance/sessions/{id}/scan.
func (h *Handler) Scan(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		httpx.WriteError(w, httpx.Validation("ID sesi tidak valid."))
		return
	}
	var req scanRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.WriteError(w, err)
		return
	}
	ctx := r.Context()
	result, err := h.svc.ScanQRCard(ctx, reqctx.UserID(ctx), reqctx.SchoolID(ctx), id, req.Token)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, result)
}

// -- Self check-in siswa (Fase 8) --

// SelfCheckinStatus — GET /api/attendance/self-checkin/status.
func (h *Handler) SelfCheckinStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	view, err := h.svc.SelfCheckinStatus(ctx, reqctx.SchoolID(ctx))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, view)
}

type selfCheckinRequest struct {
	Lat      float64 `json:"lat"`
	Lng      float64 `json:"lng"`
	Accuracy float64 `json:"accuracy"`
}

// SelfCheckin — POST /api/attendance/self-checkin. user_agent diambil dari
// header HTTP, BUKAN body (docs/05: bukti meta lat/lng/accuracy/UA).
func (h *Handler) SelfCheckin(w http.ResponseWriter, r *http.Request) {
	var req selfCheckinRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.WriteError(w, err)
		return
	}
	ctx := r.Context()
	result, err := h.svc.SelfCheckin(ctx, reqctx.SchoolID(ctx), SelfCheckinInput{
		Lat: req.Lat, Lng: req.Lng, Accuracy: req.Accuracy, UserAgent: r.UserAgent(),
	})
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, result)
}

// Anomalies — GET /api/attendance/anomalies?date=&class_id=.
func (h *Handler) Anomalies(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := r.URL.Query()
	items, err := h.svc.Anomalies(ctx, reqctx.SchoolID(ctx), q.Get("date"), queryInt64(r, "class_id"))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, items)
}
