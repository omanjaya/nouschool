package billing

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/omanjaya/nouschool/internal/platform/clock"
	"github.com/omanjaya/nouschool/internal/platform/httpx"
	"github.com/omanjaya/nouschool/internal/platform/storage"
)

// -- consumer-side interface (lihat CLAUDE.md: antar-modul lewat interface
// kecil dideklarasikan di sisi PEMAKAI, di-inject lewat constructor di
// main.go). Signature sengaja hanya memakai tipe primitif supaya modul lain
// memenuhi interface ini secara struktural TANPA billing perlu mengimpor
// package itu.

// AuditLogger adalah kebutuhan modul billing dari modul identity — dipenuhi
// *identity.Service secara struktural (pola yang sama dengan internal/tenant).
type AuditLogger interface {
	Log(ctx context.Context, schoolID, userID *int64, action, entity string, entityID *int64, oldValue, newValue any) error
}

// SchoolGateway adalah kebutuhan modul billing dari modul tenant: nama &
// slug sekolah untuk kop PDF invoice dan daftar panel super admin — dipenuhi
// *tenant.Service secara struktural lewat SchoolNameAndSlug.
type SchoolGateway interface {
	SchoolNameAndSlug(ctx context.Context, id int64) (name, slug string, err error)
}

// maxProofUploadSize — bukti transfer pdf/jpg/png maks 5MB (docs/09-billing.md scope Fase 10).
const maxProofUploadSize = 5 << 20

var allowedProofTypes = map[string]string{
	"application/pdf": "pdf",
	"image/jpeg":      "jpg",
	"image/png":       "png",
}

var (
	errProofTooLarge = httpx.Validation("Ukuran bukti transfer maksimal 5MB.")
	errProofType     = httpx.Validation("Bukti transfer harus berformat PDF, JPG, atau PNG.")
)

func detectProofExt(content []byte) (mime, ext string, err error) {
	if len(content) > maxProofUploadSize {
		return "", "", errProofTooLarge
	}
	mime = http.DetectContentType(content)
	for prefix, e := range allowedProofTypes {
		if mime == prefix {
			return mime, e, nil
		}
	}
	return "", "", errProofType
}

type cacheEntry struct {
	snap      snapshot
	expiresAt time.Time
}

// snapshot adalah potret status+fitur langganan satu sekolah, di-cache 60
// detik (docs/01-tenant.md "cached 60 dtk") supaya guard/RequireFeature/
// /api/me tidak query DB tiap request.
type snapshot struct {
	found      bool
	status     string
	planCode   string
	endsOn     time.Time
	graceUntil time.Time
	features   map[string]bool
}

// Service berisi aturan bisnis modul billing (docs/09-billing.md).
type Service struct {
	repo     billingRepository
	audit    AuditLogger
	schools  SchoolGateway
	files    *storage.Store
	clock    clock.Clock
	provider PaymentProvider // opsional — nil = pembayaran online belum dikonfigurasi

	cacheTTL time.Duration
	mu       sync.RWMutex
	cache    map[int64]cacheEntry
}

func NewService(repo *Repository, audit AuditLogger, schools SchoolGateway, files *storage.Store, clk clock.Clock) *Service {
	if clk == nil {
		clk = clock.System{}
	}
	if files == nil {
		files = storage.FromEnv()
	}
	return &Service{
		repo: repo, audit: audit, schools: schools, files: files, clock: clk,
		cacheTTL: 60 * time.Second, cache: map[int64]cacheEntry{},
	}
}

// newServiceForTest membangun Service dengan repository FAKE (in-memory,
// tanpa DB) — dipakai test di package ini saja.
func newServiceForTest(repo billingRepository, audit AuditLogger, clk clock.Clock) *Service {
	if clk == nil {
		clk = clock.System{}
	}
	return &Service{repo: repo, audit: audit, files: storage.New(""), clock: clk, cacheTTL: 60 * time.Second, cache: map[int64]cacheEntry{}}
}

// SetProvider menyuntikkan PaymentProvider SETELAH konstruksi (opsional,
// disuntik main.go bila MIDTRANS_SERVER_KEY tersedia — lihat provider.go).
func (s *Service) SetProvider(p PaymentProvider) { s.provider = p }

func (s *Service) logAudit(ctx context.Context, schoolID, actorUserID int64, action, entity string, entityID int64, oldValue, newValue any) {
	if s.audit == nil {
		return
	}
	sid, uid, eid := schoolID, actorUserID, entityID
	_ = s.audit.Log(ctx, &sid, &uid, action, entity, &eid, oldValue, newValue)
}

