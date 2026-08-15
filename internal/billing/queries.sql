-- Query modul billing (sqlc). Semua query tenant-scoped WAJIB filter school_id
-- (kecuali query lintas-sekolah utk panel super admin, ditandai eksplisit).

-- name: ListPlans :many
SELECT * FROM plans ORDER BY id;

-- name: GetPlanByCode :one
SELECT * FROM plans WHERE code = $1;

-- name: UpdatePlanName :exec
UPDATE plans SET name = $2 WHERE code = $1;

-- name: UpdatePlanFeatures :exec
UPDATE plans SET features = $2 WHERE code = $1;

-- name: ListPlanPrices :many
SELECT * FROM plan_prices WHERE plan_id = $1 ORDER BY max_students;

-- name: UpsertPlanPrice :exec
INSERT INTO plan_prices (plan_id, max_students, price_yearly)
VALUES ($1, $2, $3)
ON CONFLICT (plan_id, max_students) DO UPDATE SET price_yearly = EXCLUDED.price_yearly;

-- name: DeletePlanPricesNotIn :exec
DELETE FROM plan_prices WHERE plan_id = $1 AND max_students != ALL(sqlc.arg(keep)::int[]);

-- -- subscriptions --

-- name: GetSubscriptionBySchool :one
SELECT * FROM subscriptions WHERE school_id = $1;

-- name: UpsertSubscription :one
INSERT INTO subscriptions (school_id, plan_code, features, max_students, price, starts_on, ends_on, status)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT (school_id) DO UPDATE SET
    plan_code = EXCLUDED.plan_code,
    features = EXCLUDED.features,
    max_students = EXCLUDED.max_students,
    price = EXCLUDED.price,
    starts_on = EXCLUDED.starts_on,
    ends_on = EXCLUDED.ends_on,
    status = EXCLUDED.status
RETURNING *;

-- name: ExtendSubscription :exec
-- Goodwill (super admin) — memperpanjang ends_on tanpa mengubah plan/harga.
UPDATE subscriptions SET ends_on = ends_on + make_interval(days => sqlc.arg(days)::int), status = 'active'
WHERE school_id = sqlc.arg(school_id)::bigint;

-- name: TransitionActiveToGrace :many
-- RETURNING school_id (bukan :execrows) — Fase 12 (realtime): TickOnce perlu
-- tahu SEKOLAH MANA yang baru transisi supaya event "billing" bisa
-- ditargetkan per sekolah (Hub per sekolah, lihat internal/realtime), bukan
-- disiarkan ke semua sekolah sekaligus.
UPDATE subscriptions SET status = 'grace' WHERE status = 'active' AND ends_on < sqlc.arg(today)::date
RETURNING school_id;

-- name: TransitionGraceToReadonly :many
-- RETURNING school_id — lihat catatan TransitionActiveToGrace di atas.
UPDATE subscriptions SET status = 'readonly' WHERE status = 'grace' AND (ends_on + make_interval(days => sqlc.arg(grace_days)::int)) < sqlc.arg(today)::date
RETURNING school_id;

-- name: ListSubscriptionsForAdmin :many
-- Lintas-sekolah (panel super admin): daftar status semua sekolah.
SELECT s.*, sc.name AS school_name, sc.slug AS school_slug
FROM subscriptions s
JOIN schools sc ON sc.id = s.school_id
ORDER BY sc.name;

-- -- invoices --

-- name: NextInvoiceSeq :one
INSERT INTO invoice_number_counters (year, seq) VALUES ($1, 1)
ON CONFLICT (year) DO UPDATE SET seq = invoice_number_counters.seq + 1
RETURNING seq;

-- name: CreateInvoice :one
INSERT INTO invoices (school_id, number, subscription_id, amount, status, due_at, plan_code, max_students, created_by)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: GetInvoiceByID :one
SELECT * FROM invoices WHERE id = $1;

-- name: GetInvoiceByIDForSchool :one
SELECT * FROM invoices WHERE id = $1 AND school_id = $2;

-- name: ListInvoicesForSchool :many
SELECT * FROM invoices WHERE school_id = $1 ORDER BY created_at DESC;

-- name: SetInvoiceStatus :exec
UPDATE invoices SET status = $2 WHERE id = $1;

-- name: SetInvoicePaid :exec
UPDATE invoices SET status = 'paid', paid_at = $2 WHERE id = $1;

-- name: SetInvoiceSubscriptionID :exec
UPDATE invoices SET subscription_id = $2 WHERE id = $1;

-- -- payments --

-- name: CreateManualPayment :one
INSERT INTO payments (invoice_id, method, amount, proof_url)
VALUES ($1, 'manual_transfer', $2, $3)
RETURNING *;

-- name: CreateGatewayPayment :one
INSERT INTO payments (invoice_id, method, amount, provider, provider_ref)
VALUES ($1, 'gateway', $2, $3, $4)
RETURNING *;

-- name: VerifyManualPayment :exec
UPDATE payments SET verified_by = $2, verified_at = $3 WHERE id = $1;

-- name: GetPaymentByProviderRef :one
SELECT * FROM payments WHERE provider_ref = $1;

-- name: SetPaymentRawWebhook :exec
UPDATE payments SET raw_webhook = $2 WHERE id = $1;

-- name: ListPaymentsForInvoice :many
SELECT * FROM payments WHERE invoice_id = $1 ORDER BY created_at DESC;

-- name: GetLatestPaymentForInvoice :one
SELECT * FROM payments WHERE invoice_id = $1 ORDER BY created_at DESC LIMIT 1;

-- -- lookup lintas-modul read-only --

-- name: CountActiveStudentsForSchool :one
SELECT COUNT(*) FROM students WHERE school_id = $1 AND status = 'active';
