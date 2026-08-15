-- +goose Up
-- Modul billing — Fase 10: langganan tahunan, tier (Basic/Pro) x bracket
-- jumlah siswa, invoice, transfer manual + payment gateway (docs/09-billing.md).
-- Skema PERSIS mengikuti dokumen itu, dengan tambahan kecil di luar dokumen
-- (dicatat di komentar masing-masing) yang dibutuhkan implementasi.

-- plans — tier fitur (Basic/Pro). features adalah PETA MASTER (bukan snapshot)
-- — perubahan di sini TIDAK mengubah langganan berjalan (lihat subscriptions.features).
CREATE TABLE plans (
    id       bigserial PRIMARY KEY,
    code     text UNIQUE NOT NULL,
    name     text NOT NULL,
    features jsonb NOT NULL,
    active   boolean NOT NULL DEFAULT true
);

-- plan_prices — bracket jumlah siswa per plan (sampai 300 / 600 / 99999=unlimited).
CREATE TABLE plan_prices (
    id           bigserial PRIMARY KEY,
    plan_id      bigint NOT NULL REFERENCES plans (id),
    max_students int NOT NULL,
    price_yearly bigint NOT NULL,
    UNIQUE (plan_id, max_students)
);

-- subscriptions — SATU baris = status langganan TERKINI satu sekolah (bukan
-- riwayat; riwayat pembayaran ada di invoices/payments). Renewal MENGUBAH
-- baris yang sama (extend ends_on), bukan membuat baris baru — sesuai
-- docs/09-billing.md "ActivateSubscription ... periode baru = max(ends_on
-- lama, today) + 1 tahun". UNIQUE(school_id) menegakkan "satu baris per
-- sekolah" ini (keputusan implementasi, tidak eksplisit dilarang/diwajibkan
-- dokumen — lihat catatan laporan akhir tugas).
CREATE TABLE subscriptions (
    id           bigserial PRIMARY KEY,
    school_id    bigint NOT NULL REFERENCES schools (id),
    plan_code    text NOT NULL,
    features     jsonb NOT NULL,
    max_students int NOT NULL,
    price        bigint NOT NULL,
    starts_on    date NOT NULL,
    ends_on      date NOT NULL,
    status       text NOT NULL, -- active|grace|readonly|canceled
    UNIQUE (school_id)
);
CREATE INDEX subscriptions_status ON subscriptions (status);

-- invoice_number_counters — counter atomik utk nomor invoice global per
-- tahun (format INV/YYYY/NNNN, docs/09-billing.md "sekolah butuh utk SPJ
-- BOS"). BUKAN bagian skema docs/09 (tambahan kecil, sama pola dengan
-- platform_config di migrations/00010_notification.sql).
CREATE TABLE invoice_number_counters (
    year text PRIMARY KEY,
    seq  int NOT NULL
);

-- invoices — satu invoice per periode/tagihan (renewal, upgrade prorata, dst).
CREATE TABLE invoices (
    id              bigserial PRIMARY KEY,
    school_id       bigint NOT NULL REFERENCES schools (id),
    number          text UNIQUE NOT NULL,
    subscription_id bigint REFERENCES subscriptions (id),
    amount          bigint NOT NULL,
    status          text NOT NULL DEFAULT 'unpaid', -- unpaid|awaiting_verification|paid|void|expired
    due_at          timestamptz NOT NULL,
    paid_at         timestamptz,
    -- plan_code/max_students disimpan di invoice juga (bukan hanya lewat
    -- subscription_id, yang bisa NULL utk invoice pertama sebelum subscription
    -- pernah ada) supaya PDF invoice & rincian bracket selalu bisa dibangun
    -- tanpa bergantung subscription sudah aktif.
    plan_code       text NOT NULL,
    max_students    int NOT NULL,
    created_by      bigint,
    created_at      timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX invoices_school ON invoices (school_id, created_at DESC);
CREATE INDEX invoices_status ON invoices (status);

-- payments — dua channel (manual_transfer | gateway), satu model invoice.
CREATE TABLE payments (
    id           bigserial PRIMARY KEY,
    invoice_id   bigint NOT NULL REFERENCES invoices (id),
    method       text NOT NULL, -- manual_transfer | gateway
    amount       bigint NOT NULL,
    -- manual_transfer:
    proof_url    text,
    verified_by  bigint,
    verified_at  timestamptz,
    -- gateway:
    provider     text,          -- midtrans
    provider_ref text,          -- order_id Midtrans — idempotency key webhook
    raw_webhook  jsonb,
    created_at   timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX payments_invoice ON payments (invoice_id);
CREATE UNIQUE INDEX payments_provider_ref_unique ON payments (provider_ref) WHERE provider_ref IS NOT NULL;

-- Seeding plans + bracket harga (docs/09-billing.md scope Fase 10: nilai
-- placeholder wajar, mudah diubah super admin lewat PUT /api/admin/plans/{code}).
INSERT INTO plans (code, name, features, active) VALUES
    ('basic', 'Basic', '{"tv_dashboard":false,"whatsapp":false,"dapodik_import":false,"qr_card":true,"self_checkin":true}'::jsonb, true),
    ('pro',   'Pro',   '{"tv_dashboard":true,"whatsapp":true,"dapodik_import":true,"qr_card":true,"self_checkin":true}'::jsonb, true);

INSERT INTO plan_prices (plan_id, max_students, price_yearly)
SELECT id, b.max_students, b.price_yearly
FROM plans, (VALUES (300, 2000000::bigint), (600, 3500000::bigint), (99999, 5000000::bigint)) AS b(max_students, price_yearly)
WHERE plans.code = 'basic';

INSERT INTO plan_prices (plan_id, max_students, price_yearly)
SELECT id, b.max_students, b.price_yearly
FROM plans, (VALUES (300, 4000000::bigint), (600, 7000000::bigint), (99999, 10000000::bigint)) AS b(max_students, price_yearly)
WHERE plans.code = 'pro';

-- +goose Down
DROP TABLE payments;
DROP TABLE invoices;
DROP TABLE invoice_number_counters;
DROP TABLE subscriptions;
DROP TABLE plan_prices;
DROP TABLE plans;
