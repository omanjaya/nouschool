package platformadmin

import (
	"context"
	"errors"
	"fmt"
	"strings"
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

// PlatformRealtime adalah kebutuhan modul platformadmin dari modul realtime
// (fase 13 Gelombang 2, P5 "Pengumuman platform") — consumer-side interface
// kecil dideklarasikan di sisi PEMAKAI (lihat CLAUDE.md). BEDA dari interface
// Realtime satu-sekolah dipakai modul lain (mis. announcement.Realtime):
// pengumuman platform harus tampil di SEMUA sekolah sekaligus, jadi
// method-nya PublishAll (broadcast lintas sekolah), bukan Publish/PublishTo
// (satu sekolah). TIDAK dipenuhi *realtime.Hub secara langsung — dijembatani
// adapter kecil di cmd/server (realtimeadapter.go).
type PlatformRealtime interface {
	PublishAll(eventType string, data map[string]any)
}

// Service berisi logika modul platformadmin: menyusun ulang & mengagregasi
// data lintas modul untuk panel super admin (lihat catatan desain di model.go).
type Service struct {
	repo     platformAdminRepository
	audit    AuditLogger // opsional — nil = audit retry-all dilewati (lihat SetAudit atau constructor)
	files    *storage.Store
	clock    clock.Clock
	realtime PlatformRealtime // opsional — nil = event realtime dilewati (lihat SetRealtime)
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

// SetRealtime menyuntikkan PlatformRealtime SETELAH konstruksi (opsional,
// disuntik main.go — pola yang sama dengan modul lain; nil aman/no-op).
func (s *Service) SetRealtime(r PlatformRealtime) { s.realtime = r }

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

// -- P2.2 (fase 13 Gelombang 2): GET /api/admin/schools/{id}/onboarding --

// Onboarding — checklist status onboarding sekolah (docs/11-superadmin.md
// P2 "Checklist status onboarding tampil di detail sekolah"). ready=true
// hanya bila SEMUA checklist true.
func (s *Service) Onboarding(ctx context.Context, schoolID int64) (OnboardingStatus, error) {
	exists, err := s.repo.SchoolExists(ctx, schoolID)
	if err != nil {
		return OnboardingStatus{}, err
	}
	if !exists {
		return OnboardingStatus{}, httpx.ErrNotFound
	}
	row, err := s.repo.SchoolOnboardingStatus(ctx, schoolID)
	if err != nil {
		return OnboardingStatus{}, err
	}
	ready := row.HasActiveYear && row.HasAdmin && row.HasSubscriptionActive && row.HasStudents && row.HasSchedule
	return OnboardingStatus{
		HasActiveYear: row.HasActiveYear, HasAdmin: row.HasAdmin, HasSubscriptionActive: row.HasSubscriptionActive,
		HasStudents: row.HasStudents, HasSchedule: row.HasSchedule, Ready: ready,
	}, nil
}

// -- P5 (fase 13 Gelombang 2): platform_announcements,
// docs/11-superadmin.md "Pengumuman platform" --

const platformDateLayout = "2006-01-02"

// logPlatformAudit — SAMA seperti logAudit, tapi menyertakan entity_id (mis.
// id pengumuman platform) dan school_id SELALU nil (pengumuman platform
// tidak terikat satu sekolah). logAudit di atas TIDAK punya parameter
// entity_id (hanya dipakai retry-all yang memang tidak punya satu entitas
// spesifik) — makanya method terpisah, bukan menambah parameter ke logAudit
// dan mengubah SEMUA call site yang sudah ada.
func (s *Service) logPlatformAudit(ctx context.Context, actorUserID int64, action, entity string, entityID int64, oldValue, newValue any) {
	if s.audit == nil {
		return
	}
	uid, eid := actorUserID, entityID
	_ = s.audit.Log(ctx, nil, &uid, action, entity, &eid, oldValue, newValue)
}

// emitPlatformAnnouncement memancarkan "announcement" {} ke SEMUA sekolah
// (docs/11-superadmin.md P5 "Realtime: create/update/delete platform
// announcement -> publish event announcement ke SEMUA sekolah") — dipanggil
// SETELAH operasi sukses, best-effort (nil-safe). Data SENGAJA kosong —
// klien selalu refetch GET /api/announcements (bus read-only, pola sama
// announcement.Service.emitAnnouncement).
func (s *Service) emitPlatformAnnouncement() {
	if s.realtime == nil {
		return
	}
	s.realtime.PublishAll("announcement", map[string]any{})
}

func parsePlatformAnnouncementDates(startsAtRaw, endsAtRaw string) (time.Time, time.Time, error) {
	start, err := time.Parse(platformDateLayout, strings.TrimSpace(startsAtRaw))
	if err != nil {
		return time.Time{}, time.Time{}, httpx.Validation("Format starts_at harus YYYY-MM-DD.")
	}
	end, err := time.Parse(platformDateLayout, strings.TrimSpace(endsAtRaw))
	if err != nil {
		return time.Time{}, time.Time{}, httpx.Validation("Format ends_at harus YYYY-MM-DD.")
	}
	if start.After(end) {
		return time.Time{}, time.Time{}, httpx.Validation("starts_at tidak boleh setelah ends_at.")
	}
	return start, end, nil
}

func platformAnnouncementView(r PlatformAnnouncementRecord) PlatformAnnouncementView {
	return PlatformAnnouncementView{
		ID: r.ID, Title: r.Title, Body: r.Body,
		StartsAt: r.StartsAt.Format(platformDateLayout), EndsAt: r.EndsAt.Format(platformDateLayout),
		CreatedBy: r.CreatedBy, CreatedAt: r.CreatedAt,
	}
}

// ListPlatformAnnouncements — GET /api/admin/platform-announcements.
func (s *Service) ListPlatformAnnouncements(ctx context.Context) ([]PlatformAnnouncementView, error) {
	rows, err := s.repo.ListPlatformAnnouncements(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]PlatformAnnouncementView, 0, len(rows))
	for _, r := range rows {
		out = append(out, platformAnnouncementView(r))
	}
	return out, nil
}

// CreatePlatformAnnouncementInput adalah parameter POST /api/admin/platform-announcements.
type CreatePlatformAnnouncementInput struct {
	Title    string
	Body     string
	StartsAt string
	EndsAt   string
}

// CreatePlatformAnnouncement — POST /api/admin/platform-announcements.
func (s *Service) CreatePlatformAnnouncement(ctx context.Context, actorUserID int64, in CreatePlatformAnnouncementInput) (PlatformAnnouncementView, error) {
	title := strings.TrimSpace(in.Title)
	body := strings.TrimSpace(in.Body)
	if title == "" || body == "" {
		return PlatformAnnouncementView{}, httpx.Validation("Judul dan isi pengumuman wajib diisi.")
	}
	start, end, err := parsePlatformAnnouncementDates(in.StartsAt, in.EndsAt)
	if err != nil {
		return PlatformAnnouncementView{}, err
	}
	rec, err := s.repo.InsertPlatformAnnouncement(ctx, title, body, start, end, actorUserID)
	if err != nil {
		return PlatformAnnouncementView{}, err
	}
	s.logPlatformAudit(ctx, actorUserID, "admin.platform_announcement_create", "platform_announcement", rec.ID, nil,
		map[string]any{"title": title, "starts_at": start.Format(platformDateLayout), "ends_at": end.Format(platformDateLayout)})
	s.emitPlatformAnnouncement()
	return platformAnnouncementView(rec), nil
}

// UpdatePlatformAnnouncementInput adalah parameter PATCH /api/admin/platform-announcements/{id}.
type UpdatePlatformAnnouncementInput struct {
	Title    string
	Body     string
	StartsAt string
	EndsAt   string
}

// UpdatePlatformAnnouncement — PATCH /api/admin/platform-announcements/{id}.
func (s *Service) UpdatePlatformAnnouncement(ctx context.Context, actorUserID, id int64, in UpdatePlatformAnnouncementInput) (PlatformAnnouncementView, error) {
	title := strings.TrimSpace(in.Title)
	body := strings.TrimSpace(in.Body)
	if title == "" || body == "" {
		return PlatformAnnouncementView{}, httpx.Validation("Judul dan isi pengumuman wajib diisi.")
	}
	start, end, err := parsePlatformAnnouncementDates(in.StartsAt, in.EndsAt)
	if err != nil {
		return PlatformAnnouncementView{}, err
	}
	rec, err := s.repo.UpdatePlatformAnnouncement(ctx, id, title, body, start, end)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return PlatformAnnouncementView{}, httpx.ErrNotFound
		}
		return PlatformAnnouncementView{}, err
	}
	s.logPlatformAudit(ctx, actorUserID, "admin.platform_announcement_update", "platform_announcement", id, nil,
		map[string]any{"title": title, "starts_at": start.Format(platformDateLayout), "ends_at": end.Format(platformDateLayout)})
	s.emitPlatformAnnouncement()
	return platformAnnouncementView(rec), nil
}

