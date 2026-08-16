package grading

import (
	"context"
	"errors"
	"testing"

	"github.com/omanjaya/nouschool/internal/platform/clock"
	"github.com/omanjaya/nouschool/internal/platform/httpx"
	"github.com/omanjaya/nouschool/internal/platform/reqctx"
)

// -- fake repository (in-memory, tanpa DB) --

type fakeRepo struct {
	components   map[int64]ComponentRecord
	nextCompID   int64
	grades       map[int64]map[int64]float64 // componentID -> studentID -> score
	publications map[[4]int64]bool           // school,year,class,subject
	stars        map[int64]StarRecord
	nextStarID   int64
	classes      map[int64]ClassRef
	subjects     map[int64]SubjectRef
	roster       map[int64][]RosterStudent // classID -> students
	studentBasic map[int64][2]string       // studentID -> [name, nis]
	studentClass map[[3]int64]int64        // school,year,studentID -> classID

	// -- Fase 15 GAP 1 (rapor lanjutan) --
	tpMappings   map[tpKey]TPMappingRecord
	manualScores map[manualKey]ManualScoreRecord
}

type tpKey struct{ School, Class, Subject, Component int64 }
type manualKey struct {
	School, Year, Class, Subject, Student int64
	Kind                                  string
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		components:   map[int64]ComponentRecord{},
		grades:       map[int64]map[int64]float64{},
		publications: map[[4]int64]bool{},
		stars:        map[int64]StarRecord{},
		classes:      map[int64]ClassRef{},
		subjects:     map[int64]SubjectRef{},
		roster:       map[int64][]RosterStudent{},
		studentBasic: map[int64][2]string{},
		studentClass: map[[3]int64]int64{},
	}
}

var _ gradingRepository = (*fakeRepo)(nil)

func (f *fakeRepo) CreateComponent(ctx context.Context, in CreateComponentInput) (ComponentRecord, error) {
	f.nextCompID++
	rec := ComponentRecord{
		ID: f.nextCompID, SchoolID: in.SchoolID, AcademicYearID: in.AcademicYearID, ClassID: in.ClassID, SubjectID: in.SubjectID,
		Name: in.Name, Type: in.Type, Weight: in.Weight, Kktp: in.Kktp,
	}
	f.components[rec.ID] = rec
	return rec, nil
}

func (f *fakeRepo) UpdateComponent(ctx context.Context, schoolID, id int64, name, typ *string, weight, kktp *int) (ComponentRecord, error) {
	rec, ok := f.components[id]
	if !ok || rec.SchoolID != schoolID {
		return ComponentRecord{}, ErrNotFound
	}
	if name != nil {
		rec.Name = *name
	}
	if typ != nil {
		rec.Type = *typ
	}
	if weight != nil {
		rec.Weight = *weight
	}
	if kktp != nil {
		rec.Kktp = *kktp
	}
	f.components[id] = rec
	return rec, nil
}

func (f *fakeRepo) GetComponentByID(ctx context.Context, schoolID, id int64) (ComponentRecord, error) {
	rec, ok := f.components[id]
	if !ok || rec.SchoolID != schoolID {
		return ComponentRecord{}, ErrNotFound
	}
	return rec, nil
}

func (f *fakeRepo) ListComponentsByClassSubject(ctx context.Context, schoolID, classID, subjectID int64) ([]ComponentWithCounts, error) {
	var out []ComponentWithCounts
	for _, c := range f.components {
		if c.SchoolID == schoolID && c.ClassID == classID && c.SubjectID == subjectID {
			out = append(out, ComponentWithCounts{ComponentRecord: c, GradedCount: int64(len(f.grades[c.ID])), StudentCount: int64(len(f.roster[classID]))})
		}
	}
	return out, nil
}

func (f *fakeRepo) ListComponentsRaw(ctx context.Context, schoolID, classID, subjectID int64) ([]ComponentRecord, error) {
	var out []ComponentRecord
	for _, c := range f.components {
		if c.SchoolID == schoolID && c.ClassID == classID && c.SubjectID == subjectID {
			out = append(out, c)
		}
	}
	return out, nil
}

func (f *fakeRepo) DeleteComponent(ctx context.Context, schoolID, id int64) (int64, error) {
	rec, ok := f.components[id]
	if !ok || rec.SchoolID != schoolID {
		return 0, nil
	}
	delete(f.components, id)
	delete(f.grades, id)
	return 1, nil
}

func (f *fakeRepo) UpsertGrade(ctx context.Context, schoolID, componentID, studentID int64, score float64, gradedBy int64) error {
	if f.grades[componentID] == nil {
		f.grades[componentID] = map[int64]float64{}
	}
	f.grades[componentID][studentID] = score
	return nil
}

func (f *fakeRepo) DeleteGrade(ctx context.Context, componentID, studentID int64) error {
	delete(f.grades[componentID], studentID)
	return nil
}

func (f *fakeRepo) ListGradesForComponent(ctx context.Context, componentID int64) (map[int64]float64, error) {
	out := map[int64]float64{}
	for k, v := range f.grades[componentID] {
		out[k] = v
	}
	return out, nil
}

