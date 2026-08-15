package dashboard

import (
	"context"
	"testing"
	"time"

	"github.com/omanjaya/nouschool/internal/platform/clock"
	"github.com/omanjaya/nouschool/internal/platform/reqctx"
)

// -- fake gateways (tanpa DB) — pola yang sama dengan internal/teaching &
// internal/attendance service_test.go.

type fakeTeaching struct {
	result TeachingStatusResult
	err    error
}

func (f fakeTeaching) Status(ctx context.Context, schoolID int64, dateStr string) (TeachingStatusResult, error) {
	return f.result, f.err
}

type fakeAttendance struct{ rows []ClassAttendanceSummary }

func (f fakeAttendance) Summary(ctx context.Context, schoolID int64, dateStr string) ([]ClassAttendanceSummary, error) {
	return f.rows, nil
}

type fakeAnnouncements struct{ items []AnnouncementItem }

func (f fakeAnnouncements) ActiveOn(ctx context.Context, schoolID int64, dateStr string) ([]AnnouncementItem, error) {
	return f.items, nil
}

type fakeSchedule struct {
	number           int
	startsAt, endsAt string
	hasCurrent       bool
	nextStartsAt     string
	hasNext          bool
}

func (f fakeSchedule) CurrentPeriod(ctx context.Context, schoolID int64, at time.Time) (int, string, string, bool, string, bool, error) {
	return f.number, f.startsAt, f.endsAt, f.hasCurrent, f.nextStartsAt, f.hasNext, nil
}

func ctxAs(schoolID int64, tz string) context.Context {
	ctx := reqctx.WithUser(context.Background(), 1, "display", false)
	ctx = reqctx.WithSchool(ctx, reqctx.School{ID: schoolID, Name: "Sekolah Uji", Slug: "uji", Timezone: tz})
	return ctx
}

func mkSlot(classID int64, className, status, startsAt, endsAt string) TeachingSlot {
	return TeachingSlot{
		ClassID: classID, ClassName: className, SubjectCode: "SUB", SubjectName: "Mapel",
		TeacherID: classID, TeacherName: "Guru", Status: status, PeriodStartsAt: startsAt, PeriodEndsAt: endsAt,
	}
}

// -- Board: menyusun bagian-bagian dari gateway (fake) --

