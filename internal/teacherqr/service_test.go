package teacherqr

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/omanjaya/nouschool/internal/platform/clock"
	"github.com/omanjaya/nouschool/internal/platform/httpx"
	"github.com/omanjaya/nouschool/internal/platform/reqctx"
)

// fakeRepo — repository in-memory, tanpa DB (pola sama modul lain).
type fakeRepo struct {
	nextID int64
	rows   map[int64]*fakeRow
}

type fakeRow struct {
	id         int64
	schoolID   int64
	userID     int64
	token      string
	expiresAt  time.Time
	consumedAt *time.Time
}

func newFakeRepo() *fakeRepo { return &fakeRepo{rows: map[int64]*fakeRow{}} }

func (f *fakeRepo) DeleteExpiredForUser(ctx context.Context, schoolID, userID int64, now time.Time) error {
	for id, r := range f.rows {
		if r.schoolID == schoolID && r.userID == userID && !r.expiresAt.After(now) {
			delete(f.rows, id)
		}
	}
	return nil
}

func (f *fakeRepo) CreateToken(ctx context.Context, schoolID, userID int64, token string, expiresAt time.Time) (int64, error) {
	f.nextID++
	f.rows[f.nextID] = &fakeRow{id: f.nextID, schoolID: schoolID, userID: userID, token: token, expiresAt: expiresAt}
	return f.nextID, nil
}

func (f *fakeRepo) ConsumeToken(ctx context.Context, schoolID int64, token string, now time.Time) (int64, bool, error) {
	for _, r := range f.rows {
		if r.schoolID == schoolID && r.token == token {
			if r.consumedAt != nil || !r.expiresAt.After(now) {
				return 0, false, nil
			}
			t := now
			r.consumedAt = &t
			return r.userID, true, nil
		}
	}
	return 0, false, nil
}

func ctxAsGuru(userID int64) context.Context {
	ctx := reqctx.WithUser(context.Background(), userID, RoleGuru, false)
	return reqctx.WithSchool(ctx, reqctx.School{ID: 1, Name: "Sekolah Uji", Slug: "uji", Timezone: "Asia/Jakarta"})
}

func ctxAsRole(role string, userID int64) context.Context {
	ctx := reqctx.WithUser(context.Background(), userID, role, false)
	return reqctx.WithSchool(ctx, reqctx.School{ID: 1, Name: "Sekolah Uji", Slug: "uji", Timezone: "Asia/Jakarta"})
}

func domainStatus(t *testing.T, err error) int {
	t.Helper()
	var de *httpx.Error
	if !errors.As(err, &de) {
		t.Fatalf("expected domain error, got: %v", err)
	}
	return de.Status
}

func TestGenerateToken_OnlyGuru(t *testing.T) {
	svc := newServiceForTest(newFakeRepo(), clock.Fixed{T: time.Date(2026, 8, 16, 7, 0, 0, 0, time.UTC)})
	_, err := svc.GenerateToken(ctxAsRole("siswa", 10), 10, 1)
	if err == nil || domainStatus(t, err) != 403 {
		t.Fatalf("expected forbidden, got %v", err)
	}
	view, err := svc.GenerateToken(ctxAsGuru(5), 5, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(view.Token) != tokenLength {
		t.Fatalf("expected token length %d, got %d", tokenLength, len(view.Token))
	}
}

func TestConsumeToken_ValidThenSingleUse(t *testing.T) {
	now := time.Date(2026, 8, 16, 7, 0, 0, 0, time.UTC)
	repo := newFakeRepo()
	svc := newServiceForTest(repo, clock.Fixed{T: now})

	view, err := svc.GenerateToken(ctxAsGuru(7), 7, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	userID, err := svc.ConsumeToken(context.Background(), 1, view.Token)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if userID != 7 {
		t.Fatalf("expected userID 7, got %d", userID)
	}

	// Simulasi race: pemanggilan KEDUA pada token YANG SAMA harus gagal
	// (repository FAKE meniru atomicity UPDATE ... WHERE consumed_at IS NULL
	// yang sesungguhnya dijamin Postgres di Repository asli).
	if _, err := svc.ConsumeToken(context.Background(), 1, view.Token); err == nil {
		t.Fatal("expected error konsumsi kedua, got nil")
	} else if domainStatus(t, err) != 410 {
		t.Fatalf("expected 410, got %v", err)
	}
}

func TestConsumeToken_Expired(t *testing.T) {
	start := time.Date(2026, 8, 16, 7, 0, 0, 0, time.UTC)
	repo := newFakeRepo()
	svc := newServiceForTest(repo, clock.Fixed{T: start})

	view, err := svc.GenerateToken(ctxAsGuru(7), 7, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	svc.clock = clock.Fixed{T: start.Add(tokenTTL + time.Second)}
	if _, err := svc.ConsumeToken(context.Background(), 1, view.Token); err == nil {
		t.Fatal("expected expired error, got nil")
	} else if domainStatus(t, err) != 410 {
		t.Fatalf("expected 410, got %v", err)
	}
}

func TestConsumeToken_AcceptsWithOrWithoutPrefix(t *testing.T) {
	now := time.Date(2026, 8, 16, 7, 0, 0, 0, time.UTC)
	repo := newFakeRepo()
	svc := newServiceForTest(repo, clock.Fixed{T: now})

	view, err := svc.GenerateToken(ctxAsGuru(7), 7, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	userID, err := svc.ConsumeToken(context.Background(), 1, tokenPrefix+view.Token)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if userID != 7 {
		t.Fatalf("expected userID 7, got %d", userID)
	}
}

func TestConsumeToken_Unknown(t *testing.T) {
	svc := newServiceForTest(newFakeRepo(), clock.Fixed{T: time.Now()})
	if _, err := svc.ConsumeToken(context.Background(), 1, "token-tidak-dikenal"); err == nil {
		t.Fatal("expected error, got nil")
	} else if domainStatus(t, err) != 410 {
		t.Fatalf("expected 410, got %v", err)
	}
}
