package billing

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/omanjaya/nouschool/internal/billing/billingdb"
)

// ErrNotFound menandai baris tidak ditemukan di repository modul billing.
var ErrNotFound = errors.New("billing: data tidak ditemukan")

// billingRepository adalah kontrak yang dibutuhkan Service dari repository —
// dideklarasikan sebagai interface (dipenuhi *Repository secara struktural)
// supaya Service bisa dites dengan fake repository in-memory, tanpa DB — pola
// yang sama dengan internal/leave.
type billingRepository interface {
	ListPlans(ctx context.Context) ([]PlanRecord, error)
	GetPlan(ctx context.Context, code string) (PlanRecord, error)
	SavePlanName(ctx context.Context, code, name string) error
	SavePlanFeatures(ctx context.Context, code string, features map[string]bool) error
	SavePlanPrices(ctx context.Context, code string, prices []PriceBracket) error

	GetSubscription(ctx context.Context, schoolID int64) (SubscriptionRecord, bool, error)
	UpsertSubscription(ctx context.Context, rec SubscriptionRecord) (SubscriptionRecord, error)
	ExtendSubscription(ctx context.Context, schoolID int64, days int) error
	// TransitionActiveToGrace/TransitionGraceToReadonly mengembalikan
	// school_id sekolah yang BARU transisi (bukan sekadar jumlah baris) —
	// dipakai TickOnce memancarkan event realtime "billing" per sekolah
	// (Fase 12).
	TransitionActiveToGrace(ctx context.Context, today time.Time) ([]int64, error)
	TransitionGraceToReadonly(ctx context.Context, today time.Time) ([]int64, error)
	ListSubscriptionsForAdmin(ctx context.Context) ([]AdminSubscriptionRow, error)

	NextInvoiceNumber(ctx context.Context, year string) (int, error)
	CreateInvoice(ctx context.Context, in CreateInvoiceInput) (InvoiceRecord, error)
	GetInvoice(ctx context.Context, id int64) (InvoiceRecord, error)
	GetInvoiceForSchool(ctx context.Context, id, schoolID int64) (InvoiceRecord, error)
	ListInvoicesForSchool(ctx context.Context, schoolID int64) ([]InvoiceRecord, error)
	SetInvoiceStatus(ctx context.Context, id int64, status string) error
	SetInvoicePaid(ctx context.Context, id int64, paidAt time.Time) error
	SetInvoiceSubscriptionID(ctx context.Context, id, subscriptionID int64) error

	CreateManualPayment(ctx context.Context, invoiceID, amount int64, proofURL string) (PaymentRecord, error)
	CreateGatewayPayment(ctx context.Context, invoiceID, amount int64, provider, providerRef string) (PaymentRecord, error)
	VerifyManualPayment(ctx context.Context, paymentID, verifiedBy int64, verifiedAt time.Time) error
	GetPaymentByProviderRef(ctx context.Context, providerRef string) (PaymentRecord, bool, error)
	SetPaymentRawWebhook(ctx context.Context, paymentID int64, raw []byte) error
	ListPaymentsForInvoice(ctx context.Context, invoiceID int64) ([]PaymentRecord, error)
	LatestPaymentForInvoice(ctx context.Context, invoiceID int64) (PaymentRecord, bool, error)

	CountActiveStudents(ctx context.Context, schoolID int64) (int, error)

	// -- P6 (fase 13 Gelombang 2) --
	UpdateFeatureOverrides(ctx context.Context, schoolID int64, overrides map[string]bool) (SubscriptionRecord, error)
	ListPaidInvoicesInRange(ctx context.Context, from, to time.Time) ([]RevenueInvoiceRow, error)
}

var _ billingRepository = (*Repository)(nil)

// Repository membungkus akses data modul billing (sqlc + pgx).
type Repository struct {
	pool *pgxpool.Pool
	q    *billingdb.Queries
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool, q: billingdb.New(pool)}
}

func mapNoRows(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

func textOrNil(s string) pgtype.Text {
	if s == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: s, Valid: true}
}

