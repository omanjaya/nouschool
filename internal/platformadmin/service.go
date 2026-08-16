package platformadmin

import (
	"context"
	"fmt"
	"time"

	"github.com/omanjaya/nouschool/internal/platform/clock"
	"github.com/omanjaya/nouschool/internal/platform/httpx"
	"github.com/omanjaya/nouschool/internal/platform/storage"
)

// AuditLogger adalah kebutuhan modul platformadmin dari modul identity —
// consumer-side interface kecil dideklarasikan di sisi PEMAKAI (lihat
// CLAUDE.md), dipenuhi *identity.Service secara struktural lewat method Log.
// Dipakai HANYA untuk RetryAllOutbox (task: "Audit retry-all saja").
type AuditLogger interface {
	Log(ctx context.Context, schoolID, userID *int64, action, entity string, entityID *int64, oldValue, newValue any) error
}

// dateLayout — format tanggal pendek dipakai response (sama konvensi
// internal/billing).
const dateLayout = "2006-01-02"

// gracePeriodDays — NILAI HARUS SAMA PERSIS dengan billing.GracePeriodDays
// (didefinisikan ulang di sini karena platformadmin TIDAK boleh mengimpor
// billing untuk tipe/konstanta apa pun — lihat CLAUDE.md; pola yang sama
// dengan billing.PermBillingView vs identity.PermBillingView).
const gracePeriodDays = 14

const outboxPageSize = 50

// Service berisi logika modul platformadmin: menyusun ulang & mengagregasi
// data lintas modul untuk panel super admin (lihat catatan desain di model.go).
type Service struct {
	repo  platformAdminRepository
	audit AuditLogger // opsional — nil = audit retry-all dilewati (lihat SetAudit atau constructor)
	files *storage.Store
	clock clock.Clock
}

func NewService(repo *Repository, audit AuditLogger, files *storage.Store, clk clock.Clock) *Service {
	if clk == nil {
		clk = clock.System{}
	}
	if files == nil {
		files = storage.FromEnv()
	}
	return &Service{repo: repo, audit: audit, files: files, clock: clk}
}

// newServiceForTest membangun Service dengan repository FAKE (in-memory,
// tanpa DB) — dipakai test di package ini saja (pola sama billing.newServiceForTest).
func newServiceForTest(repo platformAdminRepository, audit AuditLogger, clk clock.Clock) *Service {
	if clk == nil {
		clk = clock.System{}
	}
	return &Service{repo: repo, audit: audit, files: storage.New(""), clock: clk}
}

func (s *Service) logAudit(ctx context.Context, schoolID *int64, actorUserID int64, action, entity string, oldValue, newValue any) {
	if s.audit == nil {
		return
	}
	uid := actorUserID
	_ = s.audit.Log(ctx, schoolID, &uid, action, entity, nil, oldValue, newValue)
}

// -- P1: GET /api/admin/overview --

// effectiveStatus menentukan status platform efektif satu sekolah:
// suspended (schools.status) menang atas apa pun; selain itu status
// subscription (active/grace/readonly), DAN fallback readonly bila sekolah
// belum pernah punya subscription sama sekali ATAU statusnya "canceled"
// (docs/11-superadmin.md P1: "status subscription efektif ... fallback
// readonly bila tanpa subscription" — canceled disamakan readonly, keputusan
// implementasi: langganan yang dibatalkan butuh aksi billing sama seperti
// readonly, bukan kategori terpisah di dashboard ini).
func effectiveStatus(row SchoolOverviewRow) string {
	if row.SchoolStatus == "suspended" {
		return "suspended"
	}
	if row.SubStatus == nil {
		return "readonly"
	}
	switch *row.SubStatus {
	case "active":
		return "active"
	case "grace":
		return "grace"
	default: // readonly, canceled, atau nilai lain tak dikenal
		return "readonly"
	}
}

