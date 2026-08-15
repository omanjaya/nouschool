package schedule

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/omanjaya/nouschool/internal/platform/clock"
	"github.com/omanjaya/nouschool/internal/platform/httpx"
	"github.com/omanjaya/nouschool/internal/platform/reqctx"
)

// -- fakeScheduleRepo: implementasi scheduleRepository in-memory (tanpa DB) —
// pola yang sama dengan internal/leave & internal/attendance
// service_test.go. CreateSlotChecked/UpdateSlotChecked memakai findConflicts
// PERSIS seperti Repository asli (repository.go) supaya perilaku yang dites
// di sini representatif.

type roomEntry struct {
	SchoolID int64
	Rec      RoomRecord
}

type classEntry struct {
	SchoolID int64
	Ref      ClassRefRow
}

type subjectEntry struct {
	SchoolID int64
	Ref      SubjectRef
}

type teacherEntry struct {
	SchoolID int64
	Ref      TeacherRef
}

type fakeScheduleRepo struct {
	periods      map[int64][]PeriodRecord // schoolID -> periods
	nextPeriodID int64

	rooms      map[int64]roomEntry
	nextRoomID int64

	classes  map[int64]classEntry
	subjects map[int64]subjectEntry
	teachers map[int64]teacherEntry

	slots      map[int64]SlotRecord
	nextSlotID int64
}

func newFakeScheduleRepo() *fakeScheduleRepo {
	return &fakeScheduleRepo{
		periods:  map[int64][]PeriodRecord{},
		rooms:    map[int64]roomEntry{},
		classes:  map[int64]classEntry{},
		subjects: map[int64]subjectEntry{},
		teachers: map[int64]teacherEntry{},
		slots:    map[int64]SlotRecord{},
	}
}

// -- periods --

func (f *fakeScheduleRepo) ListPeriods(ctx context.Context, schoolID int64) ([]PeriodRecord, error) {
	return append([]PeriodRecord{}, f.periods[schoolID]...), nil
}

func (f *fakeScheduleRepo) ReplacePeriods(ctx context.Context, schoolID int64, periods []PeriodInput) ([]PeriodRecord, error) {
	out := make([]PeriodRecord, 0, len(periods))
	for _, p := range periods {
		f.nextPeriodID++
		out = append(out, PeriodRecord{ID: f.nextPeriodID, Number: p.Number, StartsAt: p.StartsAt, EndsAt: p.EndsAt, Label: p.Label})
	}
	f.periods[schoolID] = out
	return append([]PeriodRecord{}, out...), nil
}

func (f *fakeScheduleRepo) UsedPeriodNumbers(ctx context.Context, schoolID int64) (map[int]bool, error) {
	used := map[int]bool{}
	for _, sl := range f.slots {
		if sl.SchoolID != schoolID {
			continue
		}
		for n := sl.PeriodStart; n <= sl.PeriodEnd; n++ {
			used[n] = true
		}
	}
	return used, nil
}

// -- rooms --

func (f *fakeScheduleRepo) CreateRoom(ctx context.Context, schoolID int64, name, qrToken string) (RoomRecord, error) {
	for _, e := range f.rooms {
		if e.SchoolID == schoolID && e.Rec.Name == name {
			return RoomRecord{}, ErrConflict
		}
	}
	f.nextRoomID++
	rec := RoomRecord{ID: f.nextRoomID, Name: name, QRToken: qrToken}
	f.rooms[rec.ID] = roomEntry{SchoolID: schoolID, Rec: rec}
	return rec, nil
}

func (f *fakeScheduleRepo) UpdateRoomName(ctx context.Context, schoolID, id int64, name string) (RoomRecord, error) {
	e, ok := f.rooms[id]
	if !ok || e.SchoolID != schoolID {
		return RoomRecord{}, ErrNotFound
	}
	e.Rec.Name = name
	f.rooms[id] = e
	return e.Rec, nil
}

func (f *fakeScheduleRepo) RegenerateRoomQR(ctx context.Context, schoolID, id int64, qrToken string) (RoomRecord, error) {
	e, ok := f.rooms[id]
	if !ok || e.SchoolID != schoolID {
		return RoomRecord{}, ErrNotFound
	}
	e.Rec.QRToken = qrToken
	f.rooms[id] = e
	return e.Rec, nil
}

func (f *fakeScheduleRepo) GetRoomByID(ctx context.Context, schoolID, id int64) (RoomRecord, error) {
	e, ok := f.rooms[id]
	if !ok || e.SchoolID != schoolID {
		return RoomRecord{}, ErrNotFound
	}
	return e.Rec, nil
}

