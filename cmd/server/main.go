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

	"github.com/omanjaya/nouschool/internal/attendance"
	"github.com/omanjaya/nouschool/internal/identity"
	"github.com/omanjaya/nouschool/internal/leave"
	"github.com/omanjaya/nouschool/internal/platform/clock"
	"github.com/omanjaya/nouschool/internal/platform/config"
	"github.com/omanjaya/nouschool/internal/platform/database"
	"github.com/omanjaya/nouschool/internal/platform/httpx"
	"github.com/omanjaya/nouschool/internal/platform/middleware"
	"github.com/omanjaya/nouschool/internal/platform/storage"
	"github.com/omanjaya/nouschool/internal/student"
	"github.com/omanjaya/nouschool/internal/tenant"
)

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, nil)))
	cfg := config.MustLoad()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		httpx.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	var tenantResolverMW middleware.Middleware = func(next http.Handler) http.Handler { return next }

	// DB opsional saat dev supaya server bisa hidup sebelum Postgres siap.
	if cfg.DatabaseURL != "" {
		pool, err := database.Connect(ctx, cfg.DatabaseURL)
		if err != nil {
			slog.Error("gagal konek database", "err", err)
			os.Exit(1)
		}
		defer pool.Close()
		slog.Info("database terhubung")

		cookieSecure := cfg.Env == "prod"

		// --- modul identity (auth, RBAC, session, audit) ---
		identityRepo := identity.NewRepository(pool)
		loginRateLimiter := identity.NewRateLimiter(5, 15*time.Minute, nil)
		identitySvc := identity.NewService(identityRepo, loginRateLimiter, cookieSecure)
		identityHandler := identity.NewHandler(identitySvc)

		// --- modul tenant (sekolah, domain, tahun ajaran, settings) ---
		tenantRepo := tenant.NewRepository(pool)
		// identitySvc mengimplementasikan tenant.AuditLogger secara struktural
		// (method Log) — tenant TIDAK mengimpor identity untuk tipe apa pun.
		tenantSvc := tenant.NewService(tenantRepo, identitySvc)
		settingsSvc := tenant.NewSettingsService(tenantRepo, identitySvc)
		hostResolver := tenant.NewHostResolver(tenantRepo, cfg.BaseDomain)
		tenantHandler := tenant.NewHandler(tenantSvc, settingsSvc, hostResolver)

		// --- modul student (siswa, rombel, enrollment, wali, guru, mapel, import, undangan) ---
		// identitySvc & tenantSvc memenuhi student.IdentityGateway /
		// student.AcademicYearLookup secara STRUKTURAL (consumer-side interface
		// dideklarasikan di internal/student — lihat CLAUDE.md) — student TIDAK
		// mengimpor identity maupun tenant untuk tipe apa pun.
		studentRepo := student.NewRepository(pool)
		studentSvc := student.NewService(studentRepo, identitySvc, tenantSvc)
		studentHandler := student.NewHandler(studentSvc)

		// --- modul attendance (absensi siswa mode daily) ---
		// identitySvc, tenantSvc, studentSvc memenuhi attendance.IdentityGateway /
		// attendance.AcademicYearLookup / attendance.StudentAccess secara
		// STRUKTURAL (consumer-side interface dideklarasikan di
		// internal/attendance — lihat CLAUDE.md) — attendance TIDAK mengimpor
		// identity/tenant/student untuk tipe apa pun.
		attendanceRepo := attendance.NewRepository(pool)
		attendanceSvc := attendance.NewService(attendanceRepo, identitySvc, tenantSvc, studentSvc, clock.System{})
		attendanceHandler := attendance.NewHandler(attendanceSvc)

		// --- modul leave (izin guru dengan approval engine konfigurable) ---
		// identitySvc memenuhi leave.IdentityGateway secara STRUKTURAL
		// (consumer-side interface dideklarasikan di internal/leave — lihat
		// CLAUDE.md) — leave TIDAK mengimpor identity untuk tipe apa pun.
		// platform/storage diimpor LANGSUNG (infrastruktur bersama, seperti clock).
		leaveRepo := leave.NewRepository(pool)
		leaveSvc := leave.NewService(leaveRepo, identitySvc, storage.FromEnv(), clock.System{})
		leaveHandler := leave.NewHandler(leaveSvc)

		// --- wiring routes ---
		identity.RegisterRoutes(mux, identityHandler, identitySvc.RequireAuth)
		tenant.RegisterRoutes(mux, tenantHandler, identitySvc.RequireAuth, identitySvc.RequireSuperAdmin, identitySvc.RequirePerm)
		student.RegisterRoutes(mux, studentHandler, identitySvc.RequireAuth, identitySvc.RequirePerm)
		attendance.RegisterRoutes(mux, attendanceHandler, identitySvc.RequireAuth, identitySvc.RequirePerm)
		leave.RegisterRoutes(mux, leaveHandler, identitySvc.RequireAuth, identitySvc.RequirePerm)

		tenantResolverMW = hostResolver.Middleware
	} else {
		slog.Warn("DATABASE_URL kosong — jalan tanpa database (mode dev, hanya /api/health aktif)")
	}

	// Urutan wajib: Recover -> Logger -> SecurityHeaders -> ResolveTenant -> routing
	// (per-route RequireAuth/RequirePerm dipasang saat registrasi route di atas).
	handler := middleware.Chain(mux,
		middleware.Recover,
		middleware.Logger,
		middleware.SecurityHeaders,
		tenantResolverMW,
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
