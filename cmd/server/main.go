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

	"github.com/omanjaya/nouschool/internal/announcement"
	"github.com/omanjaya/nouschool/internal/attendance"
	"github.com/omanjaya/nouschool/internal/billing"
	"github.com/omanjaya/nouschool/internal/dashboard"
	"github.com/omanjaya/nouschool/internal/identity"
	"github.com/omanjaya/nouschool/internal/leave"
	"github.com/omanjaya/nouschool/internal/notification"
	"github.com/omanjaya/nouschool/internal/platform/clock"
	"github.com/omanjaya/nouschool/internal/platform/config"
	"github.com/omanjaya/nouschool/internal/platform/database"
	"github.com/omanjaya/nouschool/internal/platform/httpx"
	"github.com/omanjaya/nouschool/internal/platform/middleware"
	"github.com/omanjaya/nouschool/internal/platform/storage"
	"github.com/omanjaya/nouschool/internal/realtime"
	"github.com/omanjaya/nouschool/internal/schedule"
	"github.com/omanjaya/nouschool/internal/student"
	"github.com/omanjaya/nouschool/internal/teaching"
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
	var billingGuardMW middleware.Middleware = func(next http.Handler) http.Handler { return next }

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

		// --- modul tenant (sekolah, domain, tahun ajaran, settings, branding,
		// custom domain, registrasi minat — Fase 1 & Fase 11) ---
		tenantRepo := tenant.NewRepository(pool)
		// identitySvc mengimplementasikan tenant.AuditLogger secara struktural
		// (method Log) — tenant TIDAK mengimpor identity untuk tipe apa pun.
		tenantSvc := tenant.NewService(tenantRepo, identitySvc)
		settingsSvc := tenant.NewSettingsService(tenantRepo, identitySvc)
		hostResolver := tenant.NewHostResolver(tenantRepo, cfg.BaseDomain)
		// domainSvc (Fase 11): custom domain end-to-end — dipakai bersama
		// hostResolver (invalidate cache setelah verify/hapus, lihat
		// internal/tenant/domain.go) & cfg.ServerIP (kosong di dev = verifikasi
		// selalu gagal dengan pesan jelas, sesuai scope tugas).
		domainSvc := tenant.NewDomainService(tenantRepo, hostResolver, identitySvc, cfg.ServerIP, cfg.BaseDomain)
		// interestSvc (Fase 11): registrasi minat sekolah dari landing page
		// (host platform, publik, rate-limit 3/jam per IP).
		interestSvc := tenant.NewInterestService(tenantRepo, clock.System{})
		tenantHandler := tenant.NewHandler(tenantSvc, settingsSvc, hostResolver, domainSvc, interestSvc, storage.FromEnv())

		// --- modul realtime (bus event WebSocket read-only server->client,
		// Fase 12, docs/ROADMAP.md) — dikonstruksi SEBELUM modul lain karena
		// hampir semua modul (student/leave/schedule/attendance/teaching/
		// announcement/notification/billing) butuh realtimeAdapter lewat
		// SetRealtime di bawah (setter, pola sama SetNotifier). realtimeAdapter
		// (realtimeadapter.go) menjembatani Hub.Publish(schoolID, Event) ke
		// consumer-side interface primitif Publish/PublishTo yang dideklarasikan
		// tiap modul pemakai — SATU instance stateless dipakai bersama semua
		// modul (method set identik). GET /api/ws didaftarkan di seksi routing
		// di bawah, di belakang requireAuth SAJA (role display BOLEH).
		realtimeHub := realtime.NewHub()
		realtimeHandler := realtime.NewHandler(realtimeHub)
		realtimeAdapter := realtimeForModules{hub: realtimeHub}

		// --- modul student (siswa, rombel, enrollment, wali, guru, mapel, import, undangan) ---
		// identitySvc & tenantSvc memenuhi student.IdentityGateway /
		// student.AcademicYearLookup secara STRUKTURAL (consumer-side interface
		// dideklarasikan di internal/student — lihat CLAUDE.md) — student TIDAK
		// mengimpor identity maupun tenant untuk tipe apa pun.
		studentRepo := student.NewRepository(pool)
		studentSvc := student.NewService(studentRepo, identitySvc, tenantSvc)
		studentSvc.SetRealtime(realtimeAdapter)
		studentHandler := student.NewHandler(studentSvc)

		// --- modul leave (izin guru dengan approval engine konfigurable) ---
		// identitySvc memenuhi leave.IdentityGateway secara STRUKTURAL
		// (consumer-side interface dideklarasikan di internal/leave — lihat
		// CLAUDE.md) — leave TIDAK mengimpor identity untuk tipe apa pun.
		// platform/storage diimpor LANGSUNG (infrastruktur bersama, seperti clock).
		leaveRepo := leave.NewRepository(pool)
		leaveSvc := leave.NewService(leaveRepo, identitySvc, storage.FromEnv(), clock.System{})
		leaveSvc.SetRealtime(realtimeAdapter)
		leaveHandler := leave.NewHandler(leaveSvc)

		// --- modul schedule (jadwal pelajaran: periods, rooms+QR, slots,
		// deteksi bentrok, copy, import, query kunci SlotNow/SlotsToday/CurrentPeriod) ---
		// identitySvc, tenantSvc, studentSvc memenuhi schedule.IdentityGateway /
		// schedule.AcademicYearLookup / schedule.StudentClassLookup secara
		// STRUKTURAL (consumer-side interface dideklarasikan di
		// internal/schedule — lihat CLAUDE.md) — schedule TIDAK mengimpor
		// identity/tenant/student untuk tipe apa pun. Dikonstruksi SEBELUM
		// attendance & teaching karena keduanya butuh scheduleSvc (fase 6).
		scheduleRepo := schedule.NewRepository(pool)
		scheduleSvc := schedule.NewService(scheduleRepo, identitySvc, tenantSvc, studentSvc, clock.System{})
		scheduleSvc.SetRealtime(realtimeAdapter)
		scheduleHandler := schedule.NewHandler(scheduleSvc)

		// --- modul attendance (absensi siswa mode daily & per-mapel) ---
		// identitySvc, tenantSvc, studentSvc memenuhi attendance.IdentityGateway /
		// attendance.AcademicYearLookup / attendance.StudentAccess /
		// attendance.TeacherLookup secara STRUKTURAL (consumer-side interface
		// dideklarasikan di internal/attendance — lihat CLAUDE.md) — attendance
		// TIDAK mengimpor identity/tenant/student untuk tipe apa pun.
		// attendance.ScheduleSlotLookup TIDAK dipenuhi langsung oleh
		// *schedule.Service (shape data beda, bukan primitif) — dijembatani
		// scheduleForAttendance (lihat scheduleadapter.go, fase 6).
		attendanceRepo := attendance.NewRepository(pool)
		attendanceSvc := attendance.NewService(attendanceRepo, identitySvc, tenantSvc, studentSvc, scheduleForAttendance{svc: scheduleSvc}, studentSvc, clock.System{})
		attendanceSvc.SetRealtime(realtimeAdapter)
		attendanceHandler := attendance.NewHandler(attendanceSvc)

		// --- modul teaching (jurnal mengajar via scan QR ruangan + monitoring
		// status mengajar, fase 6 — docs/06-teaching.md) ---
		// identitySvc & studentSvc memenuhi teaching.IdentityGateway /
		// teaching.StudentGateway secara STRUKTURAL; leaveSvc & attendanceSvc
		// memenuhi teaching.LeaveGateway / teaching.AttendanceGateway secara
		// STRUKTURAL juga (method ApprovedOn/OpenSubjectSession primitif) —
		// teaching TIDAK mengimpor identity/student/leave/attendance untuk
		// tipe apa pun. teaching.ScheduleGateway TIDAK dipenuhi langsung oleh
		// *schedule.Service (shape data beda) — dijembatani scheduleForTeaching
		// (lihat scheduleadapter.go).
		teachingRepo := teaching.NewRepository(pool)
		teachingSvc := teaching.NewService(teachingRepo, identitySvc, scheduleForTeaching{svc: scheduleSvc}, leaveSvc, attendanceSvc, studentSvc, clock.System{})
		teachingSvc.SetRealtime(realtimeAdapter)
		teachingHandler := teaching.NewHandler(teachingSvc)

		// --- modul announcement (pengumuman dashboard TV/kepsek, fase 7 —
		// docs/06-teaching.md "Pengumuman") ---
		// identitySvc memenuhi announcement.IdentityGateway secara STRUKTURAL
		// (consumer-side interface dideklarasikan di internal/announcement —
		// lihat CLAUDE.md) — announcement TIDAK mengimpor identity untuk tipe
		// apa pun.
		announcementRepo := announcement.NewRepository(pool)
		announcementSvc := announcement.NewService(announcementRepo, identitySvc, clock.System{})
		announcementSvc.SetRealtime(realtimeAdapter)
		announcementHandler := announcement.NewHandler(announcementSvc)

		// --- modul dashboard (payload gabungan TV ruang guru + kepsek, fase 7
		// — docs/06-teaching.md) ---
		// dashboard TIDAK punya repository sendiri — murni menyusun ULANG data
		// dari teaching/attendance/announcement/schedule lewat consumer-side
		// interface. Producer masing-masing mengembalikan STRUCT bernama milik
		// package sendiri (bukan primitif), jadi TIDAK dipenuhi struktural
		// langsung — dijembatani adapter kecil (lihat dashboardadapter.go).
		dashboardSvc := dashboard.NewService(
			teachingForDashboard{svc: teachingSvc},
			attendanceForDashboard{svc: attendanceSvc},
			announcementForDashboard{svc: announcementSvc},
			scheduleForDashboard{svc: scheduleSvc},
			clock.System{},
		)
		dashboardHandler := dashboard.NewHandler(dashboardSvc)

		// --- modul notification (notifikasi pluggable: in-app, web push,
		// whatsapp, email — outbox + channel provider, fase 9,
		// docs/08-notification.md) ---
		// notification TIDAK mengimpor modul lain (bahkan identity): kontak
		// (phone/email) diresolusi lewat join read-only ke `users` di
		// queries.sql SENDIRI (pola yang sama dengan leave/attendance
		// menjoin `users` utk nama) — TIDAK butuh consumer-side interface ke
		// identity. attendanceSvc & leaveSvc memenuhi notification-side
		// (mereka mendeklarasikan Notifier sendiri) — *notification.Service
		// memenuhi attendance.Notifier / leave.Notifier secara STRUKTURAL
		// lewat method Notify, disuntik lewat SetNotifier di bawah (setter,
		// BUKAN parameter constructor, supaya call site test attendance/leave
		// yang sudah ada tidak berubah — lihat catatan di
		// internal/attendance/service.go SetNotifier).
		notificationRepo := notification.NewRepository(pool)
		notificationSvc := notification.NewService(notificationRepo, clock.System{})

		vapidPublic, vapidPrivate, err := notification.LoadOrGenerateVAPIDKeys(ctx, notificationRepo, cfg.VAPIDPublicKey, cfg.VAPIDPrivateKey)
		if err != nil {
			slog.Error("gagal memuat/generate kunci VAPID", "err", err)
			os.Exit(1)
		}
		notificationSvc.SetVAPIDPublicKey(vapidPublic)
		notificationSvc.RegisterProvider(notification.ChannelWebPush, notification.NewWebPushProvider(notificationRepo, vapidPublic, vapidPrivate, cfg.VAPIDSubscriber))
		notificationSvc.RegisterProvider(notification.ChannelWhatsApp, notification.NewWhatsAppProvider(notificationRepo, cfg.WAGatewayURL, cfg.WAGatewayToken))
		notificationSvc.RegisterProvider(notification.ChannelEmail, notification.NewEmailProvider(notificationRepo, cfg.SMTPHost, cfg.SMTPPort, cfg.SMTPUsername, cfg.SMTPPassword, cfg.SMTPFrom))
		notificationSvc.SetRealtime(realtimeAdapter)
		notificationHandler := notification.NewHandler(notificationSvc)

		// attendanceSvc & leaveSvc memenuhi consumer-side interface Notifier
		// masing-masing (attendance.Notifier / leave.Notifier) secara
		// struktural lewat *notification.Service.Notify.
		attendanceSvc.SetNotifier(notificationSvc)
		leaveSvc.SetNotifier(notificationSvc)

		// --- modul billing (langganan tahunan, tier x bracket siswa, invoice,
		// transfer manual + gateway Midtrans, lifecycle grace/readonly, fase
		// 10, docs/09-billing.md) ---
		// identitySvc memenuhi billing.AuditLogger, tenantSvc memenuhi
		// billing.SchoolGateway (nama/slug sekolah utk kop PDF & panel admin)
		// secara STRUKTURAL — billing TIDAK mengimpor identity/tenant untuk
		// tipe apa pun. Dikonstruksi SETELAH identity/tenant (butuh keduanya)
		// tapi SEBELUM route wiring (attendance/dashboard butuh
		// billingSvc.RequireFeature saat registrasi route di bawah).
		billingRepo := billing.NewRepository(pool)
		billingSvc := billing.NewService(billingRepo, identitySvc, tenantSvc, storage.FromEnv(), clock.System{})
		billingSvc.SetRealtime(realtimeAdapter)
		if cfg.MidtransServerKey != "" {
			billingSvc.SetProvider(billing.NewMidtransProvider(cfg.MidtransServerKey, cfg.MidtransEnv))
		} else {
			slog.Warn("MIDTRANS_SERVER_KEY kosong — pembayaran gateway dinonaktifkan (transfer manual tetap jalan)")
		}
		billingHandler := billing.NewHandler(billingSvc)

		// identitySvc.Me() & notificationSvc.Send() memenuhi consumer-side
		// interface masing-masing (identity.BillingGateway / notification.FeatureGate)
		// secara struktural lewat *billing.Service, disuntik lewat setter
		// (pola yang sama dengan SetNotifier di atas).
		identitySvc.SetBillingGateway(billingSvc)
		notificationSvc.SetFeatureGate(billingSvc)

		// TickOnce sekali saat startup (docs/09-billing.md "job harian ...
		// idempoten") — supaya transisi grace/readonly yang seharusnya sudah
		// terjadi sebelum server restart langsung tercermin, tanpa menunggu
		// ticker jam pertama.
		if err := billingSvc.TickOnce(ctx); err != nil {
			slog.Error("billing: tick lifecycle awal gagal", "err", err)
		}

		// --- wiring routes ---
		identity.RegisterRoutes(mux, identityHandler, identitySvc.RequireAuth)
		tenant.RegisterRoutes(mux, tenantHandler, identitySvc.RequireAuth, identitySvc.RequireSuperAdmin, identitySvc.RequirePerm)
		student.RegisterRoutes(mux, studentHandler, identitySvc.RequireAuth, identitySvc.RequirePerm)
		attendance.RegisterRoutes(mux, attendanceHandler, identitySvc.RequireAuth, identitySvc.RequirePerm,
			billingSvc.RequireFeature(billing.FeatureQRCard), billingSvc.RequireFeature(billing.FeatureSelfCheckin))
		leave.RegisterRoutes(mux, leaveHandler, identitySvc.RequireAuth, identitySvc.RequirePerm)
		schedule.RegisterRoutes(mux, scheduleHandler, identitySvc.RequireAuth, identitySvc.RequirePerm)
		teaching.RegisterRoutes(mux, teachingHandler, identitySvc.RequireAuth, identitySvc.RequirePerm)
		announcement.RegisterRoutes(mux, announcementHandler, identitySvc.RequireAuth, identitySvc.RequirePerm)
		dashboard.RegisterRoutes(mux, dashboardHandler, identitySvc.RequireAuth, identitySvc.RequirePerm,
			billingSvc.RequireFeature(billing.FeatureTVDashboard))
		notification.RegisterRoutes(mux, notificationHandler, identitySvc.RequireAuth)
		billing.RegisterRoutes(mux, billingHandler, identitySvc.RequireAuth, identitySvc.RequireSuperAdmin, identitySvc.RequirePerm)
		realtime.RegisterRoutes(mux, realtimeHandler, identitySvc.RequireAuth)

		// Worker outbox — poll tiap 10 detik, batch 50 (docs/08-notification.md).
		// Berhenti dengan rapi saat ctx dibatalkan (graceful shutdown, sama
		// seperti http.Server di bawah).
		go notification.StartWorker(ctx, notificationSvc, 10*time.Second, 50)

		// Worker lifecycle billing — ticker 1 jam (docs/09-billing.md "job
		// harian ... cukup goroutine ticker"). Berhenti dengan rapi saat ctx
		// dibatalkan.
		go billing.StartLifecycleWorker(ctx, billingSvc, time.Hour)

		tenantResolverMW = hostResolver.Middleware
		billingGuardMW = billingSvc.Middleware
	} else {
		slog.Warn("DATABASE_URL kosong — jalan tanpa database (mode dev, hanya /api/health aktif)")
	}

	// Urutan wajib: Recover -> Logger -> SecurityHeaders -> ResolveTenant ->
	// SubscriptionGuard -> routing (per-route RequireAuth/RequirePerm/
	// RequireFeature dipasang saat registrasi route di atas). SubscriptionGuard
	// (billing, fase 10, docs/09-billing.md & docs/01-tenant.md "Kaitan
	// subscription") HARUS setelah ResolveTenant (butuh school_id di
	// context) dan SEBELUM routing (menggerbang di level HTTP, sebelum
	// RequireAuth — GET/exempt path tetap lolos tanpa login, mis. webhook).
	handler := middleware.Chain(mux,
		middleware.Recover,
		middleware.Logger,
		middleware.SecurityHeaders,
		tenantResolverMW,
		billingGuardMW,
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
