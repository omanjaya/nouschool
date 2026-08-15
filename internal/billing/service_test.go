package billing

import (
	"context"
	"crypto/sha512"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/omanjaya/nouschool/internal/platform/clock"
	"github.com/omanjaya/nouschool/internal/platform/httpx"
	"github.com/omanjaya/nouschool/internal/platform/reqctx"
)

// -- fakeRepo: implementasi billingRepository in-memory (tanpa DB) — pola
// yang sama dengan internal/leave/service_test.go.

type fakeRepo struct {
	plans          map[string]PlanRecord
	subscriptions  map[int64]SubscriptionRecord // key: schoolID
	nextSubID      int64
	invoices       map[int64]InvoiceRecord
	nextInvoiceID  int64
	counters       map[string]int
	payments       map[int64]PaymentRecord
	nextPaymentID  int64
	activeStudents map[int64]int
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		plans: map[string]PlanRecord{
			"basic": {
				Code: "basic", Name: "Basic", Active: true,
				Features: map[string]bool{FeatureTVDashboard: false, FeatureWhatsApp: false, FeatureQRCard: true, FeatureSelfCheckin: true},
				Prices:   []PriceBracket{{MaxStudents: 300, PriceYearly: 2_000_000}, {MaxStudents: 600, PriceYearly: 3_500_000}, {MaxStudents: 99999, PriceYearly: 5_000_000}},
			},
			"pro": {
				Code: "pro", Name: "Pro", Active: true,
				Features: map[string]bool{FeatureTVDashboard: true, FeatureWhatsApp: true, FeatureQRCard: true, FeatureSelfCheckin: true},
				Prices:   []PriceBracket{{MaxStudents: 300, PriceYearly: 4_000_000}, {MaxStudents: 600, PriceYearly: 7_000_000}, {MaxStudents: 99999, PriceYearly: 10_000_000}},
			},
		},
		subscriptions:  map[int64]SubscriptionRecord{},
		invoices:       map[int64]InvoiceRecord{},
		counters:       map[string]int{},
		payments:       map[int64]PaymentRecord{},
		activeStudents: map[int64]int{},
	}
}

func (f *fakeRepo) ListPlans(ctx context.Context) ([]PlanRecord, error) {
	out := make([]PlanRecord, 0, len(f.plans))
	for _, p := range f.plans {
		out = append(out, p)
	}
	return out, nil
}

func (f *fakeRepo) GetPlan(ctx context.Context, code string) (PlanRecord, error) {
	p, ok := f.plans[code]
	if !ok {
		return PlanRecord{}, ErrNotFound
	}
	return p, nil
}

func (f *fakeRepo) SavePlanName(ctx context.Context, code, name string) error {
	p, ok := f.plans[code]
	if !ok {
		return ErrNotFound
	}
	p.Name = name
	f.plans[code] = p
	return nil
}

func (f *fakeRepo) SavePlanFeatures(ctx context.Context, code string, features map[string]bool) error {
	p, ok := f.plans[code]
	if !ok {
		return ErrNotFound
	}
	p.Features = features
	f.plans[code] = p
	return nil
}

func (f *fakeRepo) SavePlanPrices(ctx context.Context, code string, prices []PriceBracket) error {
	p, ok := f.plans[code]
	if !ok {
		return ErrNotFound
	}
	p.Prices = prices
	f.plans[code] = p
	return nil
}

func (f *fakeRepo) GetSubscription(ctx context.Context, schoolID int64) (SubscriptionRecord, bool, error) {
	rec, ok := f.subscriptions[schoolID]
	return rec, ok, nil
}

func (f *fakeRepo) UpsertSubscription(ctx context.Context, rec SubscriptionRecord) (SubscriptionRecord, error) {
	existing, ok := f.subscriptions[rec.SchoolID]
	if ok {
		rec.ID = existing.ID
	} else {
		f.nextSubID++
		rec.ID = f.nextSubID
	}
	f.subscriptions[rec.SchoolID] = rec
	return rec, nil
}

func (f *fakeRepo) ExtendSubscription(ctx context.Context, schoolID int64, days int) error {
	rec, ok := f.subscriptions[schoolID]
	if !ok {
		return ErrNotFound
	}
	rec.EndsOn = rec.EndsOn.AddDate(0, 0, days)
	rec.Status = StatusActive
	f.subscriptions[schoolID] = rec
	return nil
}

