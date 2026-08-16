package discipline

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/omanjaya/nouschool/internal/platform/clock"
	"github.com/omanjaya/nouschool/internal/platform/httpx"
	"github.com/omanjaya/nouschool/internal/platform/reqctx"
)

// -- fake repository (in-memory, tanpa DB) --

type violationKey struct{ sessionID, studentID, typeID int64 }

type storedViolation struct {
	CreateViolationInput
	ID        int64
	CreatedAt time.Time
}

type fakeStudentRow struct{ name, nis string }

type fakeRepo struct {
	types      map[int64]ViolationTypeRecord
	nextTypeID int64

	violations      map[int64]storedViolation
	nextViolationID int64
	sessionKeys     map[violationKey]bool

	sp map[[2]int64]SPSettings // key: [schoolID, academicYearID]

	letters      map[int64]LetterRecord
	nextLetterID int64
	letterSeq    map[string]int // key: "schoolID|year"

	students map[int64]fakeStudentRow
	sessions map[int64]bool
	homeroom map[[3]int64]int64  // [schoolID, yearID, studentID] -> homeroom teacher user_id
	classNm  map[[3]int64]string // [schoolID, yearID, studentID] -> class name
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		types:       map[int64]ViolationTypeRecord{},
		violations:  map[int64]storedViolation{},
		sessionKeys: map[violationKey]bool{},
		sp:          map[[2]int64]SPSettings{},
		letters:     map[int64]LetterRecord{},
		letterSeq:   map[string]int{},
		students:    map[int64]fakeStudentRow{},
		sessions:    map[int64]bool{},
		homeroom:    map[[3]int64]int64{},
		classNm:     map[[3]int64]string{},
	}
}

var _ disciplineRepository = (*fakeRepo)(nil)

func (f *fakeRepo) CreateViolationType(ctx context.Context, schoolID int64, name string, points int, category string) (ViolationTypeRecord, error) {
	for _, t := range f.types {
		if t.SchoolID == schoolID && t.Name == name {
			return ViolationTypeRecord{}, ErrConflict
		}
	}
	f.nextTypeID++
	rec := ViolationTypeRecord{ID: f.nextTypeID, SchoolID: schoolID, Name: name, Points: points, Category: category, Active: true}
	f.types[rec.ID] = rec
	return rec, nil
}

func (f *fakeRepo) UpdateViolationType(ctx context.Context, schoolID, id int64, name *string, points *int, category *string, active *bool) (ViolationTypeRecord, error) {
	rec, ok := f.types[id]
	if !ok || rec.SchoolID != schoolID {
		return ViolationTypeRecord{}, ErrNotFound
	}
	if name != nil {
		for _, t := range f.types {
			if t.ID != id && t.SchoolID == schoolID && t.Name == *name {
				return ViolationTypeRecord{}, ErrConflict
			}
		}
		rec.Name = *name
	}
	if points != nil {
		rec.Points = *points
	}
	if category != nil {
		rec.Category = *category
	}
	if active != nil {
		rec.Active = *active
	}
	f.types[id] = rec
	return rec, nil
}

func (f *fakeRepo) GetViolationTypeByID(ctx context.Context, schoolID, id int64) (ViolationTypeRecord, error) {
	rec, ok := f.types[id]
	if !ok || rec.SchoolID != schoolID {
		return ViolationTypeRecord{}, ErrNotFound
	}
	return rec, nil
}

func (f *fakeRepo) ListViolationTypes(ctx context.Context, schoolID int64) ([]ViolationTypeRecord, error) {
	var out []ViolationTypeRecord
	for _, t := range f.types {
		if t.SchoolID == schoolID {
			out = append(out, t)
		}
	}
	return out, nil
}

func (f *fakeRepo) DeleteViolationType(ctx context.Context, schoolID, id int64) (int64, error) {
	rec, ok := f.types[id]
	if !ok || rec.SchoolID != schoolID {
		return 0, nil
	}
	delete(f.types, id)
	return 1, nil
}

func (f *fakeRepo) CountViolationsByType(ctx context.Context, schoolID, violationTypeID int64) (int64, error) {
	var n int64
	for _, v := range f.violations {
		if v.SchoolID == schoolID && v.ViolationTypeID == violationTypeID {
			n++
		}
	}
	return n, nil
}

