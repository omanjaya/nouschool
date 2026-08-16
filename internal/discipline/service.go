package discipline

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/omanjaya/nouschool/internal/platform/clock"
	"github.com/omanjaya/nouschool/internal/platform/httpx"
	"github.com/omanjaya/nouschool/internal/platform/reqctx"
)

// -- consumer-side interface (lihat CLAUDE.md: antar-modul lewat interface
// kecil dideklarasikan di sisi PEMAKAI, di-inject lewat constructor di
// main.go). Semua signature sengaja hanya memakai tipe primitif supaya
// *identity.Service, *tenant.Service, dan *student.Service memenuhi
// interface ini secara struktural TANPA discipline perlu mengimpor
// package-package itu.

// IdentityGateway adalah kebutuhan modul discipline dari modul identity:
// gerbang permission & audit log.
type IdentityGateway interface {
	HasPermission(role, perm string) bool
	Log(ctx context.Context, schoolID, userID *int64, action, entity string, entityID *int64, oldValue, newValue any) error
}

// AcademicYearLookup adalah kebutuhan modul discipline dari modul tenant:
// tahun ajaran aktif (SEMUA perhitungan poin/ambang SP dianchor ke TA
// aktif — docs/12-sion-parity.md).
type AcademicYearLookup interface {
	ActiveAcademicYearID(ctx context.Context, schoolID int64) (id int64, ok bool, err error)
}

// BrandingGateway adalah kebutuhan modul discipline dari modul tenant: nama
// aplikasi branding sekolah untuk kop surat peringatan PDF/HTML.
type BrandingGateway interface {
	BrandingAppName(ctx context.Context, schoolID int64) (string, error)
}

// StudentAccess adalah kebutuhan modul discipline dari modul student:
// object-level check (siswa boleh lihat dirinya sendiri, orang tua boleh
// lihat anaknya — CanViewStudent) dan resolusi penerima notifikasi ortu
// (GuardianUserIDs). Dipenuhi *student.Service secara struktural.
type StudentAccess interface {
	CanViewStudent(ctx context.Context, userID int64, role string, schoolID, studentID int64) error
	GuardianUserIDs(ctx context.Context, schoolID, studentID int64) ([]int64, error)
}

// Notifier adalah kebutuhan modul discipline dari modul notification —
// consumer-side interface kecil dideklarasikan di sisi PEMAKAI. Dipenuhi
// *notification.Service secara struktural lewat method Notify.
type Notifier interface {
	Notify(ctx context.Context, schoolID int64, event string, userIDs []int64, data map[string]any) error
}

// Event kanonik — nilai HARUS sama persis dengan
// notification.EventDisciplineRecorded / notification.EventDisciplineSPIssued
// (didefinisikan ulang di sini karena discipline TIDAK boleh mengimpor
// notification, lihat CLAUDE.md).
const (
	EventDisciplineRecorded = "discipline.recorded"
	EventDisciplineSPIssued = "discipline.sp_issued"
)

// SettingsGateway adalah kebutuhan modul discipline dari modul tenant: baca
// RAW json school_settings module "letters" (Fase 14 Gelombang D,
// docs/12-sion-parity.md "template surat") — dipenuhi *tenant.Repository
// secara struktural lewat method GetSetting (SAMA pola dengan
// internal/grading.SettingsGateway; BUKAN lewat tenant.SettingsService,
// parameter bertipe interface tenant.Settings tidak akan match struktural).
type SettingsGateway interface {
	GetSetting(ctx context.Context, schoolID int64, module string) (raw []byte, found bool, err error)
}

// Realtime adalah kebutuhan modul discipline dari modul realtime (bus event
// WebSocket read-only server->client) — consumer-side interface kecil
// dideklarasikan di sisi PEMAKAI. TIDAK dipenuhi *realtime.Hub secara
// langsung — dijembatani adapter kecil di cmd/server (realtimeadapter.go).
type Realtime interface {
	PublishTo(schoolID int64, eventType string, data map[string]any, roles []string, userIDs []int64)
}

// ErrNoActiveAcademicYear — sekolah belum mengaktifkan tahun ajaran (pesan
// SAMA persis dengan internal/attendance.ErrNoActiveAcademicYear).
var ErrNoActiveAcademicYear = httpx.Validation("Sekolah belum punya tahun ajaran aktif. Aktifkan dulu di menu tahun ajaran.")