func (f *fakeRepo) TransitionActiveToGrace(ctx context.Context, today time.Time) (int64, error) {
	var n int64
	for id, rec := range f.subscriptions {
		if rec.Status == StatusActive && rec.EndsOn.Before(today) {
			rec.Status = StatusGrace
			f.subscriptions[id] = rec
			n++
		}
	}
	return n, nil
}

func (f *fakeRepo) TransitionGraceToReadonly(ctx context.Context, today time.Time) (int64, error) {
	var n int64
	for id, rec := range f.subscriptions {
		if rec.Status == StatusGrace && rec.EndsOn.AddDate(0, 0, GracePeriodDays).Before(today) {
			rec.Status = StatusReadonly
			f.subscriptions[id] = rec
			n++
		}
	}
	return n, nil
}

func (f *fakeRepo) ListSubscriptionsForAdmin(ctx context.Context) ([]AdminSubscriptionRow, error) {
	return nil, nil
}

func (f *fakeRepo) NextInvoiceNumber(ctx context.Context, year string) (int, error) {
	f.counters[year]++
	return f.counters[year], nil
}

func (f *fakeRepo) CreateInvoice(ctx context.Context, in CreateInvoiceInput) (InvoiceRecord, error) {
	f.nextInvoiceID++
	rec := InvoiceRecord{
		ID: f.nextInvoiceID, SchoolID: in.SchoolID, Number: in.Number, SubscriptionID: in.SubscriptionID,
		Amount: in.Amount, Status: in.Status, DueAt: in.DueAt, PlanCode: in.PlanCode, MaxStudents: in.MaxStudents,
		CreatedBy: in.CreatedBy, CreatedAt: fixedNow,
	}
	f.invoices[rec.ID] = rec
	return rec, nil
}

func (f *fakeRepo) GetInvoice(ctx context.Context, id int64) (InvoiceRecord, error) {
	rec, ok := f.invoices[id]
	if !ok {
		return InvoiceRecord{}, ErrNotFound
	}
	return rec, nil
}

func (f *fakeRepo) GetInvoiceForSchool(ctx context.Context, id, schoolID int64) (InvoiceRecord, error) {
	rec, ok := f.invoices[id]
	if !ok || rec.SchoolID != schoolID {
		return InvoiceRecord{}, ErrNotFound
	}
	return rec, nil
}

func (f *fakeRepo) ListInvoicesForSchool(ctx context.Context, schoolID int64) ([]InvoiceRecord, error) {
	var out []InvoiceRecord
	for _, rec := range f.invoices {
		if rec.SchoolID == schoolID {
			out = append(out, rec)
		}
	}
	return out, nil
}

func (f *fakeRepo) SetInvoiceStatus(ctx context.Context, id int64, status string) error {
	rec, ok := f.invoices[id]
	if !ok {
		return ErrNotFound
	}
	rec.Status = status
	f.invoices[id] = rec
	return nil
}

func (f *fakeRepo) SetInvoicePaid(ctx context.Context, id int64, paidAt time.Time) error {
	rec, ok := f.invoices[id]
	if !ok {
		return ErrNotFound
	}
	rec.Status = InvoicePaid
	rec.PaidAt = &paidAt
	f.invoices[id] = rec
	return nil
}

func (f *fakeRepo) SetInvoiceSubscriptionID(ctx context.Context, id, subscriptionID int64) error {
	rec, ok := f.invoices[id]
	if !ok {
		return ErrNotFound
	}
	rec.SubscriptionID = subscriptionID
	f.invoices[id] = rec
	return nil
}

func (f *fakeRepo) CreateManualPayment(ctx context.Context, invoiceID, amount int64, proofURL string) (PaymentRecord, error) {
	f.nextPaymentID++
	rec := PaymentRecord{ID: f.nextPaymentID, InvoiceID: invoiceID, Method: MethodManualTransfer, Amount: amount, ProofURL: proofURL, CreatedAt: fixedNow}
	f.payments[rec.ID] = rec
	return rec, nil
}