func (f *fakeRepo) CreateStudentViolation(ctx context.Context, in CreateViolationInput) (int64, error) {
	if in.AttendanceSessionID != 0 {
		k := violationKey{in.AttendanceSessionID, in.StudentID, in.ViolationTypeID}
		if f.sessionKeys[k] {
			return 0, ErrConflict
		}
		f.sessionKeys[k] = true
	}
	f.nextViolationID++
	f.violations[f.nextViolationID] = storedViolation{CreateViolationInput: in, ID: f.nextViolationID, CreatedAt: time.Now()}
	return f.nextViolationID, nil
}

func (f *fakeRepo) detailFor(v storedViolation) RecordDetail {
	vt := f.types[v.ViolationTypeID]
	st := f.students[v.StudentID]
	className := f.classNm[[3]int64{v.SchoolID, v.AcademicYearID, v.StudentID}]
	return RecordDetail{
		ID: v.ID, OccurredOn: v.OccurredOn, Note: v.Note, AttendanceSessionID: v.AttendanceSessionID,
		CreatedAt: v.CreatedAt, StudentID: v.StudentID, StudentName: st.name, StudentNIS: st.nis, ClassName: className,
		ViolationTypeID: v.ViolationTypeID, ViolationName: vt.Name, ViolationPoints: vt.Points,
		NotedBy: v.NotedBy, NotedByName: "Guru Uji",
	}
}

func (f *fakeRepo) GetRecordByID(ctx context.Context, schoolID, id int64) (RecordDetail, error) {
	v, ok := f.violations[id]
	if !ok || v.SchoolID != schoolID {
		return RecordDetail{}, ErrNotFound
	}
	return f.detailFor(v), nil
}

func (f *fakeRepo) DeleteStudentViolation(ctx context.Context, schoolID, id int64) (int64, error) {
	v, ok := f.violations[id]
	if !ok || v.SchoolID != schoolID {
		return 0, nil
	}
	delete(f.violations, id)
	return 1, nil
}

func (f *fakeRepo) ListStudentViolations(ctx context.Context, schoolID, activeYearID int64, filter RecordFilter) ([]RecordDetail, int64, error) {
	var out []RecordDetail
	for _, v := range f.violations {
		if v.SchoolID != schoolID {
			continue
		}
		if filter.StudentID != 0 && v.StudentID != filter.StudentID {
			continue
		}
		out = append(out, f.detailFor(v))
	}
	return out, int64(len(out)), nil
}

func (f *fakeRepo) ListViolationsForStudentYear(ctx context.Context, schoolID, academicYearID, studentID int64) ([]ViolationForYear, error) {
	var out []ViolationForYear
	for _, v := range f.violations {
		if v.SchoolID == schoolID && v.AcademicYearID == academicYearID && v.StudentID == studentID {
			vt := f.types[v.ViolationTypeID]
			out = append(out, ViolationForYear{
				ID: v.ID, OccurredOn: v.OccurredOn, Note: v.Note, AttendanceSessionID: v.AttendanceSessionID,
				CreatedAt: v.CreatedAt, ViolationTypeID: v.ViolationTypeID, ViolationName: vt.Name, ViolationPoints: vt.Points,
				NotedBy: v.NotedBy, NotedByName: "Guru Uji",
			})
		}
	}
	return out, nil
}

func (f *fakeRepo) GetSPSettings(ctx context.Context, schoolID, academicYearID int64) (SPSettings, bool, error) {
	s, ok := f.sp[[2]int64{schoolID, academicYearID}]
	return s, ok, nil
}

func (f *fakeRepo) UpsertSPSettings(ctx context.Context, schoolID, academicYearID int64, in SPSettings) (SPSettings, error) {
	f.sp[[2]int64{schoolID, academicYearID}] = in
	return in, nil
}

func (f *fakeRepo) NextLetterNumber(ctx context.Context, schoolID int64, year string, level int) (string, error) {
	key := fmt.Sprintf("%d|%s", schoolID, year)
	f.letterSeq[key]++
	return formatLetterNumber(level, year, f.letterSeq[key]), nil
}