// Overview — GET /api/admin/overview (P1, satu fetch).
func (s *Service) Overview(ctx context.Context) (Overview, error) {
	now := s.clock.Now()

	schoolRows, err := s.repo.ListSchoolsOverview(ctx)
	if err != nil {
		return Overview{}, err
	}
	stats, graceList, readonlyList := bucketSchools(schoolRows)

	totalStudents, err := s.repo.CountTotalActiveStudents(ctx)
	if err != nil {
		return Overview{}, err
	}
	stats.TotalStudents = totalStudents

	yearStart := time.Date(now.Year(), 1, 1, 0, 0, 0, 0, time.UTC)
	revenue, err := s.repo.SumRevenueInRange(ctx, yearStart, yearStart.AddDate(1, 0, 0))
	if err != nil {
		return Overview{}, err
	}
	stats.RevenueYear = revenue

	leads, err := s.repo.CountLeadsSince(ctx, now.AddDate(0, 0, -7))
	if err != nil {
		return Overview{}, err
	}
	stats.Leads7d = leads

	invoicesAwaiting, err := s.repo.ListInvoicesAwaitingVerification(ctx)
	if err != nil {
		return Overview{}, err
	}
	noActiveYear, err := s.repo.ListSchoolsWithoutActiveYear(ctx)
	if err != nil {
		return Overview{}, err
	}
	outboxDead, err := s.repo.ListSchoolsWithDeadOutbox(ctx)
	if err != nil {
		return Overview{}, err
	}
	lastActivity, err := s.repo.ListSchoolLastActivity(ctx, 20)
	if err != nil {
		return Overview{}, err
	}

	return Overview{
		Stats: stats,
		Attention: Attention{
			InvoicesAwaiting:    invoicesAwaiting,
			SchoolsGrace:        graceList,
			SchoolsReadonly:     readonlyList,
			SchoolsNoActiveYear: noActiveYear,
			OutboxDead:          outboxDead,
		},
		LastActivity: lastActivity,
	}, nil
}

// bucketSchools adalah logika murni (testable tanpa DB) di balik Overview:
// mem-bucket setiap baris SchoolOverviewRow ke status efektifnya, membangun
// counter DAN daftar perlu-perhatian grace/readonly sekaligus (satu sumber
// kebenaran — lihat catatan di package doc).
func bucketSchools(rows []SchoolOverviewRow) (stats OverviewStats, graceList []SchoolGrace, readonlyList []SchoolRef) {
	graceList = []SchoolGrace{}
	readonlyList = []SchoolRef{}
	for _, row := range rows {
		switch effectiveStatus(row) {
		case "active":
			stats.SchoolsActive++
		case "grace":
			stats.SchoolsGrace++
			endsOn, graceUntil := "", ""
			if row.SubEndsOn != nil {
				endsOn = row.SubEndsOn.Format(dateLayout)
				graceUntil = row.SubEndsOn.AddDate(0, 0, gracePeriodDays).Format(dateLayout)
			}
			graceList = append(graceList, SchoolGrace{SchoolID: row.SchoolID, Name: row.Name, EndsOn: endsOn, GraceUntil: graceUntil})
		case "readonly":
			stats.SchoolsReadonly++
			readonlyList = append(readonlyList, SchoolRef{SchoolID: row.SchoolID, Name: row.Name})
		case "suspended":
			stats.SchoolsSuspended++
		}
	}
	return stats, graceList, readonlyList
}

// -- P3: GET /api/admin/schools/{id}/stats --