func textToStr(v pgtype.Text) string {
	if !v.Valid {
		return ""
	}
	return v.String
}

func int8OrNil(v int64) pgtype.Int8 {
	if v == 0 {
		return pgtype.Int8{}
	}
	return pgtype.Int8{Int64: v, Valid: true}
}

func int8ToInt64(v pgtype.Int8) int64 {
	if !v.Valid {
		return 0
	}
	return v.Int64
}

func dateOf(t time.Time) pgtype.Date {
	if t.IsZero() {
		return pgtype.Date{}
	}
	return pgtype.Date{Time: t, Valid: true}
}

func tsOf(t time.Time) pgtype.Timestamptz {
	if t.IsZero() {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: t, Valid: true}
}

func tsToPtr(v pgtype.Timestamptz) *time.Time {
	if !v.Valid {
		return nil
	}
	t := v.Time
	return &t
}

func marshalFeatures(f map[string]bool) ([]byte, error) {
	if f == nil {
		f = map[string]bool{}
	}
	return json.Marshal(f)
}

func unmarshalFeatures(raw []byte) map[string]bool {
	out := map[string]bool{}
	if len(raw) == 0 {
		return out
	}
	_ = json.Unmarshal(raw, &out)
	return out
}

// -- plans --

func planFromDB(p billingdb.Plan, prices []PriceBracket) PlanRecord {
	return PlanRecord{
		Code: p.Code, Name: p.Name, Features: unmarshalFeatures(p.Features), Active: p.Active, Prices: prices,
	}
}

func (r *Repository) pricesFor(ctx context.Context, planID int64) ([]PriceBracket, error) {
	rows, err := r.q.ListPlanPrices(ctx, planID)
	if err != nil {
		return nil, err
	}
	out := make([]PriceBracket, 0, len(rows))
	for _, row := range rows {
		out = append(out, PriceBracket{MaxStudents: int(row.MaxStudents), PriceYearly: row.PriceYearly})
	}
	return out, nil
}

func (r *Repository) ListPlans(ctx context.Context) ([]PlanRecord, error) {
	rows, err := r.q.ListPlans(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]PlanRecord, 0, len(rows))
	for _, row := range rows {
		prices, err := r.pricesFor(ctx, row.ID)
		if err != nil {
			return nil, err
		}
		out = append(out, planFromDB(row, prices))
	}
	return out, nil
}

func (r *Repository) GetPlan(ctx context.Context, code string) (PlanRecord, error) {
	row, err := r.q.GetPlanByCode(ctx, code)
	if err != nil {
		return PlanRecord{}, mapNoRows(err)
	}
	prices, err := r.pricesFor(ctx, row.ID)
	if err != nil {
		return PlanRecord{}, err
	}
	return planFromDB(row, prices), nil
}

func (r *Repository) SavePlanName(ctx context.Context, code, name string) error {
	return r.q.UpdatePlanName(ctx, billingdb.UpdatePlanNameParams{Code: code, Name: name})
}

func (r *Repository) SavePlanFeatures(ctx context.Context, code string, features map[string]bool) error {
	raw, err := marshalFeatures(features)
	if err != nil {
		return err
	}
	return r.q.UpdatePlanFeatures(ctx, billingdb.UpdatePlanFeaturesParams{Code: code, Features: raw})
}

func (r *Repository) SavePlanPrices(ctx context.Context, code string, prices []PriceBracket) error {
	plan, err := r.q.GetPlanByCode(ctx, code)
	if err != nil {
		return mapNoRows(err)
	}
	keep := make([]int32, 0, len(prices))
	for _, p := range prices {
		if err := r.q.UpsertPlanPrice(ctx, billingdb.UpsertPlanPriceParams{
			PlanID: plan.ID, MaxStudents: int32(p.MaxStudents), PriceYearly: p.PriceYearly,
		}); err != nil {
			return err
		}
		keep = append(keep, int32(p.MaxStudents))
	}
	if len(keep) == 0 {
		return nil
	}
	return r.q.DeletePlanPricesNotIn(ctx, billingdb.DeletePlanPricesNotInParams{PlanID: plan.ID, Keep: keep})
}

