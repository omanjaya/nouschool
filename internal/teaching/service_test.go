package teaching

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/omanjaya/nouschool/internal/platform/clock"
	"github.com/omanjaya/nouschool/internal/platform/httpx"
	"github.com/omanjaya/nouschool/internal/platform/reqctx"
)

// -- fakeTeachingRepo: implementasi teachingRepository in-memory (tanpa DB) —
// pola yang sama dengan internal/schedule & internal/attendance service_test.go.

type fakeTeachingRepo struct {
	journals map[int64]JournalRecord
	nextID   int64

	rooms        map[int64]RoomRef
	roomsByToken map[string]RoomRef
	classes      map[int64]ClassRef
	subjects     map[int64]SubjectRef
	teacherRefs  map[int64]TeacherRef
	teacherUsers map[int64]int64

	settings Settings
}

func newFakeTeachingRepo() *fakeTeachingRepo {
	return &fakeTeachingRepo{
		journals:     map[int64]JournalRecord{},
		rooms:        map[int64]RoomRef{},
		roomsByToken: map[string]RoomRef{},
		classes:      map[int64]ClassRef{},
		subjects:     map[int64]SubjectRef{},
		teacherRefs:  map[int64]TeacherRef{},
		teacherUsers: map[int64]int64{},
		settings:     DefaultSettings(),
	}
}

func sameDate(a, b time.Time) bool {
	return a.Year() == b.Year() && a.Month() == b.Month() && a.Day() == b.Day()
}

func (f *fakeTeachingRepo) InsertJournal(ctx context.Context, in JournalInput) (JournalRecord, error) {
	f.nextID++
	flags := in.Flags
	if flags == nil {
		flags = []string{}
	}
	rec := JournalRecord{
		ID: f.nextID, SchoolID: in.SchoolID, TeacherID: in.TeacherID, ScheduleSlotID: in.ScheduleSlotID,
		ClassID: in.ClassID, SubjectID: in.SubjectID, RoomID: in.RoomID, Date: in.Date, StartedAt: in.StartedAt, Flags: flags,
	}
	f.journals[rec.ID] = rec
	return rec, nil
}

func (f *fakeTeachingRepo) GetJournalByID(ctx context.Context, schoolID, id int64) (JournalRecord, error) {
	rec, ok := f.journals[id]
	if !ok || rec.SchoolID != schoolID {
		return JournalRecord{}, ErrNotFound
	}
	return rec, nil
}

func (f *fakeTeachingRepo) GetJournalBySlotDate(ctx context.Context, schoolID, slotID int64, date time.Time) (JournalRecord, error) {
	for _, rec := range f.journals {
		if rec.SchoolID == schoolID && rec.ScheduleSlotID == slotID && sameDate(rec.Date, date) {
			return rec, nil
		}
	}
	return JournalRecord{}, ErrNotFound
}

func (f *fakeTeachingRepo) detailFrom(rec JournalRecord) JournalDetail {
	d := JournalDetail{
		JournalRecord: rec,
		TeacherUserID: f.teacherUsers[rec.TeacherID],
		TeacherName:   f.teacherRefs[rec.TeacherID].Name,
		ClassName:     f.classes[rec.ClassID].Name,
	}
	if rec.SubjectID != 0 {
		sub := f.subjects[rec.SubjectID]
		d.SubjectCode, d.SubjectName = sub.Code, sub.Name
	}
	if rec.RoomID != 0 {
		d.RoomName = f.rooms[rec.RoomID].Name
	}
	return d
}

func (f *fakeTeachingRepo) GetJournalDetail(ctx context.Context, schoolID, id int64) (JournalDetail, error) {
	rec, err := f.GetJournalByID(ctx, schoolID, id)
	if err != nil {
		return JournalDetail{}, err
	}
	return f.detailFrom(rec), nil
}

func (f *fakeTeachingRepo) GetJournalDetailBySlotDate(ctx context.Context, schoolID, slotID int64, date time.Time) (JournalDetail, error) {
	rec, err := f.GetJournalBySlotDate(ctx, schoolID, slotID, date)
	if err != nil {
		return JournalDetail{}, err
	}
	return f.detailFrom(rec), nil
}

