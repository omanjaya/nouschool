package attendance

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/omanjaya/nouschool/internal/platform/clock"
	"github.com/omanjaya/nouschool/internal/platform/httpx"
	"github.com/omanjaya/nouschool/internal/platform/reqctx"
)

// fakeRepo implementasi attendanceRepository in-memory (tanpa DB) — dipakai
// menguji aturan bisnis (jendela edit, finalize, validasi status, tanggal
// lokal sekolah) tanpa Postgres, sama seperti pola internal/student.
type fakeRepo struct {
	classes  map[int64]ClassMeta
	roster   map[int64][]RosterStudent // classID -> siswa
	sessions map[int64]SessionRecord
	records  map[int64][]RecordRow // sessionID -> records
	settings Settings
	nextID   int64

	// -- QR kartu siswa (Fase 8) --
	qrTokens     map[int64]string // studentID -> token aktif
	qrTokenOwner map[string]int64 // token -> studentID

	// -- anomali self check-in (Fase 8): diisi langsung oleh test, bukan
	// -- diturunkan dari records (fake ini tidak menyimpan meta per-record) --
	anomalyRows []SelfCheckinMetaRow
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		classes:      map[int64]ClassMeta{},
		roster:       map[int64][]RosterStudent{},
		sessions:     map[int64]SessionRecord{},
		records:      map[int64][]RecordRow{},
		settings:     DefaultSettings(),
		nextID:       1,
		qrTokens:     map[int64]string{},
		qrTokenOwner: map[string]int64{},
	}
}

func (f *fakeRepo) ListClassesForDate(ctx context.Context, schoolID, academicYearID int64, date time.Time) ([]ClassAttendanceRow, error) {
	return nil, nil
}

func (f *fakeRepo) GetClassMeta(ctx context.Context, schoolID, classID int64) (ClassMeta, error) {
	c, ok := f.classes[classID]
	if !ok {
		return ClassMeta{}, ErrNotFound
	}
	return c, nil
}

func (f *fakeRepo) ListClassStudents(ctx context.Context, schoolID, classID int64) ([]RosterStudent, error) {
	return f.roster[classID], nil
}

func (f *fakeRepo) CreateSession(ctx context.Context, in CreateSessionInput) (SessionRecord, error) {
	for _, sess := range f.sessions {
		if sess.ClassID == in.ClassID && sameDate(sess.Date, in.Date) && sess.Type == TypeDaily {
			return SessionRecord{}, ErrConflict
		}
	}
	f.nextID++
	sess := SessionRecord{
		ID: f.nextID, SchoolID: in.SchoolID, AcademicYearID: in.AcademicYearID, ClassID: in.ClassID,
		Date: in.Date, Type: TypeDaily, OpenedBy: in.OpenedBy, Status: SessionOpen, CreatedAt: in.Date,
	}
	f.sessions[sess.ID] = sess
	return sess, nil
}

func sameDate(a, b time.Time) bool {
	return a.Year() == b.Year() && a.Month() == b.Month() && a.Day() == b.Day()
}

func (f *fakeRepo) GetSessionByClassDate(ctx context.Context, schoolID, classID int64, date time.Time) (SessionRecord, error) {
	for _, sess := range f.sessions {
		if sess.ClassID == classID && sameDate(sess.Date, date) && sess.Type == TypeDaily {
			return sess, nil
		}
	}
	return SessionRecord{}, ErrNotFound
}

func (f *fakeRepo) CreateSubjectSession(ctx context.Context, in CreateSubjectSessionInput) (SessionRecord, error) {
	for _, sess := range f.sessions {
		if sess.Type == TypeSubject && sess.ScheduleSlotID == in.ScheduleSlotID && sameDate(sess.Date, in.Date) {
			return SessionRecord{}, ErrConflict
		}
	}
	f.nextID++
	sess := SessionRecord{
		ID: f.nextID, SchoolID: in.SchoolID, AcademicYearID: in.AcademicYearID, ClassID: in.ClassID,
		Date: in.Date, Type: TypeSubject, ScheduleSlotID: in.ScheduleSlotID, OpenedBy: in.OpenedBy, Status: SessionOpen, CreatedAt: in.Date,
	}
	f.sessions[sess.ID] = sess
	return sess, nil
}

func (f *fakeRepo) GetSessionBySlotDate(ctx context.Context, schoolID, scheduleSlotID int64, date time.Time) (SessionRecord, error) {
	for _, sess := range f.sessions {
		if sess.Type == TypeSubject && sess.ScheduleSlotID == scheduleSlotID && sameDate(sess.Date, date) {
			return sess, nil
		}
	}
	return SessionRecord{}, ErrNotFound
}

func (f *fakeRepo) GetSessionByID(ctx context.Context, schoolID, id int64) (SessionRecord, error) {
	sess, ok := f.sessions[id]
	if !ok || sess.SchoolID != schoolID {
		return SessionRecord{}, ErrNotFound
	}
	return sess, nil
}

func (f *fakeRepo) GetSessionDetail(ctx context.Context, schoolID, id int64) (SessionDetailRow, error) {
	sess, ok := f.sessions[id]
	if !ok || sess.SchoolID != schoolID {
		return SessionDetailRow{}, ErrNotFound
	}
	cls := f.classes[sess.ClassID]
	return SessionDetailRow{SessionRecord: sess, ClassName: cls.Name, OpenedByName: "Guru Uji"}, nil
}

func (f *fakeRepo) FinalizeSession(ctx context.Context, schoolID, id int64) (SessionRecord, error) {
	sess, ok := f.sessions[id]
	if !ok || sess.SchoolID != schoolID {
		return SessionRecord{}, ErrNotFound
	}
	sess.Status = SessionFinalized
	f.sessions[id] = sess
	return sess, nil
}

func (f *fakeRepo) ListSessionRecords(ctx context.Context, sessionID int64) ([]RecordRow, error) {
	return f.records[sessionID], nil
}

func (f *fakeRepo) BulkUpsertRecords(ctx context.Context, schoolID, sessionID, actorUserID int64, method string, items []RecordInput) error {
	existing := f.records[sessionID]
	byStudent := make(map[int64]int, len(existing))
	for i, r := range existing {
		byStudent[r.StudentID] = i
	}
	for _, item := range items {
		row := RecordRow{StudentID: item.StudentID, Status: item.Status, Note: item.Note, Method: method, MarkedAt: time.Now()}
		if idx, ok := byStudent[item.StudentID]; ok {
			existing[idx] = row
		} else {
			existing = append(existing, row)
			byStudent[item.StudentID] = len(existing) - 1
		}
	}
	f.records[sessionID] = existing
	return nil
}

