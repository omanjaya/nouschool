package tenant

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/omanjaya/nouschool/internal/tenant/tenantdb"
)

// ErrNotFound menandai baris tidak ditemukan di repository modul tenant.
var ErrNotFound = errors.New("tenant: data tidak ditemukan")

// Repository membungkus akses data modul tenant (sqlc + pgx). Handler/service
// TIDAK pernah menyusun SQL sendiri — semua lewat sini.
type Repository struct {
	pool *pgxpool.Pool
	q    *tenantdb.Queries
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool, q: tenantdb.New(pool)}
}

func mapNoRows(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

func schoolFromDB(s tenantdb.School) School {
	return School{
		ID:           s.ID,
		Name:         s.Name,
		Slug:         s.Slug,
		CustomDomain: s.CustomDomain.String,
		Timezone:     s.Timezone,
		Status:       s.Status,
		CreatedAt:    s.CreatedAt.Time,
	}
}

func academicYearFromDB(a tenantdb.AcademicYear) AcademicYear {
	return AcademicYear{
		ID:       a.ID,
		SchoolID: a.SchoolID,
		Name:     a.Name,
		StartsOn: NewDate(a.StartsOn.Time),
		EndsOn:   NewDate(a.EndsOn.Time),
		IsActive: a.IsActive,
	}
}

func textOrNil(s string) pgtype.Text {
	if s == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: s, Valid: true}
}

// SchoolBySlug mencari sekolah aktif berdasarkan slug (subdomain default).
func (r *Repository) SchoolBySlug(ctx context.Context, slug string) (School, error) {
	row, err := r.q.GetSchoolBySlug(ctx, slug)
	if err != nil {
		return School{}, mapNoRows(err)
	}
	return schoolFromDB(row), nil
}

// SchoolByCustomDomain mencari sekolah aktif berdasarkan custom domain.
func (r *Repository) SchoolByCustomDomain(ctx context.Context, domain string) (School, error) {
	row, err := r.q.GetSchoolByCustomDomain(ctx, textOrNil(domain))
	if err != nil {
		return School{}, mapNoRows(err)
	}
	return schoolFromDB(row), nil
}

// SchoolBySlugAny mencari sekolah tanpa filter status (dipakai admin/bootstrap).
func (r *Repository) SchoolBySlugAny(ctx context.Context, slug string) (School, error) {
	row, err := r.q.GetSchoolBySlugAny(ctx, slug)
	if err != nil {
		return School{}, mapNoRows(err)
	}
	return schoolFromDB(row), nil
}

// SchoolByID mengambil sekolah apa pun statusnya berdasarkan id.
func (r *Repository) SchoolByID(ctx context.Context, id int64) (School, error) {
	row, err := r.q.GetSchoolByID(ctx, id)
	if err != nil {
		return School{}, mapNoRows(err)
	}
	return schoolFromDB(row), nil
}

func (r *Repository) CreateSchool(ctx context.Context, name, slug, timezone string) (School, error) {
	row, err := r.q.CreateSchool(ctx, tenantdb.CreateSchoolParams{Name: name, Slug: slug, Timezone: timezone})
	if err != nil {
		return School{}, err
	}
	return schoolFromDB(row), nil
}

// UpdateSchoolParams — field kosong ("") berarti tidak diubah (COALESCE di SQL).
type UpdateSchoolParams struct {
	ID           int64
	Name         string
	CustomDomain string
	Timezone     string
	Status       string
}

func (r *Repository) UpdateSchool(ctx context.Context, p UpdateSchoolParams) (School, error) {
	row, err := r.q.UpdateSchool(ctx, tenantdb.UpdateSchoolParams{
		ID:           p.ID,
		Name:         textOrNil(p.Name),
		CustomDomain: textOrNil(p.CustomDomain),
		Timezone:     textOrNil(p.Timezone),
		Status:       textOrNil(p.Status),
	})
	if err != nil {
		return School{}, mapNoRows(err)
	}
	return schoolFromDB(row), nil
}

func (r *Repository) ListSchools(ctx context.Context) ([]School, error) {
	rows, err := r.q.ListSchools(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]School, 0, len(rows))
	for _, row := range rows {
		out = append(out, schoolFromDB(row))
	}
	return out, nil
}

func (r *Repository) CreateAcademicYear(ctx context.Context, schoolID int64, name string, startsOn, endsOn time.Time, isActive bool) (AcademicYear, error) {
	row, err := r.q.CreateAcademicYear(ctx, tenantdb.CreateAcademicYearParams{
		SchoolID: schoolID,
		Name:     name,
		StartsOn: pgtype.Date{Time: startsOn, Valid: true},
		EndsOn:   pgtype.Date{Time: endsOn, Valid: true},
		IsActive: isActive,
	})
	if err != nil {
		return AcademicYear{}, err
	}
	return academicYearFromDB(row), nil
}

func (r *Repository) ListAcademicYears(ctx context.Context, schoolID int64) ([]AcademicYear, error) {
	rows, err := r.q.ListAcademicYears(ctx, schoolID)
	if err != nil {
		return nil, err
	}
	out := make([]AcademicYear, 0, len(rows))
	for _, row := range rows {
		out = append(out, academicYearFromDB(row))
	}
	return out, nil
}

func (r *Repository) AcademicYear(ctx context.Context, id, schoolID int64) (AcademicYear, error) {
	row, err := r.q.GetAcademicYear(ctx, tenantdb.GetAcademicYearParams{ID: id, SchoolID: schoolID})
	if err != nil {
		return AcademicYear{}, mapNoRows(err)
	}
	return academicYearFromDB(row), nil
}

// ActivateAcademicYear menonaktifkan semua tahun ajaran sekolah lalu
// mengaktifkan satu id target, dalam satu transaksi (exclusive is_active).
func (r *Repository) ActivateAcademicYear(ctx context.Context, id, schoolID int64) (AcademicYear, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return AcademicYear{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	qtx := r.q.WithTx(tx)
	if err := qtx.DeactivateAcademicYears(ctx, schoolID); err != nil {
		return AcademicYear{}, err
	}
	row, err := qtx.ActivateAcademicYear(ctx, tenantdb.ActivateAcademicYearParams{ID: id, SchoolID: schoolID})
	if err != nil {
		return AcademicYear{}, mapNoRows(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return AcademicYear{}, err
	}
	return academicYearFromDB(row), nil
}

// GetSetting mengembalikan settings mentah (jsonb) suatu modul, atau
// found=false bila belum ada baris tersimpan.
func (r *Repository) GetSetting(ctx context.Context, schoolID int64, module string) (raw []byte, found bool, err error) {
	row, err := r.q.GetSchoolSetting(ctx, tenantdb.GetSchoolSettingParams{SchoolID: schoolID, Module: module})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return row.Settings, true, nil
}

func (r *Repository) PutSetting(ctx context.Context, schoolID int64, module string, raw []byte, updatedBy int64) error {
	var by pgtype.Int8
	if updatedBy > 0 {
		by = pgtype.Int8{Int64: updatedBy, Valid: true}
	}
	_, err := r.q.UpsertSchoolSetting(ctx, tenantdb.UpsertSchoolSettingParams{
		SchoolID:  schoolID,
		Module:    module,
		Settings:  raw,
		UpdatedBy: by,
	})
	return err
}
