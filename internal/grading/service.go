package grading

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"strings"

	"github.com/omanjaya/nouschool/internal/platform/clock"
	"github.com/omanjaya/nouschool/internal/platform/httpx"
	"github.com/omanjaya/nouschool/internal/platform/reqctx"
)

// -- consumer-side interface (lihat CLAUDE.md: antar-modul lewat interface
// kecil dideklarasikan di sisi PEMAKAI, di-inject lewat constructor di
// main.go). Semua signature sengaja hanya memakai tipe primitif supaya
// *identity.Service, *tenant.Service/*tenant.Repository, *student.Service,
// dan *schedule.Service memenuhi interface ini secara struktural TANPA
// grading perlu mengimpor package-package itu.

// IdentityGateway adalah kebutuhan modul grading dari modul identity:
// gerbang permission & audit log.
type IdentityGateway interface {
	HasPermission(role, perm string) bool
	Log(ctx context.Context, schoolID, userID *int64, action, entity string, entityID *int64, oldValue, newValue any) error
}

// AcademicYearLookup adalah kebutuhan modul grading dari modul tenant: tahun
// ajaran aktif (komponen/publikasi/bintang dianchor ke TA aktif).
type AcademicYearLookup interface {
	ActiveAcademicYearID(ctx context.Context, schoolID int64) (id int64, ok bool, err error)
}

// SettingsGateway adalah kebutuhan modul grading dari modul tenant: baca
// RAW json school_settings module "grading" (BUKAN lewat tenant.Settings —
// parameter bertipe interface lintas modul tidak akan match struktural,
// lihat CLAUDE.md "consumer-side interface hanya primitif"). Dipenuhi
// *tenant.Repository secara struktural (method GetSetting sudah ada sejak
// Fase 1, dipakai internal oleh tenant.SettingsService.Get).
type SettingsGateway interface {
	GetSetting(ctx context.Context, schoolID int64, module string) (raw []byte, found bool, err error)
}

// StudentAccess adalah kebutuhan modul grading dari modul student:
// object-level check (siswa lihat dirinya sendiri, orang tua lihat anaknya),
// resolusi profil siswa dari user login, dan penerima notifikasi ortu.
type StudentAccess interface {
	CanViewStudent(ctx context.Context, userID int64, role string, schoolID, studentID int64) error
	MyStudentID(ctx context.Context, schoolID, userID int64) (studentID, classID int64, ok bool, err error)
	GuardianUserIDs(ctx context.Context, schoolID, studentID int64) ([]int64, error)
}

// ScheduleGateway adalah kebutuhan modul grading dari modul schedule:
// object-level guru — guru hanya boleh kelola komponen/nilai/publikasi
// untuk (class_id, subject_id) yang dia punya slot jadwal TA aktif
// (TeachesClassSubject), dan bintang kelas untuk kelas yang dia ajar mapel
// APA PUN (TeachesClass, lebih longgar — docs tugas "guru punya slot di
// KELAS siswa (mapel apa pun) -> boleh").
type ScheduleGateway interface {
	TeachesClassSubject(ctx context.Context, schoolID, teacherUserID, classID, subjectID int64) (bool, error)
	TeachesClass(ctx context.Context, schoolID, teacherUserID, classID int64) (bool, error)
}

// Event kanonik — nilai HARUS sama persis dengan notification.EventGradingPublished
// (didefinisikan ulang di sini karena grading TIDAK boleh mengimpor
// notification, lihat CLAUDE.md).
const EventGradingPublished = "grading.published"

// Notifier adalah kebutuhan modul grading dari modul notification.
type Notifier interface {
	Notify(ctx context.Context, schoolID int64, event string, userIDs []int64, data map[string]any) error
}

// Realtime adalah kebutuhan modul grading dari modul realtime — TIDAK
// dipenuhi *realtime.Hub secara langsung, dijembatani adapter kecil di
// cmd/server (pola sama modul lain).
type Realtime interface {
	PublishTo(schoolID int64, eventType string, data map[string]any, roles []string, userIDs []int64)
}

// ErrNoActiveAcademicYear — sekolah belum mengaktifkan tahun ajaran.
var ErrNoActiveAcademicYear = httpx.Validation("Sekolah belum punya tahun ajaran aktif. Aktifkan dulu di menu tahun ajaran.")

// errGradingDisabled — 404 KHUSUS (code "grading_disabled") dikembalikan
// SELURUH endpoint grading lain saat module settings enabled=false (docs
// tugas: "SEMUA endpoint grading lain -> 404 {code:grading_disabled} bila
// off, guard di service"). GET /api/grading/status (Service.Status)
// SENGAJA TIDAK memanggil requireEnabled — endpoint itu justru dipakai
// klien mengecek nyala/tidaknya modul.
var errGradingDisabled = &httpx.Error{Status: http.StatusNotFound, Code: "grading_disabled", Message: "Modul penilaian belum diaktifkan sekolah ini."}

// Service berisi aturan bisnis modul grading.
type Service struct {
	repo     gradingRepository
	identity IdentityGateway
	years    AcademicYearLookup
	settings SettingsGateway
	students StudentAccess
	schedule ScheduleGateway
	clock    clock.Clock
	notifier Notifier // opsional — nil = notifikasi dilewati
	realtime Realtime // opsional — nil = event realtime dilewati
}