// Service berisi aturan bisnis modul discipline.
type Service struct {
	repo     disciplineRepository
	identity IdentityGateway
	years    AcademicYearLookup
	branding BrandingGateway
	students StudentAccess
	clock    clock.Clock
	notifier Notifier        // opsional — nil = notifikasi dilewati
	realtime Realtime        // opsional — nil = event realtime dilewati
	settings SettingsGateway // opsional — nil = footer surat SP dilewati (Fase 14 Gelombang D)
}

func NewService(repo *Repository, identity IdentityGateway, years AcademicYearLookup, branding BrandingGateway, students StudentAccess, clk clock.Clock) *Service {
	if clk == nil {
		clk = clock.System{}
	}
	return &Service{repo: repo, identity: identity, years: years, branding: branding, students: students, clock: clk}
}

// SetNotifier / SetRealtime / SetSettingsGateway menyuntikkan dependency
// opsional SETELAH konstruksi (setter, pola yang sama dengan
// internal/leave.SetNotifier/SetRealtime — supaya call site test tidak perlu
// diubah; nil aman/no-op).
func (s *Service) SetNotifier(n Notifier)               { s.notifier = n }
func (s *Service) SetRealtime(r Realtime)               { s.realtime = r }
func (s *Service) SetSettingsGateway(g SettingsGateway) { s.settings = g }

// newServiceForTest membangun Service dengan repository FAKE (in-memory,
// tanpa DB) — dipakai test di package ini saja (service_test.go).
func newServiceForTest(repo disciplineRepository, identity IdentityGateway, years AcademicYearLookup, branding BrandingGateway, students StudentAccess, clk clock.Clock) *Service {
	return &Service{repo: repo, identity: identity, years: years, branding: branding, students: students, clock: clk}
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

func schoolToday(now time.Time, tz string) time.Time {
	local := clock.InZone(now, tz)
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.UTC)
}

func schoolTimezone(ctx context.Context) string {
	sch, _ := reqctx.SchoolFromContext(ctx)
	return sch.Timezone
}

func parseDate(raw string) (time.Time, error) {
	t, err := time.Parse("2006-01-02", strings.TrimSpace(raw))
	if err != nil {
		return time.Time{}, httpx.Validation("Format tanggal harus YYYY-MM-DD.")
	}
	return t, nil
}

func viewFromRecord(rec ViolationTypeRecord) ViolationType {
	return ViolationType{ID: rec.ID, Name: rec.Name, Points: rec.Points, Category: rec.Category, Active: rec.Active}
}

// -- GET/POST/PATCH/DELETE /api/discipline/types --

func (s *Service) ListViolationTypes(ctx context.Context, schoolID int64) ([]ViolationType, error) {
	role := reqctx.Role(ctx)
	if !s.identity.HasPermission(role, PermDisciplineRead) {
		return nil, httpx.ErrForbidden
	}
	rows, err := s.repo.ListViolationTypes(ctx, schoolID)
	if err != nil {
		return nil, err
	}
	out := make([]ViolationType, 0, len(rows))
	for _, r := range rows {
		out = append(out, viewFromRecord(r))
	}
	return out, nil
}

// CreateViolationTypeInput adalah parameter Service.CreateViolationType.
type CreateViolationTypeInput struct {
	Name     string
	Points   int
	Category string
}

func (s *Service) requireManage(ctx context.Context) error {
	if !s.identity.HasPermission(reqctx.Role(ctx), PermDisciplineManage) {
		return httpx.ErrForbidden
	}
	return nil
}

func (s *Service) CreateViolationType(ctx context.Context, actorUserID, schoolID int64, in CreateViolationTypeInput) (ViolationType, error) {
	if err := s.requireManage(ctx); err != nil {
		return ViolationType{}, err
	}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return ViolationType{}, httpx.Validation("Nama jenis pelanggaran wajib diisi.")
	}
	if in.Points <= 0 {
		return ViolationType{}, httpx.Validation("Poin harus lebih dari 0.")
	}
	rec, err := s.repo.CreateViolationType(ctx, schoolID, name, in.Points, strings.TrimSpace(in.Category))
	if err != nil {
		if errors.Is(err, ErrConflict) {
			return ViolationType{}, httpx.Conflict("Jenis pelanggaran dengan nama itu sudah ada.")
		}
		return ViolationType{}, err
	}
	s.audit(ctx, schoolID, actorUserID, "discipline.type_create", "violation_type", rec.ID, nil,
		map[string]any{"name": rec.Name, "points": rec.Points, "category": rec.Category})
	return viewFromRecord(rec), nil
}

// PatchViolationTypeInput adalah parameter Service.UpdateViolationType — nil
// = field tidak diubah (semantik PATCH, sama pola dengan schedule/student).
type PatchViolationTypeInput struct {
	Name     *string
	Points   *int
	Category *string
	Active   *bool
}

