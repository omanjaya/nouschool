// bootstrap membuat/memperbarui akun super admin (dan opsional sekolah demo)
// sekali jalan, sebelum panel admin ada UI-nya. Idempoten — aman dijalankan
// berkali-kali (upsert by email/username).
//
// Pemakaian:
//
//	go run ./cmd/bootstrap -email admin@nouschool.id -password rahasia123 -name "Admin NouSchool" [-demo]
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/omanjaya/nouschool/internal/attendance"
	"github.com/omanjaya/nouschool/internal/billing"
	"github.com/omanjaya/nouschool/internal/identity"
	"github.com/omanjaya/nouschool/internal/platform/clock"
	"github.com/omanjaya/nouschool/internal/platform/config"
	"github.com/omanjaya/nouschool/internal/platform/database"
	"github.com/omanjaya/nouschool/internal/schedule"
	"github.com/omanjaya/nouschool/internal/student"
	"github.com/omanjaya/nouschool/internal/tenant"
)

func main() {
	email := flag.String("email", "", "Email super admin (wajib)")
	password := flag.String("password", "", "Kata sandi super admin (wajib, minimal 8 karakter)")
	name := flag.String("name", "", "Nama super admin (wajib)")
	demo := flag.Bool("demo", false, "Buat juga sekolah demo (slug 'demo') + admin sekolah demo")
	flag.Parse()

	if *email == "" || *password == "" || *name == "" {
		fmt.Fprintln(os.Stderr, "gunakan: bootstrap -email <email> -password <password> -name <nama> [-demo]")
		os.Exit(2)
	}
	if len(*password) < 8 {
		fmt.Fprintln(os.Stderr, "password minimal 8 karakter")
		os.Exit(2)
	}

	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, nil)))
	cfg := config.MustLoad()
	if cfg.DatabaseURL == "" {
		slog.Error("DATABASE_URL kosong — bootstrap butuh koneksi database")
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("gagal konek database", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	identityRepo := identity.NewRepository(pool)
	tenantRepo := tenant.NewRepository(pool)

	superAdminID, err := upsertUser(ctx, identityRepo, upsertUserInput{
		Email:        *email,
		Name:         *name,
		Password:     *password,
		IsSuperAdmin: true,
	})
	if err != nil {
		slog.Error("gagal membuat/memperbarui super admin", "err", err)
		os.Exit(1)
	}
	slog.Info("super admin siap", "id", superAdminID, "email", *email)

	if !*demo {
		return
	}

	schoolID, err := ensureDemoSchool(ctx, tenantRepo)
	if err != nil {
		slog.Error("gagal menyiapkan sekolah demo", "err", err)
		os.Exit(1)
	}
	slog.Info("sekolah demo siap", "school_id", schoolID, "slug", "demo")

	if err := ensureDemoAcademicYear(ctx, tenantRepo, schoolID); err != nil {
		slog.Error("gagal menyiapkan tahun ajaran demo", "err", err)
		os.Exit(1)
	}
	slog.Info("tahun ajaran 2026/2027 aktif untuk sekolah demo")

	demoAdminID, err := upsertUser(ctx, identityRepo, upsertUserInput{
		Username: "admin",
		Name:     "Admin Demo",
		Password: "admin12345",
	})
	if err != nil {
		slog.Error("gagal membuat/memperbarui admin demo", "err", err)
		os.Exit(1)
	}
	if _, err := identityRepo.CreateMembership(ctx, demoAdminID, schoolID, identity.RoleAdminSekolah); err != nil {
		slog.Error("gagal membuat membership admin demo", "err", err)
		os.Exit(1)
	}
	slog.Info("admin sekolah demo siap", "username", "admin", "password", "admin12345", "school_id", schoolID)

	// --- data contoh modul student (Fase 2): rombel, siswa, guru, mapel ---
	activeYear, err := tenantRepo.ActiveAcademicYear(ctx, schoolID)
	if err != nil {
		slog.Error("gagal mengambil tahun ajaran aktif sekolah demo", "err", err)
		os.Exit(1)
	}

	studentRepo := student.NewRepository(pool)
	// identitySvc dipakai lewat consumer-side interface student.IdentityGateway
	// (HashPassword/CreateAccount/CreateMembership) — cukup rate limiter kosong
	// karena bootstrap tidak pernah memanggil Login.
	identitySvc := identity.NewService(identityRepo, identity.NewRateLimiter(5, 15*time.Minute, nil), false)

	classIDs, err := ensureDemoClasses(ctx, studentRepo, schoolID, activeYear.ID)
	if err != nil {
		slog.Error("gagal menyiapkan rombel demo", "err", err)
		os.Exit(1)
	}
	slog.Info("rombel demo siap", "count", len(classIDs))

	if err := ensureDemoStudents(ctx, studentRepo, schoolID, activeYear.ID, classIDs); err != nil {
		slog.Error("gagal menyiapkan siswa demo", "err", err)
		os.Exit(1)
	}
	slog.Info("siswa demo siap", "count", 8)

	if err := ensureDemoTeachers(ctx, identitySvc, identityRepo, studentRepo, schoolID); err != nil {
		slog.Error("gagal menyiapkan guru demo", "err", err)
		os.Exit(1)
	}
	slog.Info("guru demo siap", "count", 2)

	if err := ensureDemoSubjects(ctx, studentRepo, schoolID); err != nil {
		slog.Error("gagal menyiapkan mapel demo", "err", err)
		os.Exit(1)
	}
	slog.Info("mapel demo siap", "count", 3)

	if err := ensureDemoStudentAccount(ctx, identityRepo, studentRepo, schoolID); err != nil {
		slog.Error("gagal menyiapkan akun siswa demo", "err", err)
		os.Exit(1)
	}
	slog.Info("akun siswa demo siap", "username", "siswa", "password", "siswa12345", "nis", "22101")

	// --- data contoh modul leave (Fase 4): akun kepala sekolah demo ---
	kepsekID, err := upsertUser(ctx, identityRepo, upsertUserInput{
		Username: "kepsek",
		Name:     "Kepala Sekolah Demo",
		Password: "kepsek12345",
	})
	if err != nil {
		slog.Error("gagal menyiapkan akun kepala sekolah demo", "err", err)
		os.Exit(1)
	}
	if _, err := identityRepo.CreateMembership(ctx, kepsekID, schoolID, identity.RoleKepalaSekolah); err != nil {
		slog.Error("gagal membuat membership kepala sekolah demo", "err", err)
		os.Exit(1)
	}
	slog.Info("kepala sekolah demo siap", "username", "kepsek", "password", "kepsek12345", "school_id", schoolID)

	// --- data contoh modul schedule (Fase 5): periods, ruangan, jadwal demo ---
	// tenantSvc/studentSvcForSchedule dibuat KHUSUS di sini untuk memenuhi
	// schedule.AcademicYearLookup / schedule.StudentClassLookup secara
	// struktural (consumer-side interface — lihat internal/schedule/service.go);
	// bootstrap sudah pakai studentRepo/tenantRepo langsung di tempat lain.
	scheduleRepo := schedule.NewRepository(pool)
	tenantSvc := tenant.NewService(tenantRepo, identitySvc)
	studentSvcForSchedule := student.NewService(studentRepo, identitySvc, tenantSvc)
	scheduleSvc := schedule.NewService(scheduleRepo, identitySvc, tenantSvc, studentSvcForSchedule, clock.System{})

	if err := ensureDemoPeriods(ctx, scheduleSvc, scheduleRepo, superAdminID, schoolID); err != nil {
		slog.Error("gagal menyiapkan jam pelajaran demo", "err", err)
		os.Exit(1)
	}
	slog.Info("jam pelajaran demo siap (9 period: 8 KBM + istirahat)")

	roomIDs, err := ensureDemoRooms(ctx, scheduleSvc, superAdminID, schoolID)
	if err != nil {
		slog.Error("gagal menyiapkan ruangan demo", "err", err)
		os.Exit(1)
	}
	slog.Info("ruangan demo siap", "count", len(roomIDs))

	if err := ensureDemoSchedule(ctx, scheduleSvc, scheduleRepo, studentRepo, schoolID, activeYear.ID, superAdminID, classIDs); err != nil {
		slog.Error("gagal menyiapkan jadwal demo", "err", err)
		os.Exit(1)
	}
	slog.Info("jadwal demo siap", "kelas", "XII RPL 1, XI RPL 2", "hari", "Senin-Jumat")

	// --- fase 6 (teaching + attendance per-mapel): slot demo HARI INI ---
	// ensureDemoSchedule di atas hanya mengisi Senin-Jumat, jadi Sabtu/Minggu
	// SELALU kosong (SlotNow/SlotsForDayOfWeek tidak ada yang bisa ditemukan)
	// — bootstrap ini bisa dijalankan ulang kapan pun (mesin dev, CI, demo di
	// hari apa pun), jadi tambahkan 2 slot pada day_of_week HARI INI (waktu
	// lokal sekolah) supaya verifikasi end-to-end scan QR/monitoring selalu
	// reproducible, termasuk Minggu (day_of_week=0 — lihat komentar dayNames
	// di internal/schedule/model.go, diotorisasi eksplisit di scope fase 6).
	if err := ensureDemoTodaySlots(ctx, scheduleSvc, scheduleRepo, studentRepo, schoolID, activeYear.ID, superAdminID, classIDs); err != nil {
		slog.Error("gagal menyiapkan slot demo hari ini", "err", err)
		os.Exit(1)
	}
	slog.Info("slot demo HARI INI siap (fase 6, verifikasi e2e scan/monitoring)")

	// --- fase 7 (dashboard TV + kepsek): akun display demo ---
	// docs/06-teaching.md "Dashboard TV ruang guru": akun display login sekali
	// di TV/mini-PC, session panjang (1 tahun — lihat internal/identity/session.go
	// sessionTTLForRole), permission read-only (teaching:monitor, schedule:read).
	displayID, err := upsertUser(ctx, identityRepo, upsertUserInput{
		Username: "display",
		Name:     "Display TV Ruang Guru",
		Password: "display12345",
	})
	if err != nil {
		slog.Error("gagal menyiapkan akun display demo", "err", err)
		os.Exit(1)
	}
	if _, err := identityRepo.CreateMembership(ctx, displayID, schoolID, identity.RoleDisplay); err != nil {
		slog.Error("gagal membuat membership display demo", "err", err)
		os.Exit(1)
	}
	slog.Info("akun display demo siap", "username", "display", "password", "display12345", "school_id", schoolID)

	// --- fase 8 (QR kartu siswa + self check-in GPS): settings absensi demo ---
	// Mengaktifkan metode qr_card & self_checkin (default sebelumnya hanya
	// manual, lihat internal/attendance/settings.go DefaultSettings) supaya
	// verifikasi end-to-end QR scan & self check-in bisa langsung dijalankan.
	if err := ensureDemoAttendanceSettings(ctx, tenantRepo, demoAdminID, schoolID); err != nil {
		slog.Error("gagal menyiapkan settings absensi demo", "err", err)
		os.Exit(1)
	}
	slog.Info("settings absensi demo siap (methods: manual, qr_card, self_checkin)", "school_id", schoolID)

	// --- fase 10 (billing): subscription Pro AKTIF 1 tahun utk sekolah demo,
	// supaya seluruh fitur existing (tv_dashboard, whatsapp, qr_card,
	// self_checkin) tetap jalan tanpa perlu super admin klik apa pun
	// (docs/09-billing.md). Invoice manual_transfer, verified langsung.
	billingRepo := billing.NewRepository(pool)
	billingSvc := billing.NewService(billingRepo, identitySvc, tenantSvc, nil, clock.System{})
	if err := ensureDemoSubscription(ctx, billingRepo, billingSvc, demoAdminID, schoolID); err != nil {
		slog.Error("gagal menyiapkan langganan demo", "err", err)
		os.Exit(1)
	}
	slog.Info("langganan demo siap (plan pro, status active, invoice paid)", "school_id", schoolID)
}

// ensureDemoSubscription memberi sekolah demo langganan Pro AKTIF 1 tahun
// (invoice manual_transfer, langsung diverifikasi) — idempoten: bila
// sekolah demo SUDAH punya subscription berstatus active, tidak melakukan
// apa pun (aman dijalankan ulang tanpa menumpuk invoice).
func ensureDemoSubscription(ctx context.Context, repo *billing.Repository, svc *billing.Service, actorUserID, schoolID int64) error {
	existing, found, err := repo.GetSubscription(ctx, schoolID)
	if err != nil {
		return err
	}
	if found && existing.Status == billing.StatusActive {
		return nil
	}

	invView, err := svc.CreateSubscriptionInvoice(ctx, actorUserID, schoolID, "pro")
	if err != nil {
		return fmt.Errorf("buat invoice langganan demo: %w", err)
	}
	proofView, err := svc.UploadProof(ctx, actorUserID, schoolID, invView.ID, "bukti-demo.png", demoProofPNG)
	if err != nil {
		return fmt.Errorf("unggah bukti transfer demo: %w", err)
	}
	if _, err := svc.VerifyManualPayment(ctx, actorUserID, proofView.ID); err != nil {
		return fmt.Errorf("verifikasi pembayaran demo: %w", err)
	}
	return nil
}

// demoProofPNG — PNG 1x1 valid minimal (magic bytes lolos http.DetectContentType
// "image/png") dipakai sebagai bukti transfer placeholder bootstrap demo.
var demoProofPNG = []byte{
	0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52,
	0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01, 0x08, 0x06, 0x00, 0x00, 0x00, 0x1F, 0x15, 0xC4,
	0x89, 0x00, 0x00, 0x00, 0x0A, 0x49, 0x44, 0x41, 0x54, 0x78, 0x9C, 0x63, 0x00, 0x01, 0x00, 0x00,
	0x05, 0x00, 0x01, 0x0D, 0x0A, 0x2D, 0xB4, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4E, 0x44, 0xAE,
	0x42, 0x60, 0x82,
}

// ensureDemoAttendanceSettings mengaktifkan qr_card & self_checkin utk
// sekolah demo — idempoten: HANYA menulis bila sekolah ini belum pernah
// menyimpan settings module='attendance' sama sekali (mis. sudah pernah
// dikustomisasi admin lewat PUT /api/settings/attendance, bootstrap tidak
// boleh menimpanya).
func ensureDemoAttendanceSettings(ctx context.Context, tenantRepo *tenant.Repository, actorUserID, schoolID int64) error {
	_, found, err := tenantRepo.GetSetting(ctx, schoolID, "attendance")
	if err != nil {
		return err
	}
	if found {
		return nil
	}

	settingsSvc := tenant.NewSettingsService(tenantRepo, nil)
	settings := attendance.Settings{
		Methods: []attendance.Method{attendance.MethodManual, attendance.MethodQRCard, attendance.MethodSelfCheckin},
		SelfCheckin: &attendance.SelfCheckinRule{
			Lat: -6.2, Lng: 106.816666, RadiusM: 150,
			OpenFrom: "06:00",
			// NOTE DEMO: close_at sengaja dibuat SANGAT longgar (23:59) —
			// bukan nilai realistis untuk sekolah asli (yang biasanya menutup
			// jendela check-in di sekitar jam masuk, mis. 07:30) — supaya
			// verifikasi end-to-end self check-in bisa dijalankan kapan pun
			// bootstrap ini dijalankan, tanpa bergantung jam saat itu.
			CloseAt: "23:59",
		},
		LateAfterMin: 15,
	}
	// Mode & EditWindowHours sengaja dibiarkan kosong -> Validate() mengisi
	// default (daily, 24 jam) — sama seperti sekolah yang belum pernah
	// menyentuh menu settings sama sekali.
	return settingsSvc.Put(ctx, schoolID, actorUserID, "attendance", &settings)
}

// ensureDemoPeriods membuat 9 period demo (8 jam KBM + 1 Istirahat, semua
// bernomor berurutan — docs/04-schedule.md) bila sekolah belum punya period
// sama sekali. Idempoten.
func ensureDemoPeriods(ctx context.Context, svc *schedule.Service, repo *schedule.Repository, actorUserID, schoolID int64) error {
	existing, err := repo.ListPeriods(ctx, schoolID)
	if err != nil {
		return err
	}
	if len(existing) > 0 {
		return nil
	}
	periods := []schedule.ReplacePeriodInput{
		{Number: 1, StartsAt: "07:00", EndsAt: "07:40"},
		{Number: 2, StartsAt: "07:40", EndsAt: "08:20"},
		{Number: 3, StartsAt: "08:20", EndsAt: "09:00"},
		{Number: 4, StartsAt: "09:00", EndsAt: "09:30"},
		{Number: 5, StartsAt: "09:30", EndsAt: "10:00", Label: "Istirahat"},
		{Number: 6, StartsAt: "10:00", EndsAt: "11:00"},
		{Number: 7, StartsAt: "11:00", EndsAt: "12:00"},
		{Number: 8, StartsAt: "12:00", EndsAt: "13:00"},
		{Number: 9, StartsAt: "13:00", EndsAt: "14:00"},
	}
	_, err = svc.ReplacePeriods(ctx, actorUserID, schoolID, periods)
	return err
}

// ensureDemoRooms membuat 3 ruangan contoh (R-101, R-102, Lab Komputer 1)
// bila belum ada (idempoten by nama).
func ensureDemoRooms(ctx context.Context, svc *schedule.Service, actorUserID, schoolID int64) (map[string]int64, error) {
	names := []string{"R-101", "R-102", "Lab Komputer 1"}
	existing, err := svc.ListRooms(ctx, schoolID)
	if err != nil {
		return nil, err
	}
	byName := make(map[string]int64, len(existing))
	for _, r := range existing {
		byName[r.Name] = r.ID
	}
	for _, name := range names {
		if _, ok := byName[name]; ok {
			continue
		}
		room, err := svc.CreateRoom(ctx, actorUserID, schoolID, name)
		if err != nil {
			return nil, err
		}
		byName[name] = room.ID
	}
	return byName, nil
}

// demoSlotSpec adalah satu baris jadwal demo (sebelum referensi mapel/guru
// di-resolve ke ID).
type demoSlotSpec struct {
	Day                    int
	ClassName              string
	PeriodStart, PeriodEnd int
	SubjectCode            string
	TeacherEmail           string
}

// buildDemoSlots menyusun jadwal Senin-Jumat untuk XII RPL 1 & XI RPL 2:
// 2 blok 2-JP per hari per kelas (sebelum istirahat, jam ke-1-2 & 3-4),
// dipakai bergantian guru Rendi (Basis Data/Pemrograman Web) & Sari
// (Matematika) — disusun supaya TIDAK PERNAH bentrok: pada jam ke-1-2, Rendi
// mengajar XII RPL 1 sementara Sari mengajar XI RPL 2; pada jam ke-3-4
// sebaliknya (Sari -> XII RPL 1, Rendi -> XI RPL 2).
func buildDemoSlots() []demoSlotSpec {
	var out []demoSlotSpec
	for day := 1; day <= 5; day++ {
		xiiSubject := "BDT"
		xiSubject := "PWB"
		if day == 2 || day == 4 { // Selasa/Kamis: selang-seling supaya tidak monoton
			xiiSubject, xiSubject = "PWB", "BDT"
		}
		out = append(out,
			demoSlotSpec{Day: day, ClassName: "XII RPL 1", PeriodStart: 1, PeriodEnd: 2, SubjectCode: xiiSubject, TeacherEmail: "rendi@demo.sch.id"},
			demoSlotSpec{Day: day, ClassName: "XI RPL 2", PeriodStart: 1, PeriodEnd: 2, SubjectCode: "MTK", TeacherEmail: "sari@demo.sch.id"},
			demoSlotSpec{Day: day, ClassName: "XII RPL 1", PeriodStart: 3, PeriodEnd: 4, SubjectCode: "MTK", TeacherEmail: "sari@demo.sch.id"},
			demoSlotSpec{Day: day, ClassName: "XI RPL 2", PeriodStart: 3, PeriodEnd: 4, SubjectCode: xiSubject, TeacherEmail: "rendi@demo.sch.id"},
		)
	}
	return out
}

// ensureDemoSchedule mengisi jadwal XII RPL 1 & XI RPL 2 Senin-Jumat bila
// kelas tsb BELUM punya slot sama sekali (idempoten: cek slot count per
// kelas > 0 -> skip seluruh kelas itu, bukan per baris).
func ensureDemoSchedule(ctx context.Context, svc *schedule.Service, repo *schedule.Repository, studentRepo *student.Repository, schoolID, yearID, actorUserID int64, classIDs map[string]int64) error {
	existing, err := repo.ListSlotsForYear(ctx, schoolID, yearID)
	if err != nil {
		return err
	}
	doneClasses := make(map[int64]bool, len(existing))
	for _, sl := range existing {
		doneClasses[sl.ClassID] = true
	}

	subjectCache := map[string]int64{}
	teacherCache := map[string]int64{}
	getSubject := func(code string) (int64, error) {
		if id, ok := subjectCache[code]; ok {
			return id, nil
		}
		s, err := studentRepo.GetSubjectByCode(ctx, schoolID, code)
		if err != nil {
			return 0, err
		}
		subjectCache[code] = s.ID
		return s.ID, nil
	}
	getTeacher := func(email string) (int64, error) {
		if id, ok := teacherCache[email]; ok {
			return id, nil
		}
		t, err := studentRepo.GetTeacherByEmail(ctx, schoolID, email)
		if err != nil {
			return 0, err
		}
		teacherCache[email] = t.ID
		return t.ID, nil
	}

	for _, spec := range buildDemoSlots() {
		classID, ok := classIDs[spec.ClassName]
		if !ok {
			return fmt.Errorf("bootstrap: rombel %q tidak ditemukan untuk jadwal demo", spec.ClassName)
		}
		if doneClasses[classID] {
			continue
		}
		subjectID, err := getSubject(spec.SubjectCode)
		if err != nil {
			return err
		}
		teacherID, err := getTeacher(spec.TeacherEmail)
		if err != nil {
			return err
		}
		if _, err := svc.CreateSlot(ctx, actorUserID, schoolID, schedule.SlotInputRequest{
			ClassID: classID, SubjectID: subjectID, TeacherID: teacherID,
			DayOfWeek: spec.Day, PeriodStart: spec.PeriodStart, PeriodEnd: spec.PeriodEnd,
			AcademicYearID: yearID,
		}); err != nil {
			return fmt.Errorf("bootstrap: gagal membuat slot demo (%s hari %d jam %d-%d): %w",
				spec.ClassName, spec.Day, spec.PeriodStart, spec.PeriodEnd, err)
		}
	}
	return nil
}

// ensureDemoTodaySlots menambahkan 2 slot jadwal pada HARI INI (day_of_week
// waktu lokal sekolah "Asia/Jakarta" — termasuk Minggu=0) untuk XII RPL 1,
// bila kelas itu BELUM punya slot pada hari tsb sama sekali. Idempoten:
// hanya menambah bila hari itu masih kosong untuk kelas ini (tidak
// menduplikasi tiap kali bootstrap dijalankan ulang pada hari yang sama).
func ensureDemoTodaySlots(ctx context.Context, svc *schedule.Service, repo *schedule.Repository, studentRepo *student.Repository, schoolID, yearID, actorUserID int64, classIDs map[string]int64) error {
	today := int(clock.InZone(time.Now(), "Asia/Jakarta").Weekday())

	existing, err := repo.ListSlotsForYear(ctx, schoolID, yearID)
	if err != nil {
		return err
	}
	xiiID, ok := classIDs["XII RPL 1"]
	if !ok {
		return fmt.Errorf("bootstrap: rombel XII RPL 1 tidak ditemukan utk slot hari ini")
	}
	for _, sl := range existing {
		if sl.ClassID == xiiID && sl.DayOfWeek == today {
			return nil // sudah ada slot hari ini utk kelas ini -> idempoten, skip
		}
	}

	bdt, err := studentRepo.GetSubjectByCode(ctx, schoolID, "BDT")
	if err != nil {
		return err
	}
	mtk, err := studentRepo.GetSubjectByCode(ctx, schoolID, "MTK")
	if err != nil {
		return err
	}
	rendi, err := studentRepo.GetTeacherByEmail(ctx, schoolID, "rendi@demo.sch.id")
	if err != nil {
		return err
	}
	sari, err := studentRepo.GetTeacherByEmail(ctx, schoolID, "sari@demo.sch.id")
	if err != nil {
		return err
	}

	specs := []struct {
		subjectID, teacherID   int64
		periodStart, periodEnd int
	}{
		{bdt.ID, rendi.ID, 1, 2}, // 07:00-08:20 WIB
		{mtk.ID, sari.ID, 3, 4},  // 08:20-09:30 WIB
	}
	for _, sp := range specs {
		if _, err := svc.CreateSlot(ctx, actorUserID, schoolID, schedule.SlotInputRequest{
			ClassID: xiiID, SubjectID: sp.subjectID, TeacherID: sp.teacherID,
			DayOfWeek: today, PeriodStart: sp.periodStart, PeriodEnd: sp.periodEnd,
			AcademicYearID: yearID,
		}); err != nil {
			return fmt.Errorf("bootstrap: gagal membuat slot hari ini (day_of_week=%d jam %d-%d): %w", today, sp.periodStart, sp.periodEnd, err)
		}
	}
	return nil
}

// ensureDemoStudentAccount membuat akun login siswa demo (username `siswa`)
// yang tertaut ke siswa NIS 22101 — supaya alur siswa (riwayat kehadiran, dst)
// bisa diuji tanpa aktivasi kode undangan. Idempoten.
func ensureDemoStudentAccount(ctx context.Context, identityRepo *identity.Repository, studentRepo *student.Repository, schoolID int64) error {
	userID, err := upsertUser(ctx, identityRepo, upsertUserInput{
		Username: "siswa",
		Name:     "Ahmad Fauzi",
		Password: "siswa12345",
	})
	if err != nil {
		return err
	}
	if _, err := identityRepo.CreateMembership(ctx, userID, schoolID, identity.RoleSiswa); err != nil {
		return err
	}
	rec, err := studentRepo.GetStudentByNIS(ctx, schoolID, "22101")
	if err != nil {
		return err
	}
	if rec.UserID == userID {
		return nil
	}
	return studentRepo.SetStudentUserID(ctx, rec.ID, userID)
}

// ensureDemoClasses membuat 2 rombel contoh pada tahun ajaran aktif bila
// belum ada (idempoten by (school_id, academic_year_id, name)).
func ensureDemoClasses(ctx context.Context, repo *student.Repository, schoolID, yearID int64) (map[string]int64, error) {
	specs := []struct{ name, grade, major string }{
		{"XII RPL 1", "XII", "RPL"},
		{"XI RPL 2", "XI", "RPL"},
	}
	ids := make(map[string]int64, len(specs))
	for _, sp := range specs {
		existing, err := repo.GetClassByNameYear(ctx, schoolID, yearID, sp.name)
		if err == nil {
			ids[sp.name] = existing.ID
			continue
		}
		if !errors.Is(err, student.ErrNotFound) {
			return nil, err
		}
		created, err := repo.CreateClass(ctx, student.ClassRecord{
			SchoolID: schoolID, AcademicYearID: yearID, Name: sp.name, Grade: sp.grade, Major: sp.major,
		})
		if err != nil {
			return nil, err
		}
		ids[sp.name] = created.ID
	}
	return ids, nil
}

// ensureDemoStudents membuat 8 siswa contoh (NIS 22101-22108) & mendaftarkan
// masing-masing ke rombelnya bila belum ada (idempoten by (school_id, nis)).
func ensureDemoStudents(ctx context.Context, repo *student.Repository, schoolID, yearID int64, classIDs map[string]int64) error {
	specs := []struct{ nis, name, gender, class string }{
		{"22101", "Ahmad Fauzi", "L", "XII RPL 1"},
		{"22102", "Siti Nurhaliza", "P", "XII RPL 1"},
		{"22103", "Budi Santoso", "L", "XII RPL 1"},
		{"22104", "Dewi Lestari", "P", "XII RPL 1"},
		{"22105", "Rizky Pratama", "L", "XI RPL 2"},
		{"22106", "Putri Ayu Anggraini", "P", "XI RPL 2"},
		{"22107", "Agus Setiawan", "L", "XI RPL 2"},
		{"22108", "Nadia Salsabila", "P", "XI RPL 2"},
	}
	for _, sp := range specs {
		rec, err := repo.GetStudentByNIS(ctx, schoolID, sp.nis)
		if errors.Is(err, student.ErrNotFound) {
			rec, err = repo.CreateStudent(ctx, student.CreateStudentInput{
				SchoolID: schoolID, NIS: sp.nis, Name: sp.name, Gender: sp.gender,
			})
			if err != nil {
				return err
			}
		} else if err != nil {
			return err
		}
		classID, ok := classIDs[sp.class]
		if !ok {
			return fmt.Errorf("bootstrap: rombel %q tidak ditemukan untuk siswa %s", sp.class, sp.nis)
		}
		if _, err := repo.EnrollStudentsBatch(ctx, schoolID, classID, yearID, []int64{rec.ID}); err != nil {
			return err
		}
	}
	return nil
}

// ensureDemoTeachers membuat 2 akun guru contoh (idempoten by email) —
// password "guru12345" DIPAKSA (upsert) setiap kali bootstrap dijalankan,
// bukan hanya saat akun baru dibuat. Ini menutup celah dari fase 2: akun guru
// yang pernah dibuat lewat CreateAccount dengan password placeholder acak
// (mis. lewat import Excel) jadi TIDAK BISA login sampai bootstrap
// dijalankan ulang — dengan password dipaksa di sini, bootstrap selalu
// membuat akun demo bisa dipakai login (lihat Fase 4, verifikasi e2e leave).
func ensureDemoTeachers(ctx context.Context, identitySvc *identity.Service, identityRepo *identity.Repository, repo *student.Repository, schoolID int64) error {
	specs := []struct{ name, email string }{
		{"Rendi Saputra", "rendi@demo.sch.id"},
		{"Sari Wulandari", "sari@demo.sch.id"},
	}
	hash, err := identitySvc.HashPassword("guru12345")
	if err != nil {
		return err
	}
	for _, sp := range specs {
		userID, exists, err := identitySvc.UserIDByEmail(ctx, sp.email)
		if err != nil {
			return err
		}
		if !exists {
			userID, err = identitySvc.CreateAccount(ctx, sp.email, "", hash, sp.name)
			if err != nil {
				return err
			}
			slog.Info("akun guru demo dibuat", "email", sp.email, "password", "guru12345")
		} else {
			if err := identityRepo.UpdateUserPassword(ctx, userID, hash); err != nil {
				return err
			}
			slog.Info("password guru demo disetel ulang (idempoten)", "email", sp.email, "password", "guru12345")
		}
		if err := identitySvc.CreateMembership(ctx, userID, schoolID, identity.RoleGuru); err != nil {
			return err
		}
		if _, err := repo.GetTeacherByUserID(ctx, schoolID, userID); err != nil {
			if !errors.Is(err, student.ErrNotFound) {
				return err
			}
			if _, err := repo.CreateTeacher(ctx, schoolID, userID, ""); err != nil {
				return err
			}
		}
	}
	return nil
}

// ensureDemoSubjects membuat 3 mapel contoh bila belum ada (idempoten by
// (school_id, code)).
func ensureDemoSubjects(ctx context.Context, repo *student.Repository, schoolID int64) error {
	specs := []struct{ code, name string }{
		{"BDT", "Basis Data"},
		{"PWB", "Pemrograman Web"},
		{"MTK", "Matematika"},
	}
	for _, sp := range specs {
		if _, err := repo.GetSubjectByCode(ctx, schoolID, sp.code); err != nil {
			if !errors.Is(err, student.ErrNotFound) {
				return err
			}
			if _, err := repo.CreateSubject(ctx, schoolID, sp.code, sp.name); err != nil {
				return err
			}
		}
	}
	return nil
}

type upsertUserInput struct {
	Email        string
	Username     string
	Name         string
	Password     string
	IsSuperAdmin bool
}

// upsertUser mencari user by email/username; buat baru bila belum ada,
// atau perbarui password/nama/flag super admin bila sudah ada (idempoten).
func upsertUser(ctx context.Context, repo *identity.Repository, in upsertUserInput) (int64, error) {
	hash, err := identity.HashPassword(in.Password)
	if err != nil {
		return 0, fmt.Errorf("hash password: %w", err)
	}

	var (
		existing identity.User
		findErr  error
	)
	if in.Email != "" {
		existing, findErr = repo.UserByEmail(ctx, in.Email)
	} else {
		existing, findErr = repo.UserByUsername(ctx, in.Username)
	}

	if findErr == nil {
		if err := repo.UpdateUserPassword(ctx, existing.ID, hash); err != nil {
			return 0, fmt.Errorf("update password: %w", err)
		}
		if err := repo.SetUserName(ctx, existing.ID, in.Name); err != nil {
			return 0, fmt.Errorf("update nama: %w", err)
		}
		if in.IsSuperAdmin && !existing.IsSuperAdmin {
			if err := repo.SetSuperAdmin(ctx, existing.ID, true); err != nil {
				return 0, fmt.Errorf("set super admin: %w", err)
			}
		}
		return existing.ID, nil
	}
	if !errors.Is(findErr, identity.ErrNotFound) {
		return 0, findErr
	}

	created, err := repo.CreateUser(ctx, identity.CreateUserInput{
		Email:        in.Email,
		Username:     in.Username,
		PasswordHash: hash,
		Name:         in.Name,
		IsSuperAdmin: in.IsSuperAdmin,
	})
	if err != nil {
		return 0, fmt.Errorf("buat user: %w", err)
	}
	return created.ID, nil
}

// ensureDemoSchool membuat sekolah slug "demo" bila belum ada.
func ensureDemoSchool(ctx context.Context, repo *tenant.Repository) (int64, error) {
	existing, err := repo.SchoolBySlugAny(ctx, "demo")
	if err == nil {
		return existing.ID, nil
	}
	if !errors.Is(err, tenant.ErrNotFound) {
		return 0, err
	}
	sch, err := repo.CreateSchool(ctx, "Sekolah Demo", "demo", "Asia/Jakarta")
	if err != nil {
		return 0, err
	}
	return sch.ID, nil
}

// ensureDemoAcademicYear membuat & mengaktifkan tahun ajaran 2026/2027 bila
// belum ada tahun ajaran aktif untuk sekolah demo.
func ensureDemoAcademicYear(ctx context.Context, repo *tenant.Repository, schoolID int64) error {
	years, err := repo.ListAcademicYears(ctx, schoolID)
	if err != nil {
		return err
	}
	for _, y := range years {
		if y.IsActive {
			return nil // sudah ada tahun ajaran aktif, tidak perlu apa-apa
		}
	}

	const name = "2026/2027"
	var target tenant.AcademicYear
	for _, y := range years {
		if y.Name == name {
			target = y
			break
		}
	}
	if target.ID == 0 {
		startsOn := time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)
		endsOn := time.Date(2027, time.June, 30, 0, 0, 0, 0, time.UTC)
		created, err := repo.CreateAcademicYear(ctx, schoolID, name, startsOn, endsOn, false)
		if err != nil {
			return err
		}
		target = created
	}

	_, err = repo.ActivateAcademicYear(ctx, target.ID, schoolID)
	return err
}