func NewService(repo *Repository, identity IdentityGateway, years AcademicYearLookup, settings SettingsGateway, students StudentAccess, schedule ScheduleGateway, clk clock.Clock) *Service {
	if clk == nil {
		clk = clock.System{}
	}
	return &Service{repo: repo, identity: identity, years: years, settings: settings, students: students, schedule: schedule, clock: clk}
}

// SetNotifier / SetRealtime menyuntikkan dependency opsional SETELAH
// konstruksi (setter, pola yang sama dengan internal/leave & internal/discipline).
func (s *Service) SetNotifier(n Notifier) { s.notifier = n }
func (s *Service) SetRealtime(r Realtime) { s.realtime = r }

// newServiceForTest membangun Service dengan repository FAKE (in-memory,
// tanpa DB) — dipakai test di package ini saja (service_test.go).
func newServiceForTest(repo gradingRepository, identity IdentityGateway, years AcademicYearLookup, settings SettingsGateway, students StudentAccess, schedule ScheduleGateway, clk clock.Clock) *Service {
	return &Service{repo: repo, identity: identity, years: years, settings: settings, students: students, schedule: schedule, clock: clk}
}

func (s *Service) audit(ctx context.Context, schoolID, actorUserID int64, action, entity string, entityID int64, oldValue, newValue any) {
	sid, uid, eid := schoolID, actorUserID, entityID
	_ = s.identity.Log(ctx, &sid, &uid, action, entity, &eid, oldValue, newValue)
}

func (s *Service) requireActiveYear(ctx context.Context, schoolID int64) (int64, error) {
	id, ok, err := s.years.ActiveAcademicYearID(ctx, schoolID)
	if err != nil {
		return 0, err
	}
	if !ok {
		return 0, ErrNoActiveAcademicYear
	}
	return id, nil
}

// loadSettings membaca school_settings module "grading" (default bila belum
// pernah disimpan sekolah — pola yang sama dengan
// tenant.SettingsService.Get, tapi diimplementasikan lokal karena grading
// TIDAK bisa memenuhi parameter bertipe tenant.Settings, lihat catatan di
// SettingsGateway).
func (s *Service) loadSettings(ctx context.Context, schoolID int64) (Settings, error) {
	out := DefaultSettings()
	raw, found, err := s.settings.GetSetting(ctx, schoolID, "grading")
	if err != nil {
		return Settings{}, err
	}
	if !found {
		return out, nil
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return Settings{}, err
	}
	return out, nil
}

func (s *Service) requireEnabled(ctx context.Context, schoolID int64) (Settings, error) {
	settings, err := s.loadSettings(ctx, schoolID)
	if err != nil {
		return Settings{}, err
	}
	if !settings.Enabled {
		return Settings{}, errGradingDisabled
	}
	return settings, nil
}

func (s *Service) requireManage(ctx context.Context) error {
	if !s.identity.HasPermission(reqctx.Role(ctx), PermGradingManage) {
		return httpx.ErrForbidden
	}
	return nil
}

// requireClassSubjectAccess menegakkan object-level guru (docs tugas): admin
// bebas; guru HANYA boleh (class_id, subject_id) yang dia punya slot jadwal
// TA aktif (ScheduleGateway.TeachesClassSubject); role lain (mestinya sudah
// tersaring requirePerm grading:manage di routes.go, HANYA admin+guru yang
// punya perm itu) ditolak sebagai jaga-jaga.
func (s *Service) requireClassSubjectAccess(ctx context.Context, schoolID, classID, subjectID int64) error {
	role := reqctx.Role(ctx)
	if role == RoleAdminSekolah {
		return nil
	}
	if role != RoleGuru {
		return httpx.ErrForbidden
	}
	ok, err := s.schedule.TeachesClassSubject(ctx, schoolID, reqctx.UserID(ctx), classID, subjectID)
	if err != nil {
		return err
	}
	if !ok {
		return httpx.ErrForbidden
	}
	return nil
}

// requireClassAccess adalah versi LEBIH LONGGAR requireClassSubjectAccess —
// dipakai bintang kelas (docs tugas: "guru punya slot di KELAS siswa (mapel
// apa pun) -> boleh").
func (s *Service) requireClassAccess(ctx context.Context, schoolID, classID int64) error {
	role := reqctx.Role(ctx)
	if role == RoleAdminSekolah {
		return nil
	}
	if role != RoleGuru {
		return httpx.ErrForbidden
	}
	ok, err := s.schedule.TeachesClass(ctx, schoolID, reqctx.UserID(ctx), classID)
	if err != nil {
		return err
	}
	if !ok {
		return httpx.ErrForbidden
	}
	return nil
}

func (s *Service) notifierNotify(ctx context.Context, schoolID int64, event string, userIDs []int64, data map[string]any) {
	if s.notifier == nil || len(userIDs) == 0 {
		return
	}
	_ = s.notifier.Notify(ctx, schoolID, event, userIDs, data)
}

func (s *Service) emitGrading(schoolID, classID int64) {
	if s.realtime == nil {
		return
	}
	// Keputusan desain (dilaporkan): broadcast RINGAN by-role saja (BUKAN
	// menghimpun user_id siswa+ortu kelas satu per satu) — docs tugas
	// eksplisit menyarankan "cukup broadcast sekolah — ringan".
	s.realtime.PublishTo(schoolID, "grading", map[string]any{"class_id": classID},
		[]string{RoleAdminSekolah, RoleKepalaSekolah, RoleGuru}, nil)
}

