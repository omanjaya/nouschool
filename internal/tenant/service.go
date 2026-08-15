package tenant

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/omanjaya/nouschool/internal/platform/httpx"
)

// AuditLogger adalah interface kecil sisi pemakai untuk mencatat audit_log.
// Diimplementasikan oleh internal/identity.Service dan di-inject dari main.go
// (lihat aturan "antar-modul lewat interface kecil" di CLAUDE.md).
type AuditLogger interface {
	Log(ctx context.Context, schoolID, userID *int64, action, entity string, entityID *int64, oldValue, newValue any) error
}

// Service berisi aturan bisnis modul tenant: CRUD sekolah, tahun ajaran.
type Service struct {
	repo  *Repository
	audit AuditLogger
}

func NewService(repo *Repository, audit AuditLogger) *Service {
	return &Service{repo: repo, audit: audit}
}

var slugRe = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,38}[a-z0-9])?$`)

func validTimezone(tz string) bool {
	switch tz {
	case "Asia/Jakarta", "Asia/Makassar", "Asia/Jayapura":
		return true
	default:
		_, err := time.LoadLocation(tz)
		return err == nil
	}
}

func i64p(v int64) *int64 { return &v }

// CreateSchool membuat sekolah baru. Dipanggil dari panel super admin.
func (s *Service) CreateSchool(ctx context.Context, actorUserID int64, name, slug, timezone string) (School, error) {
	name = strings.TrimSpace(name)
	slug = strings.ToLower(strings.TrimSpace(slug))
	if name == "" {
		return School{}, httpx.Validation("Nama sekolah wajib diisi.")
	}
	if !slugRe.MatchString(slug) {
		return School{}, httpx.Validation("Slug hanya boleh huruf kecil, angka, dan tanda hubung (2-40 karakter).")
	}
	if timezone == "" {
		timezone = "Asia/Jakarta"
	}
	if !validTimezone(timezone) {
		return School{}, httpx.Validation("Zona waktu tidak valid.")
	}
	if _, err := s.repo.SchoolBySlugAny(ctx, slug); err == nil {
		return School{}, httpx.Validation("Slug sudah dipakai sekolah lain.")
	} else if !errors.Is(err, ErrNotFound) {
		return School{}, err
	}

	sch, err := s.repo.CreateSchool(ctx, name, slug, timezone)
	if err != nil {
		return School{}, err
	}
	if s.audit != nil {
		_ = s.audit.Log(ctx, &sch.ID, i64p(actorUserID), "school.create", "school", &sch.ID, nil, sch)
	}
	return sch, nil
}

// UpdateSchoolInput — field kosong ("") berarti tidak diubah.
type UpdateSchoolInput struct {
	Name         string
	CustomDomain string
	Timezone     string
	Status       string
}

// UpdateSchool memperbarui sebagian field sekolah (PATCH semantics).
func (s *Service) UpdateSchool(ctx context.Context, actorUserID, id int64, in UpdateSchoolInput) (School, error) {
	before, err := s.repo.SchoolByID(ctx, id)
	if err != nil {
		return School{}, err
	}
	if in.Timezone != "" && !validTimezone(in.Timezone) {
		return School{}, httpx.Validation("Zona waktu tidak valid.")
	}
	if in.Status != "" && in.Status != "active" && in.Status != "suspended" {
		return School{}, httpx.Validation("Status sekolah harus 'active' atau 'suspended'.")
	}
	after, err := s.repo.UpdateSchool(ctx, UpdateSchoolParams{
		ID:           id,
		Name:         in.Name,
		CustomDomain: in.CustomDomain,
		Timezone:     in.Timezone,
		Status:       in.Status,
	})
	if err != nil {
		return School{}, err
	}
	if s.audit != nil {
		_ = s.audit.Log(ctx, &after.ID, i64p(actorUserID), "school.update", "school", &after.ID, before, after)
	}
	return after, nil
}

func (s *Service) ListSchools(ctx context.Context) ([]School, error) {
	return s.repo.ListSchools(ctx)
}

func (s *Service) SchoolByID(ctx context.Context, id int64) (School, error) {
	return s.repo.SchoolByID(ctx, id)
}

// CreateAcademicYear menambah tahun ajaran baru (tidak otomatis aktif —
// aktivasi eksplisit lewat ActivateAcademicYear).
func (s *Service) CreateAcademicYear(ctx context.Context, actorUserID, schoolID int64, name string, startsOn, endsOn time.Time) (AcademicYear, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return AcademicYear{}, httpx.Validation("Nama tahun ajaran wajib diisi, contoh 2026/2027.")
	}
	if !endsOn.After(startsOn) {
		return AcademicYear{}, httpx.Validation("Tanggal selesai harus setelah tanggal mulai.")
	}
	ay, err := s.repo.CreateAcademicYear(ctx, schoolID, name, startsOn, endsOn, false)
	if err != nil {
		return AcademicYear{}, err
	}
	if s.audit != nil {
		_ = s.audit.Log(ctx, &schoolID, i64p(actorUserID), "academic_year.create", "academic_year", &ay.ID, nil, ay)
	}
	return ay, nil
}

func (s *Service) ListAcademicYears(ctx context.Context, schoolID int64) ([]AcademicYear, error) {
	return s.repo.ListAcademicYears(ctx, schoolID)
}

// ActiveAcademicYearID mengembalikan (id, true, nil) bila sekolah punya tahun
// ajaran aktif, (0, false, nil) bila belum ada. Signature primitif ini
// sengaja dipakai supaya modul lain (mis. student) bisa mendeklarasikan
// consumer-side interface kecil tanpa mengimpor package tenant untuk tipe
// AcademicYear (lihat aturan modular monolith di CLAUDE.md).
func (s *Service) ActiveAcademicYearID(ctx context.Context, schoolID int64) (int64, bool, error) {
	ay, err := s.repo.ActiveAcademicYear(ctx, schoolID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return 0, false, nil
		}
		return 0, false, err
	}
	return ay.ID, true, nil
}

// ActivateAcademicYear mengaktifkan satu tahun ajaran secara eksklusif
// (menonaktifkan yang lain) dalam satu transaksi.
func (s *Service) ActivateAcademicYear(ctx context.Context, actorUserID, schoolID, id int64) (AcademicYear, error) {
	if _, err := s.repo.AcademicYear(ctx, id, schoolID); err != nil {
		return AcademicYear{}, err
	}
	ay, err := s.repo.ActivateAcademicYear(ctx, id, schoolID)
	if err != nil {
		return AcademicYear{}, err
	}
	if s.audit != nil {
		_ = s.audit.Log(ctx, &schoolID, i64p(actorUserID), "academic_year.activate", "academic_year", &ay.ID, nil, ay)
	}
	return ay, nil
}