func (s *Service) UpdateViolationType(ctx context.Context, actorUserID, schoolID, id int64, in PatchViolationTypeInput) (ViolationType, error) {
	if err := s.requireManage(ctx); err != nil {
		return ViolationType{}, err
	}
	old, err := s.repo.GetViolationTypeByID(ctx, schoolID, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return ViolationType{}, httpx.ErrNotFound
		}
		return ViolationType{}, err
	}
	if in.Name != nil {
		trimmed := strings.TrimSpace(*in.Name)
		if trimmed == "" {
			return ViolationType{}, httpx.Validation("Nama jenis pelanggaran wajib diisi.")
		}
		in.Name = &trimmed
	}
	if in.Points != nil && *in.Points <= 0 {
		return ViolationType{}, httpx.Validation("Poin harus lebih dari 0.")
	}
	rec, err := s.repo.UpdateViolationType(ctx, schoolID, id, in.Name, in.Points, in.Category, in.Active)
	if err != nil {
		if errors.Is(err, ErrConflict) {
			return ViolationType{}, httpx.Conflict("Jenis pelanggaran dengan nama itu sudah ada.")
		}
		if errors.Is(err, ErrNotFound) {
			return ViolationType{}, httpx.ErrNotFound
		}
		return ViolationType{}, err
	}
	s.audit(ctx, schoolID, actorUserID, "discipline.type_update", "violation_type", id,
		map[string]any{"name": old.Name, "points": old.Points, "category": old.Category, "active": old.Active},
		map[string]any{"name": rec.Name, "points": rec.Points, "category": rec.Category, "active": rec.Active})
	return viewFromRecord(rec), nil
}

// DeleteViolationType menolak 409 bila jenis SUDAH dipakai catatan
// pelanggaran (docs tugas: "sarankan nonaktifkan active=false").
var errViolationTypeInUse = httpx.Conflict("Jenis pelanggaran ini sudah dipakai pada catatan pelanggaran. Nonaktifkan (active=false) alih-alih menghapus.")

func (s *Service) DeleteViolationType(ctx context.Context, actorUserID, schoolID, id int64) error {
	if err := s.requireManage(ctx); err != nil {
		return err
	}
	old, err := s.repo.GetViolationTypeByID(ctx, schoolID, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return httpx.ErrNotFound
		}
		return err
	}
	count, err := s.repo.CountViolationsByType(ctx, schoolID, id)
	if err != nil {
		return err
	}
	if count > 0 {
		return errViolationTypeInUse
	}
	rows, err := s.repo.DeleteViolationType(ctx, schoolID, id)
	if err != nil {
		return err
	}
	if rows == 0 {
		return httpx.ErrNotFound
	}
	s.audit(ctx, schoolID, actorUserID, "discipline.type_delete", "violation_type", id,
		map[string]any{"name": old.Name, "points": old.Points}, nil)
	return nil
}

// -- GET/PUT /api/discipline/sp-settings --

func (s *Service) GetSPSettings(ctx context.Context, schoolID int64) (SPSettings, error) {
	if err := s.requireManage(ctx); err != nil {
		return SPSettings{}, err
	}
	yearID, err := s.requireActiveYear(ctx, schoolID)
	if err != nil {
		return SPSettings{}, err
	}
	return s.getSPSettingsOrDefault(ctx, schoolID, yearID)
}

func (s *Service) getSPSettingsOrDefault(ctx context.Context, schoolID, yearID int64) (SPSettings, error) {
	settings, found, err := s.repo.GetSPSettings(ctx, schoolID, yearID)
	if err != nil {
		return SPSettings{}, err
	}
	if !found {
		return SPSettings{SP1: DefaultSP1, SP2: DefaultSP2, SP3: DefaultSP3}, nil
	}
	return settings, nil
}

func validateSPSettings(in SPSettings) error {
	if in.SP1 <= 0 {
		return httpx.Validation("Ambang SP1 harus lebih dari 0.")
	}
	if !(in.SP1 < in.SP2 && in.SP2 < in.SP3) {
		return httpx.Validation("Ambang harus berurutan naik: SP1 < SP2 < SP3.")
	}
	return nil
}