// roundHalfUp membulatkan float64 ke int, .5 ke atas (docs tugas: "bulatkan
// half-up ke int utk pencocokan label").
func roundHalfUp(v float64) int {
	return int(math.Floor(v + 0.5))
}

// computeFinal menghitung nilai akhir dinormalisasi HANYA dari komponen yang
// ADA di scores (docs/12-sion-parity.md: "nilai akhir = Σ(score*weight
// terisi) / Σ(weight terisi)") — nil bila TIDAK ADA satu pun komponen
// terisi. belowKktp = id komponen yang score-nya < kktp komponen itu (HANYA
// dievaluasi untuk komponen yang punya score).
func computeFinal(components []ComponentRecord, scores map[int64]float64) (final *float64, belowKktp []int64) {
	var sumScoreWeight, sumWeight float64
	for _, c := range components {
		score, ok := scores[c.ID]
		if !ok {
			continue
		}
		sumScoreWeight += score * float64(c.Weight)
		sumWeight += float64(c.Weight)
		if score < float64(c.Kktp) {
			belowKktp = append(belowKktp, c.ID)
		}
	}
	if sumWeight <= 0 {
		return nil, belowKktp
	}
	f := sumScoreWeight / sumWeight
	return &f, belowKktp
}

func ptrFloat(v float64) *float64 { return &v }

// -- GET /api/grading/status --

func (s *Service) Status(ctx context.Context, schoolID int64) (StatusView, error) {
	settings, err := s.loadSettings(ctx, schoolID)
	if err != nil {
		return StatusView{}, err
	}
	return StatusView{Enabled: settings.Enabled}, nil
}

// -- GET/POST/PATCH/DELETE /api/grading/components --

// ListComponentsQuery adalah parameter Service.ListComponents.
type ListComponentsQuery struct {
	ClassID   int64
	SubjectID int64
}

func (s *Service) ListComponents(ctx context.Context, schoolID int64, q ListComponentsQuery) (ComponentsPage, error) {
	if _, err := s.requireEnabled(ctx, schoolID); err != nil {
		return ComponentsPage{}, err
	}
	if err := s.requireManage(ctx); err != nil {
		return ComponentsPage{}, err
	}
	if q.ClassID == 0 || q.SubjectID == 0 {
		return ComponentsPage{}, httpx.Validation("class_id dan subject_id wajib diisi.")
	}
	if err := s.requireClassSubjectAccess(ctx, schoolID, q.ClassID, q.SubjectID); err != nil {
		return ComponentsPage{}, err
	}
	rows, err := s.repo.ListComponentsByClassSubject(ctx, schoolID, q.ClassID, q.SubjectID)
	if err != nil {
		return ComponentsPage{}, err
	}
	items := make([]ComponentView, 0, len(rows))
	totalWeight := 0
	for _, r := range rows {
		items = append(items, ComponentView{ID: r.ID, Name: r.Name, Type: r.Type, Weight: r.Weight, Kktp: r.Kktp, GradedCount: r.GradedCount, StudentCount: r.StudentCount})
		totalWeight += r.Weight
	}
	return ComponentsPage{Items: items, TotalWeight: totalWeight}, nil
}

// ComponentCreateInput adalah parameter Service.CreateComponent.
type ComponentCreateInput struct {
	ClassID   int64
	SubjectID int64
	Name      string
	Type      string
	Weight    int
	Kktp      int
}

func (s *Service) validateComponentClassSubject(ctx context.Context, schoolID, classID, subjectID int64) error {
	class, ok, err := s.repo.GetClassByID(ctx, schoolID, classID)
	if err != nil {
		return err
	}
	if !ok {
		return httpx.Validation("Kelas tidak ditemukan.")
	}
	yearID, err := s.requireActiveYear(ctx, schoolID)
	if err != nil {
		return err
	}
	if class.AcademicYearID != yearID {
		return httpx.Validation("Kelas tidak berada pada tahun ajaran aktif.")
	}
	_, ok, err = s.repo.GetSubjectByID(ctx, schoolID, subjectID)
	if err != nil {
		return err
	}
	if !ok {
		return httpx.Validation("Mata pelajaran tidak ditemukan.")
	}
	return nil
}