func (f *fakeRepo) ListGradesForComponents(ctx context.Context, componentIDs []int64) (map[int64]map[int64]float64, error) {
	out := map[int64]map[int64]float64{}
	for _, cid := range componentIDs {
		if m, ok := f.grades[cid]; ok {
			cp := map[int64]float64{}
			for k, v := range m {
				cp[k] = v
			}
			out[cid] = cp
		}
	}
	return out, nil
}

func (f *fakeRepo) UpsertPublication(ctx context.Context, schoolID, academicYearID, classID, subjectID, publishedBy int64) error {
	f.publications[[4]int64{schoolID, academicYearID, classID, subjectID}] = true
	return nil
}

func (f *fakeRepo) DeletePublication(ctx context.Context, schoolID, academicYearID, classID, subjectID int64) error {
	delete(f.publications, [4]int64{schoolID, academicYearID, classID, subjectID})
	return nil
}

func (f *fakeRepo) IsPublished(ctx context.Context, schoolID, academicYearID, classID, subjectID int64) (bool, error) {
	return f.publications[[4]int64{schoolID, academicYearID, classID, subjectID}], nil
}

func (f *fakeRepo) ListPublishedSubjectsForClass(ctx context.Context, schoolID, academicYearID, classID int64) ([]int64, error) {
	var out []int64
	for k := range f.publications {
		if k[0] == schoolID && k[1] == academicYearID && k[2] == classID {
			out = append(out, k[3])
		}
	}
	return out, nil
}

func (f *fakeRepo) CreateStar(ctx context.Context, in CreateStarInput) (StarRecord, error) {
	f.nextStarID++
	rec := StarRecord{ID: f.nextStarID, StudentID: in.StudentID, GivenByUserID: in.GivenBy, Delta: in.Delta, Note: in.Note, Visibility: in.Visibility}
	f.stars[rec.ID] = rec
	return rec, nil
}

func (f *fakeRepo) GetStarByID(ctx context.Context, schoolID, id int64) (StarRecord, error) {
	rec, ok := f.stars[id]
	if !ok {
		return StarRecord{}, ErrNotFound
	}
	return rec, nil
}

func (f *fakeRepo) DeleteStar(ctx context.Context, schoolID, id int64) (int64, error) {
	if _, ok := f.stars[id]; !ok {
		return 0, nil
	}
	delete(f.stars, id)
	return 1, nil
}

func (f *fakeRepo) ListStarsForStudent(ctx context.Context, schoolID, studentID int64, onlyVisible bool) ([]StarRecord, error) {
	var out []StarRecord
	for _, r := range f.stars {
		if r.StudentID != studentID {
			continue
		}
		if onlyVisible && r.Visibility != VisibilityStudent {
			continue
		}
		out = append(out, r)
	}
	return out, nil
}

func (f *fakeRepo) ListStarTotalsForClass(ctx context.Context, schoolID, classID int64) ([]StarTotalRecord, error) {
	totals := map[int64]int{}
	for _, st := range f.roster[classID] {
		totals[st.ID] = 0
	}
	for _, r := range f.stars {
		if _, ok := totals[r.StudentID]; ok {
			totals[r.StudentID] += r.Delta
		}
	}
	var out []StarTotalRecord
	for _, st := range f.roster[classID] {
		out = append(out, StarTotalRecord{StudentID: st.ID, Name: st.Name, NIS: st.NIS, Total: totals[st.ID]})
	}
	return out, nil
}

func (f *fakeRepo) GetClassByID(ctx context.Context, schoolID, id int64) (ClassRef, bool, error) {
	c, ok := f.classes[id]
	return c, ok, nil
}

func (f *fakeRepo) GetSubjectByID(ctx context.Context, schoolID, id int64) (SubjectRef, bool, error) {
	s, ok := f.subjects[id]
	return s, ok, nil
}

func (f *fakeRepo) ListClassRoster(ctx context.Context, schoolID, classID int64) ([]RosterStudent, error) {
	return f.roster[classID], nil
}

func (f *fakeRepo) GetStudentBasic(ctx context.Context, schoolID, studentID int64) (string, string, bool, error) {
	v, ok := f.studentBasic[studentID]
	if !ok {
		return "", "", false, nil
	}
	return v[0], v[1], true, nil
}

func (f *fakeRepo) StudentClassID(ctx context.Context, schoolID, academicYearID, studentID int64) (int64, bool, error) {
	id, ok := f.studentClass[[3]int64{schoolID, academicYearID, studentID}]
	return id, ok, nil
}

func (f *fakeRepo) ListSubjectsWithComponentsForClass(ctx context.Context, schoolID, classID int64) ([]SubjectRef, error) {
	seen := map[int64]bool{}
	var out []SubjectRef
	for _, c := range f.components {
		if c.SchoolID == schoolID && c.ClassID == classID && !seen[c.SubjectID] {
			seen[c.SubjectID] = true
			if sub, ok := f.subjects[c.SubjectID]; ok {
				out = append(out, sub)
			}
		}
	}
	return out, nil
}

// -- Fase 15 GAP 1 (rapor lanjutan) --