func (f *fakeRepo) CreateGatewayPayment(ctx context.Context, invoiceID, amount int64, provider, providerRef string) (PaymentRecord, error) {
	f.nextPaymentID++
	rec := PaymentRecord{ID: f.nextPaymentID, InvoiceID: invoiceID, Method: MethodGateway, Amount: amount, Provider: provider, ProviderRef: providerRef, CreatedAt: fixedNow}
	f.payments[rec.ID] = rec
	return rec, nil
}

func (f *fakeRepo) VerifyManualPayment(ctx context.Context, paymentID, verifiedBy int64, verifiedAt time.Time) error {
	rec, ok := f.payments[paymentID]
	if !ok {
		return ErrNotFound
	}
	rec.VerifiedBy = verifiedBy
	rec.VerifiedAt = &verifiedAt
	f.payments[paymentID] = rec
	return nil
}

func (f *fakeRepo) GetPaymentByProviderRef(ctx context.Context, providerRef string) (PaymentRecord, bool, error) {
	for _, rec := range f.payments {
		if rec.ProviderRef == providerRef {
			return rec, true, nil
		}
	}
	return PaymentRecord{}, false, nil
}

func (f *fakeRepo) SetPaymentRawWebhook(ctx context.Context, paymentID int64, raw []byte) error {
	rec, ok := f.payments[paymentID]
	if !ok {
		return ErrNotFound
	}
	rec.RawWebhook = raw
	f.payments[paymentID] = rec
	return nil
}

func (f *fakeRepo) ListPaymentsForInvoice(ctx context.Context, invoiceID int64) ([]PaymentRecord, error) {
	var out []PaymentRecord
	for _, rec := range f.payments {
		if rec.InvoiceID == invoiceID {
			out = append(out, rec)
		}
	}
	return out, nil
}

func (f *fakeRepo) LatestPaymentForInvoice(ctx context.Context, invoiceID int64) (PaymentRecord, bool, error) {
	var latest PaymentRecord
	found := false
	for _, rec := range f.payments {
		if rec.InvoiceID == invoiceID && (!found || rec.CreatedAt.After(latest.CreatedAt) || rec.ID > latest.ID) {
			latest = rec
			found = true
		}
	}
	return latest, found, nil
}

func (f *fakeRepo) CountActiveStudents(ctx context.Context, schoolID int64) (int, error) {
	return f.activeStudents[schoolID], nil
}

// fakeAudit implementasi AuditLogger in-memory.
type fakeAudit struct{ entries []string }

func (a *fakeAudit) Log(ctx context.Context, schoolID, userID *int64, action, entity string, entityID *int64, oldValue, newValue any) error {
	a.entries = append(a.entries, action)
	return nil
}

var fixedNow = time.Date(2026, 8, 16, 3, 0, 0, 0, time.UTC)

func newTestService(repo *fakeRepo, now time.Time) (*Service, *fakeAudit) {
	audit := &fakeAudit{}
	svc := newServiceForTest(repo, audit, clock.Fixed{T: now})
	return svc, audit
}

// -- bracket by student count --

func TestBracketFor(t *testing.T) {
	prices := []PriceBracket{{MaxStudents: 300, PriceYearly: 2_000_000}, {MaxStudents: 600, PriceYearly: 3_500_000}, {MaxStudents: 99999, PriceYearly: 5_000_000}}
	cases := []struct {
		count int
		want  int64
	}{
		{count: 50, want: 2_000_000},
		{count: 300, want: 2_000_000},
		{count: 301, want: 3_500_000},
		{count: 600, want: 3_500_000},
		{count: 601, want: 5_000_000},
		{count: 100000, want: 5_000_000}, // melebihi bracket terbesar -> tetap bracket terakhir
	}
	for _, c := range cases {
		got, err := bracketFor(prices, c.count)
		if err != nil {
			t.Fatalf("count=%d: %v", c.count, err)
		}
		if got.PriceYearly != c.want {
			t.Errorf("count=%d: got price %d, want %d", c.count, got.PriceYearly, c.want)
		}
	}
}

// -- lifecycle: active -> grace -> readonly by tanggal, idempoten --