func (f *fakeScheduleRepo) ListRooms(ctx context.Context, schoolID int64) ([]RoomRecord, error) {
	var out []RoomRecord
	for _, e := range f.rooms {
		if e.SchoolID == schoolID {
			out = append(out, e.Rec)
		}
	}
	return out, nil
}

func (f *fakeScheduleRepo) DeleteRoom(ctx context.Context, schoolID, id int64) error {
	delete(f.rooms, id)
	return nil
}

func (f *fakeScheduleRepo) CountSlotsForRoom(ctx context.Context, schoolID, roomID int64) (int64, error) {
	var n int64
	for _, sl := range f.slots {
		if sl.SchoolID == schoolID && sl.RoomID == roomID {
			n++
		}
	}
	return n, nil
}

// -- referensi --

func (f *fakeScheduleRepo) GetClassRef(ctx context.Context, schoolID, id int64) (ClassRefRow, error) {
	e, ok := f.classes[id]
	if !ok || e.SchoolID != schoolID {
		return ClassRefRow{}, ErrNotFound
	}
	return e.Ref, nil
}

func (f *fakeScheduleRepo) GetSubjectRef(ctx context.Context, schoolID, id int64) (SubjectRef, error) {
	e, ok := f.subjects[id]
	if !ok || e.SchoolID != schoolID {
		return SubjectRef{}, ErrNotFound
	}
	return e.Ref, nil
}

func (f *fakeScheduleRepo) GetTeacherRef(ctx context.Context, schoolID, id int64) (TeacherRef, error) {
	e, ok := f.teachers[id]
	if !ok || e.SchoolID != schoolID {
		return TeacherRef{}, ErrNotFound
	}
	return e.Ref, nil
}

func (f *fakeScheduleRepo) GetRoomRef(ctx context.Context, schoolID, id int64) (RoomRef, error) {
	e, ok := f.rooms[id]
	if !ok || e.SchoolID != schoolID {
		return RoomRef{}, ErrNotFound
	}
	return RoomRef{ID: e.Rec.ID, Name: e.Rec.Name}, nil
}

func (f *fakeScheduleRepo) LookupClassIDByName(ctx context.Context, schoolID, academicYearID int64, name string) (int64, bool, error) {
	for id, e := range f.classes {
		if e.SchoolID == schoolID && e.Ref.AcademicYearID == academicYearID && e.Ref.Name == name {
			return id, true, nil
		}
	}
	return 0, false, nil
}

func (f *fakeScheduleRepo) LookupSubjectByCode(ctx context.Context, schoolID int64, code string) (SubjectRef, bool, error) {
	for _, e := range f.subjects {
		if e.SchoolID == schoolID && e.Ref.Code == code {
			return e.Ref, true, nil
		}
	}
	return SubjectRef{}, false, nil
}

func (f *fakeScheduleRepo) LookupTeacherByEmail(ctx context.Context, schoolID int64, email string) (TeacherRef, bool, error) {
	for _, e := range f.teachers {
		if e.SchoolID == schoolID && f.teacherEmail(e.Ref.ID) == email {
			return e.Ref, true, nil
		}
	}
	return TeacherRef{}, false, nil
}

// teacherEmails simula kolom email guru (di luar TeacherRef supaya tidak
// mengubah bentuk domain — lihat setupTeacher).
var teacherEmails = map[int64]string{}

func (f *fakeScheduleRepo) teacherEmail(id int64) string { return teacherEmails[id] }

func (f *fakeScheduleRepo) LookupRoomIDByName(ctx context.Context, schoolID int64, name string) (int64, bool, error) {
	for id, e := range f.rooms {
		if e.SchoolID == schoolID && e.Rec.Name == name {
			return id, true, nil
		}
	}
	return 0, false, nil
}

// -- slots --

func (f *fakeScheduleRepo) ListSlotsForYear(ctx context.Context, schoolID, academicYearID int64) ([]SlotRecord, error) {
	var out []SlotRecord
	for _, sl := range f.slots {
		if sl.SchoolID == schoolID && sl.AcademicYearID == academicYearID {
			out = append(out, sl)
		}
	}
	return out, nil
}

func (f *fakeScheduleRepo) GetSlotByID(ctx context.Context, schoolID, id int64) (SlotRecord, error) {
	sl, ok := f.slots[id]
	if !ok || sl.SchoolID != schoolID {
		return SlotRecord{}, ErrNotFound
	}
	return sl, nil
}