func (f *fakeRepo) ReplaceTPMappings(ctx context.Context, schoolID, academicYearID, classID, subjectID int64, mappings []TPMappingInput) error {
	if f.tpMappings == nil {
		f.tpMappings = map[tpKey]TPMappingRecord{}
	}
	for k := range f.tpMappings {
		if k.School == schoolID && k.Class == classID && k.Subject == subjectID {
			delete(f.tpMappings, k)
		}
	}
	for _, m := range mappings {
		f.tpMappings[tpKey{schoolID, classID, subjectID, m.ComponentID}] = TPMappingRecord{ComponentID: m.ComponentID, TPCode: m.TPCode, Description: m.Description}
	}
	return nil
}

func (f *fakeRepo) ListTPMappingsForClassSubject(ctx context.Context, schoolID, classID, subjectID int64) ([]TPMappingRecord, error) {
	var out []TPMappingRecord
	for k, v := range f.tpMappings {
		if k.School == schoolID && k.Class == classID && k.Subject == subjectID {
			out = append(out, v)
		}
	}
	return out, nil
}

func (f *fakeRepo) ListTPMappingsForClass(ctx context.Context, schoolID, classID int64) ([]TPMappingWithSubject, error) {
	var out []TPMappingWithSubject
	for k, v := range f.tpMappings {
		if k.School == schoolID && k.Class == classID {
			sub := f.subjects[k.Subject]
			comp := f.components[k.Component]
			out = append(out, TPMappingWithSubject{
				ComponentID: v.ComponentID, SubjectID: k.Subject, SubjectName: sub.Name,
				ComponentName: comp.Name, TPCode: v.TPCode, Description: v.Description,
			})
		}
	}
	return out, nil
}

func (f *fakeRepo) UpsertManualScore(ctx context.Context, schoolID, academicYearID, classID, subjectID, studentID int64, kind string, score float64, note string, setBy int64) error {
	if f.manualScores == nil {
		f.manualScores = map[manualKey]ManualScoreRecord{}
	}
	f.manualScores[manualKey{schoolID, academicYearID, classID, subjectID, studentID, kind}] = ManualScoreRecord{StudentID: studentID, Kind: kind, Score: score, Note: note}
	return nil
}

func (f *fakeRepo) DeleteManualScore(ctx context.Context, schoolID, academicYearID, classID, subjectID, studentID int64, kind string) error {
	delete(f.manualScores, manualKey{schoolID, academicYearID, classID, subjectID, studentID, kind})
	return nil
}

func (f *fakeRepo) ListManualScoresForClassSubject(ctx context.Context, schoolID, academicYearID, classID, subjectID int64) ([]ManualScoreRecord, error) {
	var out []ManualScoreRecord
	for k, v := range f.manualScores {
		if k.School == schoolID && k.Year == academicYearID && k.Class == classID && k.Subject == subjectID {
			out = append(out, v)
		}
	}
	return out, nil
}

// -- fake gateways --

type fakeIdentity struct{ perms map[string]map[string]bool }

func (f fakeIdentity) HasPermission(role, perm string) bool { return f.perms[role][perm] }
func (f fakeIdentity) Log(ctx context.Context, schoolID, userID *int64, action, entity string, entityID *int64, oldValue, newValue any) error {
	return nil
}

func defaultPerms() map[string]map[string]bool {
	return map[string]map[string]bool{
		RoleAdminSekolah:  {PermGradingManage: true},
		RoleGuru:          {PermGradingManage: true},
		RoleKepalaSekolah: {PermGradingRead: true},
	}
}

type fakeYears struct {
	id int64
	ok bool
}

func (f fakeYears) ActiveAcademicYearID(ctx context.Context, schoolID int64) (int64, bool, error) {
	return f.id, f.ok, nil
}

type fakeSettings struct{ raw map[int64][]byte }

func (f fakeSettings) GetSetting(ctx context.Context, schoolID int64, module string) ([]byte, bool, error) {
	raw, ok := f.raw[schoolID]
	return raw, ok, nil
}

type fakeStudents struct {
	myStudentID map[int64][2]int64 // userID -> [studentID, classID]
	guardians   map[int64][]int64  // studentID -> userIDs
	viewErr     error
}

func (f fakeStudents) CanViewStudent(ctx context.Context, userID int64, role string, schoolID, studentID int64) error {
	if f.viewErr != nil {
		return f.viewErr
	}
	for _, gid := range f.guardians[studentID] {
		if gid == userID {
			return nil
		}
	}
	return httpx.ErrForbidden
}

func (f fakeStudents) MyStudentID(ctx context.Context, schoolID, userID int64) (int64, int64, bool, error) {
	v, ok := f.myStudentID[userID]
	if !ok {
		return 0, 0, false, nil
	}
	return v[0], v[1], true, nil
}

func (f fakeStudents) GuardianUserIDs(ctx context.Context, schoolID, studentID int64) ([]int64, error) {
	return f.guardians[studentID], nil
}

type fakeSchedule struct {
	teachesClassSubject map[[4]int64]bool // schoolID, teacherUserID, classID, subjectID
	teachesClass        map[[3]int64]bool // schoolID, teacherUserID, classID
}

func (f fakeSchedule) TeachesClassSubject(ctx context.Context, schoolID, teacherUserID, classID, subjectID int64) (bool, error) {
	return f.teachesClassSubject[[4]int64{schoolID, teacherUserID, classID, subjectID}], nil
}