func TestTickOnce_ActiveToGraceToReadonly(t *testing.T) {
	repo := newFakeRepo()
	repo.subscriptions[1] = SubscriptionRecord{
		ID: 1, SchoolID: 1, PlanCode: "basic", Features: map[string]bool{"qr_card": true},
		MaxStudents: 300, Price: 2_000_000, StartsOn: fixedNow.AddDate(-1, 0, 0), EndsOn: fixedNow.AddDate(0, 0, -1), Status: StatusActive,
	}
	svc, _ := newTestService(repo, fixedNow)

	if err := svc.TickOnce(context.Background()); err != nil {
		t.Fatalf("TickOnce: %v", err)
	}
	if got := repo.subscriptions[1].Status; got != StatusGrace {
		t.Fatalf("setelah ends_on lewat: status = %q, want grace", got)
	}

	// Idempoten: tick lagi hari yang sama, tetap grace (belum 14 hari).
	if err := svc.TickOnce(context.Background()); err != nil {
		t.Fatalf("TickOnce kedua: %v", err)
	}
	if got := repo.subscriptions[1].Status; got != StatusGrace {
		t.Fatalf("tick kedua (masih grace): status = %q, want grace", got)
	}

	// Maju 15 hari lewat grace period 14 hari -> readonly.
	svc2, _ := newTestService(repo, fixedNow.AddDate(0, 0, 15))
	if err := svc2.TickOnce(context.Background()); err != nil {
		t.Fatalf("TickOnce (grace->readonly): %v", err)
	}
	if got := repo.subscriptions[1].Status; got != StatusReadonly {
		t.Fatalf("setelah grace period lewat: status = %q, want readonly", got)
	}

	// Idempoten: tick lagi, tetap readonly (tidak ada transisi lanjutan tanpa pembayaran).
	if err := svc2.TickOnce(context.Background()); err != nil {
		t.Fatalf("TickOnce ulang: %v", err)
	}
	if got := repo.subscriptions[1].Status; got != StatusReadonly {
		t.Fatalf("tick ulang: status = %q, want tetap readonly", got)
	}
}

// -- ActivateSubscription: idempoten, perpanjang dari sisa masa aktif,
// snapshot fitur tidak berubah saat plan diedit setelahnya --