// today mengembalikan tanggal (tanpa jam, UTC) "hari ini" menurut clock —
// dipakai seluruh perhitungan periode/lifecycle (docs/09-billing.md).
func (s *Service) today() time.Time {
	t := s.clock.Now().UTC()
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

const dateLayout = "2006-01-02"

// -- bracket harga --

// bracketFor memilih bracket TERKECIL yang max_students-nya >= studentCount
// (prices HARUS terurut naik oleh max_students — dijamin ORDER BY di
// repository). Bracket terakhir (99999, docs/09: "unlimited") dipakai bila
// studentCount melebihi semua bracket.
func bracketFor(prices []PriceBracket, studentCount int) (PriceBracket, error) {
	if len(prices) == 0 {
		return PriceBracket{}, httpx.Validation("Plan ini belum punya harga bracket.")
	}
	for _, p := range prices {
		if studentCount <= p.MaxStudents {
			return p, nil
		}
	}
	return prices[len(prices)-1], nil
}

func (s *Service) invalidateCache(schoolID int64) {
	s.mu.Lock()
	delete(s.cache, schoolID)
	s.mu.Unlock()
}

func (s *Service) invalidateAllCache() {
	s.mu.Lock()
	s.cache = map[int64]cacheEntry{}
	s.mu.Unlock()
}

// snapshotFor mengembalikan snapshot status+fitur langganan sekolah, dari
// cache bila masih segar. Sekolah TANPA subscription sama sekali (found =
// false) diperlakukan status "readonly" — KEPUTUSAN implementasi (docs/09
// tidak eksplisit; sekolah baru dibuatkan subscription pertama oleh super
// admin saat onboarding, lihat catatan laporan akhir tugas & ROADMAP).
// Error repository (mis. DB down) diperlakukan sama (fail-closed) — gerbang
// billing sengaja gagal aman ke arah "tidak bisa menulis" daripada diam-diam
// meloloskan mutasi saat status langganan tidak bisa dipastikan.
func (s *Service) snapshotFor(ctx context.Context, schoolID int64) snapshot {
	now := s.clock.Now()
	s.mu.RLock()
	if e, ok := s.cache[schoolID]; ok && now.Before(e.expiresAt) {
		s.mu.RUnlock()
		return e.snap
	}
	s.mu.RUnlock()

	var snap snapshot
	sub, found, err := s.repo.GetSubscription(ctx, schoolID)
	if err != nil || !found {
		snap = snapshot{found: false, status: StatusReadonly}
	} else {
		snap = snapshot{
			found: true, status: sub.Status, planCode: sub.PlanCode,
			endsOn: sub.EndsOn, graceUntil: sub.GraceUntil(), features: sub.Features,
		}
	}

	s.mu.Lock()
	s.cache[schoolID] = cacheEntry{snap: snap, expiresAt: now.Add(s.cacheTTL)}
	s.mu.Unlock()
	return snap
}

// HasFeature melaporkan apakah fitur key aktif di snapshot langganan sekolah
// — dipakai modul lain (mis. notification) lewat consumer-side interface
// FeatureGate kecil (docs/09-billing.md "Feature gating").
func (s *Service) HasFeature(ctx context.Context, schoolID int64, key string) (bool, error) {
	snap := s.snapshotFor(ctx, schoolID)
	return snap.features[key], nil
}

// SubscriptionForMe mengembalikan ringkasan langganan (tipe primitif) untuk
// payload GET /api/me — dipakai modul identity lewat consumer-side interface
// BillingGateway (docs/09-billing.md "Frontend menerima daftar fitur aktif
// di payload /api/me").
func (s *Service) SubscriptionForMe(ctx context.Context, schoolID int64) (status, planCode, endsOn, graceUntil string, features []string, found bool, err error) {
	snap := s.snapshotFor(ctx, schoolID)
	if !snap.found {
		return "", "", "", "", nil, false, nil
	}
	return snap.status, snap.planCode, snap.endsOn.Format(dateLayout), snap.graceUntil.Format(dateLayout), activeFeatureKeys(snap.features), true, nil
}

// -- lifecycle (docs/09-billing.md "Lifecycle langganan") --

// TickOnce menjalankan SATU putaran transisi lifecycle (active->grace,
// grace->readonly) — idempoten, aman dipanggil berkali-kali. Dipanggil job
// ticker (lifecycle.go) DAN saat startup server, DAN bisa dipanggil manual
// (mis. verifikasi e2e / admin "cek status sekarang").
func (s *Service) TickOnce(ctx context.Context) error {
	today := s.today()
	if _, err := s.repo.TransitionActiveToGrace(ctx, today); err != nil {
		return fmt.Errorf("billing: transisi active->grace: %w", err)
	}
	if _, err := s.repo.TransitionGraceToReadonly(ctx, today); err != nil {
		return fmt.Errorf("billing: transisi grace->readonly: %w", err)
	}
	s.invalidateAllCache()
	return nil
}

// -- ActivateSubscription (docs/09-billing.md "Aktivasi = satu fungsi yang
// sama untuk kedua jalur ... idempotent") --

var errInvoiceNotActivatable = httpx.Validation("Invoice ini tidak bisa diaktivasi (sudah void/expired).")

// ActivateSubscription menandai invoice paid + meng-upsert subscriptions
// (snapshot plan saat ini, periode baru = max(ends_on lama, today) + 1
// tahun). Idempoten: invoice yang SUDAH paid tidak diproses ulang (dipanggil
// aman berkali-kali dari webhook gateway maupun verifikasi manual).
func (s *Service) ActivateSubscription(ctx context.Context, invoiceID int64) error {
	inv, err := s.repo.GetInvoice(ctx, invoiceID)
	if err != nil {
		return err
	}
	if inv.Status == InvoicePaid {
		return nil // idempoten — sudah diaktivasi sebelumnya
	}
	if inv.Status == InvoiceVoid || inv.Status == InvoiceExpired {
		return errInvoiceNotActivatable
	}

	plan, err := s.repo.GetPlan(ctx, inv.PlanCode)
	if err != nil {
		return err
	}

	existing, found, err := s.repo.GetSubscription(ctx, inv.SchoolID)
	if err != nil {
		return err
	}

	today := s.today()
	base := today
	startsOn := today
	if found {
		startsOn = existing.StartsOn
		if existing.EndsOn.After(today) {
			base = existing.EndsOn
		}
	}
	endsOn := base.AddDate(SubscriptionPeriodYears, 0, 0)

	saved, err := s.repo.UpsertSubscription(ctx, SubscriptionRecord{
		SchoolID: inv.SchoolID, PlanCode: plan.Code, Features: plan.Features,
		MaxStudents: inv.MaxStudents, Price: inv.Amount, StartsOn: startsOn, EndsOn: endsOn, Status: StatusActive,
	})
	if err != nil {
		return err
	}
	if inv.SubscriptionID == 0 {
		if err := s.repo.SetInvoiceSubscriptionID(ctx, inv.ID, saved.ID); err != nil {
			return err
		}
	}
	if err := s.repo.SetInvoicePaid(ctx, inv.ID, s.clock.Now()); err != nil {
		return err
	}
	s.invalidateCache(inv.SchoolID)

	s.logAudit(ctx, inv.SchoolID, 0, "billing.subscription_activate", "subscription", saved.ID,
		nil, map[string]any{"plan_code": saved.PlanCode, "ends_on": saved.EndsOn.Format(dateLayout), "invoice_id": inv.ID})
	return nil
}

// -- GET /api/billing & GET /api/admin/schools/{id}/billing --

func (s *Service) invoiceView(ctx context.Context, inv InvoiceRecord) InvoiceView {
	view := InvoiceView{
		ID: inv.ID, Number: inv.Number, Amount: inv.Amount, Status: inv.Status,
		DueAt: inv.DueAt, PaidAt: inv.PaidAt, PDFURL: fmt.Sprintf("/api/billing/invoices/%d/pdf", inv.ID),
	}
	pay, found, err := s.repo.LatestPaymentForInvoice(ctx, inv.ID)
	if err != nil || !found {
		return view
	}
	status := "pending"
	switch {
	case inv.Status == InvoicePaid:
		status = "paid"
	case pay.Method == MethodManualTransfer && pay.VerifiedAt == nil:
		status = "awaiting_verification"
	}
	pv := &PaymentView{Method: pay.Method, Status: status, VerifiedAt: pay.VerifiedAt, CreatedAt: pay.CreatedAt}
	if pay.ProofURL != "" {
		u := fmt.Sprintf("/api/billing/invoices/%d/proof", inv.ID)
		pv.ProofURL = &u
		view.ProofURL = &u
	}
	if pay.Provider != "" {
		p := pay.Provider
		pv.Provider = &p
	}
	view.Payment = pv
	return view
}

func (s *Service) subscriptionView(ctx context.Context, schoolID int64, sub SubscriptionRecord) (*SubscriptionView, error) {
	planName := sub.PlanCode
	if plan, err := s.repo.GetPlan(ctx, sub.PlanCode); err == nil {
		planName = plan.Name
	}
	studentCount, err := s.repo.CountActiveStudents(ctx, schoolID)
	if err != nil {
		return nil, err
	}
	var grace *string
	if sub.Status == StatusGrace || sub.Status == StatusReadonly {
		g := sub.GraceUntil().Format(dateLayout)
		grace = &g
	}
	return &SubscriptionView{
		PlanCode: sub.PlanCode, PlanName: planName, Status: sub.Status,
		StartsOn: sub.StartsOn.Format(dateLayout), EndsOn: sub.EndsOn.Format(dateLayout), GraceUntil: grace,
		MaxStudents: sub.MaxStudents, StudentCount: studentCount, Price: sub.Price, Features: activeFeatureKeys(sub.Features),
	}, nil
}

// activeFeatureKeys mengembalikan daftar TERURUT kunci fitur yang aktif
// (true) dari snapshot map fitur — bentuk yang dipakai payload JSON publik
// (SubscriptionView.Features, identity.SubscriptionForMe) supaya frontend
// cukup memakai `features.includes(key)` di mana pun (lihat catatan di model.go).
func activeFeatureKeys(features map[string]bool) []string {
	out := make([]string, 0, len(features))
	for k, v := range features {
		if v {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

// GetBillingView membangun shape lengkap GET /api/billing — dipakai baik
// endpoint tenant (schoolID dari reqctx) maupun panel super admin (schoolID
// dari path, lihat handler.go).
func (s *Service) GetBillingView(ctx context.Context, schoolID int64) (BillingView, error) {
	sub, found, err := s.repo.GetSubscription(ctx, schoolID)
	if err != nil {
		return BillingView{}, err
	}
	var subView *SubscriptionView
	if found {
		subView, err = s.subscriptionView(ctx, schoolID, sub)
		if err != nil {
			return BillingView{}, err
		}
	}
	invoices, err := s.repo.ListInvoicesForSchool(ctx, schoolID)
	if err != nil {
		return BillingView{}, err
	}
	views := make([]InvoiceView, 0, len(invoices))
	for _, inv := range invoices {
		views = append(views, s.invoiceView(ctx, inv))
	}
	return BillingView{Subscription: subView, Invoices: views}, nil
}

// -- POST /api/billing/invoices/{id}/proof --

// UploadProof menyimpan bukti transfer manual + set invoice
// awaiting_verification (docs/09-billing.md "Transfer manual"). schoolID
// WAJIB dari reqctx (isolasi tenant) — GetInvoiceForSchool menegakkannya.
func (s *Service) UploadProof(ctx context.Context, actorUserID, schoolID, invoiceID int64, filename string, content []byte) (InvoiceView, error) {
	inv, err := s.repo.GetInvoiceForSchool(ctx, invoiceID, schoolID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return InvoiceView{}, httpx.ErrNotFound
		}
		return InvoiceView{}, err
	}
	if inv.Status == InvoicePaid || inv.Status == InvoiceVoid || inv.Status == InvoiceExpired {
		return InvoiceView{}, httpx.Validation("Invoice ini sudah final, tidak bisa diunggah bukti transfer.")
	}

	_, ext, err := detectProofExt(content)
	if err != nil {
		return InvoiceView{}, err
	}
	name, err := storage.RandomFilename(ext)
	if err != nil {
		return InvoiceView{}, err
	}
	relPath := fmt.Sprintf("%d/billing/%s", schoolID, name)
	if err := s.files.Save(relPath, bytes.NewReader(content)); err != nil {
		return InvoiceView{}, err
	}

	if _, err := s.repo.CreateManualPayment(ctx, inv.ID, inv.Amount, relPath); err != nil {
		return InvoiceView{}, err
	}
	if err := s.repo.SetInvoiceStatus(ctx, inv.ID, InvoiceAwaitingVerification); err != nil {
		return InvoiceView{}, err
	}
	s.logAudit(ctx, schoolID, actorUserID, "billing.proof_upload", "invoice", inv.ID, nil, map[string]any{"filename": filename})

	updated, err := s.repo.GetInvoice(ctx, inv.ID)
	if err != nil {
		return InvoiceView{}, err
	}
	return s.invoiceView(ctx, updated), nil
}

// -- GET /api/billing/invoices/{id}/proof & /api/admin/invoices/{id}/proof --

// ProofFile adalah metadata yang dibutuhkan handler untuk stream file bukti transfer.
type ProofFile struct {
	Path        string
	ContentType string
}

// GetProof mengembalikan metadata bukti transfer TERBARU invoice ini.
// asSuperAdmin=true melewati pengecekan school_id (panel super admin bisa
// lihat bukti transfer sekolah manapun); selain itu invoice WAJIB milik schoolID.
func (s *Service) GetProof(ctx context.Context, schoolID, invoiceID int64, asSuperAdmin bool) (ProofFile, error) {
	var inv InvoiceRecord
	var err error
	if asSuperAdmin {
		inv, err = s.repo.GetInvoice(ctx, invoiceID)
	} else {
		inv, err = s.repo.GetInvoiceForSchool(ctx, invoiceID, schoolID)
	}
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return ProofFile{}, httpx.ErrNotFound
		}
		return ProofFile{}, err
	}
	pay, found, err := s.repo.LatestPaymentForInvoice(ctx, inv.ID)
	if err != nil {
		return ProofFile{}, err
	}
	if !found || pay.ProofURL == "" {
		return ProofFile{}, httpx.ErrNotFound
	}
	ct := "application/octet-stream"
	switch {
	case len(pay.ProofURL) > 4 && pay.ProofURL[len(pay.ProofURL)-4:] == ".pdf":
		ct = "application/pdf"
	case len(pay.ProofURL) > 4 && pay.ProofURL[len(pay.ProofURL)-4:] == ".jpg":
		ct = "image/jpeg"
	case len(pay.ProofURL) > 4 && pay.ProofURL[len(pay.ProofURL)-4:] == ".png":
		ct = "image/png"
	}
	return ProofFile{Path: pay.ProofURL, ContentType: ct}, nil
}

func (s *Service) OpenFile(path string) (io.ReadCloser, error) {
	return s.files.Open(path)
}

// -- GET /api/billing/invoices/{id}/pdf & /api/admin/invoices/{id}/pdf --

// GetInvoicePDF menyusun PDF invoice sederhana (docs/09-billing.md scope
// Fase 10). asSuperAdmin=true melewati pengecekan school_id.
func (s *Service) GetInvoicePDF(ctx context.Context, schoolID, invoiceID int64, asSuperAdmin bool) ([]byte, error) {
	var inv InvoiceRecord
	var err error
	if asSuperAdmin {
		inv, err = s.repo.GetInvoice(ctx, invoiceID)
	} else {
		inv, err = s.repo.GetInvoiceForSchool(ctx, invoiceID, schoolID)
	}
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, httpx.ErrNotFound
		}
		return nil, err
	}

	schoolName := fmt.Sprintf("Sekolah #%d", inv.SchoolID)
	if s.schools != nil {
		if name, _, err := s.schools.SchoolNameAndSlug(ctx, inv.SchoolID); err == nil && name != "" {
			schoolName = name
		}
	}
	planName := inv.PlanCode
	if plan, err := s.repo.GetPlan(ctx, inv.PlanCode); err == nil {
		planName = plan.Name
	}
	paidAt := ""
	if inv.PaidAt != nil {
		paidAt = inv.PaidAt.Format(dateLayout)
	}

	return BuildInvoicePDF(InvoicePDFData{
		Number: inv.Number, SchoolName: schoolName, PlanCode: inv.PlanCode, PlanName: planName,
		MaxStudents: inv.MaxStudents, Amount: inv.Amount, Status: inv.Status,
		DueAt: inv.DueAt.Format(dateLayout), PaidAt: paidAt,
	})
}