// -- subscriptions --

func subFromDB(s billingdb.Subscription) SubscriptionRecord {
	return SubscriptionRecord{
		ID: s.ID, SchoolID: s.SchoolID, PlanCode: s.PlanCode, Features: unmarshalFeatures(s.Features),
		FeatureOverrides: unmarshalFeatures(s.FeatureOverrides),
		MaxStudents:      int(s.MaxStudents), Price: s.Price, StartsOn: s.StartsOn.Time, EndsOn: s.EndsOn.Time, Status: s.Status,
	}
}

func (r *Repository) GetSubscription(ctx context.Context, schoolID int64) (SubscriptionRecord, bool, error) {
	row, err := r.q.GetSubscriptionBySchool(ctx, schoolID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return SubscriptionRecord{}, false, nil
		}
		return SubscriptionRecord{}, false, err
	}
	return subFromDB(row), true, nil
}

func (r *Repository) UpsertSubscription(ctx context.Context, rec SubscriptionRecord) (SubscriptionRecord, error) {
	features, err := marshalFeatures(rec.Features)
	if err != nil {
		return SubscriptionRecord{}, err
	}
	row, err := r.q.UpsertSubscription(ctx, billingdb.UpsertSubscriptionParams{
		SchoolID: rec.SchoolID, PlanCode: rec.PlanCode, Features: features, MaxStudents: int32(rec.MaxStudents),
		Price: rec.Price, StartsOn: dateOf(rec.StartsOn), EndsOn: dateOf(rec.EndsOn), Status: rec.Status,
	})
	if err != nil {
		return SubscriptionRecord{}, err
	}
	return subFromDB(row), nil
}

func (r *Repository) ExtendSubscription(ctx context.Context, schoolID int64, days int) error {
	return r.q.ExtendSubscription(ctx, billingdb.ExtendSubscriptionParams{Days: int32(days), SchoolID: schoolID})
}

func (r *Repository) TransitionActiveToGrace(ctx context.Context, today time.Time) ([]int64, error) {
	return r.q.TransitionActiveToGrace(ctx, dateOf(today))
}

func (r *Repository) TransitionGraceToReadonly(ctx context.Context, today time.Time) ([]int64, error) {
	return r.q.TransitionGraceToReadonly(ctx, billingdb.TransitionGraceToReadonlyParams{
		GraceDays: GracePeriodDays, Today: dateOf(today),
	})
}

func (r *Repository) ListSubscriptionsForAdmin(ctx context.Context) ([]AdminSubscriptionRow, error) {
	rows, err := r.q.ListSubscriptionsForAdmin(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]AdminSubscriptionRow, 0, len(rows))
	for _, row := range rows {
		out = append(out, AdminSubscriptionRow{
			SchoolID: row.SchoolID, SchoolName: row.SchoolName, SchoolSlug: row.SchoolSlug,
			PlanCode: row.PlanCode, Status: row.Status, EndsOn: row.EndsOn.Time.Format("2006-01-02"),
		})
	}
	return out, nil
}

// -- invoices --

func (r *Repository) NextInvoiceNumber(ctx context.Context, year string) (int, error) {
	seq, err := r.q.NextInvoiceSeq(ctx, year)
	if err != nil {
		return 0, err
	}
	return int(seq), nil
}

// CreateInvoiceInput adalah parameter Repository.CreateInvoice.
type CreateInvoiceInput struct {
	SchoolID       int64
	Number         string
	SubscriptionID int64 // 0 = NULL
	Amount         int64
	Status         string
	DueAt          time.Time
	PlanCode       string
	MaxStudents    int
	CreatedBy      int64 // 0 = NULL (sistem)
}

