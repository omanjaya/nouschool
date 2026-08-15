package leave

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

// Types — GET /api/leave/types.
func (h *Handler) Types(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	items, err := h.svc.ListTypes(ctx, reqctx.SchoolID(ctx))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"types": items})
}

const maxAttachmentUploadBytes = 6 << 20 // sedikit di atas batas 5MB supaya pesan validasi (bukan potongan) yang muncul

// SubmitRequest — POST /api/leave/requests (multipart form: type, date_start,
// date_end, reason, file opsional "attachment").
func (h *Handler) SubmitRequest(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(maxAttachmentUploadBytes); err != nil {
		httpx.WriteError(w, httpx.Validation("Gagal membaca form pengajuan."))
		return
	}

	in := SubmitInput{
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
		// lampiran opsional — tidak apa-apa.
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

// ListRequests — GET /api/leave/requests?scope=mine|all&status=.
func (h *Handler) ListRequests(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := r.URL.Query()
	items, err := h.svc.ListRequests(ctx, reqctx.SchoolID(ctx), reqctx.UserID(ctx), q.Get("scope"), q.Get("status"))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

// CancelRequest — POST /api/leave/requests/{id}/cancel.
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

// ListApprovals — GET /api/leave/approvals.
func (h *Handler) ListApprovals(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	items, err := h.svc.ListApprovals(ctx, reqctx.SchoolID(ctx), reqctx.UserID(ctx))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

type decideRequest struct {
	Decision string `json:"decision"`
	Comment  string `json:"comment"`
}

// DecideStep — POST /api/leave/approvals/{step_id}/decide.
func (h *Handler) DecideStep(w http.ResponseWriter, r *http.Request) {
	stepID, err := pathInt64(r, "step_id")
	if err != nil {
		httpx.WriteError(w, httpx.Validation("ID step tidak valid."))
		return
	}
	var req decideRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.WriteError(w, err)
		return
	}
	ctx := r.Context()
	result, err := h.svc.DecideStep(ctx, reqctx.UserID(ctx), reqctx.SchoolID(ctx), stepID, req.Decision, req.Comment)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, result)
}

// Summary — GET /api/leave/summary?from=&to=.
func (h *Handler) Summary(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := r.URL.Query()
	items, err := h.svc.Summary(ctx, reqctx.SchoolID(ctx), q.Get("from"), q.Get("to"))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, items)
}

// Attachment — GET /api/files/leave/{id}/attachment.
func (h *Handler) Attachment(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		httpx.WriteError(w, httpx.Validation("ID pengajuan tidak valid."))
		return
	}
	ctx := r.Context()
	info, err := h.svc.GetAttachment(ctx, reqctx.UserID(ctx), reqctx.SchoolID(ctx), id)
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

// sanitizeFilename membuang tanda kutip/kontrol karakter supaya header
// Content-Disposition tidak bisa disuntik lewat nama file asli.
func sanitizeFilename(name string) string {
	out := make([]rune, 0, len(name))
	for _, r := range name {
		if r == '"' || r == '\\' || r < 0x20 {
			continue
		}
		out = append(out, r)
	}
	if len(out) == 0 {
		return "lampiran"
	}
	return string(out)
}
