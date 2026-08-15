# 09 — Billing: Langganan Tahunan, Tier × Bracket Siswa, Pembayaran

Modul: `internal/billing/`. Model: langganan **tahunan** per sekolah; harga = **tier fitur (Basic/Pro) × bracket jumlah siswa**; bayar via **transfer manual (verifikasi super admin)** ATAU **payment gateway (Midtrans/Xendit)** — satu model invoice, dua channel pembayaran.

## Skema

```sql
plans (
  id        bigserial PRIMARY KEY,
  code      text UNIQUE NOT NULL,     -- 'basic', 'pro'
  name      text NOT NULL,
  features  jsonb NOT NULL,           -- {"tv_dashboard": true, "whatsapp": true, "dapodik_import": true, ...}
  active    boolean NOT NULL DEFAULT true
)

plan_prices (                          -- bracket jumlah siswa per plan
  id           bigserial PRIMARY KEY,
  plan_id      bigint NOT NULL REFERENCES plans,
  max_students int NOT NULL,           -- bracket: sampai 300, 600, 99999 (unlimited)
  price_yearly bigint NOT NULL,        -- rupiah
  UNIQUE (plan_id, max_students)
)

subscriptions (
  id           bigserial PRIMARY KEY,
  school_id    bigint NOT NULL,
  plan_code    text NOT NULL,          -- SNAPSHOT kode plan
  features     jsonb NOT NULL,         -- SNAPSHOT fitur saat subscribe (perubahan plan tidak mengubah langganan berjalan)
  max_students int NOT NULL,           -- SNAPSHOT bracket
  price        bigint NOT NULL,
  starts_on    date NOT NULL,
  ends_on      date NOT NULL,
  status       text NOT NULL           -- active|grace|readonly|canceled
)

invoices (
  id          bigserial PRIMARY KEY,
  school_id   bigint NOT NULL,
  number      text UNIQUE NOT NULL,    -- INV/2026/0001 — sekolah butuh utk SPJ BOS
  subscription_id bigint,
  amount      bigint NOT NULL,
  status      text NOT NULL DEFAULT 'unpaid',  -- unpaid|awaiting_verification|paid|void|expired
  due_at      timestamptz NOT NULL,
  paid_at     timestamptz
)

payments (
  id          bigserial PRIMARY KEY,
  invoice_id  bigint NOT NULL REFERENCES invoices,
  method      text NOT NULL,           -- 'manual_transfer' | 'gateway'
  amount      bigint NOT NULL,
  -- manual_transfer:
  proof_url   text,                    -- upload bukti transfer
  verified_by bigint, verified_at timestamptz,
  -- gateway:
  provider    text,                    -- 'midtrans' | 'xendit'
  provider_ref text,                   -- order/external id
  raw_webhook jsonb,
  created_at  timestamptz NOT NULL DEFAULT now()
)
```

## Lifecycle langganan

```
active ──(ends_on lewat)──> grace (14 hari, banner peringatan)
grace ──(14 hari lewat)──> readonly  (lihat & export OK; semua mutasi non-billing ditolak)
readonly/grace ──(invoice paid)──> active (periode baru)
```

- Transisi dijalankan job harian (`platform` cron internal, cukup goroutine ticker — tanpa dependensi eksternal).
- Status dibaca middleware tenant (lihat 01) tiap request (cached).
- Data TIDAK pernah dihapus karena nunggak; export selalu tersedia. (Kebijakan retensi sekolah churn: putuskan nanti, jangan hardcode penghapusan.)

## Feature gating

- `requireFeature("tv_dashboard")` middleware/helper — baca `subscriptions.features` (snapshot) dari context.
- Frontend menerima daftar fitur aktif di payload `/api/me` → sembunyikan menu yang tidak aktif (UX), server tetap menegakkan (security).
- Penambahan fitur baru ke kode = tambah kunci feature + default per plan di seeding `plans`.

## Bracket siswa

- Saat subscribe/renew: hitung siswa `active` → tentukan bracket → harga.
- Selama periode berjalan sekolah melebihi bracket: JANGAN blokir input siswa (operasional sekolah > penagihan). Tampilkan peringatan di panel admin + tagihan upgrade prorata sebagai invoice terpisah (boleh manual oleh super admin di fase awal).

## Dua channel pembayaran, satu alur

1. Renewal/subscribe → buat `invoice` (unpaid) → notifikasi.
2. **Gateway**: buat transaksi Midtrans/Xendit → redirect/checkout → webhook masuk (`/api/webhooks/{provider}`, verifikasi signature!, idempotent by provider_ref) → payment tercatat → invoice paid → aktivasi otomatis.
3. **Transfer manual**: sekolah upload bukti → invoice `awaiting_verification` → super admin verifikasi di panel → paid → aktivasi. Kuitansi/invoice PDF bisa diunduh (kebutuhan SPJ BOS).
4. Aktivasi = satu fungsi yang sama untuk kedua jalur (`ActivateSubscription(invoiceID)`) — idempotent.

Pilih SATU provider gateway dulu saat implementasi (rekomendasi: yang feenya paling cocok saat itu; abstraksi `PaymentProvider` interface membuat penambahan provider kedua murah).

## Panel super admin

- Kelola plans & harga, buat/void invoice, verifikasi transfer, lihat status semua sekolah (aktif/grace/readonly), perpanjang manual (goodwill).
- Semua aksi billing → audit_log.