func invoiceFromDB(i billingdb.Invoice) InvoiceRecord {
	return InvoiceRecord{
		ID: i.ID, SchoolID: i.SchoolID, Number: i.Number, SubscriptionID: int8ToInt64(i.SubscriptionID),
		Amount: i.Amount, Status: i.Status, DueAt: i.DueAt.Time, PaidAt: tsToPtr(i.PaidAt),
		PlanCode: i.PlanCode, MaxStudents: int(i.MaxStudents), CreatedBy: int8ToInt64(i.CreatedBy), CreatedAt: i.CreatedAt.Time,
	}
}

func (r *Repository) CreateInvoice(ctx context.Context, in CreateInvoiceInput) (InvoiceRecord, error) {
	row, err := r.q.CreateInvoice(ctx, billingdb.CreateInvoiceParams{
		SchoolID: in.SchoolID, Number: in.Number, SubscriptionID: int8OrNil(in.SubscriptionID),
		Amount: in.Amount, Status: in.Status, DueAt: tsOf(in.DueAt), PlanCode: in.PlanCode,
		MaxStudents: int32(in.MaxStudents), CreatedBy: int8OrNil(in.CreatedBy),
	})
	if err != nil {
		return InvoiceRecord{}, err
	}
	return invoiceFromDB(row), nil
}

func (r *Repository) GetInvoice(ctx context.Context, id int64) (InvoiceRecord, error) {
	row, err := r.q.GetInvoiceByID(ctx, id)
	if err != nil {
		return InvoiceRecord{}, mapNoRows(err)
	}
	return invoiceFromDB(row), nil
}

func (r *Repository) GetInvoiceForSchool(ctx context.Context, id, schoolID int64) (InvoiceRecord, error) {
	row, err := r.q.GetInvoiceByIDForSchool(ctx, billingdb.GetInvoiceByIDForSchoolParams{ID: id, SchoolID: schoolID})
	if err != nil {
		return InvoiceRecord{}, mapNoRows(err)
	}
	return invoiceFromDB(row), nil
}

func (r *Repository) ListInvoicesForSchool(ctx context.Context, schoolID int64) ([]InvoiceRecord, error) {
	rows, err := r.q.ListInvoicesForSchool(ctx, schoolID)
	if err != nil {
		return nil, err
	}
	out := make([]InvoiceRecord, 0, len(rows))
	for _, row := range rows {
		out = append(out, invoiceFromDB(row))
	}
	return out, nil
}

func (r *Repository) SetInvoiceStatus(ctx context.Context, id int64, status string) error {
	return r.q.SetInvoiceStatus(ctx, billingdb.SetInvoiceStatusParams{ID: id, Status: status})
}

func (r *Repository) SetInvoicePaid(ctx context.Context, id int64, paidAt time.Time) error {
	return r.q.SetInvoicePaid(ctx, billingdb.SetInvoicePaidParams{ID: id, PaidAt: tsOf(paidAt)})
}

func (r *Repository) SetInvoiceSubscriptionID(ctx context.Context, id, subscriptionID int64) error {
	return r.q.SetInvoiceSubscriptionID(ctx, billingdb.SetInvoiceSubscriptionIDParams{ID: id, SubscriptionID: int8OrNil(subscriptionID)})
}

// -- payments --

func paymentFromDB(p billingdb.Payment) PaymentRecord {
	return PaymentRecord{
		ID: p.ID, InvoiceID: p.InvoiceID, Method: p.Method, Amount: p.Amount, ProofURL: textToStr(p.ProofUrl),
		VerifiedBy: int8ToInt64(p.VerifiedBy), VerifiedAt: tsToPtr(p.VerifiedAt), Provider: textToStr(p.Provider),
		ProviderRef: textToStr(p.ProviderRef), RawWebhook: p.RawWebhook, CreatedAt: p.CreatedAt.Time,
	}
}

func (r *Repository) CreateManualPayment(ctx context.Context, invoiceID, amount int64, proofURL string) (PaymentRecord, error) {
	row, err := r.q.CreateManualPayment(ctx, billingdb.CreateManualPaymentParams{
		InvoiceID: invoiceID, Amount: amount, ProofUrl: textOrNil(proofURL),
	})
	if err != nil {
		return PaymentRecord{}, err
	}
	return paymentFromDB(row), nil
}

