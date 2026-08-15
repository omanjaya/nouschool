# 08 — Notification: In-app, Web Push, WhatsApp, Email (Pluggable)

Modul: `internal/notification/`. Keputusan: semua channel dibangun; **channel mana yang aktif diatur per sekolah oleh super admin** (bukan admin sekolah — karena WhatsApp/email memakai kredensial & biaya platform).

## Arsitektur: outbox pattern + channel provider

Modul lain TIDAK memanggil WhatsApp/email langsung. Mereka memanggil satu API:

```go
notifier.Send(ctx, Notification{
    SchoolID: sid,
    Event:    "attendance.student_absent",   // kunci template
    UserIDs:  []int64{guardianUserID},       // penerima (resolusi kontak di modul ini)
    Data:     map[string]any{"student": "Budi", "date": "2026-08-16", "status": "alpa"},
})
```

Alur: `Send` menulis baris ke `notification_outbox` (dalam transaksi yang sama dengan aksi bisnisnya bila perlu) → worker goroutine membaca outbox → untuk tiap penerima, tentukan channel aktif (settings sekolah × preferensi event × kontak tersedia) → render template → kirim via provider → tandai sent/failed (retry dengan backoff, max N, lalu `dead`).

```sql
notification_outbox (
  id         bigserial PRIMARY KEY,
  school_id  bigint NOT NULL,
  event      text NOT NULL,
  user_id    bigint NOT NULL,          -- penerima
  channel    text NOT NULL,            -- in_app|web_push|whatsapp|email
  payload    jsonb NOT NULL,           -- data ter-render / raw data
  status     text NOT NULL DEFAULT 'pending',  -- pending|sent|failed|dead
  attempts   int NOT NULL DEFAULT 0,
  next_retry_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  sent_at    timestamptz
)

notifications (        -- in-app inbox (selalu ditulis, apapun channel lain)
  id BIGSERIAL PRIMARY KEY, school_id bigint, user_id bigint,
  event text, title text, body text, link text,
  read_at timestamptz, created_at timestamptz DEFAULT now()
)

push_subscriptions (   -- Web Push per device
  id bigserial PRIMARY KEY, school_id bigint, user_id bigint,
  endpoint text UNIQUE, p256dh text, auth text, created_at timestamptz
)
```

## Channel provider (interface tunggal)

```go
type Provider interface {
    Send(ctx context.Context, msg RenderedMessage) error  // idempotent-safe
}
```

- **in_app**: tulis ke `notifications` (ini juga baseline — SELALU aktif).
- **web_push**: VAPID, lib `webpush-go`. Frontend daftar subscription via service worker. Catatan iOS: hanya jalan bila PWA di-install ke home screen — tampilkan hint.
- **whatsapp**: interface `WAGateway` dengan 2 implementasi rencana: Fonnte-style HTTP gateway (murah, cepat mulai) dan WA Business Cloud API (resmi). Konfigurasi (API key, sender) per platform ATAU per sekolah — disimpan super admin.
- **email**: SMTP (config platform), template HTML sederhana. Dipakai juga untuk reset password (jalur langsung, bukan outbox).

## Konfigurasi

- **Per sekolah (oleh super admin)**, module `notification` di school_settings: channel enabled (`{"channels": ["in_app","web_push","whatsapp"]}`), sender WhatsApp yang dipakai, kuota/bulan bila perlu.
- **Per event**: map default di kode — event mana memakai channel apa (mis. `attendance.student_absent` → WA+push ke ortu; `leave.decided` → push+in_app ke guru). Override per sekolah menyusul bila dibutuhkan.
- **Per user** (fase nanti): opt-out per channel. Jangan bangun sekarang, cukup desain kolomnya tidak menghalangi.

## Event awal

| Event | Penerima | Channel default |
|---|---|---|
| `attendance.student_absent` (alpa/sakit/izin tercatat) | ortu | wa, push, in_app |
| `attendance.self_checkin_ok` | ortu | push, in_app |
| `leave.submitted` | approver step aktif | push, in_app |
| `leave.decided` | guru pengaju | push, in_app, email |
| `invitation.created` | (via kode cetak — bukan notifikasi) | — |
| `billing.invoice_due` / `billing.grace` | admin sekolah, kepsek | email, wa, in_app |

Aturan: modul lain menambah event = daftarkan konstanta event + template + baris di tabel ini. Kirim WA massal (blast promosi) BUKAN scope modul ini.