func TestBoard_AssemblesAllSections(t *testing.T) {
	// now = 08:00 WIB = 01:00 UTC pada 2026-08-16 (Minggu).
	now := time.Date(2026, 8, 16, 1, 0, 0, 0, time.UTC)

	teaching := fakeTeaching{result: TeachingStatusResult{
		Summary: TeachingSummary{Mengajar: 1, BelumMulai: 1},
		Rows: []TeachingSlot{
			mkSlot(1, "XII RPL 1", "mengajar", "07:40", "08:20"),   // current (07:40-08:20 mencakup 08:00)
			mkSlot(2, "XI RPL 2", "belum_mulai", "09:00", "09:40"), // upcoming (mulai berikutnya)
			mkSlot(3, "X RPL 1", "belum_mulai", "10:00", "10:40"),  // BUKAN upcoming (bukan starts_at minimal)
		},
	}}
	attendance := fakeAttendance{rows: []ClassAttendanceSummary{
		{ClassID: 1, ClassName: "XII RPL 1", Total: 30, Hadir: 25, Izin: 1, SessionStatus: "open"},
	}}
	announcements := fakeAnnouncements{items: []AnnouncementItem{{ID: 1, Title: "Libur", Body: "Sekolah libur"}}}
	schedule := fakeSchedule{number: 2, startsAt: "07:40", endsAt: "08:20", hasCurrent: true, nextStartsAt: "09:00", hasNext: true}

	svc := newServiceForTest(teaching, attendance, announcements, schedule, clock.Fixed{T: now})
	ctx := ctxAs(1, "Asia/Jakarta")

	board, err := svc.Board(ctx, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if board.School.Name != "Sekolah Uji" {
		t.Errorf("expected school name Sekolah Uji, got %s", board.School.Name)
	}
	if board.Now.Date != "2026-08-16" {
		t.Errorf("expected date 2026-08-16, got %s", board.Now.Date)
	}
	if board.Now.DayLabel != "Minggu, 16 Agu 2026" {
		t.Errorf("expected day_label 'Minggu, 16 Agu 2026', got %q", board.Now.DayLabel)
	}
	if board.Now.Time != "08:00" {
		t.Errorf("expected time 08:00, got %s", board.Now.Time)
	}
	if board.Now.CurrentPeriod == nil || board.Now.CurrentPeriod.Number != 2 {
		t.Fatalf("expected current_period jam ke-2, got %+v", board.Now.CurrentPeriod)
	}
	if board.Now.NextStartsAt == nil || *board.Now.NextStartsAt != "09:00" {
		t.Fatalf("expected next_starts_at 09:00, got %v", board.Now.NextStartsAt)
	}

	if board.Teaching.Summary.Mengajar != 1 {
		t.Errorf("expected summary.mengajar=1, got %d", board.Teaching.Summary.Mengajar)
	}
	if len(board.Teaching.Current) != 1 || board.Teaching.Current[0].ClassID != 1 {
		t.Fatalf("expected current=[kelas 1] (slot jam berjalan), got %+v", board.Teaching.Current)
	}
	if len(board.Teaching.Upcoming) != 1 || board.Teaching.Upcoming[0].ClassID != 2 {
		t.Fatalf("expected upcoming=[kelas 2] (slot jam berikutnya, BUKAN kelas 3), got %+v", board.Teaching.Upcoming)
	}

	if len(board.Attendance) != 1 || board.Attendance[0].Hadir != 25 {
		t.Fatalf("expected 1 baris attendance hadir=25, got %+v", board.Attendance)
	}

	if len(board.Announcements) != 1 || board.Announcements[0].Title != "Libur" {
		t.Fatalf("expected 1 pengumuman Libur, got %+v", board.Announcements)
	}

	if board.GeneratedAt != now {
		t.Errorf("expected generated_at = now, got %v", board.GeneratedAt)
	}
}

func TestBoard_EmptySections_NoCurrentPeriod(t *testing.T) {
	now := time.Date(2026, 8, 16, 20, 0, 0, 0, time.UTC) // 03:00 WIB, di luar jam sekolah
	teaching := fakeTeaching{result: TeachingStatusResult{Summary: TeachingSummary{}, Rows: nil}}
	attendance := fakeAttendance{rows: nil}
	announcements := fakeAnnouncements{items: nil}
	schedule := fakeSchedule{hasCurrent: false, hasNext: false}

	svc := newServiceForTest(teaching, attendance, announcements, schedule, clock.Fixed{T: now})
	ctx := ctxAs(1, "Asia/Jakarta")

	board, err := svc.Board(ctx, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if board.Now.CurrentPeriod != nil {
		t.Errorf("expected current_period nil, got %+v", board.Now.CurrentPeriod)
	}
	if board.Now.NextStartsAt != nil {
		t.Errorf("expected next_starts_at nil, got %v", board.Now.NextStartsAt)
	}
	if len(board.Teaching.Current) != 0 || len(board.Teaching.Upcoming) != 0 {
		t.Errorf("expected current/upcoming kosong (bukan nil, array kosong), got %+v / %+v", board.Teaching.Current, board.Teaching.Upcoming)
	}
	if board.Teaching.Current == nil || board.Teaching.Upcoming == nil {
		t.Error("expected current/upcoming SELALU array (tidak pernah null di JSON)")
	}
	if len(board.Attendance) != 0 {
		t.Errorf("expected attendance kosong, got %+v", board.Attendance)
	}
	if len(board.Announcements) != 0 {
		t.Errorf("expected announcements kosong, got %+v", board.Announcements)
	}
}
