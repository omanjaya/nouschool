package grading

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

// -- GET /api/grading/status --

func (h *Handler) Status(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	result, err := h.svc.Status(ctx, reqctx.SchoolID(ctx))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, result)
}

// -- GET/POST/PATCH/DELETE /api/grading/components --

func (h *Handler) ListComponents(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := r.URL.Query()
	result, err := h.svc.ListComponents(ctx, reqctx.SchoolID(ctx), ListComponentsQuery{
		ClassID: queryInt64(q, "class_id"), SubjectID: queryInt64(q, "subject_id"),
	})
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, result)
}

type componentRequest struct {
	ClassID   int64  `json:"class_id"`
	SubjectID int64  `json:"subject_id"`
	Name      string `json:"name"`
	Type      string `json:"type"`
	Weight    int    `json:"weight"`
	Kktp      int    `json:"kktp"`
}

func (h *Handler) CreateComponent(w http.ResponseWriter, r *http.Request) {
	var req componentRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.WriteError(w, err)
		return
	}
	ctx := r.Context()
	result, err := h.svc.CreateComponent(ctx, reqctx.UserID(ctx), reqctx.SchoolID(ctx), ComponentCreateInput{
		ClassID: req.ClassID, SubjectID: req.SubjectID, Name: req.Name, Type: req.Type, Weight: req.Weight, Kktp: req.Kktp,
	})
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, result)
}

type componentPatchRequest struct {
	Name   *string `json:"name"`
	Type   *string `json:"type"`
	Weight *int    `json:"weight"`
	Kktp   *int    `json:"kktp"`
}

func (h *Handler) UpdateComponent(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		httpx.WriteError(w, httpx.Validation("ID komponen tidak valid."))
		return
	}
	var req componentPatchRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.WriteError(w, err)
		return
	}
	ctx := r.Context()
	result, err := h.svc.UpdateComponent(ctx, reqctx.UserID(ctx), reqctx.SchoolID(ctx), id, ComponentPatchInput{
		Name: req.Name, Type: req.Type, Weight: req.Weight, Kktp: req.Kktp,
	})
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, result)
}

func (h *Handler) DeleteComponent(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		httpx.WriteError(w, httpx.Validation("ID komponen tidak valid."))
		return
	}
	ctx := r.Context()
	if err := h.svc.DeleteComponent(ctx, reqctx.UserID(ctx), reqctx.SchoolID(ctx), id); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"status": "deleted"})
}

// -- GET/PUT /api/grading/components/{id}/grades --

func (h *Handler) GetGrades(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		httpx.WriteError(w, httpx.Validation("ID komponen tidak valid."))
		return
	}
	ctx := r.Context()
	result, err := h.svc.GetGrades(ctx, reqctx.SchoolID(ctx), id)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, result)
}

type gradeEntryRequest struct {
	StudentID int64    `json:"student_id"`
	Score     *float64 `json:"score"`
}

type putGradesRequest struct {
	Grades []gradeEntryRequest `json:"grades"`
}

func (h *Handler) PutGrades(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		httpx.WriteError(w, httpx.Validation("ID komponen tidak valid."))
		return
	}
	var req putGradesRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.WriteError(w, err)
		return
	}
	entries := make([]GradeEntryInput, 0, len(req.Grades))
	for _, g := range req.Grades {
		entries = append(entries, GradeEntryInput{StudentID: g.StudentID, Score: g.Score})
	}
	ctx := r.Context()
	result, err := h.svc.PutGrades(ctx, reqctx.UserID(ctx), reqctx.SchoolID(ctx), id, entries)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, result)
}

// -- GET /api/grading/recap --

func (h *Handler) Recap(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := r.URL.Query()
	result, err := h.svc.Recap(ctx, reqctx.SchoolID(ctx), RecapQuery{
		ClassID: queryInt64(q, "class_id"), SubjectID: queryInt64(q, "subject_id"),
	})
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, result)
}

// -- PUT /api/grading/publication --

type publicationRequest struct {
	ClassID   int64 `json:"class_id"`
	SubjectID int64 `json:"subject_id"`
	Published bool  `json:"published"`
}

func (h *Handler) PutPublication(w http.ResponseWriter, r *http.Request) {
	var req publicationRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.WriteError(w, err)
		return
	}
	ctx := r.Context()
	if err := h.svc.PutPublication(ctx, reqctx.UserID(ctx), reqctx.SchoolID(ctx), PublicationInput{
		ClassID: req.ClassID, SubjectID: req.SubjectID, Published: req.Published,
	}); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

// -- GET /api/my-grades --

func (h *Handler) MyGrades(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := r.URL.Query()
	result, err := h.svc.MyGrades(ctx, reqctx.SchoolID(ctx), MyGradesQuery{StudentID: queryInt64(q, "student_id")})
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, result)
}

// -- POST /api/grading/stars, DELETE /api/grading/stars/{id}, GET /api/grading/stars --

type starRequest struct {
	StudentID  int64  `json:"student_id"`
	Delta      int    `json:"delta"`
	Note       string `json:"note"`
	Visibility string `json:"visibility"`
}

func (h *Handler) CreateStar(w http.ResponseWriter, r *http.Request) {
	var req starRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.WriteError(w, err)
		return
	}
	ctx := r.Context()
	result, err := h.svc.CreateStar(ctx, reqctx.UserID(ctx), reqctx.SchoolID(ctx), StarCreateInput{
		StudentID: req.StudentID, Delta: req.Delta, Note: req.Note, Visibility: req.Visibility,
	})
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, result)
}

func (h *Handler) DeleteStar(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		httpx.WriteError(w, httpx.Validation("ID bintang tidak valid."))
		return
	}
	ctx := r.Context()
	if err := h.svc.DeleteStar(ctx, reqctx.UserID(ctx), reqctx.SchoolID(ctx), id); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"status": "deleted"})
}

func (h *Handler) ListStars(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := r.URL.Query()
	result, err := h.svc.ListStars(ctx, reqctx.SchoolID(ctx), ListStarsQuery{
		StudentID: queryInt64(q, "student_id"), ClassID: queryInt64(q, "class_id"),
	})
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, result)
}

// -- GET /api/my-stars --

func (h *Handler) MyStars(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := r.URL.Query()
	result, err := h.svc.MyStars(ctx, reqctx.SchoolID(ctx), MyStarsQuery{StudentID: queryInt64(q, "student_id")})
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, result)
}

// -- GET /api/grading/export --

func (h *Handler) ExportXLSX(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := r.URL.Query()
	filename, data, err := h.svc.ExportXLSX(ctx, reqctx.SchoolID(ctx), RecapQuery{
		ClassID: queryInt64(q, "class_id"), SubjectID: queryInt64(q, "subject_id"),
	})
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", `attachment; filename="`+sanitizeFilename(filename)+`"`)
	_, _ = w.Write(data)
}

// sanitizeFilename membuang tanda kutip/kontrol karakter supaya header
// Content-Disposition tidak bisa disuntik lewat nama file — pola yang sama
// dengan internal/discipline/handler.go & internal/leave/handler.go.
func sanitizeFilename(name string) string {
	out := make([]rune, 0, len(name))
	for _, r := range name {
		if r == '"' || r == '\\' || r < 0x20 {
			continue
		}
		out = append(out, r)
	}
	if len(out) == 0 {
		return "export"
	}
	return string(out)
}