func (f fakeSchedule) TeachesClass(ctx context.Context, schoolID, teacherUserID, classID int64) (bool, error) {
	return f.teachesClass[[3]int64{schoolID, teacherUserID, classID}], nil
}

type notifyCall struct {
	event   string
	userIDs []int64
	data    map[string]any
}

type fakeNotifier struct{ calls []notifyCall }

func (f *fakeNotifier) Notify(ctx context.Context, schoolID int64, event string, userIDs []int64, data map[string]any) error {
	f.calls = append(f.calls, notifyCall{event: event, userIDs: userIDs, data: data})
	return nil
}

type fakeRealtime struct{ calls int }

func (f *fakeRealtime) PublishTo(schoolID int64, eventType string, data map[string]any, roles []string, userIDs []int64) {
	f.calls++
}

// -- helpers --

func ctxFor(schoolID, userID int64, role string) context.Context {
	ctx := reqctx.WithSchool(context.Background(), reqctx.School{ID: schoolID, Timezone: "Asia/Jakarta"})
	return reqctx.WithUser(ctx, userID, role, false)
}

func enabledSettingsRaw(t *testing.T) []byte {
	t.Helper()
	return []byte(`{"enabled":true,"ranges":[{"min":0,"max":74,"label":"C"},{"min":75,"max":84,"label":"B"},{"min":85,"max":100,"label":"A"}]}`)
}

func newTestService(t *testing.T, repo *fakeRepo, enabled bool, schoolID int64) (*Service, *fakeNotifier, *fakeRealtime, fakeSchedule) {
	t.Helper()
	settingsRaw := map[int64][]byte{}
	if enabled {
		settingsRaw[schoolID] = enabledSettingsRaw(t)
	}
	sched := fakeSchedule{teachesClassSubject: map[[4]int64]bool{}, teachesClass: map[[3]int64]bool{}}
	notifier := &fakeNotifier{}
	rt := &fakeRealtime{}
	svc := newServiceForTest(repo, fakeIdentity{perms: defaultPerms()}, fakeYears{id: 1, ok: true}, fakeSettings{raw: settingsRaw},
		fakeStudents{myStudentID: map[int64][2]int64{}, guardians: map[int64][]int64{}}, sched, clock.System{})
	svc.SetNotifier(notifier)
	svc.SetRealtime(rt)
	return svc, notifier, rt, sched
}

// -- tests: computeFinal (normalisasi) --

func TestComputeFinal_WeightNot100(t *testing.T) {
	components := []ComponentRecord{
		{ID: 1, Weight: 30, Kktp: 75},
		{ID: 2, Weight: 50, Kktp: 75},
		{ID: 3, Weight: 20, Kktp: 75},
	}
	scores := map[int64]float64{1: 80, 2: 85, 3: 78}
	final, below := computeFinal(components, scores)
	if final == nil {
		t.Fatal("final tidak boleh nil")
	}
	// (80*30 + 85*50 + 78*20) / 100 = (2400+4250+1560)/100 = 82.1
	want := 82.1
	if diff := *final - want; diff > 0.001 || diff < -0.001 {
		t.Fatalf("final = %v, want %v", *final, want)
	}
	if len(below) != 0 {
		t.Fatalf("below_kktp harus kosong (semua >= 75), dapat %v", below)
	}
}

func TestComputeFinal_PartialComponents(t *testing.T) {
	// bobot TIDAK harus 100: 2 komponen weight total 40, siswa hanya punya 1 nilai.
	components := []ComponentRecord{
		{ID: 1, Weight: 30, Kktp: 75},
		{ID: 2, Weight: 10, Kktp: 75},
	}
	scores := map[int64]float64{1: 90} // komponen 2 belum dinilai
	final, _ := computeFinal(components, scores)
	if final == nil {
		t.Fatal("final tidak boleh nil (ada 1 komponen terisi)")
	}
	if *final != 90 {
		t.Fatalf("final = %v, want 90 (hanya dinormalisasi dari komponen terisi)", *final)
	}
}

func TestComputeFinal_NoScoreAtAll(t *testing.T) {
	components := []ComponentRecord{{ID: 1, Weight: 30, Kktp: 75}}
	final, below := computeFinal(components, map[int64]float64{})
	if final != nil {
		t.Fatalf("final harus nil bila tidak ada nilai sama sekali, dapat %v", *final)
	}
	if len(below) != 0 {
		t.Fatalf("below_kktp harus kosong bila tidak ada nilai, dapat %v", below)
	}
}

func TestComputeFinal_BelowKktp(t *testing.T) {
	components := []ComponentRecord{
		{ID: 1, Weight: 50, Kktp: 75},
		{ID: 2, Weight: 50, Kktp: 80},
	}
	scores := map[int64]float64{1: 70, 2: 90}
	_, below := computeFinal(components, scores)
	if len(below) != 1 || below[0] != 1 {
		t.Fatalf("below_kktp = %v, want [1]", below)
	}
}

// -- tests: label ranges (batas & pembulatan) --

func TestLabelForScore_Boundaries(t *testing.T) {
	ranges := DefaultSettings().Ranges
	cases := []struct {
		score int
		want  string
	}{
		{0, "C"}, {74, "C"}, {75, "B"}, {84, "B"}, {85, "A"}, {100, "A"},
	}
	for _, c := range cases {
		got := labelForScore(ranges, c.score)
		if got == nil || *got != c.want {
			t.Fatalf("labelForScore(%d) = %v, want %s", c.score, got, c.want)
		}
	}
}