func (f *fakeRepo) CreateWarningLetter(ctx context.Context, in CreateLetterInput) (LetterRecord, error) {
	for _, l := range f.letters {
		if l.SchoolID == in.SchoolID && l.AcademicYearID == in.AcademicYearID && l.StudentID == in.StudentID && l.Level == in.Level {
			return LetterRecord{}, ErrConflict
		}
	}
	f.nextLetterID++
	rec := LetterRecord{
		ID: f.nextLetterID, SchoolID: in.SchoolID, AcademicYearID: in.AcademicYearID, StudentID: in.StudentID,
		Level: in.Level, Number: in.Number, PointsSnapshot: in.PointsSnapshot, Snapshot: in.Snapshot,
		CreatedBy: in.CreatedBy, CreatedAt: time.Now(),
	}
	f.letters[rec.ID] = rec
	return rec, nil
}

func (f *fakeRepo) GetWarningLetterByID(ctx context.Context, schoolID, id int64) (LetterRecord, error) {
	rec, ok := f.letters[id]
	if !ok || rec.SchoolID != schoolID {
		return LetterRecord{}, ErrNotFound
	}
	return rec, nil
}

func (f *fakeRepo) ListWarningLettersForStudentYear(ctx context.Context, schoolID, academicYearID, studentID int64) ([]LetterRecord, error) {
	var out []LetterRecord
	for _, l := range f.letters {
		if l.SchoolID == schoolID && l.AcademicYearID == academicYearID && l.StudentID == studentID {
			out = append(out, l)
		}
	}
	return out, nil
}

func (f *fakeRepo) ListWarningLetters(ctx context.Context, schoolID int64, level int, limit, offset int) ([]LetterListRecord, int64, error) {
	var out []LetterListRecord
	for _, l := range f.letters {
		if l.SchoolID != schoolID {
			continue
		}
		if level != 0 && l.Level != level {
			continue
		}
		st := f.students[l.StudentID]
		out = append(out, LetterListRecord{LetterRecord: l, StudentName: st.name, StudentNIS: st.nis})
	}
	return out, int64(len(out)), nil
}

func (f *fakeRepo) DisciplineSummary(ctx context.Context, schoolID, activeYearID int64, classID int64, from, to time.Time) ([]SummaryRecord, error) {
	byStudent := map[int64]*SummaryRecord{}
	for _, v := range f.violations {
		if v.SchoolID != schoolID {
			continue
		}
		vt := f.types[v.ViolationTypeID]
		st := f.students[v.StudentID]
		rec, ok := byStudent[v.StudentID]
		if !ok {
			rec = &SummaryRecord{StudentID: v.StudentID, StudentName: st.name, StudentNIS: st.nis}
			byStudent[v.StudentID] = rec
		}
		rec.TotalPoints += vt.Points
		rec.RecordCount++
		if v.OccurredOn.After(rec.LastAt) {
			rec.LastAt = v.OccurredOn
		}
	}
	var out []SummaryRecord
	for _, r := range byStudent {
		out = append(out, *r)
	}
	return out, nil
}

func (f *fakeRepo) StudentExists(ctx context.Context, schoolID, studentID int64) (string, string, bool, error) {
	st, ok := f.students[studentID]
	if !ok {
		return "", "", false, nil
	}
	return st.name, st.nis, true, nil
}

func (f *fakeRepo) AttendanceSessionExists(ctx context.Context, schoolID, sessionID int64) (bool, error) {
	return f.sessions[sessionID], nil
}

func (f *fakeRepo) HomeroomTeacherUserID(ctx context.Context, schoolID, academicYearID, studentID int64) (int64, bool, error) {
	id, ok := f.homeroom[[3]int64{schoolID, academicYearID, studentID}]
	return id, ok, nil
}

func (f *fakeRepo) StudentClassName(ctx context.Context, schoolID, academicYearID, studentID int64) (string, error) {
	return f.classNm[[3]int64{schoolID, academicYearID, studentID}], nil
}

// -- fake consumer-side dependencies --

type fakeIdentity struct{}

// perms — mirror internal/identity/rbac.go rolePermissions untuk 3 permission
// modul ini (docs/02-identity.md).
var perms = map[string]map[string]bool{
	RoleAdminSekolah:  {PermDisciplineManage: true, PermDisciplineRecord: true, PermDisciplineRead: true},
	RoleKepalaSekolah: {PermDisciplineRead: true},
	RoleGuru:          {PermDisciplineRecord: true, PermDisciplineRead: true},
}

