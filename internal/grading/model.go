// Package grading mengurus penilaian (Fase 14 Gelombang C, docs/12-sion-parity.md):
// komponen penilaian berbobot+KKTP per kelas+mapel, input nilai, nilai akhir
// dinormalisasi (Σ score*weight / Σ weight komponen TERISI — BUKAN dibagi
// 100), publikasi nilai per kelas-mapel, dan bintang kelas (poin
// non-akademik). Modul TOGGLE per sekolah (school_settings module
// "grading") — nonaktif = 404 grading_disabled di SELURUH endpoint kecuali
// GET /api/grading/status.
package grading

import "time"

// Role kanonik dipakai modul grading (nilai HARUS sama persis dengan
// internal/identity — didefinisikan ulang di sini karena grading TIDAK
// boleh mengimpor identity, lihat CLAUDE.md).
const (
	RoleAdminSekolah  = "admin_sekolah"
	RoleKepalaSekolah = "kepala_sekolah"
	RoleGuru          = "guru"
	RoleSiswa         = "siswa"
	RoleOrangTua      = "orang_tua"
)

// Permission kanonik dipakai modul grading (lihat docs/02-identity.md).
const PermGradingManage = "grading:manage"

// PermGradingRead — Fase 15 GAP 5 (docs tugas): dipegang kepala_sekolah,
// membaca SEMUA kelas-mapel (lewati object-level guru) TANPA mutasi.
// Konstanta identity.PermGradingRead didaftarkan agent lain di rbac.go
// SECARA PARALEL — grading TIDAK boleh mengimpor identity (lihat CLAUDE.md),
// jadi nilai string di-hardcode literal di sini, HARUS SAMA PERSIS dengan
// rbac.go: "grading:read".
const PermGradingRead = "grading:read"

// Tipe komponen penilaian kanonik (migrations/00017_grading.sql CHECK).
const (
	ComponentTP      = "tp"
	ComponentSumatif = "sumatif"
	ComponentPraktik = "praktik"
	ComponentLainnya = "lainnya"
)

func validComponentType(t string) bool {
	switch t {
	case ComponentTP, ComponentSumatif, ComponentPraktik, ComponentLainnya:
		return true
	default:
		return false
	}
}

// Visibility kanonik classroom_star_events.visibility.
const (
	VisibilityPrivate = "private"
	VisibilityStudent = "student"
)

func validVisibility(v string) bool {
	return v == VisibilityPrivate || v == VisibilityStudent
}

// -- response shapes --

// StatusView adalah shape response GET /api/grading/status.
type StatusView struct {
	Enabled bool `json:"enabled"`
}

// ComponentView adalah shape satu baris response GET /api/grading/components
// dan hasil POST/PATCH satu komponen.
type ComponentView struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	Type         string `json:"type"`
	Weight       int    `json:"weight"`
	Kktp         int    `json:"kktp"`
	GradedCount  int64  `json:"graded_count"`
	StudentCount int64  `json:"student_count"`
}

// ComponentsPage adalah shape response GET /api/grading/components.
type ComponentsPage struct {
	Items       []ComponentView `json:"items"`
	TotalWeight int             `json:"total_weight"`
}

// GradeStudentRow adalah shape satu baris response
// GET /api/grading/components/{id}/grades.
type GradeStudentRow struct {
	StudentID int64    `json:"student_id"`
	Name      string   `json:"name"`
	NIS       string   `json:"nis"`
	Score     *float64 `json:"score"`
}

// ComponentGradesView adalah shape response GET /api/grading/components/{id}/grades.
type ComponentGradesView struct {
	Students []GradeStudentRow `json:"students"`
}

// -- recap --

// RecapComponentRef adalah potongan info komponen disematkan pada RecapView.
type RecapComponentRef struct {
	ID     int64  `json:"id"`
	Name   string `json:"name"`
	Type   string `json:"type"`
	Weight int    `json:"weight"`
}

// RecapStudentRow adalah shape satu baris siswa pada GET /api/grading/recap.
// Scores memakai key component_id (encoding/json mendukung map dengan key
// bertipe integer, dirender sebagai string angka — cocok dengan kontrak
// tugas "scores:{[component_id]:score|null}").
type RecapStudentRow struct {
	StudentID int64              `json:"student_id"`
	Name      string             `json:"name"`
	NIS       string             `json:"nis"`
	Scores    map[int64]*float64 `json:"scores"`
	Final     *float64           `json:"final"`
	Label     *string            `json:"label"`
	BelowKktp []int64            `json:"below_kktp"`
}

// RecapView adalah shape response GET /api/grading/recap.
type RecapView struct {
	Components []RecapComponentRef `json:"components"`
	Students   []RecapStudentRow   `json:"students"`
	Published  bool                `json:"published"`
}

// -- my-grades --

// SubjectRef adalah potongan info mapel disematkan pada SubjectGradeView.
type SubjectRef struct {
	ID   int64  `json:"id"`
	Code string `json:"code"`
	Name string `json:"name"`
}

// MyGradeComponent adalah satu baris komponen pada SubjectGradeView.
type MyGradeComponent struct {
	Name   string   `json:"name"`
	Type   string   `json:"type"`
	Weight int      `json:"weight"`
	Score  *float64 `json:"score"`
	Kktp   int      `json:"kktp"`
}

