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

	"github.com/omanjaya/nouschool/internal/identity"
	"github.com/omanjaya/nouschool/internal/platform/config"
	"github.com/omanjaya/nouschool/internal/platform/database"
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

	if err := ensureDemoTeachers(ctx, identitySvc, studentRepo, schoolID); err != nil {
		slog.Error("gagal menyiapkan guru demo", "err", err)
		os.Exit(1)
	}
	slog.Info("guru demo siap", "count", 2)

	if err := ensureDemoSubjects(ctx, studentRepo, schoolID); err != nil {
		slog.Error("gagal menyiapkan mapel demo", "err", err)
		os.Exit(1)
	}
	slog.Info("mapel demo siap", "count", 3)
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
// password contoh "guru12345" (bukan placeholder acak, supaya bisa dipakai
// login demo langsung tanpa lewat alur undangan).
func ensureDemoTeachers(ctx context.Context, identitySvc *identity.Service, repo *student.Repository, schoolID int64) error {
	specs := []struct{ name, email string }{
		{"Rendi Saputra", "rendi@demo.sch.id"},
		{"Sari Wulandari", "sari@demo.sch.id"},
	}
	for _, sp := range specs {
		userID, exists, err := identitySvc.UserIDByEmail(ctx, sp.email)
		if err != nil {
			return err
		}
		if !exists {
			hash, err := identitySvc.HashPassword("guru12345")
			if err != nil {
				return err
			}
			userID, err = identitySvc.CreateAccount(ctx, sp.email, "", hash, sp.name)
			if err != nil {
				return err
			}
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