func (f *fakeRepo) CountEnrolledForClass(ctx context.Context, classID int64) (int64, error) {
	return int64(len(f.roster[classID])), nil
}

func (f *fakeRepo) CountRecordsForSession(ctx context.Context, sessionID int64) (int64, error) {
	return int64(len(f.records[sessionID])), nil
}

func (f *fakeRepo) SummaryByDate(ctx context.Context, schoolID, academicYearID int64, date time.Time, classID int64) ([]SummaryRow, error) {
	return nil, nil
}

func (f *fakeRepo) StudentHistory(ctx context.Context, schoolID, studentID int64, from, to time.Time) ([]HistoryRow, error) {
	return nil, nil
}

func (f *fakeRepo) StudentCounts(ctx context.Context, schoolID, studentID int64, from, to time.Time) (Counts, error) {
	return Counts{}, nil
}

func (f *fakeRepo) GetSettings(ctx context.Context, schoolID int64) (Settings, error) {
	return f.settings, nil
}

// -- QR kartu siswa (Fase 8) --

func (f *fakeRepo) ListClassStudentsWithActiveToken(ctx context.Context, schoolID, classID int64) ([]QRCardRow, error) {
	out := make([]QRCardRow, 0, len(f.roster[classID]))
	for _, st := range f.roster[classID] {
		out = append(out, QRCardRow{StudentID: st.ID, Name: st.Name, NIS: st.NIS, Token: f.qrTokens[st.ID]})
	}
	return out, nil
}

func (f *fakeRepo) GetStudentBasic(ctx context.Context, schoolID, studentID int64) (RosterStudent, error) {
	for _, list := range f.roster {
		for _, st := range list {
			if st.ID == studentID {
				return st, nil
			}
		}
	}
	return RosterStudent{}, ErrNotFound
}

func (f *fakeRepo) InsertQRToken(ctx context.Context, schoolID, studentID int64, token string) error {
	if _, ok := f.qrTokens[studentID]; ok {
		return ErrConflict
	}
	f.qrTokens[studentID] = token
	f.qrTokenOwner[token] = studentID
	return nil
}

func (f *fakeRepo) RevokeActiveQRToken(ctx context.Context, schoolID, studentID int64) error {
	if tok, ok := f.qrTokens[studentID]; ok {
		delete(f.qrTokenOwner, tok)
		delete(f.qrTokens, studentID)
	}
	return nil
}

func (f *fakeRepo) GetActiveQRTokenByStudent(ctx context.Context, schoolID, studentID int64) (string, error) {
	tok, ok := f.qrTokens[studentID]
	if !ok {
		return "", ErrNotFound
	}
	return tok, nil
}

func (f *fakeRepo) ResolveActiveQRToken(ctx context.Context, schoolID int64, token string) (int64, error) {
	id, ok := f.qrTokenOwner[token]
	if !ok {
		return 0, ErrNotFound
	}
	return id, nil
}

// -- record dengan meta: scan QR & self check-in (Fase 8) --

func (f *fakeRepo) InsertRecordWithMeta(ctx context.Context, in InsertRecordInput) (RecordRow, error) {
	existing := f.records[in.SessionID]
	for _, r := range existing {
		if r.StudentID == in.StudentID {
			return RecordRow{}, ErrConflict
		}
	}
	row := RecordRow{StudentID: in.StudentID, Status: in.Status, Method: in.Method, MarkedAt: time.Now()}
	f.records[in.SessionID] = append(existing, row)
	return row, nil
}

func (f *fakeRepo) GetRecordForStudent(ctx context.Context, sessionID, studentID int64) (RecordRow, error) {
	for _, r := range f.records[sessionID] {
		if r.StudentID == studentID {
			return r, nil
		}
	}
	return RecordRow{}, ErrNotFound
}

// -- anomali self check-in (Fase 8) --

func (f *fakeRepo) SelfCheckinRecordsForDate(ctx context.Context, schoolID int64, date time.Time, classID int64) ([]SelfCheckinMetaRow, error) {
	return f.anomalyRows, nil
}

// fakeIdentity implementasi IdentityGateway minimal untuk test.
type fakeIdentity struct {
	perms map[string]map[string]bool
	logs  []auditEntry
}

type auditEntry struct {
	action   string
	oldValue any
	newValue any
}

func newFakeIdentity() *fakeIdentity {
	return &fakeIdentity{perms: map[string]map[string]bool{
		"admin_sekolah": {PermAttendanceWrite: true, PermAttendanceReport: true},
		"guru":          {PermAttendanceWrite: true, PermAttendanceReport: true},
		"orang_tua":     {},
		"siswa":         {},
	}}
}

func (f *fakeIdentity) HasPermission(role, perm string) bool { return f.perms[role][perm] }
func (f *fakeIdentity) Log(ctx context.Context, schoolID, userID *int64, action, entity string, entityID *int64, oldValue, newValue any) error {
	f.logs = append(f.logs, auditEntry{action: action, oldValue: oldValue, newValue: newValue})
	return nil
}

// fakeYears implementasi AcademicYearLookup untuk test.
type fakeYears struct{ id int64 }

func (f fakeYears) ActiveAcademicYearID(ctx context.Context, schoolID int64) (int64, bool, error) {
	if f.id == 0 {
		return 0, false, nil
	}
	return f.id, true, nil
}

// fakeStudents implementasi StudentAccess untuk test.
type fakeStudents struct {
	// (userID,studentID) yang boleh diakses oleh role siswa/orang_tua.
	allowed map[int64]int64
	// self: userID -> [studentID, classID] (profil siswa milik user login,
	// dipakai self check-in — fase 8).
	self map[int64][2]int64
}

func (f fakeStudents) CanViewStudent(ctx context.Context, userID int64, role string, schoolID, studentID int64) error {
	if f.allowed[userID] == studentID {
		return nil
	}
	return httpx.ErrForbidden
}

func (f fakeStudents) MyStudentID(ctx context.Context, schoolID, userID int64) (int64, int64, bool, error) {
	v, ok := f.self[userID]
	if !ok {
		return 0, 0, false, nil
	}
	return v[0], v[1], true, nil
}

// fakeSchedule implementasi ScheduleSlotLookup untuk test (fase 6).
type fakeSchedule struct {
	// slotID -> (classID, teacherID)
	slots      map[int64][2]int64
	slotsToday map[int64][]SlotToday // teacherID -> slot hari ini
}