func (f *fakeScheduleRepo) toSlotRecord(in SlotInput) SlotRecord {
	cls := f.classes[in.ClassID].Ref
	sub := f.subjects[in.SubjectID].Ref
	tch := f.teachers[in.TeacherID].Ref
	var roomName string
	if in.RoomID != 0 {
		roomName = f.rooms[in.RoomID].Rec.Name
	}
	return SlotRecord{
		SchoolID: in.SchoolID, AcademicYearID: in.AcademicYearID,
		ClassID: in.ClassID, ClassName: cls.Name,
		SubjectID: in.SubjectID, SubjectCode: sub.Code, SubjectName: sub.Name,
		TeacherID: in.TeacherID, TeacherName: tch.Name,
		RoomID: in.RoomID, RoomName: roomName,
		DayOfWeek: in.DayOfWeek, PeriodStart: in.PeriodStart, PeriodEnd: in.PeriodEnd,
	}
}

func (f *fakeScheduleRepo) CreateSlotChecked(ctx context.Context, in SlotInput) (SlotRecord, []SlotRecord, error) {
	existing, _ := f.ListSlotsForYear(ctx, in.SchoolID, in.AcademicYearID)
	conflicts := findConflicts(existing, 0, in.DayOfWeek, in.PeriodStart, in.PeriodEnd, in.TeacherID, in.ClassID, in.RoomID)
	if len(conflicts) > 0 {
		return SlotRecord{}, conflicts, nil
	}
	f.nextSlotID++
	rec := f.toSlotRecord(in)
	rec.ID = f.nextSlotID
	f.slots[rec.ID] = rec
	return rec, nil, nil
}

func (f *fakeScheduleRepo) UpdateSlotChecked(ctx context.Context, schoolID, id int64, in SlotInput) (SlotRecord, []SlotRecord, error) {
	if _, ok := f.slots[id]; !ok {
		return SlotRecord{}, nil, ErrNotFound
	}
	existing, _ := f.ListSlotsForYear(ctx, schoolID, in.AcademicYearID)
	conflicts := findConflicts(existing, id, in.DayOfWeek, in.PeriodStart, in.PeriodEnd, in.TeacherID, in.ClassID, in.RoomID)
	if len(conflicts) > 0 {
		return SlotRecord{}, conflicts, nil
	}
	rec := f.toSlotRecord(in)
	rec.ID = id
	f.slots[id] = rec
	return rec, nil, nil
}

func (f *fakeScheduleRepo) DeleteSlot(ctx context.Context, schoolID, id int64) error {
	delete(f.slots, id)
	return nil
}

func (f *fakeScheduleRepo) CreateSlotsBatch(ctx context.Context, ins []SlotInput) ([]SlotRecord, error) {
	out := make([]SlotRecord, 0, len(ins))
	for _, in := range ins {
		f.nextSlotID++
		rec := f.toSlotRecord(in)
		rec.ID = f.nextSlotID
		f.slots[rec.ID] = rec
		out = append(out, rec)
	}
	return out, nil
}

// -- setup helpers --

func (f *fakeScheduleRepo) setupClass(schoolID, id, yearID int64, name string) {
	f.classes[id] = classEntry{SchoolID: schoolID, Ref: ClassRefRow{ID: id, Name: name, AcademicYearID: yearID}}
}

func (f *fakeScheduleRepo) setupSubject(schoolID, id int64, code, name string) {
	f.subjects[id] = subjectEntry{SchoolID: schoolID, Ref: SubjectRef{ID: id, Code: code, Name: name}}
}

func (f *fakeScheduleRepo) setupTeacher(schoolID, id int64, name, email string) {
	f.teachers[id] = teacherEntry{SchoolID: schoolID, Ref: TeacherRef{ID: id, Name: name}}
	teacherEmails[id] = email
}

func (f *fakeScheduleRepo) setupRoom(schoolID, id int64, name string) {
	f.rooms[id] = roomEntry{SchoolID: schoolID, Rec: RoomRecord{ID: id, Name: name, QRToken: "tok"}}
}

// -- fakeIdentity / fakeYears / fakeStudents --

type fakeIdentity struct {
	perms map[string]map[string]bool
}

func newFakeIdentity() *fakeIdentity {
	return &fakeIdentity{perms: map[string]map[string]bool{
		"admin_sekolah": {PermScheduleManage: true, PermScheduleRead: true},
		"guru":          {PermScheduleRead: true},
		"siswa":         {PermScheduleRead: true},
	}}
}