func (s *Service) PutSPSettings(ctx context.Context, actorUserID, schoolID int64, in SPSettings) (SPSettings, error) {
	if err := s.requireManage(ctx); err != nil {
		return SPSettings{}, err
	}
	if err := validateSPSettings(in); err != nil {
		return SPSettings{}, err
	}
	yearID, err := s.requireActiveYear(ctx, schoolID)
	if err != nil {
		return SPSettings{}, err
	}
	old, err := s.getSPSettingsOrDefault(ctx, schoolID, yearID)
	if err != nil {
		return SPSettings{}, err
	}
	updated, err := s.repo.UpsertSPSettings(ctx, schoolID, yearID, in)
	if err != nil {
		return SPSettings{}, err
	}
	s.audit(ctx, schoolID, actorUserID, "discipline.sp_settings_update", "discipline_sp_settings", yearID,
		map[string]any{"sp1": old.SP1, "sp2": old.SP2, "sp3": old.SP3},
		map[string]any{"sp1": updated.SP1, "sp2": updated.SP2, "sp3": updated.SP3})
	return updated, nil
}

// -- POST /api/discipline/records --

// RecordInput adalah parameter Service.RecordViolation.
type RecordInput struct {
	StudentID           int64
	ViolationTypeID     int64
	AttendanceSessionID int64 // 0 = tanpa sesi
	Note                string
	OccurredOn          string // "" = default hari ini (lokal sekolah)
}

func (s *Service) RecordViolation(ctx context.Context, actorUserID, schoolID int64, in RecordInput) (RecordResult, error) {
	if !s.identity.HasPermission(reqctx.Role(ctx), PermDisciplineRecord) {
		return RecordResult{}, httpx.ErrForbidden
	}
	yearID, err := s.requireActiveYear(ctx, schoolID)
	if err != nil {
		return RecordResult{}, err
	}

	studentName, studentNIS, ok, err := s.repo.StudentExists(ctx, schoolID, in.StudentID)
	if err != nil {
		return RecordResult{}, err
	}
	if !ok {
		return RecordResult{}, httpx.Validation("Siswa tidak ditemukan.")
	}

	vt, err := s.repo.GetViolationTypeByID(ctx, schoolID, in.ViolationTypeID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return RecordResult{}, httpx.Validation("Jenis pelanggaran tidak ditemukan.")
		}
		return RecordResult{}, err
	}
	if !vt.Active {
		return RecordResult{}, httpx.Validation("Jenis pelanggaran ini sudah dinonaktifkan.")
	}

	if in.AttendanceSessionID != 0 {
		exists, err := s.repo.AttendanceSessionExists(ctx, schoolID, in.AttendanceSessionID)
		if err != nil {
			return RecordResult{}, err
		}
		if !exists {
			return RecordResult{}, httpx.Validation("Sesi presensi tidak ditemukan.")
		}
	}

	occurredOn := schoolToday(s.clock.Now(), schoolTimezone(ctx))
	if strings.TrimSpace(in.OccurredOn) != "" {
		t, err := parseDate(in.OccurredOn)
		if err != nil {
			return RecordResult{}, err
		}
		occurredOn = t
	}

	recID, err := s.repo.CreateStudentViolation(ctx, CreateViolationInput{
		SchoolID: schoolID, AcademicYearID: yearID, StudentID: in.StudentID, ViolationTypeID: in.ViolationTypeID,
		AttendanceSessionID: in.AttendanceSessionID, NotedBy: actorUserID, Note: strings.TrimSpace(in.Note), OccurredOn: occurredOn,
	})
	if err != nil {
		if errors.Is(err, ErrConflict) {
			return RecordResult{}, httpx.Validation("Pelanggaran ini sudah dicatat untuk siswa pada sesi presensi ini.")
		}
		return RecordResult{}, err
	}

	detail, err := s.repo.GetRecordByID(ctx, schoolID, recID)
	if err != nil {
		return RecordResult{}, err
	}
	recordView := detail.View()

	s.audit(ctx, schoolID, actorUserID, "discipline.record", "student_violation", recID, nil,
		map[string]any{"student_id": in.StudentID, "violation_type": vt.Name, "points": vt.Points, "occurred_on": occurredOn.Format("2006-01-02")})

	guardianIDs, _ := s.students.GuardianUserIDs(ctx, schoolID, in.StudentID)
	if len(guardianIDs) > 0 {
		_ = s.notifierNotify(ctx, schoolID, EventDisciplineRecorded, guardianIDs, map[string]any{
			"student": studentName, "type": vt.Name, "points": vt.Points,
		})
	}

	totalPoints, issued, err := s.evaluateAndIssueLetter(ctx, actorUserID, schoolID, yearID, in.StudentID, studentName, studentNIS, detail.ClassName)
	if err != nil {
		return RecordResult{}, err
	}

	s.emitDiscipline(schoolID, in.StudentID, guardianIDs)

	return RecordResult{Record: recordView, TotalPoints: totalPoints, IssuedLetter: issued}, nil
}