func (r *Repository) CreateGatewayPayment(ctx context.Context, invoiceID, amount int64, provider, providerRef string) (PaymentRecord, error) {
	row, err := r.q.CreateGatewayPayment(ctx, billingdb.CreateGatewayPaymentParams{
		InvoiceID: invoiceID, Amount: amount, Provider: textOrNil(provider), ProviderRef: textOrNil(providerRef),
	})
	if err != nil {
		return PaymentRecord{}, err
	}
	return paymentFromDB(row), nil
}

func (r *Repository) VerifyManualPayment(ctx context.Context, paymentID, verifiedBy int64, verifiedAt time.Time) error {
	return r.q.VerifyManualPayment(ctx, billingdb.VerifyManualPaymentParams{
		ID: paymentID, VerifiedBy: int8OrNil(verifiedBy), VerifiedAt: tsOf(verifiedAt),
	})
}

func (r *Repository) GetPaymentByProviderRef(ctx context.Context, providerRef string) (PaymentRecord, bool, error) {
	row, err := r.q.GetPaymentByProviderRef(ctx, textOrNil(providerRef))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return PaymentRecord{}, false, nil
		}
		return PaymentRecord{}, false, err
	}
	return paymentFromDB(row), true, nil
}

func (r *Repository) SetPaymentRawWebhook(ctx context.Context, paymentID int64, raw []byte) error {
	return r.q.SetPaymentRawWebhook(ctx, billingdb.SetPaymentRawWebhookParams{ID: paymentID, RawWebhook: raw})
}

func (r *Repository) ListPaymentsForInvoice(ctx context.Context, invoiceID int64) ([]PaymentRecord, error) {
	rows, err := r.q.ListPaymentsForInvoice(ctx, invoiceID)
	if err != nil {
		return nil, err
	}
	out := make([]PaymentRecord, 0, len(rows))
	for _, row := range rows {
		out = append(out, paymentFromDB(row))
	}
	return out, nil
}

func (r *Repository) LatestPaymentForInvoice(ctx context.Context, invoiceID int64) (PaymentRecord, bool, error) {
	row, err := r.q.GetLatestPaymentForInvoice(ctx, invoiceID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return PaymentRecord{}, false, nil
		}
		return PaymentRecord{}, false, err
	}
	return paymentFromDB(row), true, nil
}

// -- lookup lintas-modul read-only --

func (r *Repository) CountActiveStudents(ctx context.Context, schoolID int64) (int, error) {
	n, err := r.q.CountActiveStudentsForSchool(ctx, schoolID)
	if err != nil {
		return 0, err
	}
	return int(n), nil
}

// -- P6 (fase 13 Gelombang 2) --

func (r *Repository) UpdateFeatureOverrides(ctx context.Context, schoolID int64, overrides map[string]bool) (SubscriptionRecord, error) {
	raw, err := marshalFeatures(overrides)
	if err != nil {
		return SubscriptionRecord{}, err
	}
	row, err := r.q.UpdateFeatureOverrides(ctx, billingdb.UpdateFeatureOverridesParams{SchoolID: schoolID, FeatureOverrides: raw})
	if err != nil {
		return SubscriptionRecord{}, mapNoRows(err)
	}
	return subFromDB(row), nil
}

func (r *Repository) ListPaidInvoicesInRange(ctx context.Context, from, to time.Time) ([]RevenueInvoiceRow, error) {
	rows, err := r.q.ListPaidInvoicesInRange(ctx, billingdb.ListPaidInvoicesInRangeParams{FromAt: tsOf(from), ToAt: tsOf(to)})
	if err != nil {
		return nil, err
	}
	out := make([]RevenueInvoiceRow, 0, len(rows))
	for _, row := range rows {
		var paidAt time.Time
		if row.PaidAt.Valid {
			paidAt = row.PaidAt.Time
		}
		out = append(out, RevenueInvoiceRow{Number: row.Number, SchoolName: row.SchoolName, PlanCode: row.PlanCode, Amount: row.Amount, PaidAt: paidAt})
	}
	return out, nil
}
