package announcement

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/omanjaya/nouschool/internal/platform/clock"
	"github.com/omanjaya/nouschool/internal/platform/httpx"
	"github.com/omanjaya/nouschool/internal/platform/reqctx"
)

// -- consumer-side interfaces (lihat CLAUDE.md: antar-modul lewat interface
// kecil dideklarasikan di sisi PEMAKAI, di-inject lewat constructor di
// main.go). Signature primitif supaya *identity.Service memenuhi interface
// ini secara struktural TANPA announcement perlu mengimpor package identity.

// IdentityGateway adalah kebutuhan modul announcement dari modul identity:
// gerbang permission (announcement:manage) dan audit log.
type IdentityGateway interface {
	HasPermission(role, perm string) bool
	Log(ctx context.Context, schoolID, userID *int64, action, entity string, entityID *int64, oldValue, newValue any) error
}

// Realtime adalah kebutuhan modul announcement dari modul realtime (Fase 12,
// bus event WebSocket read-only server->client) — consumer-side interface
// kecil dideklarasikan di sisi PEMAKAI (lihat CLAUDE.md). TIDAK dipenuhi
// *realtime.Hub secara langsung — dijembatani adapter kecil di cmd/server
// (realtimeadapter.go).
type Realtime interface {
	Publish(schoolID int64, eventType string, data map[string]any)
}

// PlatformAnnouncementItem — kebutuhan modul announcement dari sumber
// pengumuman platform (fase 13 Gelombang 2, docs/11-superadmin.md P5
// "Pengumuman platform") — consumer-side interface primitif dideklarasikan
// di sisi PEMAKAI (lihat CLAUDE.md). Dipenuhi *platformadmin.Service secara
// STRUKTURAL lewat adapter kecil di cmd/server (platformadminadapter.go) —
// announcement TIDAK mengimpor platformadmin untuk tipe apa pun.
type PlatformAnnouncementItem struct {
	ID        int64
	Title     string
	Body      string
	StartsAt  string // YYYY-MM-DD
	EndsAt    string // YYYY-MM-DD
	CreatedAt time.Time
}

// PlatformAnnouncements adalah kebutuhan modul announcement dari modul
// platformadmin: pengumuman platform aktif pada tanggal tertentu.
type PlatformAnnouncements interface {
	ActiveOn(ctx context.Context, dateStr string) ([]PlatformAnnouncementItem, error)
}

// Service berisi aturan bisnis modul announcement.
type Service struct {
	repo     announcementRepository
	identity IdentityGateway
	clock    clock.Clock
	realtime Realtime              // opsional — nil = event realtime dilewati (lihat SetRealtime)
	platform PlatformAnnouncements // opsional — nil = pengumuman platform dilewati (lihat SetPlatformGateway)
}

func NewService(repo *Repository, identity IdentityGateway, clk clock.Clock) *Service {
	if clk == nil {
		clk = clock.System{}
	}
	return &Service{repo: repo, identity: identity, clock: clk}
}

// SetRealtime menyuntikkan Realtime SETELAH konstruksi (opsional, disuntik
// main.go — pola yang sama dengan modul lain; nil aman/no-op).
func (s *Service) SetRealtime(r Realtime) { s.realtime = r }

// SetPlatformGateway menyuntikkan PlatformAnnouncements SETELAH konstruksi
// (opsional, disuntik main.go SETELAH platformadmin dikonstruksi — pola
// setter yang sama dengan SetRealtime; nil aman/no-op -> List/ActiveOn
// TIDAK menyertakan pengumuman platform).
func (s *Service) SetPlatformGateway(p PlatformAnnouncements) { s.platform = p }

// emitAnnouncement memancarkan "announcement" {} broadcast satu sekolah
// (docs tugas Fase 12: create/update/delete) — dipanggil SETELAH operasi
// sukses, best-effort (nil-safe). Data SENGAJA kosong — klien selalu
// refetch GET /api/announcements (bus read-only).
func (s *Service) emitAnnouncement(schoolID int64) {
	if s.realtime == nil {
		return
	}
	s.realtime.Publish(schoolID, "announcement", map[string]any{})
}

// newServiceForTest membangun Service dengan repository FAKE (in-memory,
// tanpa DB) — dipakai test di package ini saja (service_test.go).
func newServiceForTest(repo announcementRepository, identity IdentityGateway, clk clock.Clock) *Service {
	return &Service{repo: repo, identity: identity, clock: clk}
}

func (s *Service) audit(ctx context.Context, schoolID, actorUserID int64, action, entity string, entityID int64, oldValue, newValue any) {
	sid, uid, eid := schoolID, actorUserID, entityID
	_ = s.identity.Log(ctx, &sid, &uid, action, entity, &eid, oldValue, newValue)
}

func schoolTimezone(ctx context.Context) string {
	sch, _ := reqctx.SchoolFromContext(ctx)
	return sch.Timezone
}

// schoolToday mengembalikan tanggal (tanpa jam) HARI INI menurut zona waktu
// sekolah tz — pola yang sama dengan internal/attendance & internal/teaching.
func schoolToday(now time.Time, tz string) time.Time {
	local := clock.InZone(now, tz)
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.UTC)
}

