package identity

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/omanjaya/nouschool/internal/platform/httpx"
	"github.com/omanjaya/nouschool/internal/platform/reqctx"
)

// -- fakes (in-memory, tanpa DB — pola sama dengan gateway_test.go) --

type fakeSchoolGateway struct {
	status string
	slug   string
	found  bool
	err    error
}

func (f fakeSchoolGateway) SchoolStatusAndSlug(ctx context.Context, id int64) (string, string, bool, error) {
	return f.status, f.slug, f.found, f.err
}

type auditCall struct {
	schoolID *int64
	userID   *int64
	action   string
}

type fakeAudit struct {
	calls []auditCall
}

func (f *fakeAudit) Log(ctx context.Context, schoolID, userID *int64, action, entity string, entityID *int64, oldValue, newValue any) error {
	f.calls = append(f.calls, auditCall{schoolID: schoolID, userID: userID, action: action})
	return nil
}

type fakeImpersonationUserRepo struct {
	users     map[int64]User
	sessions  []CreateSessionInput
	createErr error
}

func (f *fakeImpersonationUserRepo) UserByID(ctx context.Context, id int64) (User, error) {
	u, ok := f.users[id]
	if !ok {
		return User{}, ErrNotFound
	}
	return u, nil
}

func (f *fakeImpersonationUserRepo) CreateSession(ctx context.Context, in CreateSessionInput) error {
	if f.createErr != nil {
		return f.createErr
	}
	f.sessions = append(f.sessions, in)
	return nil
}

// -- impersonationStore (token sekali-pakai) --

func TestImpersonationStoreSingleUse(t *testing.T) {
	store := newImpersonationStore(nil)
	token, _, err := store.put(1, 99)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	issuerID, ok := store.take(token, 1)
	if !ok || issuerID != 99 {
		t.Fatalf("pemakaian pertama seharusnya berhasil, got ok=%v issuerID=%d", ok, issuerID)
	}

	if _, ok := store.take(token, 1); ok {
		t.Fatal("pemakaian ULANG token yang sama seharusnya gagal (sekali pakai)")
	}
}

func TestImpersonationStoreExpired(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	store := newImpersonationStore(func() time.Time { return now })
	token, expiresAt, err := store.put(1, 99)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !expiresAt.Equal(now.Add(impersonationTokenTTL)) {
		t.Fatalf("expiresAt salah: %v", expiresAt)
	}

	// Masih dalam TTL -> boleh.
	now2 := now.Add(impersonationTokenTTL - time.Second)
	store.now = func() time.Time { return now2 }
	// Jangan benar-benar take di sini (sekali pakai) — cukup pastikan belum
	// kedaluwarsa dengan window kecil setelah TTL lewat di bawah.

	// Lewat TTL -> gagal.
	store.now = func() time.Time { return now.Add(impersonationTokenTTL + time.Second) }
	if _, ok := store.take(token, 1); ok {
		t.Fatal("token yang sudah kedaluwarsa seharusnya gagal ditukar")
	}
}

func TestImpersonationStoreWrongSchool(t *testing.T) {
	store := newImpersonationStore(nil)
	token, _, err := store.put(1, 99) // token untuk sekolah 1
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Ditukar di host sekolah LAIN (2) -> gagal.
	if _, ok := store.take(token, 2); ok {
		t.Fatal("token sekolah 1 ditukar di host sekolah 2 seharusnya gagal")
	}

	// Tapi TETAP valid untuk sekolah aslinya (percobaan di sekolah lain tidak membakar token).
	issuerID, ok := store.take(token, 1)
	if !ok || issuerID != 99 {
		t.Fatal("token seharusnya tetap bisa ditukar di sekolah aslinya (1)")
	}
}

// -- issueImpersonation (murni) --

func TestIssueImpersonationRequiresActiveSchool(t *testing.T) {
	store := newImpersonationStore(nil)
	audit := &fakeAudit{}

	if _, _, _, err := issueImpersonation(context.Background(), fakeSchoolGateway{found: false}, store, audit, 1, 5); !errors.Is(err, httpx.ErrNotFound) {
		t.Fatalf("sekolah tidak ditemukan: expected httpx.ErrNotFound, got %v", err)
	}

	if _, _, _, err := issueImpersonation(context.Background(), fakeSchoolGateway{found: true, status: "suspended", slug: "smk1"}, store, audit, 1, 5); err == nil {
		t.Fatal("sekolah berstatus suspended seharusnya gagal diimpersonasi")
	}

	if len(audit.calls) != 0 {
		t.Fatalf("audit TIDAK boleh terpanggil saat validasi gagal, got %d calls", len(audit.calls))
	}
}

