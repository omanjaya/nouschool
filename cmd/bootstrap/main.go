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