func viewFromRecord(r Record) AnnouncementView {
	return AnnouncementView{
		ID: r.ID, Title: r.Title, Body: r.Body,
		StartsAt: NewDate(r.StartsAt), EndsAt: NewDate(r.EndsAt), CreatedAt: r.CreatedAt,
	}
}

func viewsFromRecords(recs []Record) []AnnouncementView {
	out := make([]AnnouncementView, 0, len(recs))
	for _, r := range recs {
		out = append(out, viewFromRecord(r))
	}
	return out
}

// -- GET /api/announcements --

// platformViews mengambil pengumuman platform aktif pada dateStr (YYYY-MM-DD)
// dan memetakannya menjadi AnnouncementView{IsPlatform:true} — dipakai List
// (activeOnly=true). s.platform nil (belum di-inject/gateway kosong) -> nil
// slice, TIDAK error (fitur opsional, lihat SetPlatformGateway).
func (s *Service) platformViews(ctx context.Context, dateStr string) ([]AnnouncementView, error) {
	if s.platform == nil {
		return nil, nil
	}
	items, err := s.platform.ActiveOn(ctx, dateStr)
	if err != nil {
		return nil, err
	}
	out := make([]AnnouncementView, 0, len(items))
	for _, it := range items {
		start, _ := time.Parse("2006-01-02", it.StartsAt)
		end, _ := time.Parse("2006-01-02", it.EndsAt)
		out = append(out, AnnouncementView{
			ID: it.ID, Title: it.Title, Body: it.Body,
			StartsAt: NewDate(start), EndsAt: NewDate(end), CreatedAt: it.CreatedAt, IsPlatform: true,
		})
	}
	return out, nil
}

// platformBoardItems — SAMA seperti platformViews, tapi bentuk BoardItem
// (dipakai ActiveOn/TV board).
func (s *Service) platformBoardItems(ctx context.Context, dateStr string) ([]BoardItem, error) {
	if s.platform == nil {
		return nil, nil
	}
	items, err := s.platform.ActiveOn(ctx, dateStr)
	if err != nil {
		return nil, err
	}
	out := make([]BoardItem, 0, len(items))
	for _, it := range items {
		out = append(out, BoardItem{ID: it.ID, Title: it.Title, Body: it.Body, IsPlatform: true})
	}
	return out, nil
}

// List — activeOnly=true: pengumuman aktif HARI INI (tanggal lokal sekolah),
// dibaca SEMUA role termasuk akun display (docs/06-teaching.md "Pengumuman"),
// DIGABUNG dengan pengumuman platform aktif (fase 13 Gelombang 2,
// docs/11-superadmin.md P5) — platform DULU, baru sekolah (urutan sama
// dengan ActiveOn/TV board di bawah); activeOnly=false: SEMUA pengumuman
// SEKOLAH SAJA (pengumuman platform dikelola lewat panel super admin
// tersendiri, /api/admin/platform-announcements — TIDAK muncul di daftar
// kelola sekolah), butuh perm announcement:manage (docs/02-identity.md:
// admin & kepsek).
func (s *Service) List(ctx context.Context, schoolID int64, activeOnly bool) ([]AnnouncementView, error) {
	if !activeOnly {
		if !s.identity.HasPermission(reqctx.Role(ctx), PermAnnouncementManage) {
			return nil, httpx.ErrForbidden
		}
		rows, err := s.repo.ListAll(ctx, schoolID)
		if err != nil {
			return nil, err
		}
		return viewsFromRecords(rows), nil
	}

	today := schoolToday(s.clock.Now(), schoolTimezone(ctx))
	todayStr := today.Format("2006-01-02")
	platform, err := s.platformViews(ctx, todayStr)
	if err != nil {
		return nil, err
	}
	rows, err := s.repo.ListActive(ctx, schoolID, today)
	if err != nil {
		return nil, err
	}
	out := make([]AnnouncementView, 0, len(platform)+len(rows))
	out = append(out, platform...)
	out = append(out, viewsFromRecords(rows)...)
	return out, nil
}

// ActiveOn — pengumuman aktif pada tanggal (YYYY-MM-DD) tertentu, TANPA
// gerbang permission (interface publik dipakai modul dashboard TV lewat
// consumer-side interface, lihat cmd/server/dashboardadapter.go — payload TV
// hanya butuh {id,title,body}, docs/06-teaching.md), DIGABUNG dengan
// pengumuman platform aktif (fase 13 Gelombang 2, docs/11-superadmin.md P5
// "TV board ... juga menyertakan them, platform dulu, lalu sekolah").
func (s *Service) ActiveOn(ctx context.Context, schoolID int64, dateStr string) ([]BoardItem, error) {
	d, err := time.Parse("2006-01-02", strings.TrimSpace(dateStr))
	if err != nil {
		return nil, httpx.Validation("Format tanggal harus YYYY-MM-DD.")
	}
	normalized := d.Format("2006-01-02")
	platform, err := s.platformBoardItems(ctx, normalized)
	if err != nil {
		return nil, err
	}
	rows, err := s.repo.ListActive(ctx, schoolID, d)
	if err != nil {
		return nil, err
	}
	out := make([]BoardItem, 0, len(platform)+len(rows))
	out = append(out, platform...)
	for _, r := range rows {
		out = append(out, BoardItem{ID: r.ID, Title: r.Title, Body: r.Body})
	}
	return out, nil
}

