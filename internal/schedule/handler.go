package schedule

import (
	"io"
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

// -- periods --

// ListPeriods — GET /api/periods.
func (h *Handler) ListPeriods(w http.ResponseWriter, r *http.Request) {
	items, err := h.svc.ListPeriods(r.Context(), reqctx.SchoolID(r.Context()))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, items)
}

type replacePeriodRequest struct {
	Number   int    `json:"number"`
	StartsAt string `json:"starts_at"`
	EndsAt   string `json:"ends_at"`
	Label    string `json:"label"`
}

// ReplacePeriods — PUT /api/periods (body: array penuh).
func (h *Handler) ReplacePeriods(w http.ResponseWriter, r *http.Request) {
	var req []replacePeriodRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.WriteError(w, err)
		return
	}
	in := make([]ReplacePeriodInput, 0, len(req))
	for _, p := range req {
		in = append(in, ReplacePeriodInput{Number: p.Number, StartsAt: p.StartsAt, EndsAt: p.EndsAt, Label: p.Label})
	}
	ctx := r.Context()
	items, err := h.svc.ReplacePeriods(ctx, reqctx.UserID(ctx), reqctx.SchoolID(ctx), in)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, items)
}

// -- period_day_overrides (Fase 14 Gelombang D) --

// GetPeriodOverrides — GET /api/periods/overrides?day=.
func (h *Handler) GetPeriodOverrides(w http.ResponseWriter, r *http.Request) {
	day, err := strconv.Atoi(r.URL.Query().Get("day"))
	if err != nil {
		httpx.WriteError(w, httpx.Validation("Parameter day wajib diisi angka 0-6."))
		return
	}
	items, err := h.svc.GetPeriodOverrides(r.Context(), reqctx.SchoolID(r.Context()), day)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, items)
}

type replacePeriodOverridesRequest struct {
	DayOfWeek int                    `json:"day_of_week"`
	Periods   []replacePeriodRequest `json:"periods"`
}

// ReplacePeriodOverrides — PUT /api/periods/overrides {day_of_week,periods}.
func (h *Handler) ReplacePeriodOverrides(w http.ResponseWriter, r *http.Request) {
	var req replacePeriodOverridesRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.WriteError(w, err)
		return
	}
	in := make([]ReplacePeriodInput, 0, len(req.Periods))
	for _, p := range req.Periods {
		in = append(in, ReplacePeriodInput{Number: p.Number, StartsAt: p.StartsAt, EndsAt: p.EndsAt, Label: p.Label})
	}
	ctx := r.Context()
	items, err := h.svc.ReplacePeriodOverrides(ctx, reqctx.UserID(ctx), reqctx.SchoolID(ctx), req.DayOfWeek, in)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, items)
}

// -- rooms --

// ListRooms — GET /api/rooms.
func (h *Handler) ListRooms(w http.ResponseWriter, r *http.Request) {
	items, err := h.svc.ListRooms(r.Context(), reqctx.SchoolID(r.Context()))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, items)
}

type roomRequest struct {
	Name string `json:"name"`
}

// CreateRoom — POST /api/rooms.
func (h *Handler) CreateRoom(w http.ResponseWriter, r *http.Request) {
	var req roomRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.WriteError(w, err)
		return
	}
	ctx := r.Context()
	room, err := h.svc.CreateRoom(ctx, reqctx.UserID(ctx), reqctx.SchoolID(ctx), req.Name)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, room)
}

// UpdateRoom — PATCH /api/rooms/{id}.
func (h *Handler) UpdateRoom(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		httpx.WriteError(w, httpx.Validation("ID ruangan tidak valid."))
		return
	}
	var req roomRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.WriteError(w, err)
		return
	}
	ctx := r.Context()
	room, err := h.svc.UpdateRoom(ctx, reqctx.UserID(ctx), reqctx.SchoolID(ctx), id, req.Name)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, room)
}

// DeleteRoom — DELETE /api/rooms/{id}.
func (h *Handler) DeleteRoom(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		httpx.WriteError(w, httpx.Validation("ID ruangan tidak valid."))
		return
	}
	ctx := r.Context()
	if err := h.svc.DeleteRoom(ctx, reqctx.UserID(ctx), reqctx.SchoolID(ctx), id); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

// RegenerateRoomQR — POST /api/rooms/{id}/regenerate-qr.
func (h *Handler) RegenerateRoomQR(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		httpx.WriteError(w, httpx.Validation("ID ruangan tidak valid."))
		return
	}
	ctx := r.Context()
	room, err := h.svc.RegenerateRoomQR(ctx, reqctx.UserID(ctx), reqctx.SchoolID(ctx), id)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, room)
}

// RoomQRPNG — GET /api/rooms/{id}/qr.png.
func (h *Handler) RoomQRPNG(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		httpx.WriteError(w, httpx.Validation("ID ruangan tidak valid."))
		return
	}
	png, err := h.svc.RoomQRPNG(r.Context(), reqctx.SchoolID(r.Context()), id)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(png)
}

// -- slots --

// ListSlots — GET /api/schedule/slots?class_id=|teacher_id=&academic_year_id=.
func (h *Handler) ListSlots(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	items, err := h.svc.ListSlots(ctx, reqctx.SchoolID(ctx), ListSlotsQuery{
		ClassID: queryInt64(r, "class_id"), TeacherID: queryInt64(r, "teacher_id"), AcademicYearID: queryInt64(r, "academic_year_id"),
	})
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, items)
}