// SubjectGradeView adalah shape satu baris response GET /api/my-grades.
type SubjectGradeView struct {
	Subject    SubjectRef         `json:"subject"`
	Final      *float64           `json:"final"`
	Label      *string            `json:"label"`
	Components []MyGradeComponent `json:"components"`
}

// MyGradesView adalah shape response GET /api/my-grades.
type MyGradesView struct {
	Subjects []SubjectGradeView `json:"subjects"`
}

// -- stars --

// StarItem adalah shape satu baris riwayat bintang (GET /api/grading/stars
// dgn student_id, dan GET /api/my-stars).
type StarItem struct {
	ID          int64     `json:"id,omitempty"`
	Delta       int       `json:"delta"`
	Note        string    `json:"note"`
	Visibility  string    `json:"visibility,omitempty"`
	GivenByName string    `json:"given_by_name"`
	CreatedAt   time.Time `json:"created_at"`
}

// StarStudentTotal adalah shape satu baris GET /api/grading/stars?class_id=.
type StarStudentTotal struct {
	StudentID int64  `json:"student_id"`
	Name      string `json:"name"`
	NIS       string `json:"nis"`
	Total     int    `json:"total"`
}

// StarResult adalah shape response POST /api/grading/stars.
type StarResult struct {
	ID    int64 `json:"id"`
	Delta int   `json:"delta"`
	Total int   `json:"total"`
}

// MyStarsView adalah shape response GET /api/my-stars.
type MyStarsView struct {
	Total int        `json:"total"`
	Items []StarItem `json:"items"`
}

// -- report (Fase 15 GAP 1 rapor lanjutan) --

// Kind kanonik report_manual_scores.kind (migrations/00023_report_config.sql CHECK).
const (
	ReportScoreKindPrevious = "previous"
	ReportScoreKindManual   = "manual"
)

func validReportScoreKind(k string) bool {
	return k == ReportScoreKindPrevious || k == ReportScoreKindManual
}

// TPMappingRow adalah shape satu baris response GET /api/grading/report/tp-mappings
// — SATU per komponen tipe 'tp' pada (class,subject), tp_code/description ""
// bila belum dipetakan.
type TPMappingRow struct {
	ComponentID   int64  `json:"component_id"`
	ComponentName string `json:"component_name"`
	TPCode        string `json:"tp_code"`
	Description   string `json:"description"`
}

// TPMappingsView adalah shape response GET /api/grading/report/tp-mappings.
type TPMappingsView struct {
	Items []TPMappingRow `json:"items"`
}

// TPMappingInput adalah satu baris parameter Service.PutTPMappings.
type TPMappingInput struct {
	ComponentID int64
	TPCode      string
	Description string
}

// ManualScoreDetail adalah potongan {score,note} disematkan pada ReportScoreRow.Manual.
type ManualScoreDetail struct {
	Score float64 `json:"score"`
	Note  string  `json:"note"`
}

// ReportScoreRow adalah shape satu baris response GET /api/grading/report/manual-scores.
type ReportScoreRow struct {
	StudentID     int64              `json:"student_id"`
	Name          string             `json:"name"`
	NIS           string             `json:"nis"`
	Previous      *float64           `json:"previous"`
	Manual        *ManualScoreDetail `json:"manual"`
	ComputedFinal *float64           `json:"computed_final"`
}

// ReportScoresView adalah shape response GET /api/grading/report/manual-scores.
type ReportScoresView struct {
	Items []ReportScoreRow `json:"items"`
}

// ManualScoreEntryInput adalah satu baris parameter Service.PutManualScores —
// Score nil = hapus baris (kind) siswa itu (docs tugas: "null = hapus").
type ManualScoreEntryInput struct {
	StudentID int64
	Kind      string
	Score     *float64
	Note      string
}

// LabelCount adalah satu baris agregasi per label pada ReportAnalysisView.
type LabelCount struct {
	Label string `json:"label"`
	Count int    `json:"count"`
}

// BelowKktpComponentCount adalah satu baris agregasi "di bawah KKTP" per
// komponen pada ReportAnalysisView.
type BelowKktpComponentCount struct {
	ComponentID int64  `json:"component_id"`
	Name        string `json:"name"`
	Count       int    `json:"count"`
}

// ResolvedSourceCount adalah shape ReportAnalysisView.ResolvedSource — jumlah
// siswa per SUMBER nilai rapor ter-resolusi (manual menang atas computed;
// none = tidak ada nilai sama sekali).
type ResolvedSourceCount struct {
	Computed int `json:"computed"`
	Manual   int `json:"manual"`
	None     int `json:"none"`
}

// ReportAnalysisView adalah shape response GET /api/grading/report/analysis.
type ReportAnalysisView struct {
	Avg                   float64                   `json:"avg"`
	Min                   float64                   `json:"min"`
	Max                   float64                   `json:"max"`
	Count                 int                       `json:"count"`
	PerLabel              []LabelCount              `json:"per_label"`
	BelowKktpPerComponent []BelowKktpComponentCount `json:"below_kktp_per_component"`
	ResolvedSource        ResolvedSourceCount       `json:"resolved_source"`
}