func (f *fakeIdentity) HasPermission(role, perm string) bool { return f.perms[role][perm] }
func (f *fakeIdentity) Log(ctx context.Context, schoolID, userID *int64, action, entity string, entityID *int64, oldValue, newValue any) error {
	return nil
}

type fakeYears struct{ activeID int64 }

func (f fakeYears) ActiveAcademicYearID(ctx context.Context, schoolID int64) (int64, bool, error) {
	if f.activeID == 0 {
		return 0, false, nil
	}
	return f.activeID, true, nil
}

type fakeStudents struct{ classByUser map[int64]int64 }

func (f fakeStudents) MyTeacherID(ctx context.Context, schoolID, userID int64) (int64, bool, error) {
	return 0, false, nil
}

func (f fakeStudents) MyClassID(ctx context.Context, schoolID, userID int64) (int64, bool, error) {
	id, ok := f.classByUser[userID]
	return id, ok, nil
}

func ctxAs(role string, userID int64, tz string) context.Context {
	ctx := reqctx.WithUser(context.Background(), userID, role, false)
	ctx = reqctx.WithSchool(ctx, reqctx.School{ID: 1, Name: "Sekolah Uji", Slug: "uji", Timezone: tz})
	return ctx
}

func newTestService(repo *fakeScheduleRepo, yearID int64, now time.Time) *Service {
	return newServiceForTest(repo, newFakeIdentity(), fakeYears{activeID: yearID}, fakeStudents{classByUser: map[int64]int64{}}, clock.Fixed{T: now})
}

func domainStatus(t *testing.T, err error) int {
	t.Helper()
	var de *httpx.Error
	if !errors.As(err, &de) {
		t.Fatalf("expected domain error, got: %v", err)
	}
	return de.Status
}

// -- ReplacePeriods --