type slotRequest struct {
	ClassID        int64 `json:"class_id"`
	SubjectID      int64 `json:"subject_id"`
	TeacherID      int64 `json:"teacher_id"`
	RoomID         int64 `json:"room_id"`
	DayOfWeek      int   `json:"day_of_week"`
	PeriodStart    int   `json:"period_start"`
	PeriodEnd      int   `json:"period_end"`
	AcademicYearID int64 `json:"academic_year_id"`
}

func slotRequestToInput(req slotRequest) SlotInputRequest {
	return SlotInputRequest{
		ClassID: req.ClassID, SubjectID: req.SubjectID, TeacherID: req.TeacherID, RoomID: req.RoomID,
		DayOfWeek: req.DayOfWeek, PeriodStart: req.PeriodStart, PeriodEnd: req.PeriodEnd, AcademicYearID: req.AcademicYearID,
	}
}

// CreateSlot — POST /api/schedule/slots.
func (h *Handler) CreateSlot(w http.ResponseWriter, r *http.Request) {
	var req slotRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.WriteError(w, err)
		return
	}
	ctx := r.Context()
	slot, err := h.svc.CreateSlot(ctx, reqctx.UserID(ctx), reqctx.SchoolID(ctx), slotRequestToInput(req))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, slot)
}

// UpdateSlot — PATCH /api/schedule/slots/{id}.
func (h *Handler) UpdateSlot(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		httpx.WriteError(w, httpx.Validation("ID slot tidak valid."))
		return
	}
	var req slotRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.WriteError(w, err)
		return
	}
	ctx := r.Context()
	slot, err := h.svc.UpdateSlot(ctx, reqctx.UserID(ctx), reqctx.SchoolID(ctx), id, slotRequestToInput(req))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, slot)
}

// DeleteSlot — DELETE /api/schedule/slots/{id}.
func (h *Handler) DeleteSlot(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		httpx.WriteError(w, httpx.Validation("ID slot tidak valid."))
		return
	}
	ctx := r.Context()
	if err := h.svc.DeleteSlot(ctx, reqctx.UserID(ctx), reqctx.SchoolID(ctx), id); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

// -- copy --

type copyRequest struct {
	FromClassID int64 `json:"from_class_id"`
	ToClassID   int64 `json:"to_class_id"`
}

// CopySchedule — POST /api/schedule/copy.
func (h *Handler) CopySchedule(w http.ResponseWriter, r *http.Request) {
	var req copyRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.WriteError(w, err)
		return
	}
	ctx := r.Context()
	result, err := h.svc.CopySchedule(ctx, reqctx.UserID(ctx), reqctx.SchoolID(ctx), CopyInput{FromClassID: req.FromClassID, ToClassID: req.ToClassID})
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, result)
}

// -- query kunci --

// Today — GET /api/schedule/today?class_id=|teacher_id=.
func (h *Handler) Today(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	items, err := h.svc.SlotsToday(ctx, reqctx.SchoolID(ctx), TodayQuery{
		ClassID: queryInt64(r, "class_id"), TeacherID: queryInt64(r, "teacher_id"),
	}, h.svc.Now())
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, items)
}

// CurrentPeriod — GET /api/schedule/current-period.
func (h *Handler) CurrentPeriod(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	result, err := h.svc.CurrentPeriod(ctx, reqctx.SchoolID(ctx), h.svc.Now())
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, result)
}

// -- import --

const maxImportFileSize = 10 << 20 // 10 MB

// ScheduleImportTemplate — GET /api/import/schedule/template.
func (h *Handler) ScheduleImportTemplate(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="template-jadwal.csv"`)
	_, _ = w.Write([]byte("rombel,hari,jam_mulai,jam_selesai,kode_mapel,email_guru,ruangan\n"))
}

func readUploadedFile(r *http.Request) (filename string, content []byte, err error) {
	if err := r.ParseMultipartForm(maxImportFileSize); err != nil {
		return "", nil, httpx.Validation("Gagal membaca form upload.")
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		return "", nil, httpx.Validation("File wajib diunggah (field 'file').")
	}
	defer file.Close()
	content, err = io.ReadAll(io.LimitReader(file, maxImportFileSize))
	if err != nil {
		return "", nil, httpx.Validation("Gagal membaca isi file.")
	}
	return header.Filename, content, nil
}

// PreviewScheduleImport — POST /api/import/schedule.
func (h *Handler) PreviewScheduleImport(w http.ResponseWriter, r *http.Request) {
	filename, content, err := readUploadedFile(r)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	preview, err := h.svc.PreviewScheduleImport(r.Context(), reqctx.SchoolID(r.Context()), filename, content)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, preview)
}

type importCommitRequest struct {
	UploadID string `json:"upload_id"`
}

// CommitScheduleImport — POST /api/import/schedule/commit.
func (h *Handler) CommitScheduleImport(w http.ResponseWriter, r *http.Request) {
	var req importCommitRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.WriteError(w, err)
		return
	}
	ctx := r.Context()
	created, skipped, err := h.svc.CommitScheduleImport(ctx, reqctx.UserID(ctx), reqctx.SchoolID(ctx), req.UploadID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"created": created, "skipped": skipped})
}