// -- POST /api/billing/invoices/{id}/pay (gateway) --

var errGatewayNotConfigured = &httpx.Error{Status: http.StatusUnprocessableEntity, Code: "gateway_not_configured", Message: "Pembayaran online belum dikonfigurasi."}

// Pay membuat transaksi gateway (Midtrans Snap) untuk invoice ini dan
// mengembalikan redirect_url (docs/09-billing.md "Gateway ... buat transaksi
// Midtrans/Xendit -> redirect/checkout"). schoolID dari reqctx menegakkan
// invoice ini milik sekolah pemanggil.
func (s *Service) Pay(ctx context.Context, actorUserID, schoolID, invoiceID int64) (string, error) {
	if s.provider == nil {
		return "", errGatewayNotConfigured
	}
	inv, err := s.repo.GetInvoiceForSchool(ctx, invoiceID, schoolID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return "", httpx.ErrNotFound
		}
		return "", err
	}
	if inv.Status == InvoicePaid {
		return "", httpx.Validation("Invoice ini sudah lunas.")
	}
	if inv.Status == InvoiceVoid || inv.Status == InvoiceExpired {
		return "", httpx.Validation("Invoice ini sudah tidak berlaku.")
	}

	schoolName, _, err := s.schools.SchoolNameAndSlug(ctx, schoolID)
	if err != nil {
		return "", err
	}

	orderID := fmt.Sprintf("INV-%d-%d", inv.ID, s.clock.Now().UnixNano())
	redirectURL, err := s.provider.CreateTransaction(ctx, TransactionRequest{
		OrderID: orderID, Amount: inv.Amount, ItemName: fmt.Sprintf("Langganan NouSchool (%s)", inv.Number), CustomerName: schoolName,
	})
	if err != nil {
		return "", err
	}
	if _, err := s.repo.CreateGatewayPayment(ctx, inv.ID, inv.Amount, s.provider.Name(), orderID); err != nil {
		return "", err
	}
	s.logAudit(ctx, schoolID, actorUserID, "billing.pay_gateway", "invoice", inv.ID, nil, map[string]any{"order_id": orderID})
	return redirectURL, nil
}

