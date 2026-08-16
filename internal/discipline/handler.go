package discipline

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

func queryInt64(q map[string][]string, name string) int64 {
	vals, ok := q[name]
	if !ok || len(vals) == 0 || vals[0] == "" {
		return 0
	}
	v, err := strconv.ParseInt(vals[0], 10, 64)
	if err != nil {
		return 0
	}
	return v
}

// -- GET/POST/PATCH/DELETE /api/discipline/types --

func (h *Handler) ListTypes(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	items, err := h.svc.ListViolationTypes(ctx, reqctx.SchoolID(ctx))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

type violationTypeRequest struct {
	Name     string `json:"name"`
	Points   int    `json:"points"`
	Category string `json:"category"`
}

func (h *Handler) CreateType(w http.ResponseWriter, r *http.Request) {
	var req violationTypeRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.WriteError(w, err)
		return
	}
	ctx := r.Context()
	result, err := h.svc.CreateViolationType(ctx, reqctx.UserID(ctx), reqctx.SchoolID(ctx), CreateViolationTypeInput{
		Name: req.Name, Points: req.Points, Category: req.Category,
	})
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, result)
}

type patchViolationTypeRequest struct {
	Name     *string `json:"name"`
	Points   *int    `json:"points"`
	Category *string `json:"category"`
	Active   *bool   `json:"active"`
}

func (h *Handler) UpdateType(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		httpx.WriteError(w, httpx.Validation("ID jenis pelanggaran tidak valid."))
		return
	}
	var req patchViolationTypeRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.WriteError(w, err)
		return
	}
	ctx := r.Context()
	result, err := h.svc.UpdateViolationType(ctx, reqctx.UserID(ctx), reqctx.SchoolID(ctx), id, PatchViolationTypeInput{
		Name: req.Name, Points: req.Points, Category: req.Category, Active: req.Active,
	})
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, result)
}

func (h *Handler) DeleteType(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		httpx.WriteError(w, httpx.Validation("ID jenis pelanggaran tidak valid."))
		return
	}
	ctx := r.Context()
	if err := h.svc.DeleteViolationType(ctx, reqctx.UserID(ctx), reqctx.SchoolID(ctx), id); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"status": "deleted"})
}

// -- GET/PUT /api/discipline/sp-settings --

func (h *Handler) GetSPSettings(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	result, err := h.svc.GetSPSettings(ctx, reqctx.SchoolID(ctx))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, result)
}

func (h *Handler) PutSPSettings(w http.ResponseWriter, r *http.Request) {
	var req SPSettings
	if err := httpx.Decode(r, &req); err != nil {
		httpx.WriteError(w, err)
		return
	}
	ctx := r.Context()
	result, err := h.svc.PutSPSettings(ctx, reqctx.UserID(ctx), reqctx.SchoolID(ctx), req)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, result)
}

// -- POST/DELETE/GET /api/discipline/records --

type recordRequest struct {
	StudentID           int64  `json:"student_id"`
	ViolationTypeID     int64  `json:"violation_type_id"`
	AttendanceSessionID int64  `json:"attendance_session_id"`
	Note                string `json:"note"`
	OccurredOn          string `json:"occurred_on"`
}

func (h *Handler) RecordViolation(w http.ResponseWriter, r *http.Request) {
	var req recordRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.WriteError(w, err)
		return
	}
	ctx := r.Context()
	result, err := h.svc.RecordViolation(ctx, reqctx.UserID(ctx), reqctx.SchoolID(ctx), RecordInput{
		StudentID: req.StudentID, ViolationTypeID: req.ViolationTypeID, AttendanceSessionID: req.AttendanceSessionID,
		Note: req.Note, OccurredOn: req.OccurredOn,
	})
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, result)
}

func (h *Handler) DeleteRecord(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		httpx.WriteError(w, httpx.Validation("ID catatan tidak valid."))
		return
	}
	ctx := r.Context()
	if err := h.svc.DeleteRecord(ctx, reqctx.UserID(ctx), reqctx.SchoolID(ctx), id); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"status": "deleted"})
}

func atoiOr(s string, def int) int {
	if s == "" {
		return def
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return v
}

func (h *Handler) ListRecords(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := r.URL.Query()
	result, err := h.svc.ListRecords(ctx, reqctx.SchoolID(ctx), ListRecordsQuery{
		StudentID: queryInt64(q, "student_id"), ClassID: queryInt64(q, "class_id"),
		From: q.Get("from"), To: q.Get("to"), Page: atoiOr(q.Get("page"), 1),
	})
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, result)
}

// -- GET /api/students/{id}/discipline --

func (h *Handler) StudentDiscipline(w http.ResponseWriter, r *http.Request) {
	studentID, err := pathInt64(r, "id")
	if err != nil {
		httpx.WriteError(w, httpx.Validation("ID siswa tidak valid."))
		return
	}
	ctx := r.Context()
	result, err := h.svc.StudentDiscipline(ctx, reqctx.SchoolID(ctx), studentID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, result)
}

// -- GET /api/discipline/letters(/{id}/pdf|/html) --

func (h *Handler) ListLetters(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := r.URL.Query()
	result, err := h.svc.ListLetters(ctx, reqctx.SchoolID(ctx), atoiOr(q.Get("level"), 0), atoiOr(q.Get("page"), 1))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, result)
}

func (h *Handler) LetterPDF(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		httpx.WriteError(w, httpx.Validation("ID surat tidak valid."))
		return
	}
	ctx := r.Context()
	data, filename, err := h.svc.LetterPDF(ctx, reqctx.SchoolID(ctx), id)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", `inline; filename="`+sanitizeFilename(filename)+`"`)
	_, _ = w.Write(data)
}

func (h *Handler) LetterHTML(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		httpx.WriteError(w, httpx.Validation("ID surat tidak valid."))
		return
	}
	ctx := r.Context()
	html, err := h.svc.LetterHTML(ctx, reqctx.SchoolID(ctx), id)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(html))
}

// sanitizeFilename membuang tanda kutip/kontrol karakter supaya header
// Content-Disposition tidak bisa disuntik lewat nama file — pola yang sama
// dengan internal/leave/handler.go.
func sanitizeFilename(name string) string {
	out := make([]rune, 0, len(name))
	for _, r := range name {
		if r == '"' || r == '\\' || r < 0x20 {
			continue
		}
		out = append(out, r)
	}
	if len(out) == 0 {
		return "surat"
	}
	return string(out)
}

// -- GET /api/discipline/summary & /api/discipline/export --

func (h *Handler) Summary(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := r.URL.Query()
	result, err := h.svc.Summary(ctx, reqctx.SchoolID(ctx), SummaryQuery{
		ClassID: queryInt64(q, "class_id"), From: q.Get("from"), To: q.Get("to"),
	})
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": result})
}

func (h *Handler) ExportXLSX(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := r.URL.Query()
	filename, data, err := h.svc.ExportSummaryXLSX(ctx, reqctx.SchoolID(ctx), SummaryQuery{
		ClassID: queryInt64(q, "class_id"), From: q.Get("from"), To: q.Get("to"),
	})
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", `attachment; filename="`+sanitizeFilename(filename)+`"`)
	_, _ = w.Write(data)
}