func (s *Service) CreateComponent(ctx context.Context, actorUserID, schoolID int64, in ComponentCreateInput) (ComponentView, error) {
	if _, err := s.requireEnabled(ctx, schoolID); err != nil {
		return ComponentView{}, err
	}
	if err := s.requireManage(ctx); err != nil {
		return ComponentView{}, err
	}
	if in.ClassID == 0 || in.SubjectID == 0 {
		return ComponentView{}, httpx.Validation("class_id dan subject_id wajib diisi.")
	}
	if err := s.requireClassSubjectAccess(ctx, schoolID, in.ClassID, in.SubjectID); err != nil {
		return ComponentView{}, err
	}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return ComponentView{}, httpx.Validation("Nama komponen wajib diisi.")
	}
	if !validComponentType(in.Type) {
		return ComponentView{}, httpx.Validation("Tipe komponen harus salah satu dari: tp, sumatif, praktik, lainnya.")
	}
	if in.Weight <= 0 {
		return ComponentView{}, httpx.Validation("Bobot harus lebih dari 0.")
	}
	if in.Kktp < 0 || in.Kktp > 100 {
		return ComponentView{}, httpx.Validation("KKTP harus di antara 0 dan 100.")
	}
	if err := s.validateComponentClassSubject(ctx, schoolID, in.ClassID, in.SubjectID); err != nil {
		return ComponentView{}, err
	}
	yearID, err := s.requireActiveYear(ctx, schoolID)
	if err != nil {
		return ComponentView{}, err
	}
	rec, err := s.repo.CreateComponent(ctx, CreateComponentInput{
		SchoolID: schoolID, AcademicYearID: yearID, ClassID: in.ClassID, SubjectID: in.SubjectID,
		Name: name, Type: in.Type, Weight: in.Weight, Kktp: in.Kktp, CreatedBy: actorUserID,
	})
	if err != nil {
		return ComponentView{}, err
	}
	s.audit(ctx, schoolID, actorUserID, "grading.component_create", "assessment_component", rec.ID, nil,
		map[string]any{"class_id": rec.ClassID, "subject_id": rec.SubjectID, "name": rec.Name, "type": rec.Type, "weight": rec.Weight, "kktp": rec.Kktp})
	return ComponentView{ID: rec.ID, Name: rec.Name, Type: rec.Type, Weight: rec.Weight, Kktp: rec.Kktp}, nil
}

// ComponentPatchInput adalah parameter Service.UpdateComponent — nil = field
// tidak diubah (semantik PATCH, pola yang sama dengan discipline).
type ComponentPatchInput struct {
	Name   *string
	Type   *string
	Weight *int
	Kktp   *int
}

func (s *Service) UpdateComponent(ctx context.Context, actorUserID, schoolID, id int64, in ComponentPatchInput) (ComponentView, error) {
	if _, err := s.requireEnabled(ctx, schoolID); err != nil {
		return ComponentView{}, err
	}
	if err := s.requireManage(ctx); err != nil {
		return ComponentView{}, err
	}
	old, err := s.repo.GetComponentByID(ctx, schoolID, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return ComponentView{}, httpx.ErrNotFound
		}
		return ComponentView{}, err
	}
	if err := s.requireClassSubjectAccess(ctx, schoolID, old.ClassID, old.SubjectID); err != nil {
		return ComponentView{}, err
	}
	if in.Name != nil {
		trimmed := strings.TrimSpace(*in.Name)
		if trimmed == "" {
			return ComponentView{}, httpx.Validation("Nama komponen wajib diisi.")
		}
		in.Name = &trimmed
	}
	if in.Type != nil && !validComponentType(*in.Type) {
		return ComponentView{}, httpx.Validation("Tipe komponen harus salah satu dari: tp, sumatif, praktik, lainnya.")
	}
	if in.Weight != nil && *in.Weight <= 0 {
		return ComponentView{}, httpx.Validation("Bobot harus lebih dari 0.")
	}
	if in.Kktp != nil && (*in.Kktp < 0 || *in.Kktp > 100) {
		return ComponentView{}, httpx.Validation("KKTP harus di antara 0 dan 100.")
	}
	rec, err := s.repo.UpdateComponent(ctx, schoolID, id, in.Name, in.Type, in.Weight, in.Kktp)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return ComponentView{}, httpx.ErrNotFound
		}
		return ComponentView{}, err
	}
	s.audit(ctx, schoolID, actorUserID, "grading.component_update", "assessment_component", id,
		map[string]any{"name": old.Name, "type": old.Type, "weight": old.Weight, "kktp": old.Kktp},
		map[string]any{"name": rec.Name, "type": rec.Type, "weight": rec.Weight, "kktp": rec.Kktp})
	return ComponentView{ID: rec.ID, Name: rec.Name, Type: rec.Type, Weight: rec.Weight, Kktp: rec.Kktp}, nil
}

// DeleteComponent menghapus komponen — CASCADE menghapus seluruh nilai
// siswa pada komponen itu (FK ON DELETE CASCADE, migrations/00017_grading.sql;
// konfirmasi tindakan ini adalah tanggung jawab frontend, docs tugas).
func (s *Service) DeleteComponent(ctx context.Context, actorUserID, schoolID, id int64) error {
	if _, err := s.requireEnabled(ctx, schoolID); err != nil {
		return err
	}
	if err := s.requireManage(ctx); err != nil {
		return err
	}
	old, err := s.repo.GetComponentByID(ctx, schoolID, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return httpx.ErrNotFound
		}
		return err
	}
	if err := s.requireClassSubjectAccess(ctx, schoolID, old.ClassID, old.SubjectID); err != nil {
		return err
	}
	rows, err := s.repo.DeleteComponent(ctx, schoolID, id)
	if err != nil {
		return err
	}
	if rows == 0 {
		return httpx.ErrNotFound
	}
	s.audit(ctx, schoolID, actorUserID, "grading.component_delete", "assessment_component", id,
		map[string]any{"name": old.Name, "class_id": old.ClassID, "subject_id": old.SubjectID}, nil)
	return nil
}

// -- GET/PUT /api/grading/components/{id}/grades --