func TestRoundHalfUp(t *testing.T) {
	cases := []struct {
		in   float64
		want int
	}{
		{74.4, 74}, {74.5, 75}, {74.51, 75}, {84.5, 85}, {0.5, 1}, {99.5, 100},
	}
	for _, c := range cases {
		if got := roundHalfUp(c.in); got != c.want {
			t.Fatalf("roundHalfUp(%v) = %d, want %d", c.in, got, c.want)
		}
	}
}

// -- tests: validasi ranges settings --

func TestSettingsValidate_ValidDefault(t *testing.T) {
	s := DefaultSettings()
	if err := s.Validate(); err != nil {
		t.Fatalf("default settings harus valid: %v", err)
	}
}

func TestSettingsValidate_GapRejected(t *testing.T) {
	s := Settings{Ranges: []GradeRange{{0, 70, "C"}, {75, 100, "A"}}} // celah 71-74
	if err := s.Validate(); err == nil {
		t.Fatal("harus menolak celah antar rentang")
	}
}

func TestSettingsValidate_OverlapRejected(t *testing.T) {
	s := Settings{Ranges: []GradeRange{{0, 80, "C"}, {75, 100, "A"}}} // tumpang tindih 75-80
	if err := s.Validate(); err == nil {
		t.Fatal("harus menolak tumpang tindih antar rentang")
	}
}

func TestSettingsValidate_MustStartAtZero(t *testing.T) {
	s := Settings{Ranges: []GradeRange{{1, 100, "A"}}}
	if err := s.Validate(); err == nil {
		t.Fatal("harus menolak rentang yang tidak mulai dari 0")
	}
}

func TestSettingsValidate_MustEndAt100(t *testing.T) {
	s := Settings{Ranges: []GradeRange{{0, 99, "A"}}}
	if err := s.Validate(); err == nil {
		t.Fatal("harus menolak rentang yang tidak berakhir di 100")
	}
}

// -- tests: guard disabled 404 --

func TestRequireEnabled_Disabled404(t *testing.T) {
	repo := newFakeRepo()
	svc, _, _, _ := newTestService(t, repo, false, 1)
	ctx := ctxFor(1, 1, RoleAdminSekolah)
	_, err := svc.ListComponents(ctx, 1, ListComponentsQuery{ClassID: 1, SubjectID: 1})
	var de *httpx.Error
	if !errors.As(err, &de) || de.Code != "grading_disabled" || de.Status != 404 {
		t.Fatalf("err = %v, want grading_disabled 404", err)
	}
}

func TestStatus_AlwaysWorksEvenWhenDisabled(t *testing.T) {
	repo := newFakeRepo()
	svc, _, _, _ := newTestService(t, repo, false, 1)
	ctx := ctxFor(1, 1, RoleSiswa)
	status, err := svc.Status(ctx, 1)
	if err != nil {
		t.Fatalf("Status tidak boleh error saat disabled: %v", err)
	}
	if status.Enabled {
		t.Fatal("Enabled harus false")
	}
}

// -- tests: object-level guru --

func TestObjectLevel_GuruWithoutSlot_Forbidden(t *testing.T) {
	repo := newFakeRepo()
	repo.classes[1] = ClassRef{ID: 1, Name: "XII RPL 1", AcademicYearID: 1}
	repo.subjects[1] = SubjectRef{ID: 1, Code: "BD", Name: "Basis Data"}
	svc, _, _, _ := newTestService(t, repo, true, 1)
	ctx := ctxFor(1, 99, RoleGuru) // guru TIDAK punya slot kelas-mapel ini
	_, err := svc.CreateComponent(ctx, 99, 1, ComponentCreateInput{ClassID: 1, SubjectID: 1, Name: "TP1", Type: ComponentTP, Weight: 30, Kktp: 75})
	if !errors.Is(err, httpx.ErrForbidden) {
		t.Fatalf("err = %v, want ErrForbidden", err)
	}
}

func TestObjectLevel_GuruWithSlot_Allowed(t *testing.T) {
	repo := newFakeRepo()
	repo.classes[1] = ClassRef{ID: 1, Name: "XII RPL 1", AcademicYearID: 1}
	repo.subjects[1] = SubjectRef{ID: 1, Code: "BD", Name: "Basis Data"}
	svc, _, _, sched := newTestService(t, repo, true, 1)
	sched.teachesClassSubject[[4]int64{1, 99, 1, 1}] = true
	ctx := ctxFor(1, 99, RoleGuru)
	view, err := svc.CreateComponent(ctx, 99, 1, ComponentCreateInput{ClassID: 1, SubjectID: 1, Name: "TP1", Type: ComponentTP, Weight: 30, Kktp: 75})
	if err != nil {
		t.Fatalf("guru dengan slot harus diizinkan: %v", err)
	}
	if view.Name != "TP1" {
		t.Fatalf("view = %+v", view)
	}
}

