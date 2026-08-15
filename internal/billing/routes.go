package billing

import (
	"net/http"

	"github.com/omanjaya/nouschool/internal/platform/middleware"
)

// RegisterRoutes memasang route modul billing. Middleware auth/permission
// disuntik dari main.go (diimplementasikan modul identity) — billing TIDAK
// mengimpor identity secara langsung (lihat aturan wiring di CLAUDE.md).
func RegisterRoutes(
	mux *http.ServeMux,
	h *Handler,
	requireAuth middleware.Middleware,
	requireSuperAdmin middleware.Middleware,
	requirePerm func(perm string) middleware.Middleware,
) {
	// Tenant — perm billing:view (admin_sekolah & kepala_sekolah, lihat
	// internal/identity/rbac.go) untuk SELURUH endpoint billing tenant,
	// termasuk mutasi (upload bukti, bayar) — docs/09-billing.md scope Fase 10.
	tenant := func(hf http.HandlerFunc) http.Handler { return requireAuth(requirePerm(PermBillingView)(hf)) }

	mux.Handle("GET /api/billing", tenant(h.GetBilling))
	mux.Handle("POST /api/billing/invoices/{id}/proof", tenant(h.UploadProof))
	mux.Handle("GET /api/billing/invoices/{id}/pdf", tenant(h.InvoicePDF))
	mux.Handle("GET /api/billing/invoices/{id}/proof", tenant(h.Proof))
	mux.Handle("POST /api/billing/invoices/{id}/pay", tenant(h.Pay))

	// Webhook gateway — TANPA auth (verifikasi lewat signature_key, lihat
	// Service.HandleGatewayWebhook). SubscriptionGuard mengecualikan prefix
	// /api/webhooks/ secara eksplisit (lihat guard.go) supaya webhook tetap
	// bisa mengaktivasi langganan sekolah yang justru sedang readonly.
	mux.HandleFunc("POST /api/webhooks/midtrans", h.MidtransWebhook)

	// Panel super admin — host platform + is_super_admin.
	admin := func(hf http.HandlerFunc) http.Handler { return requireAuth(requireSuperAdmin(hf)) }

	mux.Handle("GET /api/admin/plans", admin(h.ListPlans))
	mux.Handle("PUT /api/admin/plans/{code}", admin(h.UpdatePlan))
	mux.Handle("GET /api/admin/subscriptions", admin(h.AdminListSubscriptions))
	mux.Handle("GET /api/admin/schools/{id}/billing", admin(h.AdminGetBilling))
	mux.Handle("POST /api/admin/schools/{id}/subscriptions", admin(h.AdminCreateSubscription))
	mux.Handle("POST /api/admin/schools/{id}/subscriptions/extend", admin(h.AdminExtendSubscription))
	mux.Handle("POST /api/admin/invoices/{id}/verify", admin(h.AdminVerifyInvoice))
	mux.Handle("POST /api/admin/invoices/{id}/void", admin(h.AdminVoidInvoice))
	mux.Handle("GET /api/admin/invoices/{id}/pdf", admin(h.InvoicePDF))
	mux.Handle("GET /api/admin/invoices/{id}/proof", admin(h.Proof))
}
