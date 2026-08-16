package platformadmin

import (
	"context"
	"testing"
	"time"

	"github.com/omanjaya/nouschool/internal/platform/clock"
)

// -- fakeRepo: implementasi platformAdminRepository in-memory (tanpa DB) —
// pola yang sama dengan internal/billing/service_test.go.

type fakeRepo struct {
	schools          []SchoolOverviewRow
	invoicesAwaiting []InvoiceAwaiting
	noActiveYear     []SchoolRef
	outboxDead       []OutboxDeadBySchool
	lastActivity     []LastActivity
	totalStudents    int
	revenue          int64
	leads            int

	outboxByID map[int64]string // id -> status (dipakai RetryOutbox test)
	retried    map[int64]bool

	schoolExists      map[int64]bool
	onboarding        map[int64]OnboardingRow
	platformAnns      map[int64]PlatformAnnouncementRecord
	nextPlatformAnnID int64
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		outboxByID: map[int64]string{}, retried: map[int64]bool{},
		schoolExists: map[int64]bool{}, onboarding: map[int64]OnboardingRow{},
		platformAnns: map[int64]PlatformAnnouncementRecord{},
	}
}

// -- P2.2 --

func (f *fakeRepo) SchoolExists(ctx context.Context, schoolID int64) (bool, error) {
	return f.schoolExists[schoolID], nil
}

func (f *fakeRepo) SchoolOnboardingStatus(ctx context.Context, schoolID int64) (OnboardingRow, error) {
	return f.onboarding[schoolID], nil
}

// -- P5 --

func (f *fakeRepo) InsertPlatformAnnouncement(ctx context.Context, title, body string, startsAt, endsAt time.Time, createdBy int64) (PlatformAnnouncementRecord, error) {
	f.nextPlatformAnnID++
	rec := PlatformAnnouncementRecord{ID: f.nextPlatformAnnID, Title: title, Body: body, StartsAt: startsAt, EndsAt: endsAt, CreatedBy: createdBy, CreatedAt: time.Now()}
	f.platformAnns[rec.ID] = rec
	return rec, nil
}

func (f *fakeRepo) ListPlatformAnnouncements(ctx context.Context) ([]PlatformAnnouncementRecord, error) {
	out := make([]PlatformAnnouncementRecord, 0, len(f.platformAnns))
	for _, r := range f.platformAnns {
		out = append(out, r)
	}
	return out, nil
}

func (f *fakeRepo) ListActivePlatformAnnouncements(ctx context.Context, date time.Time) ([]PlatformAnnouncementRecord, error) {
	var out []PlatformAnnouncementRecord
	for _, r := range f.platformAnns {
		if !date.Before(r.StartsAt) && !date.After(r.EndsAt) {
			out = append(out, r)
		}
	}
	return out, nil
}

func (f *fakeRepo) UpdatePlatformAnnouncement(ctx context.Context, id int64, title, body string, startsAt, endsAt time.Time) (PlatformAnnouncementRecord, error) {
	rec, ok := f.platformAnns[id]
	if !ok {
		return PlatformAnnouncementRecord{}, ErrNotFound
	}
	rec.Title, rec.Body, rec.StartsAt, rec.EndsAt = title, body, startsAt, endsAt
	f.platformAnns[id] = rec
	return rec, nil
}

func (f *fakeRepo) DeletePlatformAnnouncement(ctx context.Context, id int64) error {
	if _, ok := f.platformAnns[id]; !ok {
		return ErrNotFound
	}
	delete(f.platformAnns, id)
	return nil
}