func (f *fakeTeachingRepo) ListJournals(ctx context.Context, schoolID int64, teacherID int64, date time.Time, monthStart, monthEnd time.Time) ([]JournalDetail, error) {
	var out []JournalDetail
	for _, rec := range f.journals {
		if rec.SchoolID != schoolID {
			continue
		}
		if teacherID != 0 && rec.TeacherID != teacherID {
			continue
		}
		if !date.IsZero() && !sameDate(rec.Date, date) {
			continue
		}
		if !monthStart.IsZero() && (rec.Date.Before(monthStart) || !rec.Date.Before(monthEnd)) {
			continue
		}
		out = append(out, f.detailFrom(rec))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].StartedAt.After(out[j].StartedAt) })
	if len(out) > 100 {
		out = out[:100]
	}
	return out, nil
}

func (f *fakeTeachingRepo) UpdateJournalNote(ctx context.Context, schoolID, id int64, material, note string) (JournalRecord, error) {
	rec, err := f.GetJournalByID(ctx, schoolID, id)
	if err != nil {
		return JournalRecord{}, err
	}
	rec.Material, rec.Note = material, note
	f.journals[id] = rec
	return rec, nil
}

func (f *fakeTeachingRepo) EndJournal(ctx context.Context, schoolID, id int64, endedAt time.Time) (JournalRecord, error) {
	rec, err := f.GetJournalByID(ctx, schoolID, id)
	if err != nil {
		return JournalRecord{}, err
	}
	rec.EndedAt = endedAt
	f.journals[id] = rec
	return rec, nil
}

func (f *fakeTeachingRepo) LookupRoomByToken(ctx context.Context, schoolID int64, token string) (RoomRef, error) {
	r, ok := f.roomsByToken[token]
	if !ok {
		return RoomRef{}, ErrNotFound
	}
	return r, nil
}

func (f *fakeTeachingRepo) LookupRoomByID(ctx context.Context, schoolID, id int64) (RoomRef, error) {
	r, ok := f.rooms[id]
	if !ok {
		return RoomRef{}, ErrNotFound
	}
	return r, nil
}

func (f *fakeTeachingRepo) LookupClassRef(ctx context.Context, schoolID, id int64) (ClassRef, error) {
	c, ok := f.classes[id]
	if !ok {
		return ClassRef{}, ErrNotFound
	}
	return c, nil
}

func (f *fakeTeachingRepo) LookupSubjectRef(ctx context.Context, schoolID, id int64) (SubjectRef, error) {
	s, ok := f.subjects[id]
	if !ok {
		return SubjectRef{}, ErrNotFound
	}
	return s, nil
}

func (f *fakeTeachingRepo) LookupTeacherUserID(ctx context.Context, schoolID, teacherID int64) (int64, error) {
	v, ok := f.teacherUsers[teacherID]
	if !ok {
		return 0, ErrNotFound
	}
	return v, nil
}

func (f *fakeTeachingRepo) LookupTeacherRef(ctx context.Context, schoolID, teacherID int64) (TeacherRef, error) {
	r, ok := f.teacherRefs[teacherID]
	if !ok {
		return TeacherRef{}, ErrNotFound
	}
	return r, nil
}

func (f *fakeTeachingRepo) GetSettings(ctx context.Context, schoolID int64) (Settings, error) {
	return f.settings, nil
}

// -- fake gateways --

type fakeIdentity struct{ perms map[string]map[string]bool }

func newFakeIdentity() *fakeIdentity {
	return &fakeIdentity{perms: map[string]map[string]bool{
		"admin_sekolah":  {PermTeachingMonitor: true},
		"kepala_sekolah": {PermTeachingMonitor: true},
		"guru":           {PermTeachingJournalWrite: true},
	}}
}

func (f *fakeIdentity) HasPermission(role, perm string) bool { return f.perms[role][perm] }
func (f *fakeIdentity) Log(ctx context.Context, schoolID, userID *int64, action, entity string, entityID *int64, oldValue, newValue any) error {
	return nil
}

type fakeSchedule struct {
	slotNow   map[int64]SlotInfo // teacherID -> slot berjalan skrg
	byID      map[int64]SlotInfo // slotID -> slot
	byDay     map[int][]SlotInfo
	curPeriod *CurrentPeriodInfo
}

