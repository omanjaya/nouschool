package identity

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/omanjaya/nouschool/internal/platform/httpx"
	"github.com/omanjaya/nouschool/internal/platform/reqctx"
)

// Fase 13 — Impersonation "Masuk sebagai Sekolah Ini" (docs/11-superadmin.md
// "Support"). Alur: super admin (host platform) minta token sekali-pakai →
// buka tab domain sekolah → tukar token jadi sesi admin_sekolah ATAS NAMA
// super admin di domain itu, tercatat audit. Cookie sesi hasil tukar
// host-only (domain sekolah) — TIDAK mengganggu sesi platform di tab lain.

const (
	// impersonationTokenTTL — token sekali-pakai hanya menjembatani loncat
	// dari tab platform ke tab domain sekolah, jadi sengaja sangat pendek.
	impersonationTokenTTL = 2 * time.Minute

	// impersonationSessionTTL — TTL KHUSUS sesi hasil tukar token, BUKAN
	// sessionTTLForRole biasa (yang 30 hari untuk admin_sekolah) — super
	// admin sedang "meminjam" identitas admin sekolah lain untuk mendukung,
	// bukan bekerja harian, jadi sesi ini sengaja pendek (2 jam).
	impersonationSessionTTL = 2 * time.Hour

	// impersonationSessionRole adalah nilai sessions.role yang DISIMPAN DI DB
	// untuk sesi hasil ExchangeImpersonation — SENGAJA BUKAN RoleAdminSekolah
	// langsung, supaya sliding renewal umum di RequireAuth (middleware.go)
	// TIDAK diam-diam memperpanjang sesi ini jadi 30 hari (bug nyata yang
	// kejadian saat verifikasi e2e: renewal generik hanya melihat sess.Role,
	// dan "admin_sekolah" biasa selalu diperpanjang). RequireAuth
	// menerjemahkan sentinel ini KEMBALI ke RoleAdminSekolah untuk
	// reqctx.Role (supaya RequirePerm/HasPermission tetap jalan normal) —
	// lihat middleware.go.
	impersonationSessionRole = "admin_sekolah:impersonating"
)

// ErrImpersonationInvalid — kegagalan APA PUN saat menukar token
// impersonation (tidak dikenal, sudah dipakai, kedaluwarsa, salah sekolah,
// atau user penerbit bukan lagi super admin). Pesan generik SATU macam
// disengaja — jangan bocorkan detail ke klien (lihat instruksi tugas).
var ErrImpersonationInvalid = &httpx.Error{
	Status:  http.StatusGone,
	Code:    "impersonation_invalid",
	Message: "Tautan support tidak berlaku. Minta tautan baru dari panel admin.",
}

// errSchoolGatewayMissing — SetSchoolGateway belum dipanggil main.go; ini
// bug wiring, bukan kesalahan pengguna, jadi httpx.WriteError akan
// menerjemahkannya jadi 500 generik (tidak bocor ke klien).
var errSchoolGatewayMissing = errors.New("identity: SchoolGateway belum di-set (lihat SetSchoolGateway)")

type impersonationEntry struct {
	schoolID     int64
	issuerUserID int64
	expiresAt    time.Time
	used         bool
}

// impersonationStore adalah penyimpanan token sekali-pakai in-memory (map +
// mutex, key token acak, TTL pendek) — pola sama dengan
// internal/student.ImportStore: sengaja in-memory (bukan DB) karena token
// hanya menjembatani dua tab browser dalam hitungan menit, bukan state yang
// perlu tahan restart server.
type impersonationStore struct {
	mu    sync.Mutex
	items map[string]*impersonationEntry
	now   func() time.Time
}

func newImpersonationStore(now func() time.Time) *impersonationStore {
	if now == nil {
		now = time.Now
	}
	return &impersonationStore{items: make(map[string]*impersonationEntry), now: now}
}

func randomImpersonationToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// prune membuang entri kedaluwarsa & sudah dipakai. Dipanggil dengan mu terkunci.
func (st *impersonationStore) prune() {
	now := st.now()
	for token, e := range st.items {
		if e.used || now.After(e.expiresAt) {
			delete(st.items, token)
		}
	}
}

// put membuat token baru untuk (schoolID, issuerUserID).
func (st *impersonationStore) put(schoolID, issuerUserID int64) (token string, expiresAt time.Time, err error) {
	token, err = randomImpersonationToken()
	if err != nil {
		return "", time.Time{}, err
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	st.prune()
	expiresAt = st.now().Add(impersonationTokenTTL)
	st.items[token] = &impersonationEntry{schoolID: schoolID, issuerUserID: issuerUserID, expiresAt: expiresAt}
	return token, expiresAt, nil
}

// take memvalidasi token & menandai used SECARA ATOMIK (satu critical
// section) bila SEMUA syarat lolos: token dikenal, belum dipakai, belum
// kedaluwarsa, dan schoolID cocok dengan host tenant saat ini (hostSchoolID).
// Kegagalan APA PUN (termasuk token milik sekolah lain) mengembalikan
// ok=false tanpa membedakan alasan — pemanggil menerjemahkan ke SATU pesan
// generik (ErrImpersonationInvalid), jangan bocorkan detail.
func (st *impersonationStore) take(token string, hostSchoolID int64) (issuerUserID int64, ok bool) {
	st.mu.Lock()
	defer st.mu.Unlock()
	e, found := st.items[token]
	if !found || e.used || st.now().After(e.expiresAt) || e.schoolID != hostSchoolID {
		return 0, false
	}
	e.used = true
	return e.issuerUserID, true
}

// -- consumer-side interface kecil untuk memisahkan logika murni (testable
// tanpa DB, pola sama dengan gateway.go generateInvitationCode/
// invitationCodeRepo) dari method Service (yang memakai *Repository nyata). --

// impersonationAuditLogger adalah kontrak minimal audit yang dibutuhkan
// issueImpersonation/exchangeImpersonation — dipenuhi *Service secara
// struktural lewat method Log (audit.go), atau fake di impersonation_test.go.
type impersonationAuditLogger interface {
	Log(ctx context.Context, schoolID, userID *int64, action, entity string, entityID *int64, oldValue, newValue any) error
}

// impersonationUserRepo adalah kontrak minimal repo yang dibutuhkan
// exchangeImpersonation — dipenuhi *Repository secara struktural.
type impersonationUserRepo interface {
	UserByID(ctx context.Context, id int64) (User, error)
	CreateSession(ctx context.Context, in CreateSessionInput) error
}

// issueImpersonation implementasi murni (testable) dari
// Service.IssueImpersonation: validasi sekolah ada & aktif, buat token,
// audit "admin.impersonate_issued".
func issueImpersonation(ctx context.Context, schools SchoolGateway, store *impersonationStore, audit impersonationAuditLogger, schoolID, issuerUserID int64) (token, slug string, expiresAt time.Time, err error) {
	status, slug, found, err := schools.SchoolStatusAndSlug(ctx, schoolID)
	if err != nil {
		return "", "", time.Time{}, err
	}
	if !found {
		return "", "", time.Time{}, httpx.ErrNotFound
	}
	if status != "active" {
		return "", "", time.Time{}, httpx.Validation("Sekolah harus berstatus aktif untuk bisa diimpersonasi.")
	}

	token, expiresAt, err = store.put(schoolID, issuerUserID)
	if err != nil {
		return "", "", time.Time{}, err
	}

	if audit != nil {
		sid, uid := schoolID, issuerUserID
		_ = audit.Log(ctx, &sid, &uid, "admin.impersonate_issued", "school", &sid, nil, nil)
	}
	return token, slug, expiresAt, nil
}

// exchangeImpersonation implementasi murni (testable) dari
// Service.ExchangeImpersonation: tukar token jadi sesi admin_sekolah ATAS
// NAMA user penerbit (super admin), TTL impersonationSessionTTL (2 jam),
// audit "admin.impersonate_started". hostSchoolID & hostSchool berasal dari
// reqctx (host tenant saat ini) — BUKAN dari body request.
func exchangeImpersonation(ctx context.Context, repo impersonationUserRepo, store *impersonationStore, audit impersonationAuditLogger, token string, hostSchoolID int64, hostSchool SchoolView, ip, userAgent string, now time.Time) (LoginResult, error) {
	if token == "" || hostSchoolID == 0 {
		return LoginResult{}, ErrImpersonationInvalid
	}
	issuerUserID, ok := store.take(token, hostSchoolID)
	if !ok {
		return LoginResult{}, ErrImpersonationInvalid
	}

	user, err := repo.UserByID(ctx, issuerUserID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return LoginResult{}, ErrImpersonationInvalid
		}
		return LoginResult{}, err
	}
	// User penerbit bisa saja kehilangan status super admin ANTARA token
	// diterbitkan dan ditukar — cek ULANG di sini, jangan percaya token saja.
	if !user.IsSuperAdmin {
		return LoginResult{}, ErrImpersonationInvalid
	}

	sessToken, tokenHash, terr := newSessionToken()
	if terr != nil {
		return LoginResult{}, terr
	}
	expiresAt := now.Add(impersonationSessionTTL)
	if err := repo.CreateSession(ctx, CreateSessionInput{
		// Role DISIMPAN sebagai sentinel impersonationSessionRole (BUKAN
		// RoleAdminSekolah langsung) — lihat catatan di deklarasi konstanta
		// di atas. RequireAuth menerjemahkannya kembali ke RoleAdminSekolah
		// untuk reqctx; UserView di bawah SUDAH pakai RoleAdminSekolah
		// (shape API tidak berubah).
		UserID: user.ID, SchoolID: &hostSchoolID, TokenHash: tokenHash, Role: impersonationSessionRole,
		ExpiresAt: expiresAt, IP: ip, UserAgent: userAgent,
	}); err != nil {
		return LoginResult{}, err
	}

	if audit != nil {
		sid, uid := hostSchoolID, user.ID
		_ = audit.Log(ctx, &sid, &uid, "admin.impersonate_started", "school", &sid, nil, nil)
	}

	view := UserView{ID: user.ID, Name: user.Name, Role: RoleAdminSekolah, IsSuperAdmin: true, School: &hostSchool}
	return LoginResult{View: view, Token: sessToken, ExpiresAt: expiresAt}, nil
}