func (s *Service) GetGrades(ctx context.Context, schoolID, componentID int64) (ComponentGradesView, error) {
	if _, err := s.requireEnabled(ctx, schoolID); err != nil {
		return ComponentGradesView{}, err
	}
	if err := s.requireManage(ctx); err != nil {
		return ComponentGradesView{}, err
	}
	comp, err := s.repo.GetComponentByID(ctx, schoolID, componentID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return ComponentGradesView{}, httpx.ErrNotFound
		}
		return ComponentGradesView{}, err
	}
	if err := s.requireClassSubjectAccess(ctx, schoolID, comp.ClassID, comp.SubjectID); err != nil {
		return ComponentGradesView{}, err
	}
	roster, err := s.repo.ListClassRoster(ctx, schoolID, comp.ClassID)
	if err != nil {
		return ComponentGradesView{}, err
	}
	scores, err := s.repo.ListGradesForComponent(ctx, componentID)
	if err != nil {
		return ComponentGradesView{}, err
	}
	out := make([]GradeStudentRow, 0, len(roster))
	for _, st := range roster {
		row := GradeStudentRow{StudentID: st.ID, Name: st.Name, NIS: st.NIS}
		if v, ok := scores[st.ID]; ok {
			row.Score = ptrFloat(v)
		}
		out = append(out, row)
	}
	return ComponentGradesView{Students: out}, nil
}

// GradeEntryInput adalah satu baris parameter Service.PutGrades — Score nil
// = hapus nilai siswa itu pada komponen ini (docs tugas: "null = hapus nilai").
type GradeEntryInput struct {
	StudentID int64
	Score     *float64
}

func (s *Service) PutGrades(ctx context.Context, actorUserID, schoolID, componentID int64, entries []GradeEntryInput) (ComponentGradesView, error) {
	if _, err := s.requireEnabled(ctx, schoolID); err != nil {
		return ComponentGradesView{}, err
	}
	if err := s.requireManage(ctx); err != nil {
		return ComponentGradesView{}, err
	}
	comp, err := s.repo.GetComponentByID(ctx, schoolID, componentID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return ComponentGradesView{}, httpx.ErrNotFound
		}
		return ComponentGradesView{}, err
	}
	if err := s.requireClassSubjectAccess(ctx, schoolID, comp.ClassID, comp.SubjectID); err != nil {
		return ComponentGradesView{}, err
	}
	roster, err := s.repo.ListClassRoster(ctx, schoolID, comp.ClassID)
	if err != nil {
		return ComponentGradesView{}, err
	}
	enrolled := make(map[int64]bool, len(roster))
	for _, st := range roster {
		enrolled[st.ID] = true
	}
	for _, e := range entries {
		if !enrolled[e.StudentID] {
			return ComponentGradesView{}, httpx.Validation("Siswa bukan anggota rombel komponen ini.")
		}
		if e.Score != nil && (*e.Score < 0 || *e.Score > 100) {
			return ComponentGradesView{}, httpx.Validation("Nilai harus di antara 0 dan 100.")
		}
	}
	// Transaksional: bulk upsert/delete SATU per SATU dalam repository (tanpa
	// Tx eksplisit di service — Repository TIDAK expose Tx generik, pola yang
	// sama modul lain memakai method repository per-baris di dalam loop
	// SELAMA validasi SUDAH lolos semua di atas sebelum mutasi pertama
	// dimulai, sehingga tidak ada state parsial akibat validasi gagal
	// di tengah; kegagalan DB murni pada baris ke-N tetap bisa meninggalkan
	// baris < N ter-apply, sama seperti presedan attendance.UpdateRecords).
	for _, e := range entries {
		if e.Score == nil {
			if err := s.repo.DeleteGrade(ctx, componentID, e.StudentID); err != nil {
				return ComponentGradesView{}, err
			}
			continue
		}
		if err := s.repo.UpsertGrade(ctx, schoolID, componentID, e.StudentID, *e.Score, actorUserID); err != nil {
			return ComponentGradesView{}, err
		}
	}
	s.audit(ctx, schoolID, actorUserID, "grading.grades_update", "assessment_component", componentID, nil,
		map[string]any{"count": len(entries)})
	s.emitGrading(schoolID, comp.ClassID)
	return s.GetGrades(ctx, schoolID, componentID)
}

// -- GET /api/grading/recap --

// RecapQuery adalah parameter Service.Recap.
type RecapQuery struct {
	ClassID   int64
	SubjectID int64
}

func (s *Service) Recap(ctx context.Context, schoolID int64, q RecapQuery) (RecapView, error) {
	if _, err := s.requireEnabled(ctx, schoolID); err != nil {
		return RecapView{}, err
	}
	if err := s.requireManage(ctx); err != nil {
		return RecapView{}, err
	}
	if q.ClassID == 0 || q.SubjectID == 0 {
		return RecapView{}, httpx.Validation("class_id dan subject_id wajib diisi.")
	}
	if err := s.requireClassSubjectAccess(ctx, schoolID, q.ClassID, q.SubjectID); err != nil {
		return RecapView{}, err
	}
	return s.buildRecap(ctx, schoolID, q.ClassID, q.SubjectID)
}

