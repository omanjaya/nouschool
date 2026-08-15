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
}

func Load() (Config, error) {
	cfg := Config{
		Env:         getenv("APP_ENV", "dev"),
		Port:        getenv("PORT", "8080"),
		DatabaseURL: os.Getenv("DATABASE_URL"),
		BaseDomain:  getenv("BASE_DOMAIN", "localhost"),
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
