package billing

import (
	"encoding/json"
	"fmt"
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

// -- tenant: GET /api/billing --

func (h *Handler) GetBilling(w http.ResponseWriter, r *http.Request) {
	view, err := h.svc.GetBillingView(r.Context(), reqctx.SchoolID(r.Context()))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, view)
}

// -- tenant: POST /api/billing/invoices/{id}/proof (multipart: file) --

func (h *Handler) UploadProof(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		httpx.WriteError(w, httpx.Validation("ID invoice tidak valid."))
		return
	}
	if err := r.ParseMultipartForm(maxProofUploadSize); err != nil {
		httpx.WriteError(w, httpx.Validation("Gagal membaca form unggah bukti transfer."))
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		httpx.WriteError(w, httpx.Validation("File bukti transfer wajib diunggah (field 'file')."))
		return
	}
	defer file.Close()
	content, err := io.ReadAll(io.LimitReader(file, maxProofUploadSize+1))
	if err != nil {
		httpx.WriteError(w, httpx.Validation("Gagal membaca isi file."))
		return
	}

	ctx := r.Context()
	view, err := h.svc.UploadProof(ctx, reqctx.UserID(ctx), reqctx.SchoolID(ctx), id, header.Filename, content)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, view)
}

// -- tenant & admin: GET .../invoices/{id}/pdf --

func (h *Handler) InvoicePDF(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		httpx.WriteError(w, httpx.Validation("ID invoice tidak valid."))
		return
	}
	ctx := r.Context()
	asAdmin := reqctx.IsPlatform(ctx)
	pdf, err := h.svc.GetInvoicePDF(ctx, reqctx.SchoolID(ctx), id, asAdmin)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", "inline; filename=\"invoice.pdf\"")
	_, _ = w.Write(pdf)
}

// -- tenant & admin: GET .../invoices/{id}/proof --

func (h *Handler) Proof(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		httpx.WriteError(w, httpx.Validation("ID invoice tidak valid."))
		return
	}
	ctx := r.Context()
	asAdmin := reqctx.IsPlatform(ctx)
	info, err := h.svc.GetProof(ctx, reqctx.SchoolID(ctx), id, asAdmin)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	f, err := h.svc.OpenFile(info.Path)
	if err != nil {
		httpx.WriteError(w, httpx.ErrNotFound)
		return
	}
	defer f.Close()
	w.Header().Set("Content-Type", info.ContentType)
	_, _ = io.Copy(w, f)
}

// -- tenant: POST /api/billing/invoices/{id}/pay --

func (h *Handler) Pay(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		httpx.WriteError(w, httpx.Validation("ID invoice tidak valid."))
		return
	}
	ctx := r.Context()
	redirectURL, err := h.svc.Pay(ctx, reqctx.UserID(ctx), reqctx.SchoolID(ctx), id)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"redirect_url": redirectURL})
}

// -- webhook: POST /api/webhooks/midtrans (TANPA auth) --

type midtransWebhookPayload struct {
	OrderID           string `json:"order_id"`
	StatusCode        string `json:"status_code"`
	GrossAmount       string `json:"gross_amount"`
	TransactionStatus string `json:"transaction_status"`
	FraudStatus       string `json:"fraud_status"`
	SignatureKey      string `json:"signature_key"`
}

func (h *Handler) MidtransWebhook(w http.ResponseWriter, r *http.Request) {
	raw, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		httpx.WriteError(w, httpx.Validation("Gagal membaca body webhook."))
		return
	}
	var payload midtransWebhookPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		httpx.WriteError(w, httpx.Validation("Format payload webhook tidak valid."))
		return
	}
	if err := h.svc.HandleGatewayWebhook(r.Context(), raw, payload.OrderID, payload.StatusCode, payload.GrossAmount,
		payload.TransactionStatus, payload.FraudStatus, payload.SignatureKey); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// -- admin: plans --

func (h *Handler) ListPlans(w http.ResponseWriter, r *http.Request) {
	plans, err := h.svc.ListPlans(r.Context())
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, plans)
}

type updatePlanRequest struct {
	Name     string          `json:"name"`
	Features map[string]bool `json:"features"`
	Prices   []PriceBracket  `json:"prices"`
}