// buildRecap adalah inti penyusunan recap — DIPISAH dari Recap supaya bisa
// dipakai ULANG oleh MyGrades & ExportXLSX tanpa mengulang gerbang
// permission/object-level (pemanggil sudah menggerbang sendiri).
func (s *Service) buildRecap(ctx context.Context, schoolID, classID, subjectID int64) (RecapView, error) {
	yearID, err := s.requireActiveYear(ctx, schoolID)
	if err != nil {
		return RecapView{}, err
	}
	components, err := s.repo.ListComponentsRaw(ctx, schoolID, classID, subjectID)
	if err != nil {
		return RecapView{}, err
	}
	roster, err := s.repo.ListClassRoster(ctx, schoolID, classID)
	if err != nil {
		return RecapView{}, err
	}
	componentIDs := make([]int64, 0, len(components))
	compRefs := make([]RecapComponentRef, 0, len(components))
	for _, c := range components {
		componentIDs = append(componentIDs, c.ID)
		compRefs = append(compRefs, RecapComponentRef{ID: c.ID, Name: c.Name, Type: c.Type, Weight: c.Weight})
	}
	gradesByComponent, err := s.repo.ListGradesForComponents(ctx, componentIDs)
	if err != nil {
		return RecapView{}, err
	}
	settings, err := s.loadSettings(ctx, schoolID)
	if err != nil {
		return RecapView{}, err
	}
	students := make([]RecapStudentRow, 0, len(roster))
	for _, st := range roster {
		scores := map[int64]*float64{}
		studentScores := map[int64]float64{}
		for _, c := range components {
			if v, ok := gradesByComponent[c.ID][st.ID]; ok {
				scores[c.ID] = ptrFloat(v)
				studentScores[c.ID] = v
			} else {
				scores[c.ID] = nil
			}
		}
		final, belowKktp := computeFinal(components, studentScores)
		var label *string
		if final != nil {
			label = labelForScore(settings.Ranges, roundHalfUp(*final))
		}
		students = append(students, RecapStudentRow{
			StudentID: st.ID, Name: st.Name, NIS: st.NIS, Scores: scores, Final: final, Label: label, BelowKktp: belowKktp,
		})
	}
	published, err := s.repo.IsPublished(ctx, schoolID, yearID, classID, subjectID)
	if err != nil {
		return RecapView{}, err
	}
	return RecapView{Components: compRefs, Students: students, Published: published}, nil
}

// -- PUT /api/grading/publication --

// PublicationInput adalah parameter Service.PutPublication.
type PublicationInput struct {
	ClassID   int64
	SubjectID int64
	Published bool
}

func (s *Service) PutPublication(ctx context.Context, actorUserID, schoolID int64, in PublicationInput) error {
	if _, err := s.requireEnabled(ctx, schoolID); err != nil {
		return err
	}
	if err := s.requireManage(ctx); err != nil {
		return err
	}
	if in.ClassID == 0 || in.SubjectID == 0 {
		return httpx.Validation("class_id dan subject_id wajib diisi.")
	}
	if err := s.requireClassSubjectAccess(ctx, schoolID, in.ClassID, in.SubjectID); err != nil {
		return err
	}
	yearID, err := s.requireActiveYear(ctx, schoolID)
	if err != nil {
		return err
	}
	if in.Published {
		if err := s.repo.UpsertPublication(ctx, schoolID, yearID, in.ClassID, in.SubjectID, actorUserID); err != nil {
			return err
		}
	} else {
		if err := s.repo.DeletePublication(ctx, schoolID, yearID, in.ClassID, in.SubjectID); err != nil {
			return err
		}
	}
	s.audit(ctx, schoolID, actorUserID, "grading.publication_set", "grade_publication", in.ClassID,
		nil, map[string]any{"class_id": in.ClassID, "subject_id": in.SubjectID, "published": in.Published})
	s.emitGrading(schoolID, in.ClassID)

	if in.Published {
		s.notifyPublished(ctx, schoolID, in.ClassID, in.SubjectID)
	}
	return nil
}

// notifyPublished mengirim notif "grading.published" ke SELURUH siswa kelas
// (yang sudah aktivasi akun) + orang tua mereka (docs tugas: "notif ...
// grading.published ke siswa kelas + ortu").
func (s *Service) notifyPublished(ctx context.Context, schoolID, classID, subjectID int64) {
	roster, err := s.repo.ListClassRoster(ctx, schoolID, classID)
	if err != nil {
		return
	}
	subject, ok, err := s.repo.GetSubjectByID(ctx, schoolID, subjectID)
	if err != nil || !ok {
		return
	}
	recipients := make([]int64, 0, len(roster)*2)
	for _, st := range roster {
		if st.UserID != 0 {
			recipients = append(recipients, st.UserID)
		}
		guardianIDs, gerr := s.students.GuardianUserIDs(ctx, schoolID, st.ID)
		if gerr == nil {
			recipients = append(recipients, guardianIDs...)
		}
	}
	s.notifierNotify(ctx, schoolID, EventGradingPublished, recipients, map[string]any{"subject": subject.Name})
}

// -- GET /api/my-grades --

// MyGradesQuery adalah parameter Service.MyGrades.
type MyGradesQuery struct {
	StudentID int64 // dipakai HANYA oleh orang tua (?student_id=)
}