func (f fakeSchedule) SlotNow(ctx context.Context, schoolID, teacherID int64, at time.Time) (SlotInfo, bool, error) {
	s, ok := f.slotNow[teacherID]
	return s, ok, nil
}

func (f fakeSchedule) SlotByID(ctx context.Context, schoolID, slotID int64) (SlotInfo, bool, error) {
	s, ok := f.byID[slotID]
	return s, ok, nil
}

func (f fakeSchedule) SlotsForDayOfWeek(ctx context.Context, schoolID int64, dayOfWeek int) ([]SlotInfo, error) {
	return f.byDay[dayOfWeek], nil
}

func (f fakeSchedule) CurrentPeriod(ctx context.Context, schoolID int64, at time.Time) (int, string, string, bool, error) {
	if f.curPeriod == nil {
		return 0, "", "", false, nil
	}
	return f.curPeriod.Number, f.curPeriod.StartsAt, f.curPeriod.EndsAt, true, nil
}

type fakeLeave struct{ approved map[string]bool } // key "userID|date"

func (f fakeLeave) ApprovedOn(ctx context.Context, schoolID, teacherUserID int64, date string) (bool, error) {
	return f.approved[fmt.Sprintf("%d|%s", teacherUserID, date)], nil
}

type fakeAttendance struct {
	calls         int
	nextSessionID int64
	sessions      map[string]int64 // key "slotID|date"
}

func newFakeAttendance() *fakeAttendance { return &fakeAttendance{sessions: map[string]int64{}} }

func (f *fakeAttendance) OpenSubjectSession(ctx context.Context, schoolID, actorUserID, scheduleSlotID int64, date string) (int64, error) {
	f.calls++
	key := fmt.Sprintf("%d|%s", scheduleSlotID, date)
	if id, ok := f.sessions[key]; ok {
		return id, nil
	}
	f.nextSessionID++
	f.sessions[key] = f.nextSessionID
	return f.nextSessionID, nil
}

type fakeStudentsGW struct{ byUser map[int64]int64 } // userID -> teacherID

func (f fakeStudentsGW) MyTeacherID(ctx context.Context, schoolID, userID int64) (int64, bool, error) {
	id, ok := f.byUser[userID]
	return id, ok, nil
}

func ctxAs(role string, userID int64, tz string) context.Context {
	ctx := reqctx.WithUser(context.Background(), userID, role, false)
	ctx = reqctx.WithSchool(ctx, reqctx.School{ID: 1, Name: "Sekolah Uji", Slug: "uji", Timezone: tz})
	return ctx
}

func domainStatus(t *testing.T, err error) int {
	t.Helper()
	var de *httpx.Error
	if !errors.As(err, &de) {
		t.Fatalf("expected domain error, got: %v", err)
	}
	return de.Status
}

// -- Scan --