func (s *Service) notifierNotify(ctx context.Context, schoolID int64, event string, userIDs []int64, data map[string]any) error {
	if s.notifier == nil {
		return nil
	}
	return s.notifier.Notify(ctx, schoolID, event, userIDs, data)
}

func (s *Service) emitDiscipline(schoolID, studentID int64, guardianIDs []int64) {
	if s.realtime == nil {
		return
	}
	s.realtime.PublishTo(schoolID, "discipline", map[string]any{"student_id": studentID},
		[]string{RoleAdminSekolah, RoleKepalaSekolah, RoleGuru}, guardianIDs)
}

// evaluateAndIssueLetter menghitung ULANG total poin siswa pada TA yearID
// (sumber tunggal: ListViolationsForStudentYear) lalu menerbitkan surat
// OTOMATIS bila total melewati ambang SP level tertinggi yang BELUM punya
// surat — MAKSIMAL SATU surat per panggilan. Keputusan sendiri (dilaporkan):
// bila poin langsung melewati >1 ambang sekaligus dalam SATU catatan (mis.
// lompat dari 0 ke atas SP3), HANYA level TERTINGGI yang terlampaui & belum
// ada surat yang diterbitkan SAAT ITU (sesuai instruksi tugas: "terbitkan
// hanya level TERTINGGI ... - laporkan"). Level di bawahnya yang terlewati
// TIDAK hilang selamanya: karena fungsi ini mengevaluasi ULANG SEMUA level
// dari nol setiap kali dipanggil, level yang sempat terlewati akan
// "menyusul" terbit pada catatan BERIKUTNYA selama masih di atas ambangnya
// dan belum pernah punya surat (lihat TestAutoIssueLetter_OncePerLevel) —
// begitu SEMUA level yang relevan sudah bersurat, catatan berikutnya di
// atas ambang yang sama tidak menerbitkan apa pun lagi.
func (s *Service) evaluateAndIssueLetter(ctx context.Context, actorUserID, schoolID, yearID, studentID int64, studentName, studentNIS, className string) (int, *IssuedLetterRef, error) {
	violations, err := s.repo.ListViolationsForStudentYear(ctx, schoolID, yearID, studentID)
	if err != nil {
		return 0, nil, err
	}
	total := 0
	for _, v := range violations {
		total += v.ViolationPoints
	}

	spSettings, err := s.getSPSettingsOrDefault(ctx, schoolID, yearID)
	if err != nil {
		return total, nil, err
	}

	existing, err := s.repo.ListWarningLettersForStudentYear(ctx, schoolID, yearID, studentID)
	if err != nil {
		return total, nil, err
	}
	issuedLevels := make(map[int]bool, len(existing))
	for _, l := range existing {
		issuedLevels[l.Level] = true
	}

	level, threshold := 0, 0
	switch {
	case total >= spSettings.SP3 && !issuedLevels[3]:
		level, threshold = 3, spSettings.SP3
	case total >= spSettings.SP2 && !issuedLevels[2]:
		level, threshold = 2, spSettings.SP2
	case total >= spSettings.SP1 && !issuedLevels[1]:
		level, threshold = 1, spSettings.SP1
	}
	if level == 0 {
		return total, nil, nil
	}

	appName, err := s.branding.BrandingAppName(ctx, schoolID)
	if err != nil {
		appName = "NouSchool"
	}
	snapViolations := make([]LetterSnapshotViolation, 0, len(violations))
	for _, v := range violations {
		snapViolations = append(snapViolations, LetterSnapshotViolation{
			Name: v.ViolationName, Points: v.ViolationPoints, Date: v.OccurredOn.Format("2006-01-02"),
		})
	}
	issuedOn := schoolToday(s.clock.Now(), schoolTimezone(ctx))
	snapshot := LetterSnapshot{
		Student:    LetterSnapshotStudent{Name: studentName, NIS: studentNIS, ClassName: className},
		Violations: snapViolations,
		Total:      total,
		Threshold:  threshold,
		SchoolName: appName,
		IssuedOn:   issuedOn.Format("2006-01-02"),
	}
	snapshotJSON, err := json.Marshal(snapshot)
	if err != nil {
		return total, nil, err
	}

	year := fmt.Sprintf("%d", s.clock.Now().Year())
	number, err := s.repo.NextLetterNumber(ctx, schoolID, year, level)
	if err != nil {
		return total, nil, err
	}

	letter, err := s.repo.CreateWarningLetter(ctx, CreateLetterInput{
		SchoolID: schoolID, AcademicYearID: yearID, StudentID: studentID, Level: level, Number: number,
		PointsSnapshot: total, Snapshot: snapshotJSON, CreatedBy: actorUserID,
	})
	if err != nil {
		if errors.Is(err, ErrConflict) {
			// Race jarang: level ini sudah diterbitkan konkuren (dua request
			// bersamaan) — bukan kegagalan, cukup anggap tidak ada surat baru
			// dari request INI (unique constraint sudah menegakkan "sekali per
			// level per TA per siswa").
			return total, nil, nil
		}
		return total, nil, err
	}

	s.audit(ctx, schoolID, actorUserID, "discipline.sp_issued", "discipline_warning_letter", letter.ID, nil,
		map[string]any{"student_id": studentID, "level": level, "number": number, "total_points": total, "threshold": threshold})

	recipients, err := s.students.GuardianUserIDs(ctx, schoolID, studentID)
	if err != nil {
		recipients = nil
	}
	if homeroomUserID, ok, herr := s.repo.HomeroomTeacherUserID(ctx, schoolID, yearID, studentID); herr == nil && ok {
		recipients = append(recipients, homeroomUserID)
	}
	if len(recipients) > 0 {
		_ = s.notifierNotify(ctx, schoolID, EventDisciplineSPIssued, recipients, map[string]any{
			"student": studentName, "level": level,
		})
	}

	return total, &IssuedLetterRef{ID: letter.ID, Level: level, Number: number}, nil
}