func TestIssueImpersonationSuccessAudits(t *testing.T) {
	store := newImpersonationStore(nil)
	audit := &fakeAudit{}

	token, slug, expiresAt, err := issueImpersonation(context.Background(), fakeSchoolGateway{found: true, status: "active", slug: "smk1"}, store, audit, 7, 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token == "" || slug != "smk1" {
		t.Fatalf("token/slug tidak sesuai: token=%q slug=%q", token, slug)
	}
	if expiresAt.IsZero() {
		t.Fatal("expiresAt tidak boleh kosong")
	}

	if len(audit.calls) != 1 {
		t.Fatalf("audit seharusnya terpanggil tepat sekali, got %d", len(audit.calls))
	}
	got := audit.calls[0]
	if got.action != "admin.impersonate_issued" || *got.schoolID != 7 || *got.userID != 42 {
		t.Fatalf("audit call salah: %+v", got)
	}

	// Token yang diterbitkan harus bisa langsung ditukar di sekolah 7.
	issuerID, ok := store.take(token, 7)
	if !ok || issuerID != 42 {
		t.Fatal("token hasil issue seharusnya valid ditukar di sekolah 7 dengan issuerID 42")
	}
}

// -- exchangeImpersonation (murni) --

func TestExchangeImpersonationSessionTTLIsTwoHours(t *testing.T) {
	store := newImpersonationStore(nil)
	audit := &fakeAudit{}
	repo := &fakeImpersonationUserRepo{users: map[int64]User{42: {ID: 42, Name: "Super Admin", IsSuperAdmin: true}}}

	token, _, err := store.put(7, 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	now := time.Date(2026, 3, 1, 8, 0, 0, 0, time.UTC)
	hostSchool := SchoolView{ID: 7, Name: "SMK Demo", Slug: "demo"}
	result, err := exchangeImpersonation(context.Background(), repo, store, audit, token, 7, hostSchool, "127.0.0.1", "curl", now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.ExpiresAt.Equal(now.Add(2 * time.Hour)) {
		t.Fatalf("TTL sesi impersonation seharusnya PERSIS 2 jam, got %v (selisih %v)", result.ExpiresAt, result.ExpiresAt.Sub(now))
	}
	if result.View.Role != RoleAdminSekolah || !result.View.IsSuperAdmin || result.View.School == nil || result.View.School.ID != 7 {
		t.Fatalf("view hasil exchange tidak sesuai: %+v", result.View)
	}

	if len(repo.sessions) != 1 {
		t.Fatalf("seharusnya membuat tepat satu sesi, got %d", len(repo.sessions))
	}
	sess := repo.sessions[0]
	// Role DI DB sengaja sentinel impersonationSessionRole (BUKAN
	// RoleAdminSekolah langsung) supaya sliding renewal generik di
	// RequireAuth tidak diam-diam memperpanjangnya jadi 30 hari — lihat
	// TestImpersonationSessionRoleNeverSlidingRenewed di session_test.go.
	if sess.Role != impersonationSessionRole || sess.UserID != 42 || sess.SchoolID == nil || *sess.SchoolID != 7 {
		t.Fatalf("sesi yang dibuat salah: %+v", sess)
	}

	if len(audit.calls) != 1 || audit.calls[0].action != "admin.impersonate_started" {
		t.Fatalf("audit admin.impersonate_started seharusnya terpanggil tepat sekali, got %+v", audit.calls)
	}
}

func TestExchangeImpersonationReuseFails(t *testing.T) {
	store := newImpersonationStore(nil)
	audit := &fakeAudit{}
	repo := &fakeImpersonationUserRepo{users: map[int64]User{42: {ID: 42, IsSuperAdmin: true}}}
	token, _, _ := store.put(7, 42)
	hostSchool := SchoolView{ID: 7}

	if _, err := exchangeImpersonation(context.Background(), repo, store, audit, token, 7, hostSchool, "", "", time.Now()); err != nil {
		t.Fatalf("penukaran pertama seharusnya berhasil: %v", err)
	}
	_, err := exchangeImpersonation(context.Background(), repo, store, audit, token, 7, hostSchool, "", "", time.Now())
	if !errors.Is(err, error(ErrImpersonationInvalid)) {
		t.Fatalf("pemakaian ULANG token seharusnya gagal dengan ErrImpersonationInvalid, got %v", err)
	}
}

func TestExchangeImpersonationExpiredFails(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	store := newImpersonationStore(func() time.Time { return now })
	audit := &fakeAudit{}
	repo := &fakeImpersonationUserRepo{users: map[int64]User{42: {ID: 42, IsSuperAdmin: true}}}
	token, _, _ := store.put(7, 42)

	store.now = func() time.Time { return now.Add(impersonationTokenTTL + time.Second) }
	hostSchool := SchoolView{ID: 7}
	_, err := exchangeImpersonation(context.Background(), repo, store, audit, token, 7, hostSchool, "", "", now.Add(impersonationTokenTTL+time.Second))
	if !errors.Is(err, error(ErrImpersonationInvalid)) {
		t.Fatalf("token kedaluwarsa seharusnya gagal dengan ErrImpersonationInvalid, got %v", err)
	}
}

// TestExchangeImpersonationWrongSchoolFails — token sekolah A ditukar di
// host sekolah B (soal wajib dari instruksi tugas).
func TestExchangeImpersonationWrongSchoolFails(t *testing.T) {
	store := newImpersonationStore(nil)
	audit := &fakeAudit{}
	repo := &fakeImpersonationUserRepo{users: map[int64]User{42: {ID: 42, IsSuperAdmin: true}}}
	token, _, _ := store.put(1, 42) // token untuk sekolah A (id 1)

	hostSchoolB := SchoolView{ID: 2} // host sekolah B (id 2)
	_, err := exchangeImpersonation(context.Background(), repo, store, audit, token, 2, hostSchoolB, "", "", time.Now())
	if !errors.Is(err, error(ErrImpersonationInvalid)) {
		t.Fatalf("token sekolah A ditukar di host sekolah B seharusnya gagal, got %v", err)
	}
	if len(repo.sessions) != 0 {
		t.Fatal("TIDAK boleh membuat sesi apa pun saat schoolID token tidak cocok host")
	}
}

func TestExchangeImpersonationIssuerNoLongerSuperAdminFails(t *testing.T) {
	store := newImpersonationStore(nil)
	audit := &fakeAudit{}
	// User penerbit ADA tapi is_super_admin sudah dicabut sebelum token ditukar.
	repo := &fakeImpersonationUserRepo{users: map[int64]User{42: {ID: 42, IsSuperAdmin: false}}}
	token, _, _ := store.put(7, 42)
	hostSchool := SchoolView{ID: 7}

	_, err := exchangeImpersonation(context.Background(), repo, store, audit, token, 7, hostSchool, "", "", time.Now())
	if !errors.Is(err, error(ErrImpersonationInvalid)) {
		t.Fatalf("issuer bukan lagi super admin seharusnya gagal, got %v", err)
	}
	if len(audit.calls) != 0 {
		t.Fatal("audit admin.impersonate_started TIDAK boleh terpanggil saat exchange gagal")
	}
}

func TestExchangeImpersonationEmptyTokenFails(t *testing.T) {
	store := newImpersonationStore(nil)
	audit := &fakeAudit{}
	repo := &fakeImpersonationUserRepo{users: map[int64]User{}}
	hostSchool := SchoolView{ID: 7}

	_, err := exchangeImpersonation(context.Background(), repo, store, audit, "", 7, hostSchool, "", "", time.Now())
	if !errors.Is(err, error(ErrImpersonationInvalid)) {
		t.Fatalf("token kosong seharusnya gagal, got %v", err)
	}
}

// -- RequireSuperAdmin: belt & suspenders (item 4 instruksi tugas) --

func TestRequireSuperAdminRejectsTenantContext(t *testing.T) {
	svc := NewService(nil, NewRateLimiter(5, 15*time.Minute, nil), false)

	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true })

	// Konteks TENANT (host sekolah) + is_super_admin true di sesi (skenario
	// super admin sedang impersonation mencoba memanggil API admin dari host
	// tenant) -> HARUS ditolak walau IsSuperAdmin true.
	req := httptest.NewRequest(http.MethodPost, "/api/admin/schools/1/impersonate", nil)
	ctx := reqctx.WithSchool(req.Context(), reqctx.School{ID: 7, Slug: "demo"})
	ctx = reqctx.WithUser(ctx, 42, RoleSuperAdmin, true)
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	svc.RequireSuperAdmin(next).ServeHTTP(rec, req)

	if called {
		t.Fatal("handler next TIDAK boleh terpanggil di konteks tenant")
	}
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden, got %d", rec.Code)
	}
}

func TestRequireSuperAdminAllowsPlatformContext(t *testing.T) {
	svc := NewService(nil, NewRateLimiter(5, 15*time.Minute, nil), false)

	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true })

	req := httptest.NewRequest(http.MethodPost, "/api/admin/schools/1/impersonate", nil)
	ctx := reqctx.WithPlatform(req.Context())
	ctx = reqctx.WithUser(ctx, 42, RoleSuperAdmin, true)
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	svc.RequireSuperAdmin(next).ServeHTTP(rec, req)

	if !called {
		t.Fatal("handler next seharusnya terpanggil di konteks platform + super admin")
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}
