// Package discipline mengurus kedisiplinan siswa (Fase 14 Gelombang A, acuan
// perilaku SION — docs/12-sion-parity.md): master jenis pelanggaran+poin per
// sekolah, catat pelanggaran siswa (opsional tertaut satu sesi presensi, unik
// per sesi+siswa+jenis), ambang SP1/SP2/SP3 per tahun ajaran, dan surat
// peringatan yang TERBIT OTOMATIS begitu total poin siswa melewati ambang
// level tertinggi yang belum punya surat — snapshot IMMUTABLE (surat = potret
// data saat terbit, tidak pernah diperbarui walau catatan pelanggaran sumber
// diubah/dihapus setelahnya).
package discipline

import (
	"fmt"
	"strings"
	"time"
)

// Role kanonik dipakai modul discipline (nilai HARUS sama persis dengan
// internal/identity — didefinisikan ulang di sini karena discipline TIDAK
// boleh mengimpor identity, lihat CLAUDE.md).
const (
	RoleAdminSekolah  = "admin_sekolah"
	RoleKepalaSekolah = "kepala_sekolah"
	RoleGuru          = "guru"
)

// Permission kanonik dipakai modul discipline (lihat docs/02-identity.md).
const (
	PermDisciplineManage = "discipline:manage"
	PermDisciplineRecord = "discipline:record"
	PermDisciplineRead   = "discipline:read"
)

// Default ambang SP1/SP2/SP3 (docs tugas Fase 14 Gelombang A) — dikembalikan
// GET /api/discipline/sp-settings TANPA menulis baris ke DB bila sekolah
// belum pernah menyimpan settingnya sendiri (pola yang sama dengan
// leave.DefaultSettings/attendance.DefaultSettings).
const (
	DefaultSP1 = 25
	DefaultSP2 = 50
	DefaultSP3 = 75
)

// Date adalah tanggal murni (tanpa jam), dikodekan JSON sebagai "YYYY-MM-DD"
// — didefinisikan ulang di sini (sama seperti internal/leave.Date /
// internal/attendance.Date) supaya discipline tidak mengimpor modul lain
// untuk tipe ini.
type Date struct{ time.Time }

func NewDate(t time.Time) Date { return Date{t} }

func (d Date) MarshalJSON() ([]byte, error) {
	if d.IsZero() {
		return []byte("null"), nil
	}
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

// -- response shapes --

// ViolationType adalah shape response master jenis pelanggaran.
type ViolationType struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Points   int    `json:"points"`
	Category string `json:"category"`
	Active   bool   `json:"active"`
}

// SPSettings adalah shape response GET/PUT /api/discipline/sp-settings.
type SPSettings struct {
	SP1 int `json:"sp1"`
	SP2 int `json:"sp2"`
	SP3 int `json:"sp3"`
}

// StudentRef adalah potongan info siswa disematkan pada RecordView/SummaryRow.
type StudentRef struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	NIS       string `json:"nis"`
	ClassName string `json:"class_name"`
}

// ViolationRef adalah potongan info jenis pelanggaran disematkan pada RecordView.
type ViolationRef struct {
	ID     int64  `json:"id"`
	Name   string `json:"name"`
	Points int    `json:"points"`
}

// RecordView adalah shape satu baris catatan pelanggaran — dipakai
// GET /api/discipline/records dan bagian "records" GET /api/students/{id}/discipline.
type RecordView struct {
	ID                  int64        `json:"id"`
	Student             StudentRef   `json:"student"`
	Violation           ViolationRef `json:"violation"`
	NotedByName         string       `json:"noted_by_name"`
	Note                string       `json:"note"`
	OccurredOn          Date         `json:"occurred_on"`
	AttendanceSessionID *int64       `json:"attendance_session_id"`
}

// RecordPage adalah shape response GET /api/discipline/records.
type RecordPage struct {
	Items []RecordView `json:"items"`
	Total int64        `json:"total"`
}

// IssuedLetterRef adalah potongan info surat disematkan pada response
// POST /api/discipline/records (null bila tidak ada surat yang terbit).
type IssuedLetterRef struct {
	ID     int64  `json:"id"`
	Level  int    `json:"level"`
	Number string `json:"number"`
}

// RecordResult adalah shape response POST /api/discipline/records.
type RecordResult struct {
	Record       RecordView       `json:"record"`
	TotalPoints  int              `json:"total_points"`
	IssuedLetter *IssuedLetterRef `json:"issued_letter"`
}

// LetterView adalah shape satu baris surat peringatan (list & bagian
// "letters" GET /api/students/{id}/discipline).
type LetterView struct {
	ID             int64     `json:"id"`
	Level          int       `json:"level"`
	Number         string    `json:"number"`
	PointsSnapshot int       `json:"points_snapshot"`
	CreatedAt      time.Time `json:"created_at"`
	// Student HANYA terisi di GET /api/discipline/letters (daftar lintas
	// siswa) — kosong (zero value) di bagian "letters" GET
	// /api/students/{id}/discipline karena sudah jelas milik siapa.
	Student *StudentRef `json:"student,omitempty"`
}

// LetterPage adalah shape response GET /api/discipline/letters.
type LetterPage struct {
	Items []LetterView `json:"items"`
	Total int64        `json:"total"`
}

// StudentDisciplineView adalah shape response GET /api/students/{id}/discipline.
type StudentDisciplineView struct {
	TotalPoints   int          `json:"total_points"`
	SPLevelReached int         `json:"sp_level_reached"` // 0 = belum ada level SP yang terlampaui
	Records       []RecordView `json:"records"`
	Letters       []LetterView `json:"letters"`
}

// SummaryRow adalah shape satu baris response GET /api/discipline/summary.
type SummaryRow struct {
	Student     StudentRef `json:"student"`
	TotalPoints int        `json:"total_points"`
	RecordCount int        `json:"record_count"`
	LastAt      *Date      `json:"last_at"`
}

// -- snapshot surat (jsonb, IMMUTABLE — lihat Service.issueLetterIfDue) --

// LetterSnapshotViolation adalah satu baris pelanggaran di dalam snapshot surat.
type LetterSnapshotViolation struct {
	Name   string `json:"name"`
	Points int    `json:"points"`
	Date   string `json:"date"` // YYYY-MM-DD
}

// LetterSnapshotStudent adalah identitas siswa di dalam snapshot surat.
type LetterSnapshotStudent struct {
	Name      string `json:"name"`
	NIS       string `json:"nis"`
	ClassName string `json:"class_name"`
}

// LetterSnapshot adalah struktur LENGKAP kolom discipline_warning_letters.snapshot
// — potret data saat surat terbit, dirender ulang apa adanya oleh PDF/HTML
// (TIDAK PERNAH query ulang ke data live, lihat docs tugas: "surat = potret").
type LetterSnapshot struct {
	Student    LetterSnapshotStudent     `json:"student"`
	Violations []LetterSnapshotViolation `json:"violations"`
	Total      int                       `json:"total"`
	Threshold  int                       `json:"threshold"`
	SchoolName string                    `json:"school_name"`
	IssuedOn   string                    `json:"issued_on"` // YYYY-MM-DD
}