func (h *Handler) UpdatePlan(w http.ResponseWriter, r *http.Request) {
	code := r.PathValue("code")
	var req updatePlanRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.WriteError(w, err)
		return
	}
	ctx := r.Context()
	view, err := h.svc.UpdatePlan(ctx, reqctx.UserID(ctx), code, UpdatePlanInput{Name: req.Name, Features: req.Features, Prices: req.Prices})
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, view)
}

// -- admin: sekolah --

func (h *Handler) AdminGetBilling(w http.ResponseWriter, r *http.Request) {
	schoolID, err := pathInt64(r, "id")
	if err != nil {
		httpx.WriteError(w, httpx.Validation("ID sekolah tidak valid."))
		return
	}
	view, err := h.svc.GetBillingView(r.Context(), schoolID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, view)
}

type createSubscriptionRequest struct {
	PlanCode string `json:"plan_code"`
}

func (h *Handler) AdminCreateSubscription(w http.ResponseWriter, r *http.Request) {
	schoolID, err := pathInt64(r, "id")
	if err != nil {
		httpx.WriteError(w, httpx.Validation("ID sekolah tidak valid."))
		return
	}
	var req createSubscriptionRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.WriteError(w, err)
		return
	}
	ctx := r.Context()
	inv, err := h.svc.CreateSubscriptionInvoice(ctx, reqctx.UserID(ctx), schoolID, req.PlanCode)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, inv)
}

type extendSubscriptionRequest struct {
	Days int `json:"days"`
}

func (h *Handler) AdminExtendSubscription(w http.ResponseWriter, r *http.Request) {
	schoolID, err := pathInt64(r, "id")
	if err != nil {
		httpx.WriteError(w, httpx.Validation("ID sekolah tidak valid."))
		return
	}
	var req extendSubscriptionRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.WriteError(w, err)
		return
	}
	ctx := r.Context()
	if err := h.svc.ExtendSubscriptionGoodwill(ctx, reqctx.UserID(ctx), schoolID, req.Days); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (h *Handler) AdminListSubscriptions(w http.ResponseWriter, r *http.Request) {
	rows, err := h.svc.ListSubscriptionsAdmin(r.Context())
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, rows)
}

// -- admin: invoices --

func (h *Handler) AdminVerifyInvoice(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		httpx.WriteError(w, httpx.Validation("ID invoice tidak valid."))
		return
	}
	ctx := r.Context()
	inv, err := h.svc.VerifyManualPayment(ctx, reqctx.UserID(ctx), id)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, inv)
}

func (h *Handler) AdminVoidInvoice(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		httpx.WriteError(w, httpx.Validation("ID invoice tidak valid."))
		return
	}
	ctx := r.Context()
	inv, err := h.svc.VoidInvoice(ctx, reqctx.UserID(ctx), id)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, inv)
}

// -- admin: P6.1 feature overrides (fase 13 Gelombang 2) --

type featureOverridesRequest struct {
	Overrides map[string]*bool `json:"overrides"`
}

// AdminSetFeatureOverrides — PUT /api/admin/schools/{id}/feature-overrides.
func (h *Handler) AdminSetFeatureOverrides(w http.ResponseWriter, r *http.Request) {
	schoolID, err := pathInt64(r, "id")
	if err != nil {
		httpx.WriteError(w, httpx.Validation("ID sekolah tidak valid."))
		return
	}
	var req featureOverridesRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.WriteError(w, err)
		return
	}
	ctx := r.Context()
	features, err := h.svc.SetFeatureOverrides(ctx, reqctx.UserID(ctx), schoolID, req.Overrides)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"features": features})
}

// -- admin: P6.3/P6.4 laporan pendapatan (fase 13 Gelombang 2) --

// AdminRevenue — GET /api/admin/revenue?year=YYYY.
func (h *Handler) AdminRevenue(w http.ResponseWriter, r *http.Request) {
	year, _ := strconv.Atoi(r.URL.Query().Get("year"))
	report, err := h.svc.GetRevenueReport(r.Context(), year)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, report)
}

// AdminRevenueExport — GET /api/admin/revenue/export?year=YYYY.
func (h *Handler) AdminRevenueExport(w http.ResponseWriter, r *http.Request) {
	year, _ := strconv.Atoi(r.URL.Query().Get("year"))
	filename, data, err := h.svc.ExportRevenueXLSX(r.Context(), year)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	_, _ = w.Write(data)
}