// -- DELETE /api/discipline/records/{id} --

// DeleteRecord menghapus HANYA catatan pelanggaran — surat yang sudah
// terbit TIDAK disentuh (snapshot immutable, docs/12-sion-parity.md).
func (s *Service) DeleteRecord(ctx context.Context, actorUserID, schoolID, id int64) error {
	if err := s.requireManage(ctx); err != nil {
		return err
	}
	old, err := s.repo.GetRecordByID(ctx, schoolID, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return httpx.ErrNotFound
		}
		return err
	}
	rows, err := s.repo.DeleteStudentViolation(ctx, schoolID, id)
	if err != nil {
		return err
	}
	if rows == 0 {
		return httpx.ErrNotFound
	}
	s.audit(ctx, schoolID, actorUserID, "discipline.record_delete", "student_violation", id,
		map[string]any{
			"student_id": old.StudentID, "violation_type": old.ViolationName, "points": old.ViolationPoints,
			"occurred_on": old.OccurredOn.Format("2006-01-02"),
		}, nil)
	return nil
}

// -- GET /api/discipline/records --

// ListRecordsQuery adalah parameter Service.ListRecords.
type ListRecordsQuery struct {
	StudentID int64
	ClassID   int64
	From      string
	To        string
	Page      int
}

const recordPageSize = 20

func (s *Service) ListRecords(ctx context.Context, schoolID int64, q ListRecordsQuery) (RecordPage, error) {
	if !s.identity.HasPermission(reqctx.Role(ctx), PermDisciplineRead) {
		return RecordPage{}, httpx.ErrForbidden
	}
	yearID, err := s.requireActiveYear(ctx, schoolID)
	if err != nil {
		return RecordPage{}, err
	}
	var from, to time.Time
	if strings.TrimSpace(q.From) != "" {
		from, err = parseDate(q.From)
		if err != nil {
			return RecordPage{}, err
		}
	}
	if strings.TrimSpace(q.To) != "" {
		to, err = parseDate(q.To)
		if err != nil {
			return RecordPage{}, err
		}
	}
	if !from.IsZero() && !to.IsZero() && from.After(to) {
		return RecordPage{}, httpx.Validation("'from' tidak boleh setelah 'to'.")
	}
	page := q.Page
	if page < 1 {
		page = 1
	}
	details, total, err := s.repo.ListStudentViolations(ctx, schoolID, yearID, RecordFilter{
		StudentID: q.StudentID, ClassID: q.ClassID, From: from, To: to, Limit: recordPageSize, Offset: (page - 1) * recordPageSize,
	})
	if err != nil {
		return RecordPage{}, err
	}
	items := make([]RecordView, 0, len(details))
	for _, d := range details {
		items = append(items, d.View())
	}
	return RecordPage{Items: items, Total: total}, nil
}

// -- GET /api/students/{id}/discipline --

// spLevelReached — level SP TERTINGGI yang ambangnya sudah terlampaui total
// poin (TERLEPAS dari apakah suratnya sudah terbit atau belum — murni
// informasi posisi siswa terhadap ambang, dipakai UI "indikator level").
func spLevelReached(total int, sp SPSettings) int {
	switch {
	case total >= sp.SP3:
		return 3
	case total >= sp.SP2:
		return 2
	case total >= sp.SP1:
		return 1
	default:
		return 0
	}
}

