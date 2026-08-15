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
}

func Load() (Config, error) {
	cfg := Config{
		Env:         getenv("APP_ENV", "dev"),
		Port:        getenv("PORT", "8080"),
		DatabaseURL: os.Getenv("DATABASE_URL"),
		BaseDomain:  getenv("BASE_DOMAIN", "localhost"),

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
