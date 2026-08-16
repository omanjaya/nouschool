package counseling

import (
	"errors"
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

const maxEvidenceUploadBytes = 6 << 20 // sedikit di atas batas 5MB (pola sama internal/leave)

// ListCounselings — GET /api/counselings?student_id=&page=.
func (h *Handler) ListCounselings(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	var studentID int64
	if v := q.Get("student_id"); v != "" {
		id, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			httpx.WriteError(w, httpx.Validation("student_id tidak valid."))
			return
		}
		studentID = id
	}
	page, _ := strconv.Atoi(q.Get("page"))
	ctx := r.Context()
	result, err := h.svc.ListCounselings(ctx, reqctx.SchoolID(ctx), studentID, page)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, result)
}

// CreateCounseling — POST /api/counselings (multipart: student_id,
// career_goals, problem_description, follow_up_plan, evidence file opsional).
func (h *Handler) CreateCounseling(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(maxEvidenceUploadBytes); err != nil {
		httpx.WriteError(w, httpx.Validation("Gagal membaca form pengajuan."))
		return
	}
	var studentID int64
	if v := r.FormValue("student_id"); v != "" {
		id, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			httpx.WriteError(w, httpx.Validation("student_id tidak valid."))
			return
		}
		studentID = id
	}
	in := CreateInput{
		StudentID:          studentID,
		CareerGoals:        r.FormValue("career_goals"),
		ProblemDescription: r.FormValue("problem_description"),
		FollowUpPlan:       r.FormValue("follow_up_plan"),
	}
	file, header, err := r.FormFile("evidence")
	switch {
	case err == nil:
		defer file.Close()
		content, rerr := io.ReadAll(io.LimitReader(file, maxEvidenceUploadBytes+1))
		if rerr != nil {
			httpx.WriteError(w, httpx.Validation("Gagal membaca isi bukti."))
			return
		}
		in.Evidence = &EvidenceUpload{Filename: header.Filename, Content: content}
	case errors.Is(err, http.ErrMissingFile):
		// evidence opsional
	default:
		httpx.WriteError(w, httpx.Validation("Gagal membaca bukti."))
		return
	}

	ctx := r.Context()
	result, err := h.svc.CreateCounseling(ctx, reqctx.UserID(ctx), reqctx.SchoolID(ctx), in)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, result)
}

type updateCounselingRequest struct {
	CareerGoals        string `json:"career_goals"`
	ProblemDescription string `json:"problem_description"`
	FollowUpPlan       string `json:"follow_up_plan"`
}

// UpdateCounseling — PATCH /api/counselings/{id}.
func (h *Handler) UpdateCounseling(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		httpx.WriteError(w, httpx.Validation("ID sesi tidak valid."))
		return
	}
	var req updateCounselingRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.WriteError(w, err)
		return
	}
	ctx := r.Context()
	result, err := h.svc.UpdateCounseling(ctx, reqctx.UserID(ctx), reqctx.SchoolID(ctx), id, UpdateInput{
		CareerGoals: req.CareerGoals, ProblemDescription: req.ProblemDescription, FollowUpPlan: req.FollowUpPlan,
	})
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, result)
}

// DeleteCounseling — DELETE /api/counselings/{id}.
func (h *Handler) DeleteCounseling(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		httpx.WriteError(w, httpx.Validation("ID sesi tidak valid."))
		return
	}
	ctx := r.Context()
	if err := h.svc.DeleteCounseling(ctx, reqctx.UserID(ctx), reqctx.SchoolID(ctx), id); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"deleted": true})
}

// Evidence — GET /api/counselings/{id}/evidence.
func (h *Handler) Evidence(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		httpx.WriteError(w, httpx.Validation("ID sesi tidak valid."))
		return
	}
	ctx := r.Context()
	info, err := h.svc.GetEvidence(ctx, reqctx.SchoolID(ctx), id)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	f, err := h.svc.OpenEvidence(info.Path)
	if err != nil {
		httpx.WriteError(w, httpx.ErrNotFound)
		return
	}
	defer f.Close()
	contentType := info.ContentType
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", `inline; filename="`+sanitizeFilename(info.Name)+`"`)
	_, _ = io.Copy(w, f)
}

// Report — GET /api/counselings/{id}/report/html.
func (h *Handler) Report(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		httpx.WriteError(w, httpx.Validation("ID sesi tidak valid."))
		return
	}
	ctx := r.Context()
	html, err := h.svc.ReportHTML(ctx, reqctx.SchoolID(ctx), id)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(html))
}

func sanitizeFilename(name string) string {
	out := make([]rune, 0, len(name))
	for _, r := range name {
		if r == '"' || r == '\\' || r < 0x20 {
			continue
		}
		out = append(out, r)
	}
	if len(out) == 0 {
		return "berkas"
	}
	return string(out)
}