func (s *Service) StudentDiscipline(ctx context.Context, schoolID, studentID int64) (StudentDisciplineView, error) {
	role := reqctx.Role(ctx)
	if !s.identity.HasPermission(role, PermDisciplineRead) {
		userID := reqctx.UserID(ctx)
		if err := s.students.CanViewStudent(ctx, userID, role, schoolID, studentID); err != nil {
			return StudentDisciplineView{}, err
		}
	}
	yearID, err := s.requireActiveYear(ctx, schoolID)
	if err != nil {
		return StudentDisciplineView{}, err
	}
	studentName, studentNIS, ok, err := s.repo.StudentExists(ctx, schoolID, studentID)
	if err != nil {
		return StudentDisciplineView{}, err
	}
	if !ok {
		return StudentDisciplineView{}, httpx.ErrNotFound
	}
	className, err := s.repo.StudentClassName(ctx, schoolID, yearID, studentID)
	if err != nil {
		return StudentDisciplineView{}, err
	}

	violations, err := s.repo.ListViolationsForStudentYear(ctx, schoolID, yearID, studentID)
	if err != nil {
		return StudentDisciplineView{}, err
	}
	total := 0
	records := make([]RecordView, 0, len(violations))
	for i := len(violations) - 1; i >= 0; i-- { // paling baru dulu (query mengembalikan ASC)
		v := violations[i]
		total += v.ViolationPoints
		var sessionID *int64
		if v.AttendanceSessionID != 0 {
			id := v.AttendanceSessionID
			sessionID = &id
		}
		records = append(records, RecordView{
			ID:                  v.ID,
			Student:             StudentRef{ID: studentID, Name: studentName, NIS: studentNIS, ClassName: className},
			Violation:           ViolationRef{ID: v.ViolationTypeID, Name: v.ViolationName, Points: v.ViolationPoints},
			NotedByName:         v.NotedByName,
			Note:                v.Note,
			OccurredOn:          NewDate(v.OccurredOn),
			AttendanceSessionID: sessionID,
		})
	}

	spSettings, err := s.getSPSettingsOrDefault(ctx, schoolID, yearID)
	if err != nil {
		return StudentDisciplineView{}, err
	}

	lettersRaw, err := s.repo.ListWarningLettersForStudentYear(ctx, schoolID, yearID, studentID)
	if err != nil {
		return StudentDisciplineView{}, err
	}
	letters := make([]LetterView, 0, len(lettersRaw))
	for _, l := range lettersRaw {
		letters = append(letters, LetterView{ID: l.ID, Level: l.Level, Number: l.Number, PointsSnapshot: l.PointsSnapshot, CreatedAt: l.CreatedAt})
	}

	return StudentDisciplineView{
		TotalPoints: total, SPLevelReached: spLevelReached(total, spSettings), Records: records, Letters: letters,
	}, nil
}

// -- GET /api/discipline/letters --

const letterPageSize = 20

func (s *Service) ListLetters(ctx context.Context, schoolID int64, level, page int) (LetterPage, error) {
	if !s.identity.HasPermission(reqctx.Role(ctx), PermDisciplineRead) {
		return LetterPage{}, httpx.ErrForbidden
	}
	if page < 1 {
		page = 1
	}
	rows, total, err := s.repo.ListWarningLetters(ctx, schoolID, level, letterPageSize, (page-1)*letterPageSize)
	if err != nil {
		return LetterPage{}, err
	}
	items := make([]LetterView, 0, len(rows))
	for _, r := range rows {
		items = append(items, LetterView{
			ID: r.ID, Level: r.Level, Number: r.Number, PointsSnapshot: r.PointsSnapshot, CreatedAt: r.CreatedAt,
			Student: &StudentRef{ID: r.StudentID, Name: r.StudentName, NIS: r.StudentNIS},
		})
	}
	return LetterPage{Items: items, Total: total}, nil
}

// -- GET /api/discipline/letters/{id}.pdf & .html --