func TestObjectLevel_AdminBypasses(t *testing.T) {
	repo := newFakeRepo()
	repo.classes[1] = ClassRef{ID: 1, Name: "XII RPL 1", AcademicYearID: 1}
	repo.subjects[1] = SubjectRef{ID: 1, Code: "BD", Name: "Basis Data"}
	svc, _, _, _ := newTestService(t, repo, true, 1)
	ctx := ctxFor(1, 1, RoleAdminSekolah)
	_, err := svc.CreateComponent(ctx, 1, 1, ComponentCreateInput{ClassID: 1, SubjectID: 1, Name: "TP1", Type: ComponentTP, Weight: 30, Kktp: 75})
	if err != nil {
		t.Fatalf("admin harus bebas object-level: %v", err)
	}
}

// -- tests: publish notif target --

func TestPutPublication_NotifiesStudentsAndGuardians(t *testing.T) {
	repo := newFakeRepo()
	repo.classes[1] = ClassRef{ID: 1, Name: "XII RPL 1", AcademicYearID: 1}
	repo.subjects[1] = SubjectRef{ID: 1, Code: "BD", Name: "Basis Data"}
	repo.roster[1] = []RosterStudent{
		{ID: 10, Name: "Siswa A", NIS: "001", UserID: 500},
		{ID: 11, Name: "Siswa B", NIS: "002", UserID: 0}, // belum aktivasi akun
	}
	svc, notifier, rt, _ := newTestService(t, repo, true, 1)
	// fakeStudents guardians butuh disuntik manual — ganti field students svc via type assertion tidak bisa (unexported); gunakan svc yang sudah dibuat & set guardians lewat repo tidak relevan, jadi rebuild manual di bawah dengan StudentAccess custom.
	students := fakeStudents{guardians: map[int64][]int64{10: {600}, 11: {601}}}
	svc = newServiceForTest(repo, fakeIdentity{perms: defaultPerms()}, fakeYears{id: 1, ok: true}, fakeSettings{raw: map[int64][]byte{1: enabledSettingsRaw(t)}},
		students, fakeSchedule{teachesClassSubject: map[[4]int64]bool{}, teachesClass: map[[3]int64]bool{}}, clock.System{})
	svc.SetNotifier(notifier)
	svc.SetRealtime(rt)

	ctx := ctxFor(1, 1, RoleAdminSekolah)
	if err := svc.PutPublication(ctx, 1, 1, PublicationInput{ClassID: 1, SubjectID: 1, Published: true}); err != nil {
		t.Fatalf("PutPublication: %v", err)
	}
	if len(notifier.calls) != 1 {
		t.Fatalf("harus ada 1 panggilan notify, dapat %d", len(notifier.calls))
	}
	call := notifier.calls[0]
	if call.event != EventGradingPublished {
		t.Fatalf("event = %s, want %s", call.event, EventGradingPublished)
	}
	want := map[int64]bool{500: true, 600: true, 601: true} // siswa A (user 500) + ortu A (600) + ortu B (601). Siswa B TANPA akun (user_id 0) tidak masuk.
	if len(call.userIDs) != len(want) {
		t.Fatalf("userIDs = %v, want set %v", call.userIDs, want)
	}
	for _, id := range call.userIDs {
		if !want[id] {
			t.Fatalf("userIDs berisi id tak terduga %d", id)
		}
	}
}

func TestPutPublication_Unpublish_NoNotify(t *testing.T) {
	repo := newFakeRepo()
	repo.classes[1] = ClassRef{ID: 1, Name: "XII RPL 1", AcademicYearID: 1}
	repo.subjects[1] = SubjectRef{ID: 1, Code: "BD", Name: "Basis Data"}
	svc, notifier, _, _ := newTestService(t, repo, true, 1)
	ctx := ctxFor(1, 1, RoleAdminSekolah)
	if err := svc.PutPublication(ctx, 1, 1, PublicationInput{ClassID: 1, SubjectID: 1, Published: false}); err != nil {
		t.Fatalf("PutPublication unpublish: %v", err)
	}
	if len(notifier.calls) != 0 {
		t.Fatalf("unpublish TIDAK boleh mengirim notif, dapat %d panggilan", len(notifier.calls))
	}
}

// -- tests: stars visibility --

func TestMyStars_OnlyVisibleToStudent(t *testing.T) {
	repo := newFakeRepo()
	repo.stars[1] = StarRecord{ID: 1, StudentID: 10, Delta: 2, Visibility: VisibilityStudent, GivenByName: "Rendi"}
	repo.stars[2] = StarRecord{ID: 2, StudentID: 10, Delta: 5, Visibility: VisibilityPrivate, GivenByName: "Rendi"}
	svc, _, _, _ := newTestService(t, repo, true, 1)
	students := fakeStudents{myStudentID: map[int64][2]int64{700: {10, 1}}, guardians: map[int64][]int64{}}
	svc = newServiceForTest(repo, fakeIdentity{perms: defaultPerms()}, fakeYears{id: 1, ok: true}, fakeSettings{raw: map[int64][]byte{1: enabledSettingsRaw(t)}},
		students, fakeSchedule{}, clock.System{})

	ctx := ctxFor(1, 700, RoleSiswa)
	result, err := svc.MyStars(ctx, 1, MyStarsQuery{})
	if err != nil {
		t.Fatalf("MyStars: %v", err)
	}
	if result.Total != 2 {
		t.Fatalf("total = %d, want 2 (HANYA visibility=student, private TIDAK dihitung)", result.Total)
	}
	if len(result.Items) != 1 {
		t.Fatalf("items = %d, want 1", len(result.Items))
	}
}