// DeletePlatformAnnouncement — DELETE /api/admin/platform-announcements/{id}.
func (s *Service) DeletePlatformAnnouncement(ctx context.Context, actorUserID, id int64) error {
	if err := s.repo.DeletePlatformAnnouncement(ctx, id); err != nil {
		if errors.Is(err, ErrNotFound) {
			return httpx.ErrNotFound
		}
		return err
	}
	s.logPlatformAudit(ctx, actorUserID, "admin.platform_announcement_delete", "platform_announcement", id, nil, nil)
	s.emitPlatformAnnouncement()
	return nil
}

// ActivePlatformAnnouncements — pengumuman platform aktif pada tanggal
// (YYYY-MM-DD) tertentu, TANPA gerbang permission (interface publik dipakai
// modul announcement lewat consumer-side interface, lihat
// cmd/server/platformadminadapter.go — pola sama
// announcement.Service.ActiveOn dipakai modul dashboard).
func (s *Service) ActivePlatformAnnouncements(ctx context.Context, dateStr string) ([]PlatformAnnouncementItem, error) {
	d, err := time.Parse(platformDateLayout, strings.TrimSpace(dateStr))
	if err != nil {
		return nil, httpx.Validation("Format tanggal harus YYYY-MM-DD.")
	}
	rows, err := s.repo.ListActivePlatformAnnouncements(ctx, d)
	if err != nil {
		return nil, err
	}
	out := make([]PlatformAnnouncementItem, 0, len(rows))
	for _, r := range rows {
		out = append(out, PlatformAnnouncementItem{
			ID: r.ID, Title: r.Title, Body: r.Body,
			StartsAt: r.StartsAt.Format(platformDateLayout), EndsAt: r.EndsAt.Format(platformDateLayout), CreatedAt: r.CreatedAt,
		})
	}
	return out, nil
}