func (fakeIdentity) HasPermission(role, perm string) bool { return perms[role][perm] }
func (fakeIdentity) Log(ctx context.Context, schoolID, userID *int64, action, entity string, entityID *int64, oldValue, newValue any) error {
	return nil
}

type fakeYears struct{ id int64 }

func (f fakeYears) ActiveAcademicYearID(ctx context.Context, schoolID int64) (int64, bool, error) {
	return f.id, true, nil
}

type fakeBranding struct{}

func (fakeBranding) BrandingAppName(ctx context.Context, schoolID int64) (string, error) {
	return "Sekolah Uji", nil
}

type fakeStudentAccess struct {
	guardianOf    map[int64][]int64
	studentUserOf map[int64]int64
}

func (f fakeStudentAccess) CanViewStudent(ctx context.Context, userID int64, role string, schoolID, studentID int64) error {
	switch role {
	case "siswa":
		if f.studentUserOf[studentID] == userID {
			return nil
		}
	case "orang_tua":
		for _, g := range f.guardianOf[studentID] {
			if g == userID {
				return nil
			}
		}
	}
	return httpx.ErrForbidden
}

func (f fakeStudentAccess) GuardianUserIDs(ctx context.Context, schoolID, studentID int64) ([]int64, error) {
	return f.guardianOf[studentID], nil
}

// -- fake SettingsGateway (Fase 14 Gelombang D: footer surat, school_settings
// module "letters") --

type fakeSettingsGateway struct {
	raw   map[string][]byte
	found map[string]bool
	err   error
}

func (f fakeSettingsGateway) GetSetting(ctx context.Context, schoolID int64, module string) ([]byte, bool, error) {
	if f.err != nil {
		return nil, false, f.err
	}
	return f.raw[module], f.found[module], nil
}

func TestSPFooterNote_RenderedWhenSet(t *testing.T) {
	svc := newTestService(newFakeRepo(), fakeStudentAccess{})
	svc.SetSettingsGateway(fakeSettingsGateway{
		raw:   map[string][]byte{"letters": []byte(`{"sp_footer_note":"Catatan dari sekolah","leave_footer_note":""}`)},
		found: map[string]bool{"letters": true},
	})
	ctx := ctxAs("admin_sekolah", 1)
	if got := svc.spFooterNote(ctx, testSchoolID); got != "Catatan dari sekolah" {
		t.Fatalf("expected catatan terbaca dari settings, got %q", got)
	}
}

func TestSPFooterNote_EmptyWhenGatewayNilOrNotFound(t *testing.T) {
	svc := newTestService(newFakeRepo(), fakeStudentAccess{}) // gateway TIDAK di-set (nil)
	ctx := ctxAs("admin_sekolah", 1)
	if got := svc.spFooterNote(ctx, testSchoolID); got != "" {
		t.Fatalf("gateway nil harus menghasilkan string kosong, got %q", got)
	}

	svc.SetSettingsGateway(fakeSettingsGateway{found: map[string]bool{}})
	if got := svc.spFooterNote(ctx, testSchoolID); got != "" {
		t.Fatalf("module belum pernah tersimpan harus menghasilkan string kosong, got %q", got)
	}
}

func TestBuildLetterPDF_FooterNoteIncreasesOutputWhenSet(t *testing.T) {
	letter := LetterRecord{Level: 1, Number: "SP1/2026/0001"}
	snap := LetterSnapshot{
		Student:    LetterSnapshotStudent{Name: "Budi", NIS: "22103", ClassName: "XII RPL 1"},
		SchoolName: "NouSchool", Total: 15, Threshold: 10, IssuedOn: "2026-08-16",
	}
	without, err := BuildLetterPDF(letter, snap, "")
	if err != nil {
		t.Fatalf("unexpected error tanpa footer: %v", err)
	}
	if len(without) < 4 || string(without[:4]) != "%PDF" {
		t.Fatal("output harus PDF valid (header %PDF)")
	}
	with, err := BuildLetterPDF(letter, snap, "Catatan tambahan dari sekolah mengenai kedisiplinan.")
	if err != nil {
		t.Fatalf("unexpected error dengan footer: %v", err)
	}
	if len(with) < 4 || string(with[:4]) != "%PDF" {
		t.Fatal("output DENGAN footer harus tetap PDF valid")
	}
	if len(with) <= len(without) {
		t.Fatalf("PDF dengan footer harus lebih besar dari tanpa footer (footer benar-benar dirender): %d vs %d", len(with), len(without))
	}
}