// -- POST /api/webhooks/midtrans --

// HandleGatewayWebhook memverifikasi signature, idempotent by order_id
// (payments.provider_ref), lalu mengaktivasi langganan bila status
// settlement/capture (docs/09-billing.md "webhook masuk ... idempotent by
// provider_ref"). TIDAK bergantung reqctx sekolah — school_id diresolusi
// dari invoice yang tertaut ke payment.
func (s *Service) HandleGatewayWebhook(ctx context.Context, raw []byte, orderID, statusCode, grossAmount, transactionStatus, fraudStatus, signatureKey string) error {
	if s.provider == nil {
		return errGatewayNotConfigured
	}
	if !s.provider.VerifySignature(orderID, statusCode, grossAmount, signatureKey) {
		return httpx.Validation("Signature webhook tidak valid.")
	}

	pay, found, err := s.repo.GetPaymentByProviderRef(ctx, orderID)
	if err != nil {
		return err
	}
	if !found {
		// order_id tidak dikenal (mis. webhook uji dari dashboard Midtrans) —
		// bukan error klien, cukup diabaikan (200, tidak ada yang diproses).
		return nil
	}
	if err := s.repo.SetPaymentRawWebhook(ctx, pay.ID, raw); err != nil {
		return err
	}

	status := s.provider.TransactionStatus(transactionStatus, fraudStatus)
	if status != TransactionStatusPaid {
		return nil // pending/deny/expire — tidak ada aktivasi, webhook tetap 200 (idempotent no-op)
	}

	inv, err := s.repo.GetInvoice(ctx, pay.InvoiceID)
	if err != nil {
		return err
	}
	if err := s.ActivateSubscription(ctx, inv.ID); err != nil {
		return err
	}
	s.logAudit(ctx, inv.SchoolID, 0, "billing.webhook_paid", "invoice", inv.ID, nil, map[string]any{"order_id": orderID})
	return nil
}

