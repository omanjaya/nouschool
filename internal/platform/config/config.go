// Package config memuat konfigurasi aplikasi dari environment variables.
// Semua secret hanya lewat env — tidak pernah hardcode (lihat CLAUDE.md).
package config

import (
	"fmt"
	"os"
)

type Config struct {
	Env         string // "dev" | "prod"
	Port        string
	DatabaseURL string
	BaseDomain  string // domain utama platform, mis. "nouschool.id"

	// ServerIP — IP publik VPS produksi (Fase 11, docs/01-tenant.md "Custom
	// domain & Caddy"). Dipakai DomainService membandingkan hasil resolve DNS
	// domain custom sekolah. Kosong (default dev) = verifikasi custom domain
	// SELALU gagal dengan pesan jelas (lihat internal/tenant/domain.go).
	ServerIP string

	// -- notification (fase 9, docs/08-notification.md) — semua opsional:
	// kosong = channel dianggap tidak dikonfigurasi (lihat
	// internal/notification Provider.Configured).
	VAPIDPublicKey  string
	VAPIDPrivateKey string
	VAPIDSubscriber string // VAPID "sub" claim, mis. "mailto:admin@nouschool.id"
	WAGatewayURL    string
	WAGatewayToken  string
	SMTPHost        string
	SMTPPort        string
	SMTPUsername    string
	SMTPPassword    string
	SMTPFrom        string

	// -- billing (fase 10, docs/09-billing.md) — payment gateway Midtrans,
	// opsional: kosong = "Pembayaran online belum dikonfigurasi." (endpoint
	// pay 422, lihat internal/billing/service.go errGatewayNotConfigured).
	MidtransServerKey string
	MidtransEnv       string // "sandbox" | "production", default "sandbox"
}

func Load() (Config, error) {
	cfg := Config{
		Env:         getenv("APP_ENV", "dev"),
		Port:        getenv("PORT", "8080"),
		DatabaseURL: os.Getenv("DATABASE_URL"),
		BaseDomain:  getenv("BASE_DOMAIN", "localhost"),
		ServerIP:    os.Getenv("SERVER_IP"),

		VAPIDPublicKey:  os.Getenv("VAPID_PUBLIC_KEY"),
		VAPIDPrivateKey: os.Getenv("VAPID_PRIVATE_KEY"),
		VAPIDSubscriber: getenv("VAPID_SUBSCRIBER", "mailto:admin@"+getenv("BASE_DOMAIN", "localhost")),
		WAGatewayURL:    os.Getenv("WA_GATEWAY_URL"),
		WAGatewayToken:  os.Getenv("WA_GATEWAY_TOKEN"),
		SMTPHost:        os.Getenv("SMTP_HOST"),
		SMTPPort:        getenv("SMTP_PORT", "587"),
		SMTPUsername:    os.Getenv("SMTP_USERNAME"),
		SMTPPassword:    os.Getenv("SMTP_PASSWORD"),
		SMTPFrom:        getenv("SMTP_FROM", "no-reply@"+getenv("BASE_DOMAIN", "localhost")),

		MidtransServerKey: os.Getenv("MIDTRANS_SERVER_KEY"),
		MidtransEnv:       getenv("MIDTRANS_ENV", "sandbox"),
	}
	if cfg.Env != "dev" && cfg.DatabaseURL == "" {
		return cfg, fmt.Errorf("config: DATABASE_URL wajib diisi di luar mode dev")
	}
	return cfg, nil
}

func MustLoad() Config {
	cfg, err := Load()
	if err != nil {
		panic(err)
	}
	return cfg
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
