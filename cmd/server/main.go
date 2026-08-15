// main merakit seluruh aplikasi. HANYA wiring — tanpa logika bisnis.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/omanjaya/nouschool/internal/platform/config"
	"github.com/omanjaya/nouschool/internal/platform/database"
	"github.com/omanjaya/nouschool/internal/platform/httpx"
	"github.com/omanjaya/nouschool/internal/platform/middleware"
)

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, nil)))
	cfg := config.MustLoad()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	// DB opsional saat dev supaya server bisa hidup sebelum Postgres siap.
	if cfg.DatabaseURL != "" {
		pool, err := database.Connect(ctx, cfg.DatabaseURL)
		if err != nil {
			slog.Error("gagal konek database", "err", err)
			os.Exit(1)
		}
		defer pool.Close()
		slog.Info("database terhubung")
		_ = pool // fase 1: di-inject ke repository tiap modul
	} else {
		slog.Warn("DATABASE_URL kosong — jalan tanpa database (mode dev)")
	}

	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		httpx.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	// Dipanggil Caddy (On-Demand TLS) untuk memvalidasi custom domain.
	// Fase 1: cek ke tabel schools; sekarang tolak semua kecuali base domain.
	mux.HandleFunc("GET /internal/check-domain", func(w http.ResponseWriter, r *http.Request) {
		domain := r.URL.Query().Get("domain")
		if domain == cfg.BaseDomain {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	handler := middleware.Chain(mux,
		middleware.Recover,
		middleware.Logger,
		middleware.SecurityHeaders,
	)

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		slog.Info("server jalan", "port", cfg.Port, "env", cfg.Env)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server berhenti", "err", err)
			stop()
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
	slog.Info("server dimatikan dengan rapi")
}