// -- helpers --

const testYearID = 1
const testSchoolID = 1

func ctxAs(role string, userID int64) context.Context {
	ctx := reqctx.WithUser(context.Background(), userID, role, false)
	ctx = reqctx.WithSchool(ctx, reqctx.School{ID: testSchoolID, Name: "Sekolah Uji", Slug: "uji", Timezone: "Asia/Jakarta"})
	return ctx
}

func newTestService(repo *fakeRepo, students fakeStudentAccess) *Service {
	return newServiceForTest(repo, fakeIdentity{}, fakeYears{id: testYearID}, fakeBranding{}, students,
		clock.Fixed{T: time.Date(2026, 8, 16, 8, 0, 0, 0, time.UTC)})
}

func seedStudent(f *fakeRepo, id int64, name, nis, className string) {
	f.students[id] = fakeStudentRow{name: name, nis: nis}
	f.classNm[[3]int64{testSchoolID, testYearID, id}] = className
}

func seedType(t *testing.T, svc *Service, name string, points int) int64 {
	t.Helper()
	vt, err := svc.CreateViolationType(ctxAs(RoleAdminSekolah, 1), 1, testSchoolID, CreateViolationTypeInput{Name: name, Points: points})
	if err != nil {
		t.Fatalf("seedType %s: %v", name, err)
	}
	return vt.ID
}

func setSPSettings(t *testing.T, svc *Service, sp1, sp2, sp3 int) {
	t.Helper()
	if _, err := svc.PutSPSettings(ctxAs(RoleAdminSekolah, 1), 1, testSchoolID, SPSettings{SP1: sp1, SP2: sp2, SP3: sp3}); err != nil {
		t.Fatalf("setSPSettings: %v", err)
	}
}

// -- tests --