func (f fakeSchedule) SlotOwnership(ctx context.Context, schoolID, slotID int64) (int64, int64, bool, error) {
	v, ok := f.slots[slotID]
	if !ok {
		return 0, 0, false, nil
	}
	return v[0], v[1], true, nil
}

func (f fakeSchedule) SlotsTodayForTeacher(ctx context.Context, schoolID, teacherID int64, at time.Time) ([]SlotToday, error) {
	return f.slotsToday[teacherID], nil
}

// fakeTeachers implementasi TeacherLookup untuk test (fase 6).
type fakeTeachers struct {
	// userID -> teacherID
	byUser map[int64]int64
}

func (f fakeTeachers) MyTeacherID(ctx context.Context, schoolID, userID int64) (int64, bool, error) {
	id, ok := f.byUser[userID]
	return id, ok, nil
}

func ctxAs(role string, userID int64, tz string) context.Context {
	ctx := reqctx.WithUser(context.Background(), userID, role, false)
	ctx = reqctx.WithSchool(ctx, reqctx.School{ID: 1, Name: "Sekolah Uji", Slug: "uji", Timezone: tz})
	return ctx
}

// -- checkEditWindow (fungsi murni) --

func TestCheckEditWindow(t *testing.T) {
	created := time.Date(2026, 8, 1, 8, 0, 0, 0, time.UTC)

	t.Run("dalam window OK", func(t *testing.T) {
		now := created.Add(2 * time.Hour)
		if err := checkEditWindow(created, now, 24); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("lewat window ditolak", func(t *testing.T) {
		now := created.Add(25 * time.Hour)
		err := checkEditWindow(created, now, 24)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		var de *httpx.Error
		if !errors.As(err, &de) || de.Status != 403 {
			t.Fatalf("expected 403 domain error, got: %v", err)
		}
	})
}

// -- schoolToday (tanggal lokal sekolah) --

func TestSchoolToday(t *testing.T) {
	// 23:30 WIB (16:30 UTC) tanggal 1 Agustus 2026 harus tetap tanggal 1
	// Agustus lokal, BUKAN "lompat" ke 2 Agustus UTC.
	utcNow := time.Date(2026, 8, 1, 16, 30, 0, 0, time.UTC)
	got := schoolToday(utcNow, "Asia/Jakarta")
	if got.Year() != 2026 || got.Month() != time.August || got.Day() != 1 {
		t.Fatalf("expected 2026-08-01, got %s", got.Format("2006-01-02"))
	}

	// Pastikan memang beda dari kalau dihitung naif dari UTC + offset yang salah:
	// 16:30 UTC + 7 jam = 23:30 WIB, MASIH tanggal 1. Uji kasus mepet tengah malam:
	// 17:00 UTC = 00:00 WIB (2 Agustus) -> harus jadi 2 Agustus.
	utcMidnight := time.Date(2026, 8, 1, 17, 0, 0, 0, time.UTC)
	got2 := schoolToday(utcMidnight, "Asia/Jakarta")
	if got2.Day() != 2 {
		t.Fatalf("expected tanggal 2 (00:00 WIB), got %s", got2.Format("2006-01-02"))
	}
}

// TestCreateSessionUsesSchoolLocalDate — sesi dibuat memakai tanggal LOKAL
// sekolah dari clock.Fixed, bukan tanggal UTC (docs/05-attendance.md).
func TestCreateSessionUsesSchoolLocalDate(t *testing.T) {
	repo := newFakeRepo()
	repo.classes[1] = ClassMeta{ID: 1, Name: "XII RPL 1", AcademicYearID: 1}
	repo.roster[1] = []RosterStudent{{ID: 10, Name: "Siswa A", NIS: "001"}}

	// 23:30 WIB pada 1 Agustus 2026 = 16:30 UTC.
	fixed := clock.Fixed{T: time.Date(2026, 8, 1, 16, 30, 0, 0, time.UTC)}
	svc := newServiceForTest(repo, newFakeIdentity(), fakeYears{id: 1}, fakeStudents{}, fakeSchedule{}, fakeTeachers{}, fixed)

	ctx := ctxAs("guru", 999, "Asia/Jakarta")
	detail, err := svc.CreateSession(ctx, 999, 1, NewSessionInput{ClassID: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if detail.Session.Date.Format("2006-01-02") != "2026-08-01" {
		t.Fatalf("expected tanggal sesi 2026-08-01 (lokal WIB), got %s", detail.Session.Date.Format("2006-01-02"))
	}

	// Panggilan kedua di tanggal yang sama harus idempoten (sesi yang sama, bukan baru).
	detail2, err := svc.CreateSession(ctx, 999, 1, NewSessionInput{ClassID: 1})
	if err != nil {
		t.Fatalf("unexpected error on second call: %v", err)
	}
	if detail2.Session.ID != detail.Session.ID {
		t.Fatalf("expected sesi sama (idempoten), got id berbeda: %d vs %d", detail.Session.ID, detail2.Session.ID)
	}
}

// -- UpdateRecords: jendela edit & finalized lock --

func setupSessionWithOneStudent(repo *fakeRepo, createdAt time.Time, status string) int64 {
	repo.classes[1] = ClassMeta{ID: 1, Name: "XII RPL 1", AcademicYearID: 1}
	repo.roster[1] = []RosterStudent{{ID: 10, Name: "Siswa A", NIS: "001"}}
	sess := SessionRecord{
		ID: 100, SchoolID: 1, AcademicYearID: 1, ClassID: 1, Date: createdAt, Type: TypeDaily,
		OpenedBy: 999, Status: status, CreatedAt: createdAt,
	}
	repo.sessions[sess.ID] = sess
	return sess.ID
}

func TestUpdateRecordsEditWindow(t *testing.T) {
	created := time.Date(2026, 8, 1, 7, 0, 0, 0, time.UTC)

	t.Run("guru dalam window OK", func(t *testing.T) {
		repo := newFakeRepo()
		sessionID := setupSessionWithOneStudent(repo, created, SessionOpen)
		fixed := clock.Fixed{T: created.Add(2 * time.Hour)}
		svc := newServiceForTest(repo, newFakeIdentity(), fakeYears{id: 1}, fakeStudents{}, fakeSchedule{}, fakeTeachers{}, fixed)

		ctx := ctxAs("guru", 999, "Asia/Jakarta")
		_, err := svc.UpdateRecords(ctx, 999, 1, sessionID, []RecordInputRequest{{StudentID: 10, Status: StatusHadir}})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("guru lewat window 403", func(t *testing.T) {
		repo := newFakeRepo()
		sessionID := setupSessionWithOneStudent(repo, created, SessionOpen)
		fixed := clock.Fixed{T: created.Add(25 * time.Hour)}
		svc := newServiceForTest(repo, newFakeIdentity(), fakeYears{id: 1}, fakeStudents{}, fakeSchedule{}, fakeTeachers{}, fixed)

		ctx := ctxAs("guru", 999, "Asia/Jakarta")
		_, err := svc.UpdateRecords(ctx, 999, 1, sessionID, []RecordInputRequest{{StudentID: 10, Status: StatusHadir}})
		var de *httpx.Error
		if !errors.As(err, &de) || de.Status != 403 {
			t.Fatalf("expected 403, got: %v", err)
		}
	})

	t.Run("admin lewat window tetap OK", func(t *testing.T) {
		repo := newFakeRepo()
		sessionID := setupSessionWithOneStudent(repo, created, SessionOpen)
		fixed := clock.Fixed{T: created.Add(48 * time.Hour)}
		svc := newServiceForTest(repo, newFakeIdentity(), fakeYears{id: 1}, fakeStudents{}, fakeSchedule{}, fakeTeachers{}, fixed)

		ctx := ctxAs("admin_sekolah", 1, "Asia/Jakarta")
		_, err := svc.UpdateRecords(ctx, 1, 1, sessionID, []RecordInputRequest{{StudentID: 10, Status: StatusHadir}})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("guru sesi finalized 403", func(t *testing.T) {
		repo := newFakeRepo()
		sessionID := setupSessionWithOneStudent(repo, created, SessionFinalized)
		fixed := clock.Fixed{T: created.Add(time.Hour)}
		svc := newServiceForTest(repo, newFakeIdentity(), fakeYears{id: 1}, fakeStudents{}, fakeSchedule{}, fakeTeachers{}, fixed)

		ctx := ctxAs("guru", 999, "Asia/Jakarta")
		_, err := svc.UpdateRecords(ctx, 999, 1, sessionID, []RecordInputRequest{{StudentID: 10, Status: StatusHadir}})
		if !errors.Is(err, ErrSessionLocked) {
			t.Fatalf("expected ErrSessionLocked, got: %v", err)
		}
	})

	t.Run("admin sesi finalized tetap OK", func(t *testing.T) {
		repo := newFakeRepo()
		sessionID := setupSessionWithOneStudent(repo, created, SessionFinalized)
		fixed := clock.Fixed{T: created.Add(time.Hour)}
		svc := newServiceForTest(repo, newFakeIdentity(), fakeYears{id: 1}, fakeStudents{}, fakeSchedule{}, fakeTeachers{}, fixed)

		ctx := ctxAs("admin_sekolah", 1, "Asia/Jakarta")
		_, err := svc.UpdateRecords(ctx, 1, 1, sessionID, []RecordInputRequest{{StudentID: 10, Status: StatusHadir}})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("status tidak valid ditolak", func(t *testing.T) {
		repo := newFakeRepo()
		sessionID := setupSessionWithOneStudent(repo, created, SessionOpen)
		fixed := clock.Fixed{T: created.Add(time.Hour)}
		svc := newServiceForTest(repo, newFakeIdentity(), fakeYears{id: 1}, fakeStudents{}, fakeSchedule{}, fakeTeachers{}, fixed)

		ctx := ctxAs("guru", 999, "Asia/Jakarta")
		_, err := svc.UpdateRecords(ctx, 999, 1, sessionID, []RecordInputRequest{{StudentID: 10, Status: "ngasal"}})
		var de *httpx.Error
		if !errors.As(err, &de) || de.Status != 422 {
			t.Fatalf("expected 422 validation, got: %v", err)
		}
	})

	t.Run("perubahan pada record yang sudah ada nilainya tercatat audit", func(t *testing.T) {
		repo := newFakeRepo()
		sessionID := setupSessionWithOneStudent(repo, created, SessionOpen)
		fixed := clock.Fixed{T: created.Add(time.Hour)}
		identity := newFakeIdentity()
		svc := newServiceForTest(repo, identity, fakeYears{id: 1}, fakeStudents{}, fakeSchedule{}, fakeTeachers{}, fixed)
		ctx := ctxAs("guru", 999, "Asia/Jakarta")

		// isian pertama: hadir — belum ada nilai sebelumnya -> TIDAK diaudit.
		if _, err := svc.UpdateRecords(ctx, 999, 1, sessionID, []RecordInputRequest{{StudentID: 10, Status: StatusHadir}}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(identity.logs) != 0 {
			t.Fatalf("expected belum ada audit pada isian pertama, got %d entri", len(identity.logs))
		}

		// ubah jadi sakit — record SUDAH ada nilainya -> HARUS diaudit.
		if _, err := svc.UpdateRecords(ctx, 999, 1, sessionID, []RecordInputRequest{{StudentID: 10, Status: StatusSakit}}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		found := false
		for _, e := range identity.logs {
			if e.action == "attendance.update" {
				found = true
			}
		}
		if !found {
			t.Fatal("expected audit_log attendance.update pada perubahan record")
		}
	})
}

// -- Finalize --

func TestFinalizeRejectsUnmarked(t *testing.T) {
	repo := newFakeRepo()
	repo.classes[1] = ClassMeta{ID: 1, Name: "XII RPL 1", AcademicYearID: 1}
	repo.roster[1] = []RosterStudent{
		{ID: 10, Name: "Siswa A", NIS: "001"},
		{ID: 11, Name: "Siswa B", NIS: "002"},
	}
	created := time.Date(2026, 8, 1, 7, 0, 0, 0, time.UTC)
	sess := SessionRecord{ID: 100, SchoolID: 1, AcademicYearID: 1, ClassID: 1, Date: created, Type: TypeDaily, OpenedBy: 999, Status: SessionOpen, CreatedAt: created}
	repo.sessions[sess.ID] = sess
	// Hanya 1 dari 2 siswa diisi.
	repo.records[sess.ID] = []RecordRow{{StudentID: 10, Status: StatusHadir, Method: "manual"}}

	fixed := clock.Fixed{T: created.Add(time.Hour)}
	svc := newServiceForTest(repo, newFakeIdentity(), fakeYears{id: 1}, fakeStudents{}, fakeSchedule{}, fakeTeachers{}, fixed)
	ctx := ctxAs("guru", 999, "Asia/Jakarta")

	_, err := svc.Finalize(ctx, 999, 1, sess.ID)
	var de *httpx.Error
	if !errors.As(err, &de) || de.Status != 422 {
		t.Fatalf("expected 422 validation (masih ada belum diabsen), got: %v", err)
	}

	// Lengkapi siswa kedua, lalu finalize harus sukses.
	repo.records[sess.ID] = append(repo.records[sess.ID], RecordRow{StudentID: 11, Status: StatusAlpa, Method: "manual"})
	result, err := svc.Finalize(ctx, 999, 1, sess.ID)
	if err != nil {
		t.Fatalf("unexpected error setelah lengkap: %v", err)
	}
	if result.Session.Status != SessionFinalized {
		t.Fatalf("expected status finalized, got %s", result.Session.Status)
	}
	if result.UnmarkedCount != 0 {
		t.Fatalf("expected unmarked_count 0, got %d", result.UnmarkedCount)
	}
}

// -- StudentHistory: object-level access --

func TestStudentHistoryAccess(t *testing.T) {
	repo := newFakeRepo()
	fixed := clock.Fixed{T: time.Date(2026, 8, 1, 7, 0, 0, 0, time.UTC)}

	t.Run("attendance:report OK untuk siapa saja", func(t *testing.T) {
		svc := newServiceForTest(repo, newFakeIdentity(), fakeYears{id: 1}, fakeStudents{}, fakeSchedule{}, fakeTeachers{}, fixed)
		ctx := ctxAs("guru", 999, "Asia/Jakarta")
		if _, err := svc.StudentHistory(ctx, 1, 10, "", ""); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("orang tua anak sendiri OK", func(t *testing.T) {
		svc := newServiceForTest(repo, newFakeIdentity(), fakeYears{id: 1}, fakeStudents{allowed: map[int64]int64{500: 10}}, fakeSchedule{}, fakeTeachers{}, fixed)
		ctx := ctxAs("orang_tua", 500, "Asia/Jakarta")
		if _, err := svc.StudentHistory(ctx, 1, 10, "", ""); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("orang tua anak lain FORBIDDEN", func(t *testing.T) {
		svc := newServiceForTest(repo, newFakeIdentity(), fakeYears{id: 1}, fakeStudents{allowed: map[int64]int64{500: 10}}, fakeSchedule{}, fakeTeachers{}, fixed)
		ctx := ctxAs("orang_tua", 500, "Asia/Jakarta")
		_, err := svc.StudentHistory(ctx, 1, 20, "", "")
		if err != httpx.ErrForbidden {
			t.Fatalf("expected ErrForbidden, got: %v", err)
		}
	})
}

// -- OpenSubjectSession & CreateSession(schedule_slot_id) — fase 6 --

func TestOpenSubjectSessionCreatesAndIsIdempotent(t *testing.T) {
	repo := newFakeRepo()
	repo.classes[1] = ClassMeta{ID: 1, Name: "XII RPL 1", AcademicYearID: 1}
	sched := fakeSchedule{slots: map[int64][2]int64{50: {1, 300}}} // slot 50 -> kelas 1, guru 300
	fixed := clock.Fixed{T: time.Date(2026, 8, 10, 1, 0, 0, 0, time.UTC)}
	svc := newServiceForTest(repo, newFakeIdentity(), fakeYears{id: 1}, fakeStudents{}, sched, fakeTeachers{}, fixed)
	ctx := ctxAs("guru", 999, "Asia/Jakarta")

	sessionID, err := svc.OpenSubjectSession(ctx, 1, 999, 50, "2026-08-10")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sessionID == 0 {
		t.Fatal("expected sessionID != 0")
	}

	// Panggilan kedua slot+tanggal sama -> sesi yang sama (idempoten).
	sessionID2, err := svc.OpenSubjectSession(ctx, 1, 999, 50, "2026-08-10")
	if err != nil {
		t.Fatalf("unexpected error kedua: %v", err)
	}
	if sessionID2 != sessionID {
		t.Fatalf("expected idempoten, got %d vs %d", sessionID, sessionID2)
	}

	sess, err := repo.GetSessionByID(context.Background(), 1, sessionID)
	if err != nil {
		t.Fatalf("unexpected error ambil sesi: %v", err)
	}
	if sess.Type != TypeSubject || sess.ScheduleSlotID != 50 {
		t.Fatalf("expected sesi type=subject schedule_slot_id=50, got type=%s slot=%d", sess.Type, sess.ScheduleSlotID)
	}
}

func TestOpenSubjectSessionUnknownSlot(t *testing.T) {
	repo := newFakeRepo()
	fixed := clock.Fixed{T: time.Date(2026, 8, 10, 1, 0, 0, 0, time.UTC)}
	svc := newServiceForTest(repo, newFakeIdentity(), fakeYears{id: 1}, fakeStudents{}, fakeSchedule{}, fakeTeachers{}, fixed)
	ctx := ctxAs("guru", 999, "Asia/Jakarta")

	_, err := svc.OpenSubjectSession(ctx, 1, 999, 999, "2026-08-10")
	var de *httpx.Error
	if !errors.As(err, &de) || de.Status != 422 {
		t.Fatalf("expected 422 (slot tidak ditemukan), got: %v", err)
	}
}

func TestCreateSessionFromSlot_ObjectLevelOwnership(t *testing.T) {
	repo := newFakeRepo()
	repo.classes[1] = ClassMeta{ID: 1, Name: "XII RPL 1", AcademicYearID: 1}
	sched := fakeSchedule{slots: map[int64][2]int64{50: {1, 300}}} // slot 50 milik teacherID 300
	teachers := fakeTeachers{byUser: map[int64]int64{999: 300, 888: 301}}
	fixed := clock.Fixed{T: time.Date(2026, 8, 10, 1, 0, 0, 0, time.UTC)}

	t.Run("guru pemilik slot OK", func(t *testing.T) {
		svc := newServiceForTest(repo, newFakeIdentity(), fakeYears{id: 1}, fakeStudents{}, sched, teachers, fixed)
		ctx := ctxAs("guru", 999, "Asia/Jakarta")
		detail, err := svc.CreateSession(ctx, 999, 1, NewSessionInput{ScheduleSlotID: 50})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if detail.Session.Type != TypeSubject {
			t.Fatalf("expected type subject, got %s", detail.Session.Type)
		}
	})

	t.Run("guru lain BUKAN pemilik slot 403", func(t *testing.T) {
		svc := newServiceForTest(repo, newFakeIdentity(), fakeYears{id: 1}, fakeStudents{}, sched, teachers, fixed)
		ctx := ctxAs("guru", 888, "Asia/Jakarta")
		_, err := svc.CreateSession(ctx, 888, 1, NewSessionInput{ScheduleSlotID: 50})
		if err != httpx.ErrForbidden {
			t.Fatalf("expected ErrForbidden, got: %v", err)
		}
	})

	t.Run("admin_sekolah bebas slot siapapun", func(t *testing.T) {
		svc := newServiceForTest(repo, newFakeIdentity(), fakeYears{id: 1}, fakeStudents{}, sched, teachers, fixed)
		ctx := ctxAs("admin_sekolah", 1, "Asia/Jakarta")
		_, err := svc.CreateSession(ctx, 1, 1, NewSessionInput{ScheduleSlotID: 50})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

// -- SlotsToday --

func TestSlotsTodayGuru(t *testing.T) {
	repo := newFakeRepo()
	repo.roster[1] = []RosterStudent{{ID: 10, Name: "Siswa A", NIS: "001"}}
	fixed := clock.Fixed{T: time.Date(2026, 8, 10, 1, 0, 0, 0, time.UTC)} // 08:00 WIB
	sched := fakeSchedule{
		slots: map[int64][2]int64{50: {1, 300}},
		slotsToday: map[int64][]SlotToday{
			300: {{ID: 50, ClassID: 1, ClassName: "XII RPL 1", SubjectID: 5, SubjectCode: "BDT", SubjectName: "Basis Data", PeriodStart: 1, PeriodEnd: 2, StartsAt: "07:00", EndsAt: "08:20", IsNow: true}},
		},
	}
	teachers := fakeTeachers{byUser: map[int64]int64{999: 300}}
	svc := newServiceForTest(repo, newFakeIdentity(), fakeYears{id: 1}, fakeStudents{}, sched, teachers, fixed)
	ctx := ctxAs("guru", 999, "Asia/Jakarta")

	// Belum ada sesi dibuka -> session nil.
	items, err := svc.SlotsToday(ctx, 1, 999)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 || items[0].Session != nil {
		t.Fatalf("expected 1 slot tanpa sesi, got: %+v", items)
	}

	// Buka sesi lalu isi 1 record -> marked_count=1, total=1.
	sessionID, err := svc.OpenSubjectSession(ctx, 1, 999, 50, "2026-08-10")
	if err != nil {
		t.Fatalf("unexpected error open session: %v", err)
	}
	if err := repo.BulkUpsertRecords(ctx, 1, sessionID, 999, "manual", []RecordInput{{StudentID: 10, Status: StatusHadir}}); err != nil {
		t.Fatalf("unexpected error upsert: %v", err)
	}

	items2, err := svc.SlotsToday(ctx, 1, 999)
	if err != nil {
		t.Fatalf("unexpected error kedua: %v", err)
	}
	if len(items2) != 1 || items2[0].Session == nil {
		t.Fatalf("expected 1 slot DENGAN sesi, got: %+v", items2)
	}
	if items2[0].Session.MarkedCount != 1 || items2[0].Session.Total != 1 {
		t.Fatalf("expected marked_count=1 total=1, got %+v", items2[0].Session)
	}

	// Bukan guru (tidak ada di fakeTeachers) -> daftar kosong, bukan error.
	ctxAdmin := ctxAs("admin_sekolah", 1, "Asia/Jakarta")
	itemsAdmin, err := svc.SlotsToday(ctxAdmin, 1, 1)
	if err != nil {
		t.Fatalf("unexpected error admin: %v", err)
	}
	if len(itemsAdmin) != 0 {
		t.Fatalf("expected daftar kosong utk non-guru, got %d", len(itemsAdmin))
	}
}

// -- haversineMeters (fungsi murni, Fase 8) --

func TestHaversineMeters(t *testing.T) {
	t.Run("titik sama = 0 meter", func(t *testing.T) {
		d := haversineMeters(-6.2, 106.816666, -6.2, 106.816666)
		if d > 0.001 {
			t.Fatalf("expected ~0, got %f", d)
		}
	})

	t.Run("dalam radius 150m", func(t *testing.T) {
		// ~0.0009 derajat lat ~= 100 m.
		d := haversineMeters(-6.2, 106.816666, -6.2009, 106.816666)
		if d > 150 {
			t.Fatalf("expected dalam radius 150m, got %.1f m", d)
		}
	})

	t.Run("luar radius 150m", func(t *testing.T) {
		// ~0.01 derajat lat ~= 1.1 km.
		d := haversineMeters(-6.2, 106.816666, -6.21, 106.816666)
		if d < 500 {
			t.Fatalf("expected jauh di luar radius, got %.1f m", d)
		}
	})
}

// -- Self check-in siswa (Fase 8) --

// wibTime membangun waktu lokal WIB (dipakai clock.Fixed di test self
// check-in) — memakai time.Date langsung dengan lokasi Asia/Jakarta supaya
// jam yang ditulis di test SAMA dengan jam lokal yang dibaca schoolToday/
// clock.InZone di service.go (bebas dari kesalahan hitung offset manual).
func wibTime(y int, m time.Month, d, h, min int) time.Time {
	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		panic(err)
	}
	return time.Date(y, m, d, h, min, 0, 0, loc)
}

// selfCheckinSettings — jendela 06:00-07:30 WIB, radius 150m dari titik
// (-6.2, 106.816666), toleransi telat 15 menit (docs/05-attendance.md).
func selfCheckinSettings() Settings {
	return Settings{
		Mode:            ModeDaily,
		Methods:         []Method{MethodManual, MethodSelfCheckin},
		SelfCheckin:     &SelfCheckinRule{Lat: -6.2, Lng: 106.816666, RadiusM: 150, OpenFrom: "06:00", CloseAt: "07:30"},
		EditWindowHours: 24,
		LateAfterMin:    15,
	}
}

func TestSelfCheckinWindow(t *testing.T) {
	t.Run("sebelum jendela dibuka ditolak", func(t *testing.T) {
		repo := newFakeRepo()
		repo.settings = selfCheckinSettings()
		students := fakeStudents{self: map[int64][2]int64{500: {10, 1}}}
		fixed := clock.Fixed{T: wibTime(2026, 8, 10, 5, 0)}
		svc := newServiceForTest(repo, newFakeIdentity(), fakeYears{id: 1}, students, fakeSchedule{}, fakeTeachers{}, fixed)
		ctx := ctxAs("siswa", 500, "Asia/Jakarta")

		_, err := svc.SelfCheckin(ctx, 1, SelfCheckinInput{Lat: -6.2, Lng: 106.816666, Accuracy: 5})
		var de *httpx.Error
		if !errors.As(err, &de) || de.Status != 422 {
			t.Fatalf("expected 422 (belum dibuka), got: %v", err)
		}
	})

	t.Run("dalam jendela sukses", func(t *testing.T) {
		repo := newFakeRepo()
		repo.settings = selfCheckinSettings()
		students := fakeStudents{self: map[int64][2]int64{500: {10, 1}}}
		// 06:10 WIB: dalam jendela (06:00-07:30) DAN dalam toleransi telat
		// (open_from+15 menit = 06:15) -> sukses dengan status hadir.
		fixed := clock.Fixed{T: wibTime(2026, 8, 10, 6, 10)}
		svc := newServiceForTest(repo, newFakeIdentity(), fakeYears{id: 1}, students, fakeSchedule{}, fakeTeachers{}, fixed)
		ctx := ctxAs("siswa", 500, "Asia/Jakarta")

		result, err := svc.SelfCheckin(ctx, 1, SelfCheckinInput{Lat: -6.2, Lng: 106.816666, Accuracy: 5})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Status != StatusHadir {
			t.Fatalf("expected hadir, got %s", result.Status)
		}
	})

	t.Run("setelah jendela ditutup ditolak", func(t *testing.T) {
		repo := newFakeRepo()
		repo.settings = selfCheckinSettings()
		students := fakeStudents{self: map[int64][2]int64{500: {10, 1}}}
		fixed := clock.Fixed{T: wibTime(2026, 8, 10, 8, 0)}
		svc := newServiceForTest(repo, newFakeIdentity(), fakeYears{id: 1}, students, fakeSchedule{}, fakeTeachers{}, fixed)
		ctx := ctxAs("siswa", 500, "Asia/Jakarta")

		_, err := svc.SelfCheckin(ctx, 1, SelfCheckinInput{Lat: -6.2, Lng: 106.816666, Accuracy: 5})
		var de *httpx.Error
		if !errors.As(err, &de) || de.Status != 422 {
			t.Fatalf("expected 422 (sudah ditutup), got: %v", err)
		}
	})
}

func TestSelfCheckinLateThreshold(t *testing.T) {
	t.Run("tepat di batas open_from+late_after_min -> hadir", func(t *testing.T) {
		repo := newFakeRepo()
		repo.settings = selfCheckinSettings() // open 06:00 + late_after 15 -> batas 06:15
		students := fakeStudents{self: map[int64][2]int64{500: {10, 1}}}
		fixed := clock.Fixed{T: wibTime(2026, 8, 10, 6, 15)}
		svc := newServiceForTest(repo, newFakeIdentity(), fakeYears{id: 1}, students, fakeSchedule{}, fakeTeachers{}, fixed)
		ctx := ctxAs("siswa", 500, "Asia/Jakarta")

		result, err := svc.SelfCheckin(ctx, 1, SelfCheckinInput{Lat: -6.2, Lng: 106.816666, Accuracy: 5})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Status != StatusHadir {
			t.Fatalf("expected hadir tepat di batas, got %s", result.Status)
		}
	})

	t.Run("lewat batas open_from+late_after_min -> terlambat", func(t *testing.T) {
		repo := newFakeRepo()
		repo.settings = selfCheckinSettings()
		students := fakeStudents{self: map[int64][2]int64{500: {10, 1}}}
		fixed := clock.Fixed{T: wibTime(2026, 8, 10, 6, 20)}
		svc := newServiceForTest(repo, newFakeIdentity(), fakeYears{id: 1}, students, fakeSchedule{}, fakeTeachers{}, fixed)
		ctx := ctxAs("siswa", 500, "Asia/Jakarta")

		result, err := svc.SelfCheckin(ctx, 1, SelfCheckinInput{Lat: -6.2, Lng: 106.816666, Accuracy: 5})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Status != StatusTerlambat {
			t.Fatalf("expected terlambat, got %s", result.Status)
		}
	})
}

func TestSelfCheckinDoubleRejected(t *testing.T) {
	repo := newFakeRepo()
	repo.settings = selfCheckinSettings()
	students := fakeStudents{self: map[int64][2]int64{500: {10, 1}}}
	fixed := clock.Fixed{T: wibTime(2026, 8, 10, 6, 30)}
	svc := newServiceForTest(repo, newFakeIdentity(), fakeYears{id: 1}, students, fakeSchedule{}, fakeTeachers{}, fixed)
	ctx := ctxAs("siswa", 500, "Asia/Jakarta")

	if _, err := svc.SelfCheckin(ctx, 1, SelfCheckinInput{Lat: -6.2, Lng: 106.816666, Accuracy: 5}); err != nil {
		t.Fatalf("unexpected error pada check-in pertama: %v", err)
	}

	_, err := svc.SelfCheckin(ctx, 1, SelfCheckinInput{Lat: -6.2, Lng: 106.816666, Accuracy: 5})
	var de *httpx.Error
	if !errors.As(err, &de) || de.Status != 422 {
		t.Fatalf("expected 422 (sudah check-in hari ini), got: %v", err)
	}
}

func TestSelfCheckinOutsideRadius(t *testing.T) {
	repo := newFakeRepo()
	repo.settings = selfCheckinSettings() // radius 150m
	students := fakeStudents{self: map[int64][2]int64{500: {10, 1}}}
	fixed := clock.Fixed{T: wibTime(2026, 8, 10, 6, 30)}
	svc := newServiceForTest(repo, newFakeIdentity(), fakeYears{id: 1}, students, fakeSchedule{}, fakeTeachers{}, fixed)
	ctx := ctxAs("siswa", 500, "Asia/Jakarta")

	// ~1.1 km dari titik sekolah — jauh di luar radius 150m.
	_, err := svc.SelfCheckin(ctx, 1, SelfCheckinInput{Lat: -6.21, Lng: 106.816666, Accuracy: 5})
	var de *httpx.Error
	if !errors.As(err, &de) || de.Status != 422 {
		t.Fatalf("expected 422 (luar radius), got: %v", err)
	}
	if !strings.Contains(de.Message, "dari sekolah") {
		t.Fatalf("expected pesan error menyebut jarak dari sekolah, got: %s", de.Message)
	}
}

// -- QR kartu siswa: scan (Fase 8) --

func setupScanSession(repo *fakeRepo) int64 {
	repo.classes[1] = ClassMeta{ID: 1, Name: "XII RPL 1", AcademicYearID: 1}
	repo.roster[1] = []RosterStudent{{ID: 10, Name: "Siswa A", NIS: "001"}}
	created := time.Date(2026, 8, 1, 7, 0, 0, 0, time.UTC)
	sess := SessionRecord{ID: 100, SchoolID: 1, AcademicYearID: 1, ClassID: 1, Date: created, Type: TypeDaily, OpenedBy: 999, Status: SessionOpen, CreatedAt: created}
	repo.sessions[sess.ID] = sess
	return sess.ID
}

func TestScanQRCardRevoked(t *testing.T) {
	repo := newFakeRepo()
	sessionID := setupScanSession(repo)
	if err := repo.InsertQRToken(context.Background(), 1, 10, "tokenlama0000000000001"); err != nil {
		t.Fatalf("unexpected error setup token: %v", err)
	}
	if err := repo.RevokeActiveQRToken(context.Background(), 1, 10); err != nil {
		t.Fatalf("unexpected error revoke: %v", err)
	}

	fixed := clock.Fixed{T: time.Date(2026, 8, 1, 8, 0, 0, 0, time.UTC)}
	svc := newServiceForTest(repo, newFakeIdentity(), fakeYears{id: 1}, fakeStudents{}, fakeSchedule{}, fakeTeachers{}, fixed)
	ctx := ctxAs("guru", 999, "Asia/Jakarta")

	_, err := svc.ScanQRCard(ctx, 999, 1, sessionID, "tokenlama0000000000001")
	var de *httpx.Error
	if !errors.As(err, &de) || de.Status != 404 || de.Code != "qr_unknown" {
		t.Fatalf("expected 404 qr_unknown (token dicabut), got: %v", err)
	}
}

func TestScanQRCardWrongClass(t *testing.T) {
	repo := newFakeRepo()
	sessionID := setupScanSession(repo)
	// Siswa 20 terdaftar di rombel LAIN (kelas 2), bukan kelas 1 (kelas sesi).
	repo.roster[2] = []RosterStudent{{ID: 20, Name: "Siswa Lain", NIS: "002"}}
	if err := repo.InsertQRToken(context.Background(), 1, 20, "tokensiswalain00000001"); err != nil {
		t.Fatalf("unexpected error setup token: %v", err)
	}

	fixed := clock.Fixed{T: time.Date(2026, 8, 1, 8, 0, 0, 0, time.UTC)}
	svc := newServiceForTest(repo, newFakeIdentity(), fakeYears{id: 1}, fakeStudents{}, fakeSchedule{}, fakeTeachers{}, fixed)
	ctx := ctxAs("guru", 999, "Asia/Jakarta")

	_, err := svc.ScanQRCard(ctx, 999, 1, sessionID, "tokensiswalain00000001")
	var de *httpx.Error
	if !errors.As(err, &de) || de.Status != 422 {
		t.Fatalf("expected 422 (bukan anggota rombel sesi), got: %v", err)
	}
}

// TestScanQRCardAlreadyMarkedNoDowngrade — record yang SUDAH ada (mis. guru
// menandai manual "sakit" duluan) TIDAK PERNAH ditimpa scan QR berikutnya;
// scan hanya mengembalikan status yang tersimpan dengan already_marked=true
// (docs/05-attendance.md).
func TestScanQRCardAlreadyMarkedNoDowngrade(t *testing.T) {
	repo := newFakeRepo()
	sessionID := setupScanSession(repo)
	if err := repo.InsertQRToken(context.Background(), 1, 10, "tokensiswaA0000000001"); err != nil {
		t.Fatalf("unexpected error setup token: %v", err)
	}
	repo.records[sessionID] = []RecordRow{{StudentID: 10, Status: StatusSakit, Method: "manual", MarkedAt: time.Now()}}

	fixed := clock.Fixed{T: time.Date(2026, 8, 1, 8, 0, 0, 0, time.UTC)}
	svc := newServiceForTest(repo, newFakeIdentity(), fakeYears{id: 1}, fakeStudents{}, fakeSchedule{}, fakeTeachers{}, fixed)
	ctx := ctxAs("guru", 999, "Asia/Jakarta")

	result, err := svc.ScanQRCard(ctx, 999, 1, sessionID, "tokensiswaA0000000001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.AlreadyMarked {
		t.Fatal("expected already_marked=true")
	}
	if result.Status != StatusSakit {
		t.Fatalf("expected status TIDAK berubah (sakit), got %s", result.Status)
	}
}

// -- anomali self check-in (Fase 8) --

func TestAnomaliesGroupingAndAccuracy(t *testing.T) {
	repo := newFakeRepo()
	repo.anomalyRows = []SelfCheckinMetaRow{
		{StudentID: 1, StudentName: "Andi", ClassName: "XI RPL 2", Lat: -6.2, Lng: 106.816666, Accuracy: 10},
		{StudentID: 2, StudentName: "Budi", ClassName: "XI RPL 2", Lat: -6.2, Lng: 106.816666, Accuracy: 10},
		// Beda di desimal ke-6 -> setelah dibulatkan 5 desimal, SAMA dengan Andi/Budi.
		{StudentID: 3, StudentName: "Citra", ClassName: "XI RPL 2", Lat: -6.200001, Lng: 106.816667, Accuracy: 10},
		{StudentID: 4, StudentName: "Dedi", ClassName: "XI RPL 2", Lat: -6.25, Lng: 106.85, Accuracy: 500},
		{StudentID: 5, StudentName: "Eka", ClassName: "XI RPL 2", Lat: -6.3, Lng: 106.9, Accuracy: 10},
	}
	fixed := clock.Fixed{T: time.Date(2026, 8, 10, 1, 0, 0, 0, time.UTC)}
	svc := newServiceForTest(repo, newFakeIdentity(), fakeYears{id: 1}, fakeStudents{}, fakeSchedule{}, fakeTeachers{}, fixed)
	ctx := ctxAs("admin_sekolah", 1, "Asia/Jakarta")

	items, err := svc.Anomalies(ctx, 1, "2026-08-10", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	byStudent := map[int64][]string{}
	for _, it := range items {
		byStudent[it.StudentID] = append(byStudent[it.StudentID], it.Issue)
	}
	for _, id := range []int64{1, 2, 3} {
		found := false
		for _, issue := range byStudent[id] {
			if issue == "same_location" {
				found = true
			}
		}
		if !found {
			t.Fatalf("expected siswa %d terflag same_location, got %v", id, byStudent[id])
		}
	}
	found := false
	for _, issue := range byStudent[4] {
		if issue == "low_accuracy" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected siswa 4 terflag low_accuracy, got %v", byStudent[4])
	}
	if len(byStudent[5]) != 0 {
		t.Fatalf("expected siswa 5 TIDAK ada anomali, got %v", byStudent[5])
	}
}