// getLetterAuthorized menegakkan otorisasi (discipline:read ATAU siswa
// pemilik/orang tua siswa pemilik surat, lewat StudentAccess.CanViewStudent
// — pola yang sama dengan attendance.StudentHistory & leave.GetAttachment).
func (s *Service) getLetterAuthorized(ctx context.Context, schoolID, letterID int64) (LetterRecord, LetterSnapshot, error) {
	letter, err := s.repo.GetWarningLetterByID(ctx, schoolID, letterID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return LetterRecord{}, LetterSnapshot{}, httpx.ErrNotFound
		}
		return LetterRecord{}, LetterSnapshot{}, err
	}
	role := reqctx.Role(ctx)
	if !s.identity.HasPermission(role, PermDisciplineRead) {
		userID := reqctx.UserID(ctx)
		if err := s.students.CanViewStudent(ctx, userID, role, schoolID, letter.StudentID); err != nil {
			return LetterRecord{}, LetterSnapshot{}, err
		}
	}
	var snap LetterSnapshot
	if err := json.Unmarshal(letter.Snapshot, &snap); err != nil {
		return LetterRecord{}, LetterSnapshot{}, err
	}
	return letter, snap, nil
}

// spFooterNote membaca school_settings module "letters" -> sp_footer_note
// (Fase 14 Gelombang D, docs/12-sion-parity.md "template surat") — "" bila
// belum diset, gateway belum disuntik, atau baris belum tersimpan (BUKAN
// error keras, catatan footer opsional murni kosmetik).
func (s *Service) spFooterNote(ctx context.Context, schoolID int64) string {
	if s.settings == nil {
		return ""
	}
	raw, found, err := s.settings.GetSetting(ctx, schoolID, "letters")
	if err != nil || !found {
		return ""
	}
	var v struct {
		SPFooterNote string `json:"sp_footer_note"`
	}
	if err := json.Unmarshal(raw, &v); err != nil {
		return ""
	}
	return v.SPFooterNote
}

// LetterPDF mengembalikan (bytes PDF, nama file, error) — DIRENDER dari
// snapshot (potret saat terbit), bukan data live — KECUALI catatan footer
// (school_settings "letters".sp_footer_note), yang SENGAJA dibaca LIVE saat
// render (bukan bagian snapshot) supaya admin bisa mengubah catatan tanpa
// perlu menerbitkan ulang surat lama.
func (s *Service) LetterPDF(ctx context.Context, schoolID, letterID int64) ([]byte, string, error) {
	letter, snap, err := s.getLetterAuthorized(ctx, schoolID, letterID)
	if err != nil {
		return nil, "", err
	}
	data, err := BuildLetterPDF(letter, snap, s.spFooterNote(ctx, schoolID))
	if err != nil {
		return nil, "", err
	}
	return data, fmt.Sprintf("Surat %s.pdf", sanitizeNumberForFilename(letter.Number)), nil
}

// LetterHTML mengembalikan HTML surat (string) — pola yang sama dengan LetterPDF.
func (s *Service) LetterHTML(ctx context.Context, schoolID, letterID int64) (string, error) {
	letter, snap, err := s.getLetterAuthorized(ctx, schoolID, letterID)
	if err != nil {
		return "", err
	}
	return RenderLetterHTML(letter, snap), nil
}

func sanitizeNumberForFilename(number string) string {
	return strings.ReplaceAll(number, "/", "-")
}

// -- GET /api/discipline/summary & /api/discipline/export --

// SummaryQuery adalah parameter Service.Summary.
type SummaryQuery struct {
	ClassID int64
	From    string
	To      string
}

func (s *Service) Summary(ctx context.Context, schoolID int64, q SummaryQuery) ([]SummaryRow, error) {
	if !s.identity.HasPermission(reqctx.Role(ctx), PermDisciplineRead) {
		return nil, httpx.ErrForbidden
	}
	yearID, err := s.requireActiveYear(ctx, schoolID)
	if err != nil {
		return nil, err
	}
	var from, to time.Time
	if strings.TrimSpace(q.From) != "" {
		from, err = parseDate(q.From)
		if err != nil {
			return nil, err
		}
	}
	if strings.TrimSpace(q.To) != "" {
		to, err = parseDate(q.To)
		if err != nil {
			return nil, err
		}
	}
	if !from.IsZero() && !to.IsZero() && from.After(to) {
		return nil, httpx.Validation("'from' tidak boleh setelah 'to'.")
	}
	rows, err := s.repo.DisciplineSummary(ctx, schoolID, yearID, q.ClassID, from, to)
	if err != nil {
		return nil, err
	}
	out := make([]SummaryRow, 0, len(rows))
	for _, r := range rows {
		row := SummaryRow{
			Student:     StudentRef{ID: r.StudentID, Name: r.StudentName, NIS: r.StudentNIS, ClassName: r.ClassName},
			TotalPoints: r.TotalPoints, RecordCount: r.RecordCount,
		}
		if !r.LastAt.IsZero() {
			d := NewDate(r.LastAt)
			row.LastAt = &d
		}
		out = append(out, row)
	}
	return out, nil
}