// -- admin: plans --

func (s *Service) ListPlans(ctx context.Context) ([]PlanView, error) {
	plans, err := s.repo.ListPlans(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]PlanView, 0, len(plans))
	for _, p := range plans {
		out = append(out, PlanView{Code: p.Code, Name: p.Name, Active: p.Active, Features: p.Features, Prices: p.Prices})
	}
	return out, nil
}

// UpdatePlanInput adalah parameter Service.UpdatePlan — Features/Prices
// kosong (nil) tidak diubah; Name kosong ("") juga tidak diubah (frontend
// SELALU mengirim nama saat ini, lihat web/src/features/admin/PlansPage.tsx
// handleSave: `{name: plan.name, features, prices}`).
type UpdatePlanInput struct {
	Name     string
	Features map[string]bool
	Prices   []PriceBracket
}

// UpdatePlan mengubah nama/fitur/harga MASTER plan (TIDAK mengubah snapshot
// langganan yang sedang berjalan — docs/09-billing.md "perubahan plan tidak
// mengubah langganan berjalan").
func (s *Service) UpdatePlan(ctx context.Context, actorUserID int64, code string, in UpdatePlanInput) (PlanView, error) {
	if _, err := s.repo.GetPlan(ctx, code); err != nil {
		if errors.Is(err, ErrNotFound) {
			return PlanView{}, httpx.ErrNotFound
		}
		return PlanView{}, err
	}
	if in.Name != "" {
		if err := s.repo.SavePlanName(ctx, code, in.Name); err != nil {
			return PlanView{}, err
		}
	}
	if in.Features != nil {
		if err := s.repo.SavePlanFeatures(ctx, code, in.Features); err != nil {
			return PlanView{}, err
		}
	}
	if in.Prices != nil {
		for _, p := range in.Prices {
			if p.MaxStudents <= 0 || p.PriceYearly < 0 {
				return PlanView{}, httpx.Validation("Bracket harga tidak valid.")
			}
		}
		if err := s.repo.SavePlanPrices(ctx, code, in.Prices); err != nil {
			return PlanView{}, err
		}
	}
	s.logAudit(ctx, 0, actorUserID, "billing.plan_update", "plan", 0, nil, map[string]any{"code": code})

	updated, err := s.repo.GetPlan(ctx, code)
	if err != nil {
		return PlanView{}, err
	}
	return PlanView{Code: updated.Code, Name: updated.Name, Active: updated.Active, Features: updated.Features, Prices: updated.Prices}, nil
}