func (f *fakeRepo) ListSchoolsOverview(ctx context.Context) ([]SchoolOverviewRow, error) {
	return f.schools, nil
}
func (f *fakeRepo) ListInvoicesAwaitingVerification(ctx context.Context) ([]InvoiceAwaiting, error) {
	return f.invoicesAwaiting, nil
}
func (f *fakeRepo) ListSchoolsWithoutActiveYear(ctx context.Context) ([]SchoolRef, error) {
	return f.noActiveYear, nil
}
func (f *fakeRepo) ListSchoolsWithDeadOutbox(ctx context.Context) ([]OutboxDeadBySchool, error) {
	return f.outboxDead, nil
}
func (f *fakeRepo) ListSchoolLastActivity(ctx context.Context, limit int) ([]LastActivity, error) {
	return f.lastActivity, nil
}
func (f *fakeRepo) CountTotalActiveStudents(ctx context.Context) (int, error) {
	return f.totalStudents, nil
}
func (f *fakeRepo) SumRevenueInRange(ctx context.Context, from, to time.Time) (int64, error) {
	return f.revenue, nil
}
func (f *fakeRepo) CountLeadsSince(ctx context.Context, since time.Time) (int, error) {
	return f.leads, nil
}

func (f *fakeRepo) CountTeachers(ctx context.Context, schoolID int64) (int, error) { return 0, nil }
func (f *fakeRepo) CountActiveStudentsForSchool(ctx context.Context, schoolID int64) (int, error) {
	return 0, nil
}
func (f *fakeRepo) CountClasses(ctx context.Context, schoolID int64) (int, error) { return 0, nil }
func (f *fakeRepo) CountAttendanceSessionsSince(ctx context.Context, schoolID int64, sinceDate time.Time) (int, error) {
	return 0, nil
}
func (f *fakeRepo) CountJournalsSince(ctx context.Context, schoolID int64, sinceDate time.Time) (int, error) {
	return 0, nil
}
func (f *fakeRepo) CountOutboxByStatusSince(ctx context.Context, schoolID int64, since time.Time) (OutboxCounts, error) {
	return OutboxCounts{}, nil
}
func (f *fakeRepo) ListLastLoginsByRole(ctx context.Context, schoolID int64) ([]LastLoginByRole, error) {
	return nil, nil
}

func (f *fakeRepo) ListOutboxAdmin(ctx context.Context, status string, schoolID int64, limit, offset int) ([]OutboxItem, error) {
	return nil, nil
}
func (f *fakeRepo) CountOutboxAdmin(ctx context.Context, status string, schoolID int64) (int64, error) {
	return 0, nil
}
func (f *fakeRepo) GetOutboxStatus(ctx context.Context, id int64) (string, error) {
	status, ok := f.outboxByID[id]
	if !ok {
		return "", ErrNotFound
	}
	return status, nil
}
func (f *fakeRepo) RetryOutboxRow(ctx context.Context, id int64, now time.Time) error {
	f.outboxByID[id] = "pending"
	f.retried[id] = true
	return nil
}
func (f *fakeRepo) RetryAllOutbox(ctx context.Context, status string, schoolID int64, now time.Time) (int64, error) {
	var n int64
	for id, s := range f.outboxByID {
		if s == status {
			f.outboxByID[id] = "pending"
			n++
		}
	}
	return n, nil
}

var _ platformAdminRepository = (*fakeRepo)(nil)

type fakeAudit struct {
	calls []string
}

func (f *fakeAudit) Log(ctx context.Context, schoolID, userID *int64, action, entity string, entityID *int64, oldValue, newValue any) error {
	f.calls = append(f.calls, action)
	return nil
}

func strPtr(s string) *string { return &s }

// -- Overview: bucketing status subscription --

