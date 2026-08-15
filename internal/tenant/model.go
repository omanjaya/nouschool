// Package tenant mengurus sekolah, resolusi domain, tahun ajaran, dan
// school_settings (lihat docs/01-tenant.md).
package tenant

import (
	"fmt"
	"strings"
	"time"
)

// Date adalah tanggal murni (tanpa jam) untuk field seperti starts_on/ends_on,
// dikodekan JSON sebagai "YYYY-MM-DD".
type Date struct{ time.Time }

func NewDate(t time.Time) Date { return Date{t} }

func (d Date) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf("%q", d.Format("2006-01-02"))), nil
}

func (d *Date) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	if s == "" || s == "null" {
		return nil
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return fmt.Errorf("format tanggal harus YYYY-MM-DD: %w", err)
	}
	d.Time = t
	return nil
}

// School adalah representasi domain dari baris tabel schools.
type School struct {
	ID           int64     `json:"id"`
	Name         string    `json:"name"`
	Slug         string    `json:"slug"`
	CustomDomain string    `json:"custom_domain,omitempty"` // "" bila belum pakai custom domain
	Timezone     string    `json:"timezone"`
	Status       string    `json:"status"` // active|suspended
	CreatedAt    time.Time `json:"created_at"`
}

// AcademicYear adalah representasi domain dari academic_years.
type AcademicYear struct {
	ID       int64  `json:"id"`
	SchoolID int64  `json:"school_id"`
	Name     string `json:"name"`
	StartsOn Date   `json:"starts_on"`
	EndsOn   Date   `json:"ends_on"`
	IsActive bool   `json:"is_active"`
}

// ResolvedHost adalah hasil resolusi Host header oleh HostResolver.
type ResolvedHost struct {
	IsPlatform bool
	School     School // kosong bila IsPlatform true
}