func (s *Service) MyGrades(ctx context.Context, schoolID int64, q MyGradesQuery) (MyGradesView, error) {
	if _, err := s.requireEnabled(ctx, schoolID); err != nil {
		return MyGradesView{}, err
	}
	role := reqctx.Role(ctx)
	userID := reqctx.UserID(ctx)
	var studentID int64
	switch role {
	case RoleSiswa:
		sid, _, ok, err := s.students.MyStudentID(ctx, schoolID, userID)
		if err != nil {
			return MyGradesView{}, err
		}
		if !ok {
			return MyGradesView{}, httpx.ErrForbidden
		}
		studentID = sid
	case RoleOrangTua:
		if q.StudentID == 0 {
			return MyGradesView{}, httpx.Validation("student_id wajib diisi.")
		}
		if err := s.students.CanViewStudent(ctx, userID, role, schoolID, q.StudentID); err != nil {
			return MyGradesView{}, err
		}
		studentID = q.StudentID
	default:
		return MyGradesView{}, httpx.ErrForbidden
	}

	yearID, err := s.requireActiveYear(ctx, schoolID)
	if err != nil {
		return MyGradesView{}, err
	}
	classID, ok, err := s.repo.StudentClassID(ctx, schoolID, yearID, studentID)
	if err != nil {
		return MyGradesView{}, err
	}
	if !ok {
		return MyGradesView{Subjects: []SubjectGradeView{}}, nil
	}
	subjectIDs, err := s.repo.ListPublishedSubjectsForClass(ctx, schoolID, yearID, classID)
	if err != nil {
		return MyGradesView{}, err
	}
	settings, err := s.loadSettings(ctx, schoolID)
	if err != nil {
		return MyGradesView{}, err
	}
	out := make([]SubjectGradeView, 0, len(subjectIDs))
	for _, subjectID := range subjectIDs {
		subject, ok, err := s.repo.GetSubjectByID(ctx, schoolID, subjectID)
		if err != nil {
			return MyGradesView{}, err
		}
		if !ok {
			continue
		}
		components, err := s.repo.ListComponentsRaw(ctx, schoolID, classID, subjectID)
		if err != nil {
			return MyGradesView{}, err
		}
		componentIDs := make([]int64, 0, len(components))
		for _, c := range components {
			componentIDs = append(componentIDs, c.ID)
		}
		gradesByComponent, err := s.repo.ListGradesForComponents(ctx, componentIDs)
		if err != nil {
			return MyGradesView{}, err
		}
		studentScores := map[int64]float64{}
		compViews := make([]MyGradeComponent, 0, len(components))
		for _, c := range components {
			var scorePtr *float64
			if v, ok := gradesByComponent[c.ID][studentID]; ok {
				scorePtr = ptrFloat(v)
				studentScores[c.ID] = v
			}
			compViews = append(compViews, MyGradeComponent{Name: c.Name, Type: c.Type, Weight: c.Weight, Score: scorePtr, Kktp: c.Kktp})
		}
		final, _ := computeFinal(components, studentScores)
		var label *string
		if final != nil {
			label = labelForScore(settings.Ranges, roundHalfUp(*final))
		}
		out = append(out, SubjectGradeView{
			Subject: SubjectRef{ID: subject.ID, Code: subject.Code, Name: subject.Name},
			Final:   final, Label: label, Components: compViews,
		})
	}
	return MyGradesView{Subjects: out}, nil
}

// -- POST /api/grading/stars, DELETE /api/grading/stars/{id}, GET /api/grading/stars --

// StarCreateInput adalah parameter Service.CreateStar.
type StarCreateInput struct {
	StudentID  int64
	Delta      int
	Note       string
	Visibility string
}

func (s *Service) CreateStar(ctx context.Context, actorUserID, schoolID int64, in StarCreateInput) (StarResult, error) {
	if _, err := s.requireEnabled(ctx, schoolID); err != nil {
		return StarResult{}, err
	}
	if err := s.requireManage(ctx); err != nil {
		return StarResult{}, err
	}
	if in.StudentID == 0 {
		return StarResult{}, httpx.Validation("student_id wajib diisi.")
	}
	if in.Delta == 0 {
		return StarResult{}, httpx.Validation("delta tidak boleh 0.")
	}
	if in.Visibility == "" {
		in.Visibility = VisibilityPrivate
	}
	if !validVisibility(in.Visibility) {
		return StarResult{}, httpx.Validation("visibility harus private atau student.")
	}
	_, _, ok, err := s.repo.GetStudentBasic(ctx, schoolID, in.StudentID)
	if err != nil {
		return StarResult{}, err
	}
	if !ok {
		return StarResult{}, httpx.Validation("Siswa tidak ditemukan.")
	}
	yearID, err := s.requireActiveYear(ctx, schoolID)
	if err != nil {
		return StarResult{}, err
	}
	classID, ok, err := s.repo.StudentClassID(ctx, schoolID, yearID, in.StudentID)
	if err != nil {
		return StarResult{}, err
	}
	if !ok {
		return StarResult{}, httpx.Validation("Siswa belum terdaftar di rombel pada tahun ajaran aktif.")
	}
	if err := s.requireClassAccess(ctx, schoolID, classID); err != nil {
		return StarResult{}, err
	}
	rec, err := s.repo.CreateStar(ctx, CreateStarInput{
		SchoolID: schoolID, AcademicYearID: yearID, StudentID: in.StudentID, GivenBy: actorUserID,
		Delta: in.Delta, Note: strings.TrimSpace(in.Note), Visibility: in.Visibility,
	})
	if err != nil {
		return StarResult{}, err
	}
	s.audit(ctx, schoolID, actorUserID, "grading.star_create", "classroom_star_event", rec.ID, nil,
		map[string]any{"student_id": in.StudentID, "delta": in.Delta, "visibility": in.Visibility})

	all, err := s.repo.ListStarsForStudent(ctx, schoolID, in.StudentID, false)
	total := 0
	if err == nil {
		for _, r := range all {
			total += r.Delta
		}
	}
	s.emitGrading(schoolID, classID)
	return StarResult{ID: rec.ID, Delta: rec.Delta, Total: total}, nil
}