func TestOverviewBucketsSchoolStatus(t *testing.T) {
	repo := newFakeRepo()
	repo.schools = []SchoolOverviewRow{
		{SchoolID: 1, Name: "Aktif", SchoolStatus: "active", SubStatus: strPtr("active")},
		{SchoolID: 2, Name: "Grace", SchoolStatus: "active", SubStatus: strPtr("grace"), SubEndsOn: timePtr(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))},
		{SchoolID: 3, Name: "Readonly", SchoolStatus: "active", SubStatus: strPtr("readonly")},
		{SchoolID: 4, Name: "TanpaSubscription", SchoolStatus: "active", SubStatus: nil},
		{SchoolID: 5, Name: "Suspended", SchoolStatus: "suspended", SubStatus: strPtr("active")},
		{SchoolID: 6, Name: "Canceled", SchoolStatus: "active", SubStatus: strPtr("canceled")},
	}
	svc := newServiceForTest(repo, nil, clock.Fixed{T: time.Date(2026, 8, 16, 0, 0, 0, 0, time.UTC)})

	ov, err := svc.Overview(context.Background())
	if err != nil {
		t.Fatalf("Overview error: %v", err)
	}
	if ov.Stats.SchoolsActive != 1 {
		t.Errorf("SchoolsActive = %d, ingin 1", ov.Stats.SchoolsActive)
	}
	if ov.Stats.SchoolsGrace != 1 {
		t.Errorf("SchoolsGrace = %d, ingin 1", ov.Stats.SchoolsGrace)
	}
	// readonly harus mencakup: "Readonly" eksplisit + "TanpaSubscription" (fallback) + "Canceled" = 3
	if ov.Stats.SchoolsReadonly != 3 {
		t.Errorf("SchoolsReadonly = %d, ingin 3 (readonly eksplisit + fallback tanpa-subscription + canceled)", ov.Stats.SchoolsReadonly)
	}
	if ov.Stats.SchoolsSuspended != 1 {
		t.Errorf("SchoolsSuspended = %d, ingin 1", ov.Stats.SchoolsSuspended)
	}
	if len(ov.Attention.SchoolsReadonly) != 3 {
		t.Errorf("attention.schools_readonly = %d baris, ingin 3", len(ov.Attention.SchoolsReadonly))
	}
	if len(ov.Attention.SchoolsGrace) != 1 || ov.Attention.SchoolsGrace[0].SchoolID != 2 {
		t.Fatalf("attention.schools_grace tidak sesuai: %+v", ov.Attention.SchoolsGrace)
	}
	// grace_until = ends_on + 14 hari (gracePeriodDays)
	if ov.Attention.SchoolsGrace[0].EndsOn != "2026-01-01" || ov.Attention.SchoolsGrace[0].GraceUntil != "2026-01-15" {
		t.Errorf("ends_on/grace_until salah: %+v", ov.Attention.SchoolsGrace[0])
	}
}

func timePtr(t time.Time) *time.Time { return &t }

// -- RetryOutbox: transisi status --

func TestRetryOutboxDeadToPending(t *testing.T) {
	repo := newFakeRepo()
	repo.outboxByID[1] = "dead"
	svc := newServiceForTest(repo, nil, clock.System{})

	if err := svc.RetryOutbox(context.Background(), 1); err != nil {
		t.Fatalf("RetryOutbox error: %v", err)
	}
	if repo.outboxByID[1] != "pending" {
		t.Errorf("status = %q, ingin pending", repo.outboxByID[1])
	}
}

func TestRetryOutboxFailedToPending(t *testing.T) {
	repo := newFakeRepo()
	repo.outboxByID[1] = "failed"
	svc := newServiceForTest(repo, nil, clock.System{})

	if err := svc.RetryOutbox(context.Background(), 1); err != nil {
		t.Fatalf("RetryOutbox error: %v", err)
	}
	if repo.outboxByID[1] != "pending" {
		t.Errorf("status = %q, ingin pending", repo.outboxByID[1])
	}
}

func TestRetryOutboxPendingRejected(t *testing.T) {
	repo := newFakeRepo()
	repo.outboxByID[1] = "pending"
	svc := newServiceForTest(repo, nil, clock.System{})

	err := svc.RetryOutbox(context.Background(), 1)
	if err == nil {
		t.Fatal("ingin error (status pending bukan failed/dead), dapat nil")
	}
	if err != errOutboxNotRetryable {
		t.Errorf("error = %v, ingin errOutboxNotRetryable", err)
	}
}