// -- tests: delta != 0 --

func TestCreateStar_DeltaZeroRejected(t *testing.T) {
	repo := newFakeRepo()
	repo.studentBasic[10] = [2]string{"Siswa A", "001"}
	repo.studentClass[[3]int64{1, 1, 10}] = 1
	svc, _, _, _ := newTestService(t, repo, true, 1)
	ctx := ctxFor(1, 1, RoleAdminSekolah)
	_, err := svc.CreateStar(ctx, 1, 1, StarCreateInput{StudentID: 10, Delta: 0, Visibility: VisibilityPrivate})
	if err == nil {
		t.Fatal("delta=0 harus ditolak")
	}
}

func TestCreateStar_GuruNeedsClassAccess(t *testing.T) {
	repo := newFakeRepo()
	repo.studentBasic[10] = [2]string{"Siswa A", "001"}
	repo.studentClass[[3]int64{1, 1, 10}] = 1
	svc, _, _, sched := newTestService(t, repo, true, 1)
	ctx := ctxFor(1, 99, RoleGuru)
	_, err := svc.CreateStar(ctx, 99, 1, StarCreateInput{StudentID: 10, Delta: 1, Visibility: VisibilityPrivate})
	if !errors.Is(err, httpx.ErrForbidden) {
		t.Fatalf("guru tanpa slot kelas siswa harus 403, dapat %v", err)
	}
	sched.teachesClass[[3]int64{1, 99, 1}] = true
	_, err = svc.CreateStar(ctx, 99, 1, StarCreateInput{StudentID: 10, Delta: 1, Visibility: VisibilityPrivate})
	if err != nil {
		t.Fatalf("guru dengan slot kelas harus diizinkan (mapel apa pun): %v", err)
	}
}

// -- tests: Fase 15 GAP 5 (grading:read kepala_sekolah) --

func TestRequireReadAccess_KepsekBypassesObjectLevel(t *testing.T) {
	repo := newFakeRepo()
	repo.classes[1] = ClassRef{ID: 1, Name: "XII RPL 1", AcademicYearID: 1}
	repo.subjects[1] = SubjectRef{ID: 1, Code: "BD", Name: "Basis Data"}
	svc, _, _, _ := newTestService(t, repo, true, 1)
	ctx := ctxFor(1, 50, RoleKepalaSekolah) // TIDAK punya slot jadwal apa pun
	if _, err := svc.Recap(ctx, 1, RecapQuery{ClassID: 1, SubjectID: 1}); err != nil {
		t.Fatalf("kepsek (grading:read) harus bisa baca recap kelas-mapel APA PUN: %v", err)
	}
	if _, err := svc.ListComponents(ctx, 1, ListComponentsQuery{ClassID: 1, SubjectID: 1}); err != nil {
		t.Fatalf("kepsek (grading:read) harus bisa baca components: %v", err)
	}
	if _, err := svc.ReportAnalysis(ctx, 1, 1, 1); err != nil {
		t.Fatalf("kepsek (grading:read) harus bisa baca report/analysis: %v", err)
	}
}

func TestRequireReadAccess_KepsekCannotMutate(t *testing.T) {
	repo := newFakeRepo()
	repo.classes[1] = ClassRef{ID: 1, Name: "XII RPL 1", AcademicYearID: 1}
	repo.subjects[1] = SubjectRef{ID: 1, Code: "BD", Name: "Basis Data"}
	svc, _, _, _ := newTestService(t, repo, true, 1)
	ctx := ctxFor(1, 50, RoleKepalaSekolah)
	_, err := svc.CreateComponent(ctx, 50, 1, ComponentCreateInput{ClassID: 1, SubjectID: 1, Name: "TP1", Type: ComponentTP, Weight: 30, Kktp: 75})
	if !errors.Is(err, httpx.ErrForbidden) {
		t.Fatalf("kepsek TIDAK boleh mutasi (POST components), got %v", err)
	}
}

func TestRequireReadAccess_NoPermission_Forbidden(t *testing.T) {
	repo := newFakeRepo()
	repo.classes[1] = ClassRef{ID: 1, Name: "XII RPL 1", AcademicYearID: 1}
	repo.subjects[1] = SubjectRef{ID: 1, Code: "BD", Name: "Basis Data"}
	svc, _, _, _ := newTestService(t, repo, true, 1)
	ctx := ctxFor(1, 50, RoleOrangTua) // TIDAK punya grading:manage maupun grading:read
	if _, err := svc.Recap(ctx, 1, RecapQuery{ClassID: 1, SubjectID: 1}); !errors.Is(err, httpx.ErrForbidden) {
		t.Fatalf("role tanpa grading:manage/grading:read harus 403, got %v", err)
	}
}

// -- tests: Fase 15 GAP 1 (rapor lanjutan) --