// DeleteStar — pembuat sendiri ATAU admin_sekolah (docs tugas: "pembuat/admin").
func (s *Service) DeleteStar(ctx context.Context, actorUserID, schoolID, id int64) error {
	if _, err := s.requireEnabled(ctx, schoolID); err != nil {
		return err
	}
	if err := s.requireManage(ctx); err != nil {
		return err
	}
	old, err := s.repo.GetStarByID(ctx, schoolID, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return httpx.ErrNotFound
		}
		return err
	}
	role := reqctx.Role(ctx)
	if role != RoleAdminSekolah && old.GivenByUserID != actorUserID {
		return httpx.ErrForbidden
	}
	rows, err := s.repo.DeleteStar(ctx, schoolID, id)
	if err != nil {
		return err
	}
	if rows == 0 {
		return httpx.ErrNotFound
	}
	s.audit(ctx, schoolID, actorUserID, "grading.star_delete", "classroom_star_event", id,
		map[string]any{"student_id": old.StudentID, "delta": old.Delta}, nil)
	return nil
}

// ListStarsQuery adalah parameter Service.ListStars — TEPAT SATU dari
// StudentID/ClassID wajib diisi.
type ListStarsQuery struct {
	StudentID int64
	ClassID   int64
}

// ListStarsResult adalah shape response GET /api/grading/stars — Items
// (student_id) TERISI bila query pakai student_id; Totals (class_id) TERISI
// bila query pakai class_id.
type ListStarsResult struct {
	Items  []StarItem         `json:"items,omitempty"`
	Total  int                `json:"total,omitempty"`
	Totals []StarStudentTotal `json:"totals,omitempty"`
}

func (s *Service) ListStars(ctx context.Context, schoolID int64, q ListStarsQuery) (ListStarsResult, error) {
	if _, err := s.requireEnabled(ctx, schoolID); err != nil {
		return ListStarsResult{}, err
	}
	if err := s.requireManage(ctx); err != nil {
		return ListStarsResult{}, err
	}
	if q.StudentID == 0 && q.ClassID == 0 {
		return ListStarsResult{}, httpx.Validation("student_id atau class_id wajib diisi.")
	}
	if q.StudentID != 0 {
		rows, err := s.repo.ListStarsForStudent(ctx, schoolID, q.StudentID, false)
		if err != nil {
			return ListStarsResult{}, err
		}
		items := make([]StarItem, 0, len(rows))
		total := 0
		for _, r := range rows {
			items = append(items, StarItem{ID: r.ID, Delta: r.Delta, Note: r.Note, Visibility: r.Visibility, GivenByName: r.GivenByName, CreatedAt: r.CreatedAt})
			total += r.Delta
		}
		return ListStarsResult{Items: items, Total: total}, nil
	}
	rows, err := s.repo.ListStarTotalsForClass(ctx, schoolID, q.ClassID)
	if err != nil {
		return ListStarsResult{}, err
	}
	totals := make([]StarStudentTotal, 0, len(rows))
	for _, r := range rows {
		totals = append(totals, StarStudentTotal{StudentID: r.StudentID, Name: r.Name, NIS: r.NIS, Total: r.Total})
	}
	return ListStarsResult{Totals: totals}, nil
}

// -- GET /api/my-stars --

// MyStarsQuery adalah parameter Service.MyStars.
type MyStarsQuery struct {
	StudentID int64 // dipakai HANYA oleh orang tua (?student_id=)
}

// MyStars — SELALU hanya visibility='student' (total & items, keputusan
// dilaporkan di docs tugas: "total & items sama-sama hanya visible").
func (s *Service) MyStars(ctx context.Context, schoolID int64, q MyStarsQuery) (MyStarsView, error) {
	if _, err := s.requireEnabled(ctx, schoolID); err != nil {
		return MyStarsView{}, err
	}
	role := reqctx.Role(ctx)
	userID := reqctx.UserID(ctx)
	var studentID int64
	switch role {
	case RoleSiswa:
		sid, _, ok, err := s.students.MyStudentID(ctx, schoolID, userID)
		if err != nil {
			return MyStarsView{}, err
		}
		if !ok {
			return MyStarsView{}, httpx.ErrForbidden
		}
		studentID = sid
	case RoleOrangTua:
		if q.StudentID == 0 {
			return MyStarsView{}, httpx.Validation("student_id wajib diisi.")
		}
		if err := s.students.CanViewStudent(ctx, userID, role, schoolID, q.StudentID); err != nil {
			return MyStarsView{}, err
		}
		studentID = q.StudentID
	default:
		return MyStarsView{}, httpx.ErrForbidden
	}
	rows, err := s.repo.ListStarsForStudent(ctx, schoolID, studentID, true)
	if err != nil {
		return MyStarsView{}, err
	}
	items := make([]StarItem, 0, len(rows))
	total := 0
	for _, r := range rows {
		items = append(items, StarItem{Delta: r.Delta, Note: r.Note, GivenByName: r.GivenByName, CreatedAt: r.CreatedAt})
		total += r.Delta
	}
	return MyStarsView{Total: total, Items: items}, nil
}