// SchoolStats — GET /api/admin/schools/{id}/stats. uploadsBytes best-effort
// (error storage diabaikan -> 0, "0 bila belum ada folder" sesuai task).
func (s *Service) SchoolStats(ctx context.Context, schoolID int64) (SchoolStats, error) {
	now := s.clock.Now()
	since7 := now.AddDate(0, 0, -7)
	since30 := now.AddDate(0, 0, -30)

	teachers, err := s.repo.CountTeachers(ctx, schoolID)
	if err != nil {
		return SchoolStats{}, err
	}
	students, err := s.repo.CountActiveStudentsForSchool(ctx, schoolID)
	if err != nil {
		return SchoolStats{}, err
	}
	classes, err := s.repo.CountClasses(ctx, schoolID)
	if err != nil {
		return SchoolStats{}, err
	}
	sessions7, err := s.repo.CountAttendanceSessionsSince(ctx, schoolID, since7)
	if err != nil {
		return SchoolStats{}, err
	}
	journals7, err := s.repo.CountJournalsSince(ctx, schoolID, since7)
	if err != nil {
		return SchoolStats{}, err
	}
	outboxCounts, err := s.repo.CountOutboxByStatusSince(ctx, schoolID, since30)
	if err != nil {
		return SchoolStats{}, err
	}
	lastLogins, err := s.repo.ListLastLoginsByRole(ctx, schoolID)
	if err != nil {
		return SchoolStats{}, err
	}

	uploadsBytes, _ := s.files.DirSize(fmt.Sprintf("%d", schoolID))

	return SchoolStats{
		Teachers: teachers, Students: students, Classes: classes,
		AttendanceSessions7d: sessions7, Journals7d: journals7,
		Notifications30d: outboxCounts, LastLogins: lastLogins, UploadsBytes: uploadsBytes,
	}, nil
}

// -- P4.4: outbox global --

// ListOutbox — GET /api/admin/outbox?status=&school_id=&page= (status
// default "dead" sesuai task).
func (s *Service) ListOutbox(ctx context.Context, status string, schoolID int64, page int) (OutboxPage, error) {
	if status == "" {
		status = "dead"
	}
	if page < 1 {
		page = 1
	}
	items, err := s.repo.ListOutboxAdmin(ctx, status, schoolID, outboxPageSize, (page-1)*outboxPageSize)
	if err != nil {
		return OutboxPage{}, err
	}
	total, err := s.repo.CountOutboxAdmin(ctx, status, schoolID)
	if err != nil {
		return OutboxPage{}, err
	}
	return OutboxPage{Items: items, Total: total}, nil
}

var errOutboxNotRetryable = httpx.Validation("Baris outbox ini bukan berstatus failed/dead, tidak bisa diulang.")

// RetryOutbox — POST /api/admin/outbox/{id}/retry: set pending, attempts
// TETAP, next_retry_at=now supaya worker existing (internal/notification)
// mengambilnya di poll berikutnya. 422 bila status BUKAN failed/dead.
func (s *Service) RetryOutbox(ctx context.Context, id int64) error {
	status, err := s.repo.GetOutboxStatus(ctx, id)
	if err != nil {
		if err == ErrNotFound {
			return httpx.ErrNotFound
		}
		return err
	}
	if status != "failed" && status != "dead" {
		return errOutboxNotRetryable
	}
	return s.repo.RetryOutboxRow(ctx, id, s.clock.Now())
}

var errRetryAllStatus = httpx.Validation("status harus 'dead' atau 'failed'.")

// RetryAllOutbox — POST /api/admin/outbox/retry-all {school_id?, status}.
// schoolID=0 berarti SEMUA sekolah. Audit admin.outbox_retry_all (task:
// "Audit retry-all saja").
func (s *Service) RetryAllOutbox(ctx context.Context, actorUserID, schoolID int64, status string) (int, error) {
	if status != "dead" && status != "failed" {
		return 0, errRetryAllStatus
	}
	n, err := s.repo.RetryAllOutbox(ctx, status, schoolID, s.clock.Now())
	if err != nil {
		return 0, err
	}
	var sidPtr *int64
	if schoolID != 0 {
		sidPtr = &schoolID
	}
	s.logAudit(ctx, sidPtr, actorUserID, "admin.outbox_retry_all", "notification_outbox",
		nil, map[string]any{"status": status, "school_id": schoolID, "retried": n})
	return int(n), nil
}
