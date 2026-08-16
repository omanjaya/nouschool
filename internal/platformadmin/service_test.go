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
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{outboxByID: map[int64]string{}, retried: map[int64]bool{}}
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
