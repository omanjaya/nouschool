-- +goose Up
-- Modul notification (notifikasi pluggable: in-app, web push, WhatsApp,
-- email). Lihat docs/08-notification.md — arsitektur outbox + channel
-- provider. Tabel di bawah PERSIS mengikuti skema di dokumen itu.

-- notifications — inbox in-app, SELALU ditulis apapun channel lain yang aktif
-- (baseline, docs/08-notification.md "Channel provider").
CREATE TABLE notifications (
    id         bigserial PRIMARY KEY,
    school_id  bigint NOT NULL REFERENCES schools (id),
    user_id    bigint NOT NULL REFERENCES users (id),
    event      text NOT NULL,
    title      text NOT NULL,
    body       text NOT NULL,
    link       text,
    read_at    timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX notifications_user ON notifications (school_id, user_id, created_at DESC);
CREATE INDEX notifications_user_unread ON notifications (school_id, user_id) WHERE read_at IS NULL;

-- notification_outbox — satu baris per (penerima x channel non-in_app aktif).
-- Worker goroutine memprosesnya secara berkala (lihat internal/notification/worker.go).
CREATE TABLE notification_outbox (
    id             bigserial PRIMARY KEY,
    school_id      bigint NOT NULL REFERENCES schools (id),
    event          text NOT NULL,
    user_id        bigint NOT NULL REFERENCES users (id),
    channel        text NOT NULL,           -- web_push|whatsapp|email (in_app tidak lewat outbox)
    payload        jsonb NOT NULL,          -- title/body/link ter-render + data mentah
    status         text NOT NULL DEFAULT 'pending', -- pending|sent|failed|dead
    attempts       int NOT NULL DEFAULT 0,
    next_retry_at  timestamptz,
    created_at     timestamptz NOT NULL DEFAULT now(),
    sent_at        timestamptz
);
CREATE INDEX notification_outbox_due ON notification_outbox (status, next_retry_at);
CREATE INDEX notification_outbox_school ON notification_outbox (school_id);

-- push_subscriptions — Web Push per device (docs/08-notification.md "web_push").
CREATE TABLE push_subscriptions (
    id         bigserial PRIMARY KEY,
    school_id  bigint NOT NULL REFERENCES schools (id),
    user_id    bigint NOT NULL REFERENCES users (id),
    endpoint   text UNIQUE NOT NULL,
    p256dh     text NOT NULL,
    auth       text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX push_subscriptions_user ON push_subscriptions (school_id, user_id);

-- platform_config — konfigurasi kecil level platform yang perlu STABIL antar
-- restart (mis. pasangan kunci VAPID web push di-generate sekali saat
-- startup pertama, lalu dipakai ulang — lihat internal/notification/webpush.go).
-- BUKAN bagian skema docs/08 (tambahan kecil sesuai instruksi tugas fase 9).
CREATE TABLE platform_config (
    key   text PRIMARY KEY,
    value text NOT NULL
);

-- +goose Down
DROP TABLE platform_config;
DROP TABLE push_subscriptions;
DROP TABLE notification_outbox;
DROP TABLE notifications;