func TestActivateSubscription_NewSchool(t *testing.T) {
	repo := newFakeRepo()
	repo.activeStudents[1] = 50
	svc, _ := newTestService(repo, fixedNow)

	inv, err := repo.CreateInvoice(context.Background(), CreateInvoiceInput{
		SchoolID: 1, Number: "INV/2026/0001", Amount: 2_000_000, Status: InvoiceUnpaid,
		DueAt: fixedNow, PlanCode: "basic", MaxStudents: 300,
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := svc.ActivateSubscription(context.Background(), inv.ID); err != nil {
		t.Fatalf("ActivateSubscription: %v", err)
	}
	sub := repo.subscriptions[1]
	if sub.Status != StatusActive {
		t.Fatalf("status = %q, want active", sub.Status)
	}
	today := time.Date(fixedNow.Year(), fixedNow.Month(), fixedNow.Day(), 0, 0, 0, 0, time.UTC)
	wantEndsOn := today.AddDate(1, 0, 0)
	if !sub.EndsOn.Equal(wantEndsOn) {
		t.Fatalf("ends_on = %v, want %v (today + 1 tahun, sekolah baru)", sub.EndsOn, wantEndsOn)
	}
	if !sub.Features["qr_card"] {
		t.Fatalf("snapshot fitur qr_card harus true (dari plan basic)")
	}

	// Idempoten: invoice sudah paid, panggil lagi tidak mengubah ends_on.
	if err := svc.ActivateSubscription(context.Background(), inv.ID); err != nil {
		t.Fatalf("ActivateSubscription idempoten: %v", err)
	}
	if got := repo.subscriptions[1].EndsOn; !got.Equal(wantEndsOn) {
		t.Fatalf("ends_on berubah setelah panggilan idempoten: got %v, want %v", got, wantEndsOn)
	}
}

func TestActivateSubscription_RenewExtendsFromRemainingPeriod(t *testing.T) {
	repo := newFakeRepo()
	repo.activeStudents[1] = 50
	// Subscription MASIH aktif, ends_on 100 hari lagi.
	existingEndsOn := fixedNow.AddDate(0, 0, 100)
	repo.subscriptions[1] = SubscriptionRecord{
		ID: 1, SchoolID: 1, PlanCode: "basic", Features: map[string]bool{"qr_card": true},
		MaxStudents: 300, Price: 2_000_000, StartsOn: fixedNow.AddDate(-1, 0, 0), EndsOn: existingEndsOn, Status: StatusActive,
	}
	svc, _ := newTestService(repo, fixedNow)

	inv, err := repo.CreateInvoice(context.Background(), CreateInvoiceInput{
		SchoolID: 1, Number: "INV/2026/0002", SubscriptionID: 1, Amount: 2_000_000, Status: InvoiceUnpaid,
		DueAt: fixedNow, PlanCode: "basic", MaxStudents: 300,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.ActivateSubscription(context.Background(), inv.ID); err != nil {
		t.Fatalf("ActivateSubscription: %v", err)
	}

	// Formula docs/09: periode baru = max(ends_on lama, today) + 1 tahun.
	want := existingEndsOn.AddDate(1, 0, 0)
	if got := repo.subscriptions[1].EndsOn; !got.Equal(want) {
		t.Fatalf("ends_on renewal = %v, want %v (existing ends_on + 1 tahun)", got, want)
	}
}

func TestActivateSubscription_SnapshotUnaffectedByLaterPlanEdit(t *testing.T) {
	repo := newFakeRepo()
	repo.activeStudents[1] = 50
	svc, _ := newTestService(repo, fixedNow)

	inv, err := repo.CreateInvoice(context.Background(), CreateInvoiceInput{
		SchoolID: 1, Number: "INV/2026/0001", Amount: 2_000_000, Status: InvoiceUnpaid,
		DueAt: fixedNow, PlanCode: "basic", MaxStudents: 300,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.ActivateSubscription(context.Background(), inv.ID); err != nil {
		t.Fatal(err)
	}
	if repo.subscriptions[1].Features[FeatureTVDashboard] {
		t.Fatalf("basic seharusnya belum punya tv_dashboard saat aktivasi")
	}

	// Plan basic diedit SETELAH aktivasi (super admin menambahkan tv_dashboard).
	if err := repo.SavePlanFeatures(context.Background(), "basic", map[string]bool{FeatureTVDashboard: true, FeatureQRCard: true}); err != nil {
		t.Fatal(err)
	}

	// Snapshot langganan yang SUDAH berjalan TIDAK berubah (docs/09-billing.md
	// "perubahan plan tidak mengubah langganan berjalan").
	if repo.subscriptions[1].Features[FeatureTVDashboard] {
		t.Fatalf("snapshot subscription seharusnya TIDAK berubah setelah plan diedit")
	}
}

func TestActivateSubscription_VoidInvoiceRejected(t *testing.T) {
	repo := newFakeRepo()
	svc, _ := newTestService(repo, fixedNow)
	inv, _ := repo.CreateInvoice(context.Background(), CreateInvoiceInput{
		SchoolID: 1, Number: "INV/2026/0001", Amount: 2_000_000, Status: InvoiceVoid,
		DueAt: fixedNow, PlanCode: "basic", MaxStudents: 300,
	})
	err := svc.ActivateSubscription(context.Background(), inv.ID)
	if !errors.Is(err, errInvoiceNotActivatable) {
		t.Fatalf("err = %v, want errInvoiceNotActivatable", err)
	}
}

// -- nomor invoice berurutan per tahun --

func TestNextInvoiceNumber_SequentialPerYear(t *testing.T) {
	repo := newFakeRepo()
	svc, _ := newTestService(repo, fixedNow)
	n1, err := svc.nextInvoiceNumber(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	n2, err := svc.nextInvoiceNumber(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if n1 != "INV/2026/0001" || n2 != "INV/2026/0002" {
		t.Fatalf("got %q, %q — want INV/2026/0001, INV/2026/0002", n1, n2)
	}

	// Tahun berbeda -> counter mulai dari 1 lagi (global per tahun, lintas sekolah).
	svcNextYear, _ := newTestService(repo, time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC))
	n3, err := svcNextYear.nextInvoiceNumber(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if n3 != "INV/2027/0001" {
		t.Fatalf("got %q, want INV/2027/0001", n3)
	}
}

// -- guard: mutasi ditolak saat readonly, GET & /api/billing lolos --

func TestGuard_ReadonlyBlocksNonGetExceptExemptPaths(t *testing.T) {
	repo := newFakeRepo()
	// Sekolah 1: tanpa subscription sama sekali -> diperlakukan readonly.
	svc, _ := newTestService(repo, fixedNow)

	ctx := reqctx.WithSchool(context.Background(), reqctx.School{ID: 1, Name: "Sekolah Uji"})
	ctx = reqctx.WithUser(ctx, 10, "admin_sekolah", false)

	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true; w.WriteHeader(http.StatusOK) })

	// POST mutasi biasa -> 402 subscription_readonly.
	req := httptest.NewRequest(http.MethodPost, "/api/students", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	svc.Middleware(next).ServeHTTP(rec, req)
	if rec.Code != http.StatusPaymentRequired {
		t.Fatalf("POST mutasi: status = %d, want 402", rec.Code)
	}
	if called {
		t.Fatalf("handler seharusnya TIDAK dipanggil saat readonly")
	}

	// GET selalu lolos.
	called = false
	req = httptest.NewRequest(http.MethodGet, "/api/students", nil).WithContext(ctx)
	rec = httptest.NewRecorder()
	svc.Middleware(next).ServeHTTP(rec, req)
	if !called || rec.Code != http.StatusOK {
		t.Fatalf("GET seharusnya lolos: called=%v status=%d", called, rec.Code)
	}

	// POST /api/billing/... tetap lolos (exempt) — supaya sekolah readonly
	// masih bisa membayar.
	called = false
	req = httptest.NewRequest(http.MethodPost, "/api/billing/invoices/1/pay", nil).WithContext(ctx)
	rec = httptest.NewRecorder()
	svc.Middleware(next).ServeHTTP(rec, req)
	if !called || rec.Code != http.StatusOK {
		t.Fatalf("POST /api/billing/... seharusnya lolos: called=%v status=%d", called, rec.Code)
	}

	// POST /api/auth/... tetap lolos.
	called = false
	req = httptest.NewRequest(http.MethodPost, "/api/auth/login", nil).WithContext(ctx)
	rec = httptest.NewRecorder()
	svc.Middleware(next).ServeHTTP(rec, req)
	if !called || rec.Code != http.StatusOK {
		t.Fatalf("POST /api/auth/... seharusnya lolos: called=%v status=%d", called, rec.Code)
	}

	// Host platform TIDAK PERNAH digerbang.
	called = false
	platCtx := reqctx.WithPlatform(context.Background())
	req = httptest.NewRequest(http.MethodPost, "/api/admin/schools", nil).WithContext(platCtx)
	rec = httptest.NewRecorder()
	svc.Middleware(next).ServeHTTP(rec, req)
	if !called || rec.Code != http.StatusOK {
		t.Fatalf("host platform seharusnya tidak pernah digerbang: called=%v status=%d", called, rec.Code)
	}
}

func TestGuard_ActiveSubscriptionAllowsMutation(t *testing.T) {
	repo := newFakeRepo()
	repo.subscriptions[1] = SubscriptionRecord{
		ID: 1, SchoolID: 1, PlanCode: "basic", Features: map[string]bool{},
		StartsOn: fixedNow, EndsOn: fixedNow.AddDate(1, 0, 0), Status: StatusActive,
	}
	svc, _ := newTestService(repo, fixedNow)
	ctx := reqctx.WithSchool(context.Background(), reqctx.School{ID: 1})

	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true; w.WriteHeader(http.StatusOK) })
	req := httptest.NewRequest(http.MethodPost, "/api/students", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	svc.Middleware(next).ServeHTTP(rec, req)
	if !called || rec.Code != http.StatusOK {
		t.Fatalf("subscription active: mutasi seharusnya lolos: called=%v status=%d", called, rec.Code)
	}
}

// -- RequireFeature --

func TestRequireFeature(t *testing.T) {
	repo := newFakeRepo()
	repo.subscriptions[1] = SubscriptionRecord{
		ID: 1, SchoolID: 1, PlanCode: "basic", Features: map[string]bool{FeatureQRCard: true, FeatureTVDashboard: false},
		StartsOn: fixedNow, EndsOn: fixedNow.AddDate(1, 0, 0), Status: StatusActive,
	}
	svc, _ := newTestService(repo, fixedNow)
	ctx := reqctx.WithSchool(context.Background(), reqctx.School{ID: 1})

	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true; w.WriteHeader(http.StatusOK) })

	// Fitur aktif -> lolos.
	req := httptest.NewRequest(http.MethodGet, "/api/attendance/qr-cards", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	svc.RequireFeature(FeatureQRCard)(next).ServeHTTP(rec, req)
	if !called || rec.Code != http.StatusOK {
		t.Fatalf("fitur aktif seharusnya lolos: called=%v status=%d", called, rec.Code)
	}

	// Fitur tidak aktif -> 402.
	called = false
	req = httptest.NewRequest(http.MethodGet, "/api/tv/board", nil).WithContext(ctx)
	rec = httptest.NewRecorder()
	svc.RequireFeature(FeatureTVDashboard)(next).ServeHTTP(rec, req)
	if called {
		t.Fatalf("fitur nonaktif seharusnya TIDAK memanggil handler")
	}
	if rec.Code != http.StatusPaymentRequired {
		t.Fatalf("status = %d, want 402", rec.Code)
	}
}

// -- webhook signature (valid/invalid/idempotent) --

func TestHandleGatewayWebhook_SignatureAndIdempotent(t *testing.T) {
	repo := newFakeRepo()
	repo.activeStudents[1] = 50
	svc, _ := newTestService(repo, fixedNow)
	provider := NewMidtransProvider("dummy-server-key", "sandbox")
	svc.SetProvider(provider)

	inv, err := repo.CreateInvoice(context.Background(), CreateInvoiceInput{
		SchoolID: 1, Number: "INV/2026/0001", Amount: 2_000_000, Status: InvoiceUnpaid,
		DueAt: fixedNow, PlanCode: "basic", MaxStudents: 300,
	})
	if err != nil {
		t.Fatal(err)
	}
	pay, err := repo.CreateGatewayPayment(context.Background(), inv.ID, inv.Amount, "midtrans", "INV-1-123")
	if err != nil {
		t.Fatal(err)
	}
	_ = pay

	orderID, statusCode, grossAmount := "INV-1-123", "200", "2000000"

	// Signature invalid -> ditolak, TIDAK mengaktivasi.
	err = svc.HandleGatewayWebhook(context.Background(), []byte(`{}`), orderID, statusCode, grossAmount, "settlement", "", "signature-salah")
	var domainErr *httpx.Error
	if !errors.As(err, &domainErr) || domainErr.Code != "validation" {
		t.Fatalf("signature invalid: err = %v, want validation error", err)
	}
	if repo.invoices[inv.ID].Status != InvoiceUnpaid {
		t.Fatalf("invoice seharusnya TIDAK berubah saat signature invalid")
	}

	// Signature valid -> aktivasi.
	validSig := validMidtransSignature(orderID, statusCode, grossAmount, "dummy-server-key")
	if err := svc.HandleGatewayWebhook(context.Background(), []byte(`{"raw":true}`), orderID, statusCode, grossAmount, "settlement", "", validSig); err != nil {
		t.Fatalf("webhook valid: %v", err)
	}
	if repo.invoices[inv.ID].Status != InvoicePaid {
		t.Fatalf("invoice seharusnya paid setelah webhook settlement valid")
	}
	endsOnAfterFirst := repo.subscriptions[1].EndsOn

	// Idempotent: webhook kedua (order_id sama, status sama) -> TIDAK
	// mengubah ends_on lagi (ActivateSubscription no-op, invoice sudah paid).
	if err := svc.HandleGatewayWebhook(context.Background(), []byte(`{"raw":true}`), orderID, statusCode, grossAmount, "settlement", "", validSig); err != nil {
		t.Fatalf("webhook kedua (idempotent): %v", err)
	}
	if got := repo.subscriptions[1].EndsOn; !got.Equal(endsOnAfterFirst) {
		t.Fatalf("ends_on berubah pada webhook kedua (harus idempotent): got %v, want %v", got, endsOnAfterFirst)
	}
}

// validMidtransSignature menghitung signature yang SAMA persis dengan
// MidtransProvider.VerifySignature (sha512(order_id+status_code+gross_amount+ServerKey))
// — dipakai test untuk mensimulasikan webhook Midtrans yang sah.
func validMidtransSignature(orderID, statusCode, grossAmount, serverKey string) string {
	sum := sha512.Sum512([]byte(orderID + statusCode + grossAmount + serverKey))
	return hex.EncodeToString(sum[:])
}
