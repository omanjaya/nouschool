package announcement

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/omanjaya/nouschool/internal/platform/clock"
	"github.com/omanjaya/nouschool/internal/platform/httpx"
	"github.com/omanjaya/nouschool/internal/platform/reqctx"
)

// -- fakeAnnouncementRepo: implementasi announcementRepository in-memory
// (tanpa DB) — pola yang sama dengan internal/teaching & internal/attendance
// service_test.go.

type fakeAnnouncementRepo struct {
	rows   map[int64]Record
	nextID int64
}

func newFakeAnnouncementRepo() *fakeAnnouncementRepo {
	return &fakeAnnouncementRepo{rows: map[int64]Record{}}
}

func sameOrBetween(d, start, end time.Time) bool {
	return !d.Before(start) && !d.After(end)
}

func (f *fakeAnnouncementRepo) Insert(ctx context.Context, in InsertInput) (Record, error) {
	f.nextID++
	rec := Record{
		ID: f.nextID, SchoolID: in.SchoolID, Title: in.Title, Body: in.Body,
		StartsAt: in.StartsAt, EndsAt: in.EndsAt, CreatedBy: in.CreatedBy, CreatedAt: time.Now(),
	}
	f.rows[rec.ID] = rec
	return rec, nil
}

func (f *fakeAnnouncementRepo) GetByID(ctx context.Context, schoolID, id int64) (Record, error) {
	rec, ok := f.rows[id]
	if !ok || rec.SchoolID != schoolID {
		return Record{}, ErrNotFound
	}
	return rec, nil
}

func (f *fakeAnnouncementRepo) ListAll(ctx context.Context, schoolID int64) ([]Record, error) {
	var out []Record
	for _, r := range f.rows {
		if r.SchoolID == schoolID {
			out = append(out, r)
		}
	}
	return out, nil
}

func (f *fakeAnnouncementRepo) ListActive(ctx context.Context, schoolID int64, date time.Time) ([]Record, error) {
	var out []Record
	for _, r := range f.rows {
		if r.SchoolID == schoolID && sameOrBetween(date, r.StartsAt, r.EndsAt) {
			out = append(out, r)
		}
	}
	return out, nil
}

func (f *fakeAnnouncementRepo) Update(ctx context.Context, schoolID, id int64, title, body string, startsAt, endsAt time.Time) (Record, error) {
	rec, err := f.GetByID(ctx, schoolID, id)
	if err != nil {
		return Record{}, err
	}
	rec.Title, rec.Body, rec.StartsAt, rec.EndsAt = title, body, startsAt, endsAt
	f.rows[id] = rec
	return rec, nil
}

func (f *fakeAnnouncementRepo) Delete(ctx context.Context, schoolID, id int64) error {
	rec, err := f.GetByID(ctx, schoolID, id)
	if err != nil {
		return err
	}
	delete(f.rows, rec.ID)
	return nil
}

// -- fake identity gateway --

type fakeIdentity struct{ perms map[string]bool }

func newFakeIdentity() *fakeIdentity {
	return &fakeIdentity{perms: map[string]bool{"admin_sekolah": true, "kepala_sekolah": true}}
}

func (f *fakeIdentity) HasPermission(role, perm string) bool {
	if perm != PermAnnouncementManage {
		return false
	}
	return f.perms[role]
}

func (f *fakeIdentity) Log(ctx context.Context, schoolID, userID *int64, action, entity string, entityID *int64, oldValue, newValue any) error {
	return nil
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

// -- List(active): tanggal lokal sekolah, bukan UTC --

func TestList_ActiveByLocalDate(t *testing.T) {
	repo := newFakeAnnouncementRepo()
	// pengumuman aktif 10-20 Agustus 2026.
	repo.rows[1] = Record{
		ID: 1, SchoolID: 1, Title: "Libur", Body: "Sekolah libur",
		StartsAt: time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC),
		EndsAt:   time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC),
	}
	repo.nextID = 1

	// 2026-08-15 23:30 UTC = 2026-08-16 06:30 WIB -> tanggal lokal sudah lewat
	// dari rentang aktif (16 Agustus > 20? tidak, masih dalam rentang) — pakai
	// kasus yang lebih tajam: 2026-08-20 23:30 UTC = 2026-08-21 06:30 WIB,
	// tanggal lokal SUDAH lewat rentang aktif (21 > 20) walau UTC (20) masih
	// dalam rentang — inilah yang diuji: harus pakai tanggal LOKAL, bukan UTC.
	fixed := clock.Fixed{T: time.Date(2026, 8, 20, 23, 30, 0, 0, time.UTC)}
	svc := newServiceForTest(repo, newFakeIdentity(), fixed)
	ctx := ctxAs("display", 1, "Asia/Jakarta")

	items, err := svc.List(ctx, 1, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected 0 pengumuman aktif (tanggal lokal sudah lewat), got %d", len(items))
	}

	// Sebaliknya: 2026-08-09 20:00 UTC = 2026-08-10 03:00 WIB -> tanggal lokal
	// SUDAH masuk rentang aktif (10 Agustus) walau UTC (9) belum.
	fixed2 := clock.Fixed{T: time.Date(2026, 8, 9, 20, 0, 0, 0, time.UTC)}
	svc2 := newServiceForTest(repo, newFakeIdentity(), fixed2)
	items2, err := svc2.List(ctx, 1, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items2) != 1 {
		t.Fatalf("expected 1 pengumuman aktif (tanggal lokal sudah masuk rentang), got %d", len(items2))
	}

	// Pertengahan rentang -> aktif dengan timezone WIT (UTC+9) juga diuji.
	fixed3 := clock.Fixed{T: time.Date(2026, 8, 15, 12, 0, 0, 0, time.UTC)}
	svc3 := newServiceForTest(repo, newFakeIdentity(), fixed3)
	ctxWIT := ctxAs("display", 1, "Asia/Jayapura")
	items3, err := svc3.List(ctxWIT, 1, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items3) != 1 {
		t.Fatalf("expected 1 pengumuman aktif (WIT), got %d", len(items3))
	}
}