func TestRetryOutboxSentRejected(t *testing.T) {
	repo := newFakeRepo()
	repo.outboxByID[1] = "sent"
	svc := newServiceForTest(repo, nil, clock.System{})

	if err := svc.RetryOutbox(context.Background(), 1); err != errOutboxNotRetryable {
		t.Errorf("error = %v, ingin errOutboxNotRetryable", err)
	}
}

func TestRetryOutboxNotFound(t *testing.T) {
	repo := newFakeRepo()
	svc := newServiceForTest(repo, nil, clock.System{})

	if err := svc.RetryOutbox(context.Background(), 999); err == nil {
		t.Fatal("ingin error not found")
	}
}

// -- RetryAllOutbox: audit --

func TestRetryAllOutboxAudits(t *testing.T) {
	repo := newFakeRepo()
	repo.outboxByID[1] = "dead"
	repo.outboxByID[2] = "dead"
	repo.outboxByID[3] = "failed"
	audit := &fakeAudit{}
	svc := newServiceForTest(repo, audit, clock.System{})

	n, err := svc.RetryAllOutbox(context.Background(), 42, 0, "dead")
	if err != nil {
		t.Fatalf("RetryAllOutbox error: %v", err)
	}
	if n != 2 {
		t.Errorf("retried = %d, ingin 2", n)
	}
	if len(audit.calls) != 1 || audit.calls[0] != "admin.outbox_retry_all" {
		t.Errorf("audit calls = %v, ingin [admin.outbox_retry_all]", audit.calls)
	}
}

func TestRetryAllOutboxInvalidStatus(t *testing.T) {
	repo := newFakeRepo()
	svc := newServiceForTest(repo, nil, clock.System{})

	if _, err := svc.RetryAllOutbox(context.Background(), 1, 0, "sent"); err != errRetryAllStatus {
		t.Errorf("error = %v, ingin errRetryAllStatus", err)
	}
}

// -- Onboarding: P2.2 --

func TestOnboardingNotFound(t *testing.T) {
	repo := newFakeRepo()
	svc := newServiceForTest(repo, nil, clock.System{})

	if _, err := svc.Onboarding(context.Background(), 999); err == nil {
		t.Fatal("ingin error not found (sekolah tidak ada)")
	}
}

func TestOnboardingReadyOnlyWhenAllTrue(t *testing.T) {
	repo := newFakeRepo()
	repo.schoolExists[7] = true
	repo.onboarding[7] = OnboardingRow{HasActiveYear: true, HasAdmin: true, HasSubscriptionActive: true, HasStudents: true, HasSchedule: false}
	svc := newServiceForTest(repo, nil, clock.System{})

	status, err := svc.Onboarding(context.Background(), 7)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status.Ready {
		t.Fatal("ready seharusnya false (has_schedule false)")
	}
	if !status.HasActiveYear || !status.HasAdmin || !status.HasSubscriptionActive || !status.HasStudents {
		t.Errorf("field lain seharusnya ikut true: %+v", status)
	}

	repo.onboarding[7] = OnboardingRow{HasActiveYear: true, HasAdmin: true, HasSubscriptionActive: true, HasStudents: true, HasSchedule: true}
	status2, err := svc.Onboarding(context.Background(), 7)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !status2.Ready {
		t.Fatal("ready seharusnya true (semua checklist true)")
	}
}

// -- Platform announcements: P5 --

type fakePlatformRealtime struct {
	calls []string
}

func (f *fakePlatformRealtime) PublishAll(eventType string, data map[string]any) {
	f.calls = append(f.calls, eventType)
}

func TestActivePlatformAnnouncementsByDate(t *testing.T) {
	repo := newFakeRepo()
	repo.platformAnns[1] = PlatformAnnouncementRecord{
		ID: 1, Title: "Maintenance", Body: "Server maintenance",
		StartsAt: time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC),
		EndsAt:   time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC),
	}
	svc := newServiceForTest(repo, nil, clock.System{})

	items, err := svc.ActivePlatformAnnouncements(context.Background(), "2026-08-15")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 || items[0].ID != 1 || items[0].Title != "Maintenance" {
		t.Fatalf("expected 1 item Maintenance, got %+v", items)
	}

	items2, err := svc.ActivePlatformAnnouncements(context.Background(), "2026-09-01")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items2) != 0 {
		t.Fatalf("expected 0 item di luar rentang, got %d", len(items2))
	}

	if _, err := svc.ActivePlatformAnnouncements(context.Background(), "tanggal-salah"); err == nil {
		t.Fatal("expected error format tanggal salah")
	}
}