// -- admin: subscriptions & invoices --

// CreateSubscriptionInvoice — POST /api/admin/schools/{id}/subscriptions
// {plan_code}: hitung bracket dari jumlah siswa AKTIF, buat invoice unpaid
// (docs/09-billing.md "Bracket siswa": dihitung saat subscribe/renew).
// Renewal (subscription sudah ada) memakai alur & fungsi yang SAMA.
func (s *Service) CreateSubscriptionInvoice(ctx context.Context, actorUserID, schoolID int64, planCode string) (InvoiceView, error) {
	plan, err := s.repo.GetPlan(ctx, planCode)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return InvoiceView{}, httpx.Validation("Plan tidak dikenal: " + planCode)
		}
		return InvoiceView{}, err
	}
	if !plan.Active {
		return InvoiceView{}, httpx.Validation("Plan ini sudah tidak aktif ditawarkan.")
	}

	studentCount, err := s.repo.CountActiveStudents(ctx, schoolID)
	if err != nil {
		return InvoiceView{}, err
	}
	bracket, err := bracketFor(plan.Prices, studentCount)
	if err != nil {
		return InvoiceView{}, err
	}

	existing, found, err := s.repo.GetSubscription(ctx, schoolID)
	if err != nil {
		return InvoiceView{}, err
	}
	var subscriptionID int64
	if found {
		subscriptionID = existing.ID
	}

	number, err := s.nextInvoiceNumber(ctx)
	if err != nil {
		return InvoiceView{}, err
	}

	inv, err := s.repo.CreateInvoice(ctx, CreateInvoiceInput{
		SchoolID: schoolID, Number: number, SubscriptionID: subscriptionID, Amount: bracket.PriceYearly,
		Status: InvoiceUnpaid, DueAt: s.clock.Now().Add(7 * 24 * time.Hour), PlanCode: plan.Code,
		MaxStudents: bracket.MaxStudents, CreatedBy: actorUserID,
	})
	if err != nil {
		return InvoiceView{}, err
	}
	s.logAudit(ctx, schoolID, actorUserID, "billing.invoice_create", "invoice", inv.ID, nil,
		map[string]any{"number": inv.Number, "plan_code": plan.Code, "amount": inv.Amount})
	return s.invoiceView(ctx, inv), nil
}

