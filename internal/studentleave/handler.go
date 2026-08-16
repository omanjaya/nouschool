package studentleave

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

const maxAttachmentUploadBytes = 6 << 20 // sedikit di atas batas 5MB (pola sama internal/leave)

// SubmitRequest — POST /api/student-leave (multipart: student_id (orang tua
// saja), type, date_start, date_end, reason, file opsional "attachment").
func (h *Handler) SubmitRequest(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(maxAttachmentUploadBytes); err != nil {
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
	in := SubmitInput{
		StudentID: studentID,
		Type:      r.FormValue("type"),
		DateStart: r.FormValue("date_start"),
		DateEnd:   r.FormValue("date_end"),
		Reason:    r.FormValue("reason"),
	}
	file, header, err := r.FormFile("attachment")
	switch {
	case err == nil:
		defer file.Close()
		content, err := io.ReadAll(io.LimitReader(file, maxAttachmentUploadBytes+1))
		if err != nil {
			httpx.WriteError(w, httpx.Validation("Gagal membaca isi lampiran."))
			return
		}
		in.Attachment = &AttachmentUpload{Filename: header.Filename, Content: content}
	case errors.Is(err, http.ErrMissingFile):
		// lampiran opsional
	default:
		httpx.WriteError(w, httpx.Validation("Gagal membaca lampiran."))
		return
	}

	ctx := r.Context()
	result, err := h.svc.SubmitRequest(ctx, reqctx.UserID(ctx), reqctx.SchoolID(ctx), in)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, result)
}

// ListRequests — GET /api/student-leave?scope=mine|queue|all&status=.
func (h *Handler) ListRequests(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := r.URL.Query()
	result, err := h.svc.ListRequests(ctx, reqctx.UserID(ctx), reqctx.SchoolID(ctx), q.Get("scope"), q.Get("status"))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, result)
}

type decisionRequest struct {
	Decision string `json:"decision"`
	Comment  string `json:"comment"`
}

func (h *Handler) ReviewHomeroom(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		httpx.WriteError(w, httpx.Validation("ID pengajuan tidak valid."))
		return
	}
	var req decisionRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.WriteError(w, err)
		return
	}
	ctx := r.Context()
	result, err := h.svc.ReviewHomeroom(ctx, reqctx.UserID(ctx), reqctx.SchoolID(ctx), id, req.Decision, req.Comment)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, result)
}

func (h *Handler) IssueBK(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		httpx.WriteError(w, httpx.Validation("ID pengajuan tidak valid."))
		return
	}
	var req decisionRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.WriteError(w, err)
		return
	}
	ctx := r.Context()
	result, err := h.svc.IssueBK(ctx, reqctx.UserID(ctx), reqctx.SchoolID(ctx), id, req.Decision, req.Comment)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, result)
}

func (h *Handler) CancelRequest(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		httpx.WriteError(w, httpx.Validation("ID pengajuan tidak valid."))
		return
	}
	ctx := r.Context()
	if err := h.svc.CancelRequest(ctx, reqctx.UserID(ctx), reqctx.SchoolID(ctx), id); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"status": "canceled"})
}

func (h *Handler) Attachment(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		httpx.WriteError(w, httpx.Validation("ID pengajuan tidak valid."))
		return
	}
	ctx := r.Context()
	info, err := h.svc.GetAttachment(ctx, reqctx.SchoolID(ctx), id)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	f, err := h.svc.OpenAttachment(info.Path)
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

func (h *Handler) LetterPDF(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		httpx.WriteError(w, httpx.Validation("ID pengajuan tidak valid."))
		return
	}
	ctx := r.Context()
	data, filename, err := h.svc.LetterPDF(ctx, reqctx.SchoolID(ctx), id, r.Host)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", `inline; filename="`+sanitizeFilename(filename)+`"`)
	_, _ = w.Write(data)
}

// PublicVerify — GET /api/public/leave-verify?token= (PUBLIK, host tenant).
func (h *Handler) PublicVerify(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	result, err := h.svc.PublicVerify(ctx, reqctx.SchoolID(ctx), r.URL.Query().Get("token"))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, result)
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