func TestAutoIssueLetter_OncePerLevel(t *testing.T) {
	repo := newFakeRepo()
	students := fakeStudentAccess{guardianOf: map[int64][]int64{}}
	svc := newTestService(repo, students)
	seedStudent(repo, 10, "Budi", "22101", "XII RPL 1")
	typeID := seedType(t, svc, "Terlambat", 15)
	setSPSettings(t, svc, 10, 20, 30)

	res1, err := svc.RecordViolation(ctxAs(RoleGuru, 2), 2, testSchoolID, RecordInput{StudentID: 10, ViolationTypeID: typeID})
	if err != nil {
		t.Fatalf("record 1: %v", err)
	}
	if res1.TotalPoints != 15 || res1.IssuedLetter == nil || res1.IssuedLetter.Level != 1 {
		t.Fatalf("record 1: total=%d issued=%+v (mau total=15, level 1 terbit)", res1.TotalPoints, res1.IssuedLetter)
	}

	// Record kedua: total 30 >= sp1(10) TAPI level1 SUDAH terbit -> TIDAK
	// terbit lagi utk level1 (ambang sama tidak menerbitkan ulang).
	// total 30 juga >= sp3(30) -> level3 seharusnya terbit (level tertinggi
	// belum ada surat).
	res2, err := svc.RecordViolation(ctxAs(RoleGuru, 2), 2, testSchoolID, RecordInput{StudentID: 10, ViolationTypeID: typeID})
	if err != nil {
		t.Fatalf("record 2: %v", err)
	}
	if res2.TotalPoints != 30 || res2.IssuedLetter == nil || res2.IssuedLetter.Level != 3 {
		t.Fatalf("record 2: total=%d issued=%+v (mau total=30, level 3 terbit)", res2.TotalPoints, res2.IssuedLetter)
	}

	// Record ketiga: total 45. Level3 SUDAH terbit (skip), TAPI level2
	// (20) masih terlampaui DAN belum pernah punya surat (dilewati saat
	// lompat di record kedua) -> evaluasi ULANG setiap kali RecordViolation
	// dipanggil berarti level2 "menyusul" terbit di sini. Perilaku ini
	// KONSISTEN dengan aturan "terbitkan level tertinggi yang terlampaui &
	// belum ada surat" diterapkan pada SETIAP pemanggilan (bukan kejadian
	// satu kali) — dilaporkan sebagai keputusan turunan.
	res3, err := svc.RecordViolation(ctxAs(RoleGuru, 2), 2, testSchoolID, RecordInput{StudentID: 10, ViolationTypeID: typeID})
	if err != nil {
		t.Fatalf("record 3: %v", err)
	}
	if res3.TotalPoints != 45 || res3.IssuedLetter == nil || res3.IssuedLetter.Level != 2 {
		t.Fatalf("record 3: total=%d issued=%+v (mau total=45, level 2 'menyusul' terbit)", res3.TotalPoints, res3.IssuedLetter)
	}

	// Record keempat: total 60, SEMUA level (1/2/3) sudah punya surat ->
	// TIDAK ada surat baru lagi ("record berikutnya di atas ambang sama
	// TIDAK menerbitkan lagi").
	res4, err := svc.RecordViolation(ctxAs(RoleGuru, 2), 2, testSchoolID, RecordInput{StudentID: 10, ViolationTypeID: typeID})
	if err != nil {
		t.Fatalf("record 4: %v", err)
	}
	if res4.TotalPoints != 60 || res4.IssuedLetter != nil {
		t.Fatalf("record 4: total=%d issued=%+v (mau total=60, TIDAK ada surat baru — semua level sudah terbit)", res4.TotalPoints, res4.IssuedLetter)
	}

	letters, err := repo.ListWarningLettersForStudentYear(context.Background(), testSchoolID, testYearID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(letters) != 3 {
		t.Fatalf("mau tepat 3 surat (level 1, 2, 3 — masing-masing sekali), dapat %d", len(letters))
	}
}

func TestAutoIssueLetter_LevelJump_HanyaLevelTertinggi(t *testing.T) {
	repo := newFakeRepo()
	students := fakeStudentAccess{guardianOf: map[int64][]int64{}}
	svc := newTestService(repo, students)
	seedStudent(repo, 11, "Sari", "22102", "XI RPL 2")
	typeID := seedType(t, svc, "Berkelahi", 100)
	setSPSettings(t, svc, 10, 20, 30)

	res, err := svc.RecordViolation(ctxAs(RoleGuru, 2), 2, testSchoolID, RecordInput{StudentID: 11, ViolationTypeID: typeID})
	if err != nil {
		t.Fatalf("record: %v", err)
	}
	if res.IssuedLetter == nil || res.IssuedLetter.Level != 3 {
		t.Fatalf("lompat langsung > sp3 dlm 1 catatan: mau HANYA level 3 terbit, dapat %+v", res.IssuedLetter)
	}
	letters, _ := repo.ListWarningLettersForStudentYear(context.Background(), testSchoolID, testYearID, 11)
	if len(letters) != 1 {
		t.Fatalf("mau tepat 1 surat (level 3 saja, level 1 & 2 TIDAK ikut terbit), dapat %d surat", len(letters))
	}
	if letters[0].Level != 3 {
		t.Fatalf("surat yang terbit harus level 3, dapat level %d", letters[0].Level)
	}
}

func TestLetterNumber_Sequential(t *testing.T) {
	repo := newFakeRepo()
	students := fakeStudentAccess{guardianOf: map[int64][]int64{}}
	svc := newTestService(repo, students)
	seedStudent(repo, 20, "Andi", "22110", "X A")
	seedStudent(repo, 21, "Beni", "22111", "X A")
	typeID := seedType(t, svc, "Merokok", 50)
	setSPSettings(t, svc, 10, 20, 30)

	res1, err := svc.RecordViolation(ctxAs(RoleGuru, 2), 2, testSchoolID, RecordInput{StudentID: 20, ViolationTypeID: typeID})
	if err != nil || res1.IssuedLetter == nil {
		t.Fatalf("siswa 1: %v %+v", err, res1.IssuedLetter)
	}
	res2, err := svc.RecordViolation(ctxAs(RoleGuru, 2), 2, testSchoolID, RecordInput{StudentID: 21, ViolationTypeID: typeID})
	if err != nil || res2.IssuedLetter == nil {
		t.Fatalf("siswa 2: %v %+v", err, res2.IssuedLetter)
	}
	// clock.Fixed di newTestService = 2026 — nomor surat berdasar tahun clock,
	// bukan time.Now() (lihat Service.evaluateAndIssueLetter).
	want1 := formatLetterNumber(3, "2026", 1)
	want2 := formatLetterNumber(3, "2026", 2)
	if res1.IssuedLetter.Number != want1 {
		t.Fatalf("nomor surat 1 = %q, mau %q", res1.IssuedLetter.Number, want1)
	}
	if res2.IssuedLetter.Number != want2 {
		t.Fatalf("nomor surat 2 = %q, mau %q", res2.IssuedLetter.Number, want2)
	}
}

func TestRecordViolation_UniquePerSession(t *testing.T) {
	repo := newFakeRepo()
	students := fakeStudentAccess{guardianOf: map[int64][]int64{}}
	svc := newTestService(repo, students)
	seedStudent(repo, 30, "Citra", "22120", "X B")
	repo.sessions[500] = true
	typeID := seedType(t, svc, "Atribut tidak lengkap", 10)
	setSPSettings(t, svc, 100, 200, 300) // ambang tinggi, tidak relevan test ini

	in := RecordInput{StudentID: 30, ViolationTypeID: typeID, AttendanceSessionID: 500}
	if _, err := svc.RecordViolation(ctxAs(RoleGuru, 2), 2, testSchoolID, in); err != nil {
		t.Fatalf("catatan pertama harus sukses: %v", err)
	}
	_, err := svc.RecordViolation(ctxAs(RoleGuru, 2), 2, testSchoolID, in)
	if err == nil {
		t.Fatal("catatan kedua (sesi+siswa+jenis SAMA) harus ditolak, malah sukses")
	}
	var herr *httpx.Error
	if !errors.As(err, &herr) || herr.Status != 422 {
		t.Fatalf("mau 422 validation, dapat %v", err)
	}
}

func TestStudentDiscipline_ObjectLevel_GuardianOtherChild403(t *testing.T) {
	repo := newFakeRepo()
	seedStudent(repo, 40, "Dedi", "22130", "X C")
	seedStudent(repo, 41, "Eka", "22131", "X C")
	students := fakeStudentAccess{
		guardianOf: map[int64][]int64{40: {900}}, // user 900 = ortu siswa 40 SAJA
	}
	svc := newTestService(repo, students)

	// Ortu (user 900) boleh lihat anaknya sendiri (siswa 40).
	if _, err := svc.StudentDiscipline(ctxAs("orang_tua", 900), testSchoolID, 40); err != nil {
		t.Fatalf("ortu lihat anak sendiri harus 200, dapat %v", err)
	}
	// Ortu yang SAMA mencoba lihat siswa 41 (BUKAN anaknya) -> 403.
	_, err := svc.StudentDiscipline(ctxAs("orang_tua", 900), testSchoolID, 41)
	if !errors.Is(err, httpx.ErrForbidden) {
		t.Fatalf("ortu lihat anak orang lain harus 403, dapat %v", err)
	}
}

func TestDeleteViolationType_InUse409(t *testing.T) {
	repo := newFakeRepo()
	students := fakeStudentAccess{guardianOf: map[int64][]int64{}}
	svc := newTestService(repo, students)
	seedStudent(repo, 50, "Fajar", "22140", "X D")
	typeID := seedType(t, svc, "Membolos", 15)
	setSPSettings(t, svc, 100, 200, 300)

	if _, err := svc.RecordViolation(ctxAs(RoleGuru, 2), 2, testSchoolID, RecordInput{StudentID: 50, ViolationTypeID: typeID}); err != nil {
		t.Fatalf("record: %v", err)
	}

	err := svc.DeleteViolationType(ctxAs(RoleAdminSekolah, 1), 1, testSchoolID, typeID)
	var herr *httpx.Error
	if !errors.As(err, &herr) || herr.Status != 409 {
		t.Fatalf("hapus jenis yang sudah dipakai harus 409, dapat %v", err)
	}
}

func TestDeleteRecord_SnapshotImmutable(t *testing.T) {
	repo := newFakeRepo()
	students := fakeStudentAccess{guardianOf: map[int64][]int64{}}
	svc := newTestService(repo, students)
	seedStudent(repo, 60, "Gita", "22150", "X E")
	typeID := seedType(t, svc, "Merokok", 50)
	setSPSettings(t, svc, 10, 20, 30)

	res, err := svc.RecordViolation(ctxAs(RoleGuru, 2), 2, testSchoolID, RecordInput{StudentID: 60, ViolationTypeID: typeID})
	if err != nil || res.IssuedLetter == nil {
		t.Fatalf("record: %v %+v", err, res.IssuedLetter)
	}
	letterID := res.IssuedLetter.ID

	before, err := repo.GetWarningLetterByID(context.Background(), testSchoolID, letterID)
	if err != nil {
		t.Fatal(err)
	}

	// Hapus catatan pelanggaran sumber.
	if err := svc.DeleteRecord(ctxAs(RoleAdminSekolah, 1), 1, testSchoolID, res.Record.ID); err != nil {
		t.Fatalf("delete record: %v", err)
	}

	after, err := repo.GetWarningLetterByID(context.Background(), testSchoolID, letterID)
	if err != nil {
		t.Fatal(err)
	}
	if after.PointsSnapshot != before.PointsSnapshot {
		t.Fatalf("points_snapshot berubah setelah record dihapus: sebelum=%d sesudah=%d (harus TETAP)", before.PointsSnapshot, after.PointsSnapshot)
	}
	if string(after.Snapshot) != string(before.Snapshot) {
		t.Fatal("snapshot jsonb berubah setelah record dihapus — harus IMMUTABLE")
	}
	var snap LetterSnapshot
	if err := json.Unmarshal(after.Snapshot, &snap); err != nil {
		t.Fatal(err)
	}
	if snap.Total != 50 {
		t.Fatalf("snapshot.total harus tetap 50 (potret saat terbit), dapat %d", snap.Total)
	}

	// Total poin LIVE (dari ListViolationsForStudentYear, sumber
	// StudentDiscipline) sekarang 0 karena record sudah dihapus — snapshot
	// tidak boleh ikut berubah (sudah dicek di atas).
	view, err := svc.StudentDiscipline(ctxAs(RoleAdminSekolah, 1), testSchoolID, 60)
	if err != nil {
		t.Fatal(err)
	}
	if view.TotalPoints != 0 {
		t.Fatalf("total poin live harus 0 setelah record dihapus, dapat %d", view.TotalPoints)
	}
	if len(view.Letters) != 1 || view.Letters[0].PointsSnapshot != 50 {
		t.Fatalf("surat tetap muncul dgn snapshot poin 50 walau record sudah dihapus, dapat %+v", view.Letters)
	}
}

func TestSPSettings_ValidationOrder(t *testing.T) {
	repo := newFakeRepo()
	students := fakeStudentAccess{guardianOf: map[int64][]int64{}}
	svc := newTestService(repo, students)
	_, err := svc.PutSPSettings(ctxAs(RoleAdminSekolah, 1), 1, testSchoolID, SPSettings{SP1: 50, SP2: 20, SP3: 30})
	if err == nil {
		t.Fatal("sp1<sp2<sp3 dilanggar harus ditolak")
	}
}

func TestGetSPSettings_DefaultWithoutRow(t *testing.T) {
	repo := newFakeRepo()
	students := fakeStudentAccess{guardianOf: map[int64][]int64{}}
	svc := newTestService(repo, students)
	got, err := svc.GetSPSettings(ctxAs(RoleAdminSekolah, 1), testSchoolID)
	if err != nil {
		t.Fatal(err)
	}
	if got.SP1 != DefaultSP1 || got.SP2 != DefaultSP2 || got.SP3 != DefaultSP3 {
		t.Fatalf("default sp-settings mau %d/%d/%d, dapat %+v", DefaultSP1, DefaultSP2, DefaultSP3, got)
	}
}

func TestRecordViolation_RequiresPermission(t *testing.T) {
	repo := newFakeRepo()
	students := fakeStudentAccess{guardianOf: map[int64][]int64{}}
	svc := newTestService(repo, students)
	seedStudent(repo, 70, "Hana", "22160", "X F")
	typeID := seedType(t, svc, "Terlambat", 5)

	// Siswa TIDAK punya discipline:record.
	_, err := svc.RecordViolation(ctxAs("siswa", 999), 999, testSchoolID, RecordInput{StudentID: 70, ViolationTypeID: typeID})
	if !errors.Is(err, httpx.ErrForbidden) {
		t.Fatalf("siswa mencatat pelanggaran harus 403, dapat %v", err)
	}
}