// -- validasi bersama Create/Update --

func parseAnnouncementDates(startsAtRaw, endsAtRaw string) (time.Time, time.Time, error) {
	start, err := time.Parse("2006-01-02", strings.TrimSpace(startsAtRaw))
	if err != nil {
		return time.Time{}, time.Time{}, httpx.Validation("Format starts_at harus YYYY-MM-DD.")
	}
	end, err := time.Parse("2006-01-02", strings.TrimSpace(endsAtRaw))
	if err != nil {
		return time.Time{}, time.Time{}, httpx.Validation("Format ends_at harus YYYY-MM-DD.")
	}
	if start.After(end) {
		return time.Time{}, time.Time{}, httpx.Validation("starts_at tidak boleh setelah ends_at.")
	}
	return start, end, nil
}

// -- POST /api/announcements --

// CreateInput adalah parameter POST /api/announcements.
type CreateInput struct {
	Title    string
	Body     string
	StartsAt string
	EndsAt   string
}

func (s *Service) Create(ctx context.Context, actorUserID, schoolID int64, in CreateInput) (AnnouncementView, error) {
	if !s.identity.HasPermission(reqctx.Role(ctx), PermAnnouncementManage) {
		return AnnouncementView{}, httpx.ErrForbidden
	}
	title := strings.TrimSpace(in.Title)
	body := strings.TrimSpace(in.Body)
	if title == "" || body == "" {
		return AnnouncementView{}, httpx.Validation("Judul dan isi pengumuman wajib diisi.")
	}
	start, end, err := parseAnnouncementDates(in.StartsAt, in.EndsAt)
	if err != nil {
		return AnnouncementView{}, err
	}

	rec, err := s.repo.Insert(ctx, InsertInput{
		SchoolID: schoolID, Title: title, Body: body, StartsAt: start, EndsAt: end, CreatedBy: actorUserID,
	})
	if err != nil {
		return AnnouncementView{}, err
	}
	s.audit(ctx, schoolID, actorUserID, "announcement.create", "announcement", rec.ID, nil, map[string]any{
		"title": title, "starts_at": start.Format("2006-01-02"), "ends_at": end.Format("2006-01-02"),
	})
	s.emitAnnouncement(schoolID)
	return viewFromRecord(rec), nil
}

// -- PATCH /api/announcements/{id} --

// UpdateInput adalah parameter PATCH /api/announcements/{id}. Semua field
// SELALU diterapkan (replace-all, bukan partial-patch) — pola yang sama
// dengan internal/teaching.PatchInput (keputusan sendiri: cukup sederhana
// utk entitas pengumuman, konsisten dgn presedan modul teaching).
type UpdateInput struct {
	Title    string
	Body     string
	StartsAt string
	EndsAt   string
}

func (s *Service) Update(ctx context.Context, actorUserID, schoolID, id int64, in UpdateInput) (AnnouncementView, error) {
	if !s.identity.HasPermission(reqctx.Role(ctx), PermAnnouncementManage) {
		return AnnouncementView{}, httpx.ErrForbidden
	}
	title := strings.TrimSpace(in.Title)
	body := strings.TrimSpace(in.Body)
	if title == "" || body == "" {
		return AnnouncementView{}, httpx.Validation("Judul dan isi pengumuman wajib diisi.")
	}
	start, end, err := parseAnnouncementDates(in.StartsAt, in.EndsAt)
	if err != nil {
		return AnnouncementView{}, err
	}

	rec, err := s.repo.Update(ctx, schoolID, id, title, body, start, end)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return AnnouncementView{}, httpx.ErrNotFound
		}
		return AnnouncementView{}, err
	}
	s.audit(ctx, schoolID, actorUserID, "announcement.update", "announcement", id, nil, map[string]any{
		"title": title, "starts_at": start.Format("2006-01-02"), "ends_at": end.Format("2006-01-02"),
	})
	s.emitAnnouncement(schoolID)
	return viewFromRecord(rec), nil
}

// -- DELETE /api/announcements/{id} --

func (s *Service) Delete(ctx context.Context, actorUserID, schoolID, id int64) error {
	if !s.identity.HasPermission(reqctx.Role(ctx), PermAnnouncementManage) {
		return httpx.ErrForbidden
	}
	if _, err := s.repo.GetByID(ctx, schoolID, id); err != nil {
		if errors.Is(err, ErrNotFound) {
			return httpx.ErrNotFound
		}
		return err
	}
	if err := s.repo.Delete(ctx, schoolID, id); err != nil {
		if errors.Is(err, ErrNotFound) {
			return httpx.ErrNotFound
		}
		return err
	}
	s.audit(ctx, schoolID, actorUserID, "announcement.delete", "announcement", id, nil, nil)
	s.emitAnnouncement(schoolID)
	return nil
}