func (s *Service) nextInvoiceNumber(ctx context.Context) (string, error) {
	year := fmt.Sprintf("%d", s.clock.Now().Year())
	seq, err := s.repo.NextInvoiceNumber(ctx, year)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("INV/%s/%04d", year, seq), nil
}

// VerifyManualPayment — POST /api/admin/invoices/{id}/verify: transfer
// manual disetujui super admin -> paid -> ActivateSubscription (docs/09).
func (s *Service) VerifyManualPayment(ctx context.Context, actorUserID, invoiceID int64) (InvoiceView, error) {
	inv, err := s.repo.GetInvoice(ctx, invoiceID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return InvoiceView{}, httpx.ErrNotFound
		}
		return InvoiceView{}, err
	}
	if inv.Status == InvoicePaid {
		return s.invoiceView(ctx, inv), nil // idempoten
	}
	if inv.Status != InvoiceUnpaid && inv.Status != InvoiceAwaitingVerification {
		return InvoiceView{}, httpx.Validation("Invoice ini sudah final (void/expired), tidak bisa diverifikasi.")
	}
	// Super admin BOLEH memverifikasi invoice yang MASIH unpaid (belum ada
	// bukti transfer diunggah sekolah, mis. konfirmasi manual dari mutasi
	// rekening) — bukan hanya yang sudah awaiting_verification (frontend:
	// web/src/features/admin/SchoolDetailPage.tsx tombol "Verifikasi
	// Pembayaran" tampil utk status unpaid MAUPUN awaiting_verification).
	pay, found, err := s.repo.LatestPaymentForInvoice(ctx, inv.ID)
	if err != nil {
		return InvoiceView{}, err
	}
	if !found {
		pay, err = s.repo.CreateManualPayment(ctx, inv.ID, inv.Amount, "")
		if err != nil {
			return InvoiceView{}, err
		}
	}
	if err := s.repo.VerifyManualPayment(ctx, pay.ID, actorUserID, s.clock.Now()); err != nil {
		return InvoiceView{}, err
	}
	if err := s.ActivateSubscription(ctx, inv.ID); err != nil {
		return InvoiceView{}, err
	}
	s.logAudit(ctx, inv.SchoolID, actorUserID, "billing.verify_manual", "invoice", inv.ID, nil, nil)

	updated, err := s.repo.GetInvoice(ctx, inv.ID)
	if err != nil {
		return InvoiceView{}, err
	}
	return s.invoiceView(ctx, updated), nil
}