// IssueImpersonation — dipanggil handler POST /api/admin/schools/{id}/impersonate
// (host platform, RequireSuperAdmin). issuerUserID adalah reqctx.UserID(ctx)
// (super admin yang sedang login).
func (s *Service) IssueImpersonation(ctx context.Context, schoolID, issuerUserID int64) (token, slug string, expiresAt time.Time, err error) {
	if s.schools == nil {
		return "", "", time.Time{}, errSchoolGatewayMissing
	}
	return issueImpersonation(ctx, s.schools, s.impersonation, s, schoolID, issuerUserID)
}

// ExchangeImpersonation — dipanggil handler POST /api/auth/impersonate
// (PUBLIK, host TENANT). Sekolah tujuan diambil dari reqctx (host saat ini),
// BUKAN dari token maupun body.
func (s *Service) ExchangeImpersonation(ctx context.Context, token, ip, userAgent string) (LoginResult, error) {
	sch, ok := reqctx.SchoolFromContext(ctx)
	if !ok {
		return LoginResult{}, ErrImpersonationInvalid
	}
	hostSchool := SchoolView{ID: sch.ID, Name: sch.Name, Slug: sch.Slug}
	return exchangeImpersonation(ctx, s.repo, s.impersonation, s, token, sch.ID, hostSchool, ip, userAgent, time.Now())
}