func TestList_ActiveOutsideRange(t *testing.T) {
	repo := newFakeAnnouncementRepo()
	repo.rows[1] = Record{
		ID: 1, SchoolID: 1, Title: "Ujian", Body: "Jadwal ujian",
		StartsAt: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
		EndsAt:   time.Date(2026, 9, 5, 0, 0, 0, 0, time.UTC),
	}
	repo.nextID = 1
	fixed := clock.Fixed{T: time.Date(2026, 8, 15, 3, 0, 0, 0, time.UTC)} // sebelum rentang
	svc := newServiceForTest(repo, newFakeIdentity(), fixed)
	ctx := ctxAs("guru", 1, "Asia/Jakarta")

	items, err := svc.List(ctx, 1, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected 0 pengumuman aktif (di luar rentang), got %d", len(items))
	}
}

// -- List tanpa active: butuh perm announcement:manage --

func TestList_AllRequiresPermission(t *testing.T) {
	repo := newFakeAnnouncementRepo()
	fixed := clock.Fixed{T: time.Date(2026, 8, 15, 3, 0, 0, 0, time.UTC)}
	svc := newServiceForTest(repo, newFakeIdentity(), fixed)

	t.Run("admin OK", func(t *testing.T) {
		ctx := ctxAs("admin_sekolah", 1, "Asia/Jakarta")
		if _, err := svc.List(ctx, 1, false); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("display FORBIDDEN", func(t *testing.T) {
		ctx := ctxAs("display", 1, "Asia/Jakarta")
		_, err := svc.List(ctx, 1, false)
		if status := domainStatus(t, err); status != 403 {
			t.Fatalf("expected 403, got %d", status)
		}
	})

	t.Run("guru FORBIDDEN", func(t *testing.T) {
		ctx := ctxAs("guru", 1, "Asia/Jakarta")
		_, err := svc.List(ctx, 1, false)
		if status := domainStatus(t, err); status != 403 {
			t.Fatalf("expected 403, got %d", status)
		}
	})
}

// -- Create: validasi & permission --

func TestCreate_ValidationAndPermission(t *testing.T) {
	repo := newFakeAnnouncementRepo()
	fixed := clock.Fixed{T: time.Date(2026, 8, 15, 3, 0, 0, 0, time.UTC)}
	svc := newServiceForTest(repo, newFakeIdentity(), fixed)

	t.Run("kepsek OK", func(t *testing.T) {
		ctx := ctxAs("kepala_sekolah", 1, "Asia/Jakarta")
		view, err := svc.Create(ctx, 1, 1, CreateInput{Title: "Rapat", Body: "Rapat guru", StartsAt: "2026-08-15", EndsAt: "2026-08-16"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if view.Title != "Rapat" {
			t.Fatalf("expected title Rapat, got %s", view.Title)
		}
	})

	t.Run("guru FORBIDDEN", func(t *testing.T) {
		ctx := ctxAs("guru", 1, "Asia/Jakarta")
		_, err := svc.Create(ctx, 1, 1, CreateInput{Title: "X", Body: "Y", StartsAt: "2026-08-15", EndsAt: "2026-08-16"})
		if status := domainStatus(t, err); status != 403 {
			t.Fatalf("expected 403, got %d", status)
		}
	})

	t.Run("title kosong ditolak", func(t *testing.T) {
		ctx := ctxAs("admin_sekolah", 1, "Asia/Jakarta")
		_, err := svc.Create(ctx, 1, 1, CreateInput{Title: "  ", Body: "Y", StartsAt: "2026-08-15", EndsAt: "2026-08-16"})
		if status := domainStatus(t, err); status != 422 {
			t.Fatalf("expected 422, got %d", status)
		}
	})

	t.Run("body kosong ditolak", func(t *testing.T) {
		ctx := ctxAs("admin_sekolah", 1, "Asia/Jakarta")
		_, err := svc.Create(ctx, 1, 1, CreateInput{Title: "X", Body: "  ", StartsAt: "2026-08-15", EndsAt: "2026-08-16"})
		if status := domainStatus(t, err); status != 422 {
			t.Fatalf("expected 422, got %d", status)
		}
	})

	t.Run("starts_at setelah ends_at ditolak", func(t *testing.T) {
		ctx := ctxAs("admin_sekolah", 1, "Asia/Jakarta")
		_, err := svc.Create(ctx, 1, 1, CreateInput{Title: "X", Body: "Y", StartsAt: "2026-08-20", EndsAt: "2026-08-15"})
		if status := domainStatus(t, err); status != 422 {
			t.Fatalf("expected 422, got %d", status)
		}
	})

	t.Run("format tanggal salah ditolak", func(t *testing.T) {
		ctx := ctxAs("admin_sekolah", 1, "Asia/Jakarta")
		_, err := svc.Create(ctx, 1, 1, CreateInput{Title: "X", Body: "Y", StartsAt: "15-08-2026", EndsAt: "2026-08-16"})
		if status := domainStatus(t, err); status != 422 {
			t.Fatalf("expected 422, got %d", status)
		}
	})

	t.Run("starts_at sama dengan ends_at diterima", func(t *testing.T) {
		ctx := ctxAs("admin_sekolah", 1, "Asia/Jakarta")
		if _, err := svc.Create(ctx, 1, 1, CreateInput{Title: "X", Body: "Y", StartsAt: "2026-08-15", EndsAt: "2026-08-15"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

// -- Update & Delete --

func TestUpdateDelete(t *testing.T) {
	repo := newFakeAnnouncementRepo()
	fixed := clock.Fixed{T: time.Date(2026, 8, 15, 3, 0, 0, 0, time.UTC)}
	svc := newServiceForTest(repo, newFakeIdentity(), fixed)
	adminCtx := ctxAs("admin_sekolah", 1, "Asia/Jakarta")

	created, err := svc.Create(adminCtx, 1, 1, CreateInput{Title: "A", Body: "B", StartsAt: "2026-08-15", EndsAt: "2026-08-16"})
	if err != nil {
		t.Fatalf("unexpected error create: %v", err)
	}

	t.Run("update guru forbidden", func(t *testing.T) {
		guruCtx := ctxAs("guru", 1, "Asia/Jakarta")
		_, err := svc.Update(guruCtx, 1, 1, created.ID, UpdateInput{Title: "C", Body: "D", StartsAt: "2026-08-15", EndsAt: "2026-08-16"})
		if status := domainStatus(t, err); status != 403 {
			t.Fatalf("expected 403, got %d", status)
		}
	})

	t.Run("update OK", func(t *testing.T) {
		updated, err := svc.Update(adminCtx, 1, 1, created.ID, UpdateInput{Title: "C", Body: "D", StartsAt: "2026-08-15", EndsAt: "2026-08-17"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if updated.Title != "C" || updated.Body != "D" {
			t.Fatalf("expected title/body diperbarui, got %+v", updated)
		}
	})

	t.Run("update not found", func(t *testing.T) {
		_, err := svc.Update(adminCtx, 1, 1, 999, UpdateInput{Title: "C", Body: "D", StartsAt: "2026-08-15", EndsAt: "2026-08-16"})
		if status := domainStatus(t, err); status != 404 {
			t.Fatalf("expected 404, got %d", status)
		}
	})

	t.Run("delete guru forbidden", func(t *testing.T) {
		guruCtx := ctxAs("guru", 1, "Asia/Jakarta")
		err := svc.Delete(guruCtx, 1, 1, created.ID)
		if status := domainStatus(t, err); status != 403 {
			t.Fatalf("expected 403, got %d", status)
		}
	})

	t.Run("delete OK", func(t *testing.T) {
		if err := svc.Delete(adminCtx, 1, 1, created.ID); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if _, err := repo.GetByID(context.Background(), 1, created.ID); !errors.Is(err, ErrNotFound) {
			t.Fatalf("expected pengumuman terhapus, got err=%v", err)
		}
	})

	t.Run("delete not found", func(t *testing.T) {
		err := svc.Delete(adminCtx, 1, 1, 999)
		if status := domainStatus(t, err); status != 404 {
			t.Fatalf("expected 404, got %d", status)
		}
	})
}

// -- ActiveOn (dipakai dashboard) --

func TestActiveOn(t *testing.T) {
	repo := newFakeAnnouncementRepo()
	repo.rows[1] = Record{
		ID: 1, SchoolID: 1, Title: "Libur", Body: "Sekolah libur",
		StartsAt: time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC),
		EndsAt:   time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC),
	}
	repo.nextID = 1
	svc := newServiceForTest(repo, newFakeIdentity(), clock.System{})
	ctx := context.Background()

	items, err := svc.ActiveOn(ctx, 1, "2026-08-15")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 || items[0].ID != 1 || items[0].Title != "Libur" {
		t.Fatalf("expected 1 item Libur, got %+v", items)
	}

	items2, err := svc.ActiveOn(ctx, 1, "2026-09-01")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items2) != 0 {
		t.Fatalf("expected 0 item di luar rentang, got %d", len(items2))
	}

	if _, err := svc.ActiveOn(ctx, 1, "tanggal-salah"); err == nil {
		t.Fatal("expected error format tanggal salah")
	}
}