func TestReplacePeriods_ValidAccepted(t *testing.T) {
	repo := newFakeScheduleRepo()
	svc := newTestService(repo, 10, time.Now())
	ctx := ctxAs("admin_sekolah", 1, "Asia/Jakarta")

	items, err := svc.ReplacePeriods(ctx, 1, 1, []ReplacePeriodInput{
		{Number: 1, StartsAt: "07:00", EndsAt: "07:45"},
		{Number: 2, StartsAt: "07:45", EndsAt: "08:30"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 periods, got %d", len(items))
	}
}

func TestReplacePeriods_OverlapRejected(t *testing.T) {
	repo := newFakeScheduleRepo()
	svc := newTestService(repo, 10, time.Now())
	ctx := ctxAs("admin_sekolah", 1, "Asia/Jakarta")

	_, err := svc.ReplacePeriods(ctx, 1, 1, []ReplacePeriodInput{
		{Number: 1, StartsAt: "07:00", EndsAt: "08:00"},
		{Number: 2, StartsAt: "07:30", EndsAt: "08:30"}, // tumpang tindih dgn jam ke-1
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if status := domainStatus(t, err); status != 422 {
		t.Fatalf("expected 422, got %d", status)
	}
}

func TestReplacePeriods_NonSequentialRejected(t *testing.T) {
	repo := newFakeScheduleRepo()
	svc := newTestService(repo, 10, time.Now())
	ctx := ctxAs("admin_sekolah", 1, "Asia/Jakarta")

	_, err := svc.ReplacePeriods(ctx, 1, 1, []ReplacePeriodInput{
		{Number: 1, StartsAt: "07:00", EndsAt: "07:45"},
		{Number: 3, StartsAt: "07:45", EndsAt: "08:30"}, // lompat ke 3, harusnya 2
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if status := domainStatus(t, err); status != 422 {
		t.Fatalf("expected 422, got %d", status)
	}
}

func TestReplacePeriods_RejectsWhenNumberStillUsedBySlots(t *testing.T) {
	repo := newFakeScheduleRepo()
	repo.setupClass(1, 100, 10, "XII RPL 1")
	repo.setupSubject(1, 200, "BDT", "Basis Data")
	repo.setupTeacher(1, 300, "Rendi", "rendi@demo.sch.id")
	svc := newTestService(repo, 10, time.Now())
	ctx := ctxAs("admin_sekolah", 1, "Asia/Jakarta")

	// slot dulu ada di jam ke-3.
	if _, err := svc.CreateSlot(ctx, 1, 1, SlotInputRequest{ClassID: 100, SubjectID: 200, TeacherID: 300, DayOfWeek: 1, PeriodStart: 3, PeriodEnd: 3}); err != nil {
		t.Fatalf("unexpected error setup slot: %v", err)
	}

	// replace periods TANPA jam ke-3 -> harus ditolak 409.
	_, err := svc.ReplacePeriods(ctx, 1, 1, []ReplacePeriodInput{
		{Number: 1, StartsAt: "07:00", EndsAt: "07:45"},
		{Number: 2, StartsAt: "07:45", EndsAt: "08:30"},
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if status := domainStatus(t, err); status != 409 {
		t.Fatalf("expected 409, got %d", status)
	}
}

// -- CurrentPeriod --

func TestCurrentPeriod_InsidePeriod(t *testing.T) {
	repo := newFakeScheduleRepo()
	repo.periods[1] = []PeriodRecord{
		{ID: 1, Number: 1, StartsAt: "07:00", EndsAt: "07:45"},
		{ID: 2, Number: 2, StartsAt: "07:45", EndsAt: "08:30"},
	}
	// 07:15 WIB = 00:15 UTC.
	now := time.Date(2026, 8, 17, 0, 15, 0, 0, time.UTC)
	svc := newTestService(repo, 10, now)
	ctx := ctxAs("guru", 1, "Asia/Jakarta")

	view, err := svc.CurrentPeriod(ctx, 1, svc.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if view.Period == nil || view.Period.Number != 1 {
		t.Fatalf("expected period 1, got %+v", view)
	}
}

func TestCurrentPeriod_OutsidePeriod(t *testing.T) {
	repo := newFakeScheduleRepo()
	repo.periods[1] = []PeriodRecord{
		{ID: 1, Number: 1, StartsAt: "07:00", EndsAt: "07:45"},
		{ID: 2, Number: 2, StartsAt: "08:00", EndsAt: "08:45"},
	}
	// 07:50 WIB — di antara jam ke-1 dan ke-2.
	now := time.Date(2026, 8, 17, 0, 50, 0, 0, time.UTC)
	svc := newTestService(repo, 10, now)
	ctx := ctxAs("guru", 1, "Asia/Jakarta")

	view, err := svc.CurrentPeriod(ctx, 1, svc.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if view.Period != nil {
		t.Fatalf("expected no current period, got %+v", view.Period)
	}
	if view.NextStartsAt == nil || *view.NextStartsAt != "08:00" {
		t.Fatalf("expected next_starts_at 08:00, got %+v", view.NextStartsAt)
	}
}

func TestCurrentPeriod_TimezoneRespected(t *testing.T) {
	repo := newFakeScheduleRepo()
	repo.periods[1] = []PeriodRecord{{ID: 1, Number: 1, StartsAt: "07:00", EndsAt: "07:45"}}
	// 00:15 UTC = 07:15 WIB (dalam jam ke-1) TAPI = 09:15 WIT (Asia/Jayapura, di luar jam ke-1).
	now := time.Date(2026, 8, 17, 0, 15, 0, 0, time.UTC)

	svcWIB := newTestService(repo, 10, now)
	viewWIB, err := svcWIB.CurrentPeriod(ctxAs("guru", 1, "Asia/Jakarta"), 1, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if viewWIB.Period == nil {
		t.Fatal("expected current period utk WIB")
	}

	svcWIT := newTestService(repo, 10, now)
	viewWIT, err := svcWIT.CurrentPeriod(ctxAs("guru", 1, "Asia/Jayapura"), 1, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if viewWIT.Period != nil {
		t.Fatalf("expected TIDAK ada current period utk WIT (sudah lewat jam ke-1), got %+v", viewWIT.Period)
	}
}

// -- deteksi bentrok (CreateSlot) --

func setupBasicRefs(repo *fakeScheduleRepo) {
	repo.setupClass(1, 100, 10, "XII RPL 1")
	repo.setupClass(1, 101, 10, "XI RPL 2")
	repo.setupSubject(1, 200, "BDT", "Basis Data")
	repo.setupSubject(1, 201, "PWB", "Pemrograman Web")
	repo.setupTeacher(1, 300, "Rendi", "rendi@demo.sch.id")
	repo.setupTeacher(1, 301, "Sari", "sari@demo.sch.id")
	repo.setupRoom(1, 400, "R-101")
}

func TestCreateSlot_TeacherConflictRejected(t *testing.T) {
	repo := newFakeScheduleRepo()
	setupBasicRefs(repo)
	svc := newTestService(repo, 10, time.Now())
	ctx := ctxAs("admin_sekolah", 1, "Asia/Jakarta")

	if _, err := svc.CreateSlot(ctx, 1, 1, SlotInputRequest{ClassID: 100, SubjectID: 200, TeacherID: 300, DayOfWeek: 1, PeriodStart: 3, PeriodEnd: 4}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// guru Rendi (300) dijadwalkan lagi di kelas LAIN, jam beririsan sebagian (4-5 vs 3-4).
	_, err := svc.CreateSlot(ctx, 1, 1, SlotInputRequest{ClassID: 101, SubjectID: 200, TeacherID: 300, DayOfWeek: 1, PeriodStart: 4, PeriodEnd: 5})
	if err == nil {
		t.Fatal("expected bentrok error, got nil")
	}
	if status := domainStatus(t, err); status != 422 {
		t.Fatalf("expected 422, got %d", status)
	}
}

func TestCreateSlot_ClassConflictRejected(t *testing.T) {
	repo := newFakeScheduleRepo()
	setupBasicRefs(repo)
	svc := newTestService(repo, 10, time.Now())
	ctx := ctxAs("admin_sekolah", 1, "Asia/Jakarta")

	if _, err := svc.CreateSlot(ctx, 1, 1, SlotInputRequest{ClassID: 100, SubjectID: 200, TeacherID: 300, DayOfWeek: 1, PeriodStart: 1, PeriodEnd: 2}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// kelas yang sama (100) dijadwalkan mapel lain, guru lain, jam sama persis.
	_, err := svc.CreateSlot(ctx, 1, 1, SlotInputRequest{ClassID: 100, SubjectID: 201, TeacherID: 301, DayOfWeek: 1, PeriodStart: 1, PeriodEnd: 2})
	if err == nil {
		t.Fatal("expected bentrok error, got nil")
	}
}

func TestCreateSlot_RoomConflictRejected(t *testing.T) {
	repo := newFakeScheduleRepo()
	setupBasicRefs(repo)
	svc := newTestService(repo, 10, time.Now())
	ctx := ctxAs("admin_sekolah", 1, "Asia/Jakarta")

	if _, err := svc.CreateSlot(ctx, 1, 1, SlotInputRequest{ClassID: 100, SubjectID: 200, TeacherID: 300, RoomID: 400, DayOfWeek: 2, PeriodStart: 1, PeriodEnd: 2}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// ruang sama, kelas & guru beda, jam beririsan sebagian.
	_, err := svc.CreateSlot(ctx, 1, 1, SlotInputRequest{ClassID: 101, SubjectID: 201, TeacherID: 301, RoomID: 400, DayOfWeek: 2, PeriodStart: 2, PeriodEnd: 3})
	if err == nil {
		t.Fatal("expected bentrok error (ruang), got nil")
	}
}

func TestCreateSlot_NonOverlappingPeriodsAllowed(t *testing.T) {
	repo := newFakeScheduleRepo()
	setupBasicRefs(repo)
	svc := newTestService(repo, 10, time.Now())
	ctx := ctxAs("admin_sekolah", 1, "Asia/Jakarta")

	if _, err := svc.CreateSlot(ctx, 1, 1, SlotInputRequest{ClassID: 100, SubjectID: 200, TeacherID: 300, DayOfWeek: 1, PeriodStart: 1, PeriodEnd: 2}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// guru sama, kelas sama, tapi jam TIDAK beririsan -> boleh.
	if _, err := svc.CreateSlot(ctx, 1, 1, SlotInputRequest{ClassID: 100, SubjectID: 201, TeacherID: 300, DayOfWeek: 1, PeriodStart: 3, PeriodEnd: 4}); err != nil {
		t.Fatalf("expected sukses (tidak beririsan), got error: %v", err)
	}
}

func TestCreateSlot_DifferentAcademicYearNoConflict(t *testing.T) {
	repo := newFakeScheduleRepo()
	repo.setupClass(1, 100, 10, "XII RPL 1")
	repo.setupClass(1, 102, 20, "XII RPL 1") // kelas sama nama, TA lain
	repo.setupSubject(1, 200, "BDT", "Basis Data")
	repo.setupTeacher(1, 300, "Rendi", "rendi@demo.sch.id")
	svc := newTestService(repo, 10, time.Now())
	ctx := ctxAs("admin_sekolah", 1, "Asia/Jakarta")

	if _, err := svc.CreateSlot(ctx, 1, 1, SlotInputRequest{ClassID: 100, SubjectID: 200, TeacherID: 300, DayOfWeek: 1, PeriodStart: 1, PeriodEnd: 2, AcademicYearID: 10}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// guru & jam sama persis, tapi TA BEDA (20) -> tidak boleh bentrok.
	if _, err := svc.CreateSlot(ctx, 1, 1, SlotInputRequest{ClassID: 102, SubjectID: 200, TeacherID: 300, DayOfWeek: 1, PeriodStart: 1, PeriodEnd: 2, AcademicYearID: 20}); err != nil {
		t.Fatalf("expected sukses (TA beda tidak bentrok), got error: %v", err)
	}
}

// -- CopySchedule --

// TestCopySchedule_SkipsConflictingSlot menyalin jadwal LINTAS tahun ajaran
// (docs/04-schedule.md Builder UI: "duplikat dari kelas lain / tahun ajaran
// sebelumnya sebagai titik awal") — satu-satunya cara realistis bentrok guru
// muncul saat copy: TA tujuan (20) sudah independen punya jadwal guru yang
// sama di jam yang sama untuk kelas LAIN, sedangkan TA sumber (10) tidak
// pernah bisa punya slot bentrok internal (sudah dicegah saat dibuat).
func TestCopySchedule_SkipsConflictingSlot(t *testing.T) {
	repo := newFakeScheduleRepo()
	setupBasicRefs(repo)                     // kelas 100 (TA 10), 101 (TA 10), mapel 200/201, guru 300/301, ruang 400
	repo.setupClass(1, 102, 20, "XI RPL 3")  // kelas TUJUAN, TA 20 (BEDA), kosong
	repo.setupClass(1, 103, 20, "XII RPL 4") // kelas LAIN di TA 20 yang SUDAH pakai guru Rendi
	svc := newTestService(repo, 10, time.Now())
	ctx := ctxAs("admin_sekolah", 1, "Asia/Jakarta")

	// sumber (100, TA 10): 2 slot, guru Rendi(300) & Sari(301).
	if _, err := svc.CreateSlot(ctx, 1, 1, SlotInputRequest{ClassID: 100, SubjectID: 200, TeacherID: 300, DayOfWeek: 1, PeriodStart: 1, PeriodEnd: 2, AcademicYearID: 10}); err != nil {
		t.Fatalf("setup gagal: %v", err)
	}
	if _, err := svc.CreateSlot(ctx, 1, 1, SlotInputRequest{ClassID: 100, SubjectID: 201, TeacherID: 301, DayOfWeek: 2, PeriodStart: 1, PeriodEnd: 2, AcademicYearID: 10}); err != nil {
		t.Fatalf("setup gagal: %v", err)
	}
	// kelas 103 (TA 20, TA TUJUAN) sudah punya guru Rendi di hari/jam yang
	// SAMA dengan slot sumber pertama — valid karena TA 20 independen dari TA 10.
	if _, err := svc.CreateSlot(ctx, 1, 1, SlotInputRequest{ClassID: 103, SubjectID: 200, TeacherID: 300, DayOfWeek: 1, PeriodStart: 1, PeriodEnd: 2, AcademicYearID: 20}); err != nil {
		t.Fatalf("setup gagal: %v", err)
	}

	result, err := svc.CopySchedule(ctx, 1, 1, CopyInput{FromClassID: 100, ToClassID: 102})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Copied != 1 {
		t.Fatalf("expected 1 slot copied (1 skipped karena bentrok guru di TA tujuan), got %d", result.Copied)
	}
	if len(result.Skipped) != 1 {
		t.Fatalf("expected 1 skipped, got %+v", result.Skipped)
	}
}

func TestCopySchedule_RejectsWhenDestinationNotEmpty(t *testing.T) {
	repo := newFakeScheduleRepo()
	setupBasicRefs(repo)
	svc := newTestService(repo, 10, time.Now())
	ctx := ctxAs("admin_sekolah", 1, "Asia/Jakarta")

	if _, err := svc.CreateSlot(ctx, 1, 1, SlotInputRequest{ClassID: 100, SubjectID: 200, TeacherID: 300, DayOfWeek: 1, PeriodStart: 1, PeriodEnd: 2}); err != nil {
		t.Fatalf("setup gagal: %v", err)
	}
	if _, err := svc.CreateSlot(ctx, 1, 1, SlotInputRequest{ClassID: 101, SubjectID: 201, TeacherID: 301, DayOfWeek: 3, PeriodStart: 1, PeriodEnd: 2}); err != nil {
		t.Fatalf("setup gagal: %v", err)
	}

	_, err := svc.CopySchedule(ctx, 1, 1, CopyInput{FromClassID: 100, ToClassID: 101})
	if err == nil {
		t.Fatal("expected error (tujuan sudah punya jadwal), got nil")
	}
	if status := domainStatus(t, err); status != 422 {
		t.Fatalf("expected 422, got %d", status)
	}
}

// -- ListSlots object-level (siswa) --

func TestListSlots_StudentObjectLevel(t *testing.T) {
	repo := newFakeScheduleRepo()
	setupBasicRefs(repo)
	svc := newTestService(repo, 10, time.Now())
	admin := ctxAs("admin_sekolah", 1, "Asia/Jakarta")
	if _, err := svc.CreateSlot(admin, 1, 1, SlotInputRequest{ClassID: 100, SubjectID: 200, TeacherID: 300, DayOfWeek: 1, PeriodStart: 1, PeriodEnd: 2}); err != nil {
		t.Fatalf("setup gagal: %v", err)
	}

	svc.students = fakeStudents{classByUser: map[int64]int64{500: 100}}
	studentCtx := ctxAs("siswa", 500, "Asia/Jakarta")

	// siswa minta class_id miliknya sendiri -> boleh.
	items, err := svc.ListSlots(studentCtx, 1, ListSlotsQuery{ClassID: 100})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 slot, got %d", len(items))
	}

	// siswa minta class_id kelas LAIN -> ditolak.
	_, err = svc.ListSlots(studentCtx, 1, ListSlotsQuery{ClassID: 101})
	if err == nil {
		t.Fatal("expected forbidden, got nil")
	}
	if status := domainStatus(t, err); status != 403 {
		t.Fatalf("expected 403, got %d", status)
	}
}

// -- SlotsForDayOfWeek & SlotOwnership (fase 6, dipakai modul teaching &
// attendance lewat consumer-side interface) --

func TestSlotsForDayOfWeek(t *testing.T) {
	repo := newFakeScheduleRepo()
	setupBasicRefs(repo)
	svc := newTestService(repo, 10, time.Now())
	admin := ctxAs("admin_sekolah", 1, "Asia/Jakarta")

	if _, err := svc.CreateSlot(admin, 1, 1, SlotInputRequest{ClassID: 100, SubjectID: 200, TeacherID: 300, DayOfWeek: 1, PeriodStart: 1, PeriodEnd: 2}); err != nil {
		t.Fatalf("setup gagal: %v", err)
	}
	if _, err := svc.CreateSlot(admin, 1, 1, SlotInputRequest{ClassID: 100, SubjectID: 200, TeacherID: 300, DayOfWeek: 2, PeriodStart: 1, PeriodEnd: 2}); err != nil {
		t.Fatalf("setup gagal: %v", err)
	}

	senin, err := svc.SlotsForDayOfWeek(admin, 1, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(senin) != 1 {
		t.Fatalf("expected 1 slot Senin, got %d", len(senin))
	}

	// Minggu (0) — TIDAK ada slot ditambahkan -> kosong, TAPI tidak error
	// (hari itu sendiri valid sejak fase 6, lihat model.go dayNames).
	minggu, err := svc.SlotsForDayOfWeek(admin, 1, 0)
	if err != nil {
		t.Fatalf("unexpected error hari Minggu: %v", err)
	}
	if len(minggu) != 0 {
		t.Fatalf("expected 0 slot Minggu, got %d", len(minggu))
	}
}

func TestSlotOwnership(t *testing.T) {
	repo := newFakeScheduleRepo()
	setupBasicRefs(repo)
	svc := newTestService(repo, 10, time.Now())
	admin := ctxAs("admin_sekolah", 1, "Asia/Jakarta")

	created, err := svc.CreateSlot(admin, 1, 1, SlotInputRequest{ClassID: 100, SubjectID: 200, TeacherID: 300, DayOfWeek: 1, PeriodStart: 1, PeriodEnd: 2})
	if err != nil {
		t.Fatalf("setup gagal: %v", err)
	}

	classID, teacherID, ok, err := svc.SlotOwnership(admin, 1, created.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok || classID != 100 || teacherID != 300 {
		t.Fatalf("expected class=100 teacher=300 ok=true, got class=%d teacher=%d ok=%v", classID, teacherID, ok)
	}

	_, _, ok2, err := svc.SlotOwnership(admin, 1, 99999)
	if err != nil {
		t.Fatalf("unexpected error slot tidak ada: %v", err)
	}
	if ok2 {
		t.Fatal("expected ok=false utk slot tidak ditemukan")
	}
}