func TestPutTPMappings_ReplaceAndValidation(t *testing.T) {
	repo := newFakeRepo()
	repo.classes[1] = ClassRef{ID: 1, Name: "XII RPL 1", AcademicYearID: 1}
	repo.subjects[1] = SubjectRef{ID: 1, Code: "BD", Name: "Basis Data"}
	svc, _, _, _ := newTestService(t, repo, true, 1)
	ctx := ctxFor(1, 1, RoleAdminSekolah)

	tp, err := svc.CreateComponent(ctx, 1, 1, ComponentCreateInput{ClassID: 1, SubjectID: 1, Name: "TP1", Type: ComponentTP, Weight: 30, Kktp: 75})
	if err != nil {
		t.Fatalf("create tp component: %v", err)
	}
	sumatif, err := svc.CreateComponent(ctx, 1, 1, ComponentCreateInput{ClassID: 1, SubjectID: 1, Name: "Sumatif", Type: ComponentSumatif, Weight: 70, Kktp: 75})
	if err != nil {
		t.Fatalf("create sumatif component: %v", err)
	}

	if _, err := svc.PutTPMappings(ctx, 1, 1, 1, 1, []TPMappingInput{{ComponentID: sumatif.ID, TPCode: "X"}}); err == nil {
		t.Fatal("harus menolak mapping komponen bertipe non-tp")
	}

	view, err := svc.PutTPMappings(ctx, 1, 1, 1, 1, []TPMappingInput{{ComponentID: tp.ID, TPCode: "10.1", Description: "Deskripsi"}})
	if err != nil {
		t.Fatalf("put tp mapping: %v", err)
	}
	if len(view.Items) != 1 || view.Items[0].TPCode != "10.1" {
		t.Fatalf("view = %+v", view)
	}

	// Replace penuh: PUT dengan list kosong menghapus mapping sebelumnya.
	view2, err := svc.PutTPMappings(ctx, 1, 1, 1, 1, []TPMappingInput{})
	if err != nil {
		t.Fatalf("put tp mapping kosong: %v", err)
	}
	for _, item := range view2.Items {
		if item.TPCode != "" {
			t.Fatalf("mapping harus sudah dihapus (replace penuh), dapat %+v", item)
		}
	}
}

func TestManualScores_ManualWinsOverComputed(t *testing.T) {
	repo := newFakeRepo()
	repo.classes[1] = ClassRef{ID: 1, Name: "XII RPL 1", AcademicYearID: 1}
	repo.subjects[1] = SubjectRef{ID: 1, Code: "BD", Name: "Basis Data"}
	repo.roster[1] = []RosterStudent{{ID: 10, Name: "Siswa A", NIS: "001"}, {ID: 11, Name: "Siswa B", NIS: "002"}}
	svc, _, _, _ := newTestService(t, repo, true, 1)
	ctx := ctxFor(1, 1, RoleAdminSekolah)

	comp, err := svc.CreateComponent(ctx, 1, 1, ComponentCreateInput{ClassID: 1, SubjectID: 1, Name: "TP1", Type: ComponentTP, Weight: 100, Kktp: 75})
	if err != nil {
		t.Fatalf("create component: %v", err)
	}
	seventy := 70.0
	if _, err := svc.PutGrades(ctx, 1, 1, comp.ID, []GradeEntryInput{{StudentID: 10, Score: &seventy}}); err != nil {
		t.Fatalf("put grades: %v", err)
	}

	score90, score75 := 90.0, 75.0
	if _, err := svc.PutManualScores(ctx, 1, 1, 1, 1, []ManualScoreEntryInput{
		{StudentID: 10, Kind: ReportScoreKindManual, Score: &score90},
		{StudentID: 11, Kind: ReportScoreKindPrevious, Score: &score75},
	}); err != nil {
		t.Fatalf("put manual scores: %v", err)
	}

	view, err := svc.GetManualScores(ctx, 1, 1, 1)
	if err != nil {
		t.Fatalf("get manual scores: %v", err)
	}
	byStudent := map[int64]ReportScoreRow{}
	for _, row := range view.Items {
		byStudent[row.StudentID] = row
	}
	a := byStudent[10]
	if a.Manual == nil || a.Manual.Score != 90 {
		t.Fatalf("siswa A manual = %+v, want 90", a.Manual)
	}
	if a.ComputedFinal == nil || *a.ComputedFinal != 70 {
		t.Fatalf("siswa A computed_final = %v, want 70 (computed final TETAP disematkan meski manual menang)", a.ComputedFinal)
	}
	b := byStudent[11]
	if b.Previous == nil || *b.Previous != 75 {
		t.Fatalf("siswa B previous = %v, want 75", b.Previous)
	}

	analysis, err := svc.ReportAnalysis(ctx, 1, 1, 1)
	if err != nil {
		t.Fatalf("report analysis: %v", err)
	}
	if analysis.ResolvedSource.Manual != 1 {
		t.Fatalf("resolved_source.manual = %d, want 1", analysis.ResolvedSource.Manual)
	}
	if analysis.ResolvedSource.None != 1 {
		t.Fatalf("resolved_source.none = %d, want 1 (siswa B hanya previous, TANPA manual/computed)", analysis.ResolvedSource.None)
	}
	if analysis.Count != 1 {
		t.Fatalf("count = %d, want 1 (hanya siswa yang ter-resolusi dihitung avg/min/max)", analysis.Count)
	}
	if analysis.Avg != 90 {
		t.Fatalf("avg = %v, want 90 (manual MENANG atas computed final 70)", analysis.Avg)
	}
}