func TestScan_SlotFound_CreatesJournalAndSession(t *testing.T) {
	repo := newFakeTeachingRepo()
	repo.roomsByToken["tok-101"] = RoomRef{ID: 10, Name: "R-101"}
	repo.teacherUsers[300] = 999
	repo.teacherRefs[300] = TeacherRef{ID: 300, Name: "Rendi"}

	slot := SlotInfo{
		ID: 50, ClassID: 1, ClassName: "XII RPL 1", SubjectID: 5, SubjectCode: "BDT", SubjectName: "Basis Data",
		TeacherID: 300, TeacherName: "Rendi", RoomID: 10, RoomName: "R-101",
		PeriodStart: 1, PeriodEnd: 2, PeriodStartsAt: "07:40", PeriodEndsAt: "08:20",
	}
	sched := fakeSchedule{slotNow: map[int64]SlotInfo{300: slot}, byID: map[int64]SlotInfo{50: slot}}
	att := newFakeAttendance()
	students := fakeStudentsGW{byUser: map[int64]int64{999: 300}}

	// 08:00 WIB = 01:00 UTC pada 2026-08-10 -> dalam periode 07:40-08:20.
	fixed := clock.Fixed{T: time.Date(2026, 8, 10, 1, 0, 0, 0, time.UTC)}
	svc := newServiceForTest(repo, newFakeIdentity(), sched, fakeLeave{}, att, students, fixed)
	ctx := ctxAs("guru", 999, "Asia/Jakarta")

	result, err := svc.Scan(ctx, 999, 1, ScanInput{RoomToken: "tok-101"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.NeedsManual {
		t.Fatal("expected needs_manual=false")
	}
	if result.Journal == nil {
		t.Fatal("expected journal terisi")
	}
	if result.Journal.Status != JournalOngoing {
		t.Fatalf("expected status ongoing, got %s", result.Journal.Status)
	}
	if len(result.Journal.Flags) != 0 {
		t.Fatalf("expected tanpa flag (ruang cocok), got %v", result.Journal.Flags)
	}
	if result.AttendanceSessionID == nil {
		t.Fatal("expected attendance_session_id terisi")
	}
	if att.calls != 1 {
		t.Fatalf("expected OpenSubjectSession dipanggil 1x, got %d", att.calls)
	}
	if len(repo.journals) != 1 {
		t.Fatalf("expected 1 journal tersimpan, got %d", len(repo.journals))
	}
}

func TestScan_Idempotent(t *testing.T) {
	repo := newFakeTeachingRepo()
	repo.roomsByToken["tok-101"] = RoomRef{ID: 10, Name: "R-101"}
	slot := SlotInfo{
		ID: 50, ClassID: 1, ClassName: "XII RPL 1", SubjectID: 5, SubjectCode: "BDT", SubjectName: "Basis Data",
		TeacherID: 300, TeacherName: "Rendi", RoomID: 10, RoomName: "R-101",
		PeriodStart: 1, PeriodEnd: 2, PeriodStartsAt: "07:40", PeriodEndsAt: "08:20",
	}
	sched := fakeSchedule{slotNow: map[int64]SlotInfo{300: slot}, byID: map[int64]SlotInfo{50: slot}}
	att := newFakeAttendance()
	students := fakeStudentsGW{byUser: map[int64]int64{999: 300}}
	fixed := clock.Fixed{T: time.Date(2026, 8, 10, 1, 0, 0, 0, time.UTC)}
	svc := newServiceForTest(repo, newFakeIdentity(), sched, fakeLeave{}, att, students, fixed)
	ctx := ctxAs("guru", 999, "Asia/Jakarta")

	first, err := svc.Scan(ctx, 999, 1, ScanInput{RoomToken: "tok-101"})
	if err != nil {
		t.Fatalf("unexpected error scan pertama: %v", err)
	}
	second, err := svc.Scan(ctx, 999, 1, ScanInput{RoomToken: "tok-101"})
	if err != nil {
		t.Fatalf("unexpected error scan kedua: %v", err)
	}
	if second.Journal.ID != first.Journal.ID {
		t.Fatalf("expected journal sama (idempoten), got %d vs %d", first.Journal.ID, second.Journal.ID)
	}
	if len(repo.journals) != 1 {
		t.Fatalf("expected TETAP 1 journal setelah scan ulang, got %d", len(repo.journals))
	}
	if *second.AttendanceSessionID != *first.AttendanceSessionID {
		t.Fatalf("expected sesi absen sama, got %d vs %d", *first.AttendanceSessionID, *second.AttendanceSessionID)
	}
	if att.calls != 2 {
		t.Fatalf("expected OpenSubjectSession dipanggil 2x (idempoten di sisi attendance), got %d", att.calls)
	}
}

func TestScan_RoomMismatchFlag(t *testing.T) {
	repo := newFakeTeachingRepo()
	repo.roomsByToken["tok-lab"] = RoomRef{ID: 99, Name: "Lab Komputer 1"}
	slot := SlotInfo{
		ID: 50, ClassID: 1, ClassName: "XII RPL 1", SubjectID: 5, SubjectCode: "BDT", SubjectName: "Basis Data",
		TeacherID: 300, TeacherName: "Rendi", RoomID: 10, RoomName: "R-101", // slot terjadwal di R-101
		PeriodStart: 1, PeriodEnd: 2, PeriodStartsAt: "07:40", PeriodEndsAt: "08:20",
	}
	sched := fakeSchedule{slotNow: map[int64]SlotInfo{300: slot}, byID: map[int64]SlotInfo{50: slot}}
	att := newFakeAttendance()
	students := fakeStudentsGW{byUser: map[int64]int64{999: 300}}
	fixed := clock.Fixed{T: time.Date(2026, 8, 10, 1, 0, 0, 0, time.UTC)}
	svc := newServiceForTest(repo, newFakeIdentity(), sched, fakeLeave{}, att, students, fixed)
	ctx := ctxAs("guru", 999, "Asia/Jakarta")

	result, err := svc.Scan(ctx, 999, 1, ScanInput{RoomToken: "tok-lab"}) // scan di Lab, bukan R-101
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Journal.Flags) != 1 || result.Journal.Flags[0] != FlagRoomMismatch {
		t.Fatalf("expected flag room_mismatch, got %v", result.Journal.Flags)
	}
	if result.Journal.Room == nil || result.Journal.Room.ID != 99 {
		t.Fatalf("expected room aktual = Lab Komputer 1 (99), got %+v", result.Journal.Room)
	}
}

func TestScan_NeedsManual_NoSlot(t *testing.T) {
	repo := newFakeTeachingRepo()
	repo.roomsByToken["tok-101"] = RoomRef{ID: 10, Name: "R-101"}
	sched := fakeSchedule{} // tidak ada slot berjalan utk guru manapun
	att := newFakeAttendance()
	students := fakeStudentsGW{byUser: map[int64]int64{999: 300}}
	fixed := clock.Fixed{T: time.Date(2026, 8, 10, 1, 0, 0, 0, time.UTC)}
	svc := newServiceForTest(repo, newFakeIdentity(), sched, fakeLeave{}, att, students, fixed)
	ctx := ctxAs("guru", 999, "Asia/Jakarta")

	result, err := svc.Scan(ctx, 999, 1, ScanInput{RoomToken: "tok-101"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.NeedsManual {
		t.Fatal("expected needs_manual=true")
	}
	if result.Journal != nil {
		t.Fatal("expected TIDAK ada journal dibuat")
	}
	if result.Room == nil || result.Room.ID != 10 {
		t.Fatalf("expected room diisi (10), got %+v", result.Room)
	}
	if len(repo.journals) != 0 {
		t.Fatalf("expected 0 journal, got %d", len(repo.journals))
	}
	if att.calls != 0 {
		t.Fatalf("expected sesi absen TIDAK dibuka, got %d panggilan", att.calls)
	}
}

// -- derivasi status jurnal: ongoing -> done saat periode berakhir --

func TestJournalStatus_OngoingThenDoneByPeriodEnd(t *testing.T) {
	repo := newFakeTeachingRepo()
	repo.roomsByToken["tok-101"] = RoomRef{ID: 10, Name: "R-101"}
	slot := SlotInfo{
		ID: 50, ClassID: 1, ClassName: "XII RPL 1", SubjectID: 5, SubjectCode: "BDT", SubjectName: "Basis Data",
		TeacherID: 300, TeacherName: "Rendi", RoomID: 10, RoomName: "R-101",
		PeriodStart: 1, PeriodEnd: 2, PeriodStartsAt: "07:40", PeriodEndsAt: "08:20",
	}
	sched := fakeSchedule{slotNow: map[int64]SlotInfo{300: slot}, byID: map[int64]SlotInfo{50: slot}}
	students := fakeStudentsGW{byUser: map[int64]int64{999: 300}}
	ctx := ctxAs("guru", 999, "Asia/Jakarta")

	// Scan pada 08:00 WIB (dalam periode) memakai svcEarly.
	early := clock.Fixed{T: time.Date(2026, 8, 10, 1, 0, 0, 0, time.UTC)} // 08:00 WIB
	svcEarly := newServiceForTest(repo, newFakeIdentity(), sched, fakeLeave{}, newFakeAttendance(), students, early)
	res, err := svcEarly.Scan(ctx, 999, 1, ScanInput{RoomToken: "tok-101"})
	if err != nil {
		t.Fatalf("unexpected error scan: %v", err)
	}
	if res.Journal.Status != JournalOngoing {
		t.Fatalf("expected ongoing saat dalam periode, got %s", res.Journal.Status)
	}

	// Baca ulang jurnal yang SAMA (repo sama) pada 09:00 WIB (setelah 08:20) -> done.
	late := clock.Fixed{T: time.Date(2026, 8, 10, 2, 0, 0, 0, time.UTC)} // 09:00 WIB
	svcLate := newServiceForTest(repo, newFakeIdentity(), sched, fakeLeave{}, newFakeAttendance(), students, late)
	list, err := svcLate.ListJournals(ctx, 1, ListQuery{Scope: "mine"})
	if err != nil {
		t.Fatalf("unexpected error list: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 journal, got %d", len(list))
	}
	if list[0].Status != JournalDone {
		t.Fatalf("expected done setelah periode berakhir, got %s", list[0].Status)
	}
}

// -- GET /api/teaching/status: semua cabang derivasi --

func TestStatus_AllBranches(t *testing.T) {
	repo := newFakeTeachingRepo()
	date := time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC)
	dow := int(date.Weekday())
	dateStr := date.Format("2006-01-02")

	// now = 08:00 WIB = 01:00 UTC.
	now := time.Date(2026, 8, 10, 1, 0, 0, 0, time.UTC)

	mk := func(id, teacherID int64, startsAt, endsAt string) SlotInfo {
		return SlotInfo{
			ID: id, ClassID: id, ClassName: fmt.Sprintf("Kelas %d", id), SubjectID: id, SubjectCode: "SUB", SubjectName: "Mapel",
			TeacherID: teacherID, TeacherName: fmt.Sprintf("Guru %d", teacherID),
			PeriodStart: 1, PeriodEnd: 1, PeriodStartsAt: startsAt, PeriodEndsAt: endsAt,
		}
	}

	slotMengajar := mk(501, 301, "07:40", "08:20")    // journal ada, now di dalam -> mengajar
	slotSelesai := mk(502, 302, "06:00", "07:00")     // journal ada, now sudah lewat -> selesai
	slotIzin := mk(503, 303, "07:00", "07:40")        // tanpa journal, lewat, TAPI izin -> izin (menang atas belum_masuk)
	slotBelumMulai1 := mk(504, 304, "09:00", "10:00") // belum mulai
	slotBelumMulai2 := mk(505, 305, "07:55", "08:30") // dalam grace period (threshold 10 menit, mulai 07:55, now 08:00)
	slotBelumMasuk1 := mk(506, 306, "07:40", "08:30") // overdue (>10 menit) tapi slot masih berjalan
	slotBelumMasuk2 := mk(507, 307, "06:00", "07:00") // slot sudah lewat lama, tanpa journal, tanpa izin

	repo.journals[1] = JournalRecord{ID: 1, SchoolID: 1, TeacherID: 301, ScheduleSlotID: 501, ClassID: 501, Date: date, StartedAt: now}
	repo.journals[2] = JournalRecord{ID: 2, SchoolID: 1, TeacherID: 302, ScheduleSlotID: 502, ClassID: 502, Date: date, StartedAt: now}
	for _, teacherID := range []int64{303, 304, 305, 306, 307} {
		repo.teacherUsers[teacherID] = teacherID * 10
	}

	sched := fakeSchedule{byDay: map[int][]SlotInfo{
		dow: {slotMengajar, slotSelesai, slotIzin, slotBelumMulai1, slotBelumMulai2, slotBelumMasuk1, slotBelumMasuk2},
	}}
	lv := fakeLeave{approved: map[string]bool{fmt.Sprintf("%d|%s", int64(303*10), dateStr): true}}

	svc := newServiceForTest(repo, newFakeIdentity(), sched, lv, newFakeAttendance(), fakeStudentsGW{}, clock.Fixed{T: now})
	ctx := ctxAs("admin_sekolah", 1, "Asia/Jakarta")

	result, err := svc.Status(ctx, 1, dateStr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	byID := map[int64]string{}
	for _, row := range result.Rows {
		byID[row.Slot.ID] = row.Status
	}
	want := map[int64]string{
		501: StatusMengajar,
		502: StatusSelesai,
		503: StatusIzin,
		504: StatusBelumMulai,
		505: StatusBelumMulai,
		506: StatusBelumMasuk,
		507: StatusBelumMasuk,
	}
	for id, wantStatus := range want {
		if got := byID[id]; got != wantStatus {
			t.Errorf("slot %d: expected status %s, got %s", id, wantStatus, got)
		}
	}

	if result.Summary.Mengajar != 1 || result.Summary.Selesai != 1 || result.Summary.Izin != 1 ||
		result.Summary.BelumMulai != 2 || result.Summary.BelumMasuk != 2 {
		t.Fatalf("summary tidak sesuai: %+v", result.Summary)
	}
}

// -- PATCH: jendela H+2 & akses pemilik/admin --

func setupJournalForPatch(repo *fakeTeachingRepo, date time.Time) int64 {
	repo.journals[1] = JournalRecord{ID: 1, SchoolID: 1, TeacherID: 300, ClassID: 1, Date: date, StartedAt: date, Flags: []string{FlagUnscheduled}}
	repo.nextID = 1
	return 1
}

func TestPatchJournal_WindowH2(t *testing.T) {
	date := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	students := fakeStudentsGW{byUser: map[int64]int64{999: 300}}

	t.Run("dalam H+2 OK", func(t *testing.T) {
		repo := newFakeTeachingRepo()
		id := setupJournalForPatch(repo, date)
		fixed := clock.Fixed{T: date.AddDate(0, 0, 2)}
		svc := newServiceForTest(repo, newFakeIdentity(), fakeSchedule{}, fakeLeave{}, newFakeAttendance(), students, fixed)
		ctx := ctxAs("guru", 999, "Asia/Jakarta")
		if _, err := svc.UpdateJournal(ctx, 999, 1, id, PatchInput{Material: "Materi A"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("lewat H+2 ditolak utk guru", func(t *testing.T) {
		repo := newFakeTeachingRepo()
		id := setupJournalForPatch(repo, date)
		fixed := clock.Fixed{T: date.AddDate(0, 0, 3)}
		svc := newServiceForTest(repo, newFakeIdentity(), fakeSchedule{}, fakeLeave{}, newFakeAttendance(), students, fixed)
		ctx := ctxAs("guru", 999, "Asia/Jakarta")
		_, err := svc.UpdateJournal(ctx, 999, 1, id, PatchInput{Material: "Materi B"})
		if status := domainStatus(t, err); status != 403 {
			t.Fatalf("expected 403, got %d", status)
		}
	})

	t.Run("admin bebas jendela", func(t *testing.T) {
		repo := newFakeTeachingRepo()
		id := setupJournalForPatch(repo, date)
		fixed := clock.Fixed{T: date.AddDate(0, 0, 10)}
		svc := newServiceForTest(repo, newFakeIdentity(), fakeSchedule{}, fakeLeave{}, newFakeAttendance(), students, fixed)
		ctx := ctxAs("admin_sekolah", 1, "Asia/Jakarta")
		if _, err := svc.UpdateJournal(ctx, 1, 1, id, PatchInput{Material: "Materi C"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestPatchJournal_OwnerOnly(t *testing.T) {
	repo := newFakeTeachingRepo()
	id := setupJournalForPatch(repo, date2026Aug1())
	students := fakeStudentsGW{byUser: map[int64]int64{999: 300, 888: 301}} // 888 guru LAIN
	fixed := clock.Fixed{T: date2026Aug1()}
	svc := newServiceForTest(repo, newFakeIdentity(), fakeSchedule{}, fakeLeave{}, newFakeAttendance(), students, fixed)

	t.Run("pemilik OK", func(t *testing.T) {
		ctx := ctxAs("guru", 999, "Asia/Jakarta")
		if _, err := svc.UpdateJournal(ctx, 999, 1, id, PatchInput{Material: "Materi"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("guru lain FORBIDDEN", func(t *testing.T) {
		ctx := ctxAs("guru", 888, "Asia/Jakarta")
		_, err := svc.UpdateJournal(ctx, 888, 1, id, PatchInput{Material: "Materi Nakal"})
		if err != httpx.ErrForbidden {
			t.Fatalf("expected ErrForbidden, got: %v", err)
		}
	})
}

func date2026Aug1() time.Time { return time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC) }