// VoidInvoice — POST /api/admin/invoices/{id}/void.
func (s *Service) VoidInvoice(ctx context.Context, actorUserID, invoiceID int64) (InvoiceView, error) {
	inv, err := s.repo.GetInvoice(ctx, invoiceID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return InvoiceView{}, httpx.ErrNotFound
		}
		return InvoiceView{}, err
	}
	if inv.Status == InvoicePaid {
		return InvoiceView{}, httpx.Validation("Invoice yang sudah lunas tidak bisa di-void.")
	}
	if err := s.repo.SetInvoiceStatus(ctx, inv.ID, InvoiceVoid); err != nil {
		return InvoiceView{}, err
	}
	s.logAudit(ctx, inv.SchoolID, actorUserID, "billing.invoice_void", "invoice", inv.ID,
		map[string]any{"status": inv.Status}, map[string]any{"status": InvoiceVoid})

	updated, err := s.repo.GetInvoice(ctx, inv.ID)
	if err != nil {
		return InvoiceView{}, err
	}
	return s.invoiceView(ctx, updated), nil
}

// ExtendSubscriptionGoodwill — POST /api/admin/schools/{id}/subscriptions/extend
// {days}: perpanjangan tanpa pembayaran (goodwill super admin), audit wajib
// (docs/09-billing.md "Panel super admin ... perpanjang manual (goodwill)").
func (s *Service) ExtendSubscriptionGoodwill(ctx context.Context, actorUserID, schoolID int64, days int) error {
	if days <= 0 {
		return httpx.Validation("Jumlah hari perpanjangan harus lebih dari 0.")
	}
	if _, found, err := s.repo.GetSubscription(ctx, schoolID); err != nil {
		return err
	} else if !found {
		return httpx.Validation("Sekolah ini belum pernah punya langganan — buat langganan pertama dulu.")
	}
	if err := s.repo.ExtendSubscription(ctx, schoolID, days); err != nil {
		return err
	}
	s.invalidateCache(schoolID)
	s.logAudit(ctx, schoolID, actorUserID, "billing.extend_goodwill", "subscription", schoolID, nil, map[string]any{"days": days})
	return nil
}

// ListSubscriptionsAdmin — status semua sekolah (panel super admin).
func (s *Service) ListSubscriptionsAdmin(ctx context.Context) ([]AdminSubscriptionRow, error) {
	return s.repo.ListSubscriptionsForAdmin(ctx)
}