func TestCreatePlatformAnnouncementValidationAuditRealtime(t *testing.T) {
	repo := newFakeRepo()
	audit := &fakeAudit{}
	rt := &fakePlatformRealtime{}
	svc := newServiceForTest(repo, audit, clock.System{})
	svc.SetRealtime(rt)

	_, err := svc.CreatePlatformAnnouncement(context.Background(), 1, CreatePlatformAnnouncementInput{Title: "  ", Body: "X", StartsAt: "2026-08-15", EndsAt: "2026-08-16"})
	if err == nil {
		t.Fatal("ingin error validasi (title kosong)")
	}

	view, err := svc.CreatePlatformAnnouncement(context.Background(), 1, CreatePlatformAnnouncementInput{Title: "Rilis", Body: "Fitur baru", StartsAt: "2026-08-15", EndsAt: "2026-08-16"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if view.Title != "Rilis" {
		t.Fatalf("title = %q, ingin Rilis", view.Title)
	}
	if len(audit.calls) != 1 || audit.calls[0] != "admin.platform_announcement_create" {
		t.Fatalf("audit admin.platform_announcement_create seharusnya tercatat, got %+v", audit.calls)
	}
	if len(rt.calls) != 1 || rt.calls[0] != "announcement" {
		t.Fatalf("PublishAll(announcement) seharusnya dipanggil tepat sekali, got %v", rt.calls)
	}
}

func TestUpdateDeletePlatformAnnouncement(t *testing.T) {
	repo := newFakeRepo()
	audit := &fakeAudit{}
	rt := &fakePlatformRealtime{}
	svc := newServiceForTest(repo, audit, clock.System{})
	svc.SetRealtime(rt)

	created, err := svc.CreatePlatformAnnouncement(context.Background(), 1, CreatePlatformAnnouncementInput{Title: "A", Body: "B", StartsAt: "2026-08-15", EndsAt: "2026-08-16"})
	if err != nil {
		t.Fatalf("unexpected error create: %v", err)
	}

	updated, err := svc.UpdatePlatformAnnouncement(context.Background(), 1, created.ID, UpdatePlatformAnnouncementInput{Title: "C", Body: "D", StartsAt: "2026-08-15", EndsAt: "2026-08-17"})
	if err != nil {
		t.Fatalf("unexpected error update: %v", err)
	}
	if updated.Title != "C" {
		t.Fatalf("title = %q, ingin C", updated.Title)
	}

	if _, err := svc.UpdatePlatformAnnouncement(context.Background(), 1, 999, UpdatePlatformAnnouncementInput{Title: "C", Body: "D", StartsAt: "2026-08-15", EndsAt: "2026-08-16"}); err == nil {
		t.Fatal("ingin error not found")
	}

	if err := svc.DeletePlatformAnnouncement(context.Background(), 1, created.ID); err != nil {
		t.Fatalf("unexpected error delete: %v", err)
	}
	if err := svc.DeletePlatformAnnouncement(context.Background(), 1, created.ID); err == nil {
		t.Fatal("ingin error not found (sudah dihapus)")
	}

	// audit: create + update + delete + delete(not found tidak audit) = 3
	if len(audit.calls) != 3 {
		t.Fatalf("audit calls = %d, ingin 3 (create+update+delete)", len(audit.calls))
	}
	// realtime: create + update + delete SUKSES = 3 (delete gagal TIDAK memancarkan)
	if len(rt.calls) != 3 {
		t.Fatalf("realtime calls = %d, ingin 3", len(rt.calls))
	}
}
