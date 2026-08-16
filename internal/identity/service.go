package identity

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/omanjaya/nouschool/internal/platform/httpx"
	"github.com/omanjaya/nouschool/internal/platform/reqctx"
)

var (
	// ErrInvalidCredentials — identifier/password salah, atau (di host
	// platform) user bukan super admin. Pesan generik sengaja tidak
	// membocorkan mana yang salah (identifier vs password).
	ErrInvalidCredentials = &httpx.Error{
		Status:  http.StatusUnauthorized,
		Code:    "invalid_credentials",
		Message: "Email/username atau kata sandi salah.",
	}
	ErrTooManyAttempts = &httpx.Error{
		Status:  http.StatusTooManyRequests,
		Code:    "too_many_attempts",
		Message: "Terlalu banyak percobaan gagal. Coba lagi dalam 15 menit.",
	}
)

// BillingGateway adalah kebutuhan modul identity dari modul billing (fase 10,
// docs/09-billing.md "Frontend menerima daftar fitur aktif di payload
// /api/me") — consumer-side interface kecil dideklarasikan di sisi PEMAKAI
// (lihat CLAUDE.md). Dipenuhi *billing.Service secara struktural lewat
// method SubscriptionForMe (signature primitif) — identity TIDAK mengimpor
// billing untuk tipe apa pun. found=false berarti sekolah belum pernah
// punya subscription sama sekali.
type BillingGateway interface {
	SubscriptionForMe(ctx context.Context, schoolID int64) (status, planCode, endsOn, graceUntil string, features []string, found bool, err error)
}

// SchoolGateway adalah kebutuhan modul identity dari modul tenant (fase 13,
// fitur impersonation super admin, docs/11-superadmin.md) — dipenuhi
// *tenant.Service secara struktural lewat SchoolStatusAndSlug. Signature
// primitif supaya identity TIDAK perlu mengimpor package tenant untuk tipe
// apa pun (lihat CLAUDE.md).
type SchoolGateway interface {
	SchoolStatusAndSlug(ctx context.Context, id int64) (status, slug string, found bool, err error)
}

// Service berisi aturan bisnis modul identity: login/logout, sesi, RBAC.
type Service struct {
	repo          *Repository
	rateLimiter   *RateLimiter
	cookieSecure  bool
	billing       BillingGateway      // opsional — nil = subscription/features dilewati di /api/me (lihat SetBillingGateway)
	schools       SchoolGateway       // opsional — nil = IssueImpersonation gagal (lihat SetSchoolGateway)
	impersonation *impersonationStore // token sekali-pakai impersonation (fase 13) — selalu terisi, in-memory
}

func NewService(repo *Repository, rateLimiter *RateLimiter, cookieSecure bool) *Service {
	return &Service{
		repo: repo, rateLimiter: rateLimiter, cookieSecure: cookieSecure,
		impersonation: newImpersonationStore(nil),
	}
}

// SetBillingGateway menyuntikkan BillingGateway SETELAH konstruksi (opsional,
// disuntik main.go — billing dikonstruksi SETELAH identity karena butuh
// identitySvc untuk audit log, pola yang sama dengan
// internal/attendance.SetNotifier; nil aman/no-op).
func (s *Service) SetBillingGateway(b BillingGateway) { s.billing = b }

// SetSchoolGateway menyuntikkan SchoolGateway SETELAH konstruksi (opsional,
// disuntik main.go — tenant dikonstruksi SETELAH identity, pola yang sama
// dengan SetBillingGateway di atas). Tanpa ini, IssueImpersonation
// mengembalikan error (lihat impersonation.go).
func (s *Service) SetSchoolGateway(g SchoolGateway) { s.schools = g }

// LoginInput adalah parameter POST /api/auth/login.
type LoginInput struct {
	Identifier string
	Password   string
	IP         string
	UserAgent  string
}

// LoginResult dikembalikan Login: view untuk response, token mentah untuk
// cookie, dan waktu kedaluwarsanya.
type LoginResult struct {
	View      UserView
	Token     string
	ExpiresAt time.Time
}

// Login mengautentikasi identifier (email atau username) + password.
//
// Konteks host (platform / tenant) diambil dari request context, hasil
// ResolveTenant middleware yang berjalan sebelum handler ini:
//   - Host platform: hanya user is_super_admin yang boleh login; sesi tidak
//     terikat sekolah (school_id NULL).
//   - Host tenant (subdomain/custom domain sekolah): user harus punya
//     membership aktif di sekolah itu; role aktif dipilih dari prioritas
//     docs/02, daftar semua role disertakan di response.
func (s *Service) Login(ctx context.Context, in LoginInput) (LoginResult, error) {
	identifier := strings.TrimSpace(in.Identifier)
	if identifier == "" || in.Password == "" {
		return LoginResult{}, httpx.Validation("Email/username dan kata sandi wajib diisi.")
	}

	if s.rateLimiter.Blocked(in.IP) || s.rateLimiter.Blocked(identifier) {
		return LoginResult{}, ErrTooManyAttempts
	}

	fail := func() error {
		s.rateLimiter.RecordFailure(in.IP)
		s.rateLimiter.RecordFailure(identifier)
		return ErrInvalidCredentials
	}

	user, err := s.repo.UserByEmail(ctx, identifier)
	if errors.Is(err, ErrNotFound) {
		user, err = s.repo.UserByUsername(ctx, identifier)
	}
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return LoginResult{}, fail()
		}
		return LoginResult{}, err
	}

	ok, verr := VerifyPassword(user.PasswordHash, in.Password)
	if verr != nil || !ok {
		return LoginResult{}, fail()
	}

	var (
		schoolIDPtr *int64
		role        string
		view        UserView
	)

	if reqctx.IsPlatform(ctx) {
		if !user.IsSuperAdmin {
			return LoginResult{}, fail()
		}
		role = RoleSuperAdmin
		view = UserView{ID: user.ID, Name: user.Name, Role: role, IsSuperAdmin: true, School: nil}
	} else {
		sch, ok := reqctx.SchoolFromContext(ctx)
		if !ok {
			return LoginResult{}, fail()
		}
		memberships, merr := s.repo.ListActiveMemberships(ctx, user.ID, sch.ID)
		if merr != nil {
			return LoginResult{}, merr
		}
		if len(memberships) == 0 {
			return LoginResult{}, fail()
		}
		roles := make([]string, 0, len(memberships))
		for _, m := range memberships {
			roles = append(roles, m.Role)
		}
		role = PickActiveRole(roles)
		id := sch.ID
		schoolIDPtr = &id
		view = UserView{
			ID: user.ID, Name: user.Name, Role: role, Roles: roles, IsSuperAdmin: user.IsSuperAdmin,
			School: &SchoolView{ID: sch.ID, Name: sch.Name, Slug: sch.Slug},
		}
		if role == RoleSiswa {
			if sid, serr := s.repo.StudentIDByUser(ctx, user.ID, sch.ID); serr == nil {
				view.StudentID = sid
			}
		}
	}

	token, tokenHash, terr := newSessionToken()
	if terr != nil {
		return LoginResult{}, terr
	}
	expiresAt := time.Now().Add(sessionTTLForRole(role))

	if err := s.repo.CreateSession(ctx, CreateSessionInput{
		UserID: user.ID, SchoolID: schoolIDPtr, TokenHash: tokenHash, Role: role,
		ExpiresAt: expiresAt, IP: in.IP, UserAgent: in.UserAgent,
	}); err != nil {
		return LoginResult{}, err
	}

	s.rateLimiter.Reset(in.IP)
	s.rateLimiter.Reset(identifier)

	return LoginResult{View: view, Token: token, ExpiresAt: expiresAt}, nil
}

// Logout menghapus sesi (berdasarkan token cookie mentah). Token kosong atau
// tidak valid dianggap sudah logout (tidak error).
func (s *Service) Logout(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	hash, err := hashToken(token)
	if err != nil {
		return nil
	}
	return s.repo.DeleteSessionByTokenHash(ctx, hash)
}

// Me membangun UserView dari sesi aktif di context (diisi RequireAuth).
func (s *Service) Me(ctx context.Context) (UserView, error) {
	userID := reqctx.UserID(ctx)
	if userID == 0 {
		return UserView{}, httpx.ErrUnauthorized
	}
	user, err := s.repo.UserByID(ctx, userID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return UserView{}, httpx.ErrUnauthorized
		}
		return UserView{}, err
	}
	view := UserView{
		ID:           user.ID,
		Name:         user.Name,
		Role:         reqctx.Role(ctx),
		IsSuperAdmin: reqctx.IsSuperAdmin(ctx),
	}
	if sch, ok := reqctx.SchoolFromContext(ctx); ok {
		view.School = &SchoolView{ID: sch.ID, Name: sch.Name, Slug: sch.Slug}
		if view.Role == RoleSiswa {
			if sid, serr := s.repo.StudentIDByUser(ctx, userID, sch.ID); serr == nil {
				view.StudentID = sid
			}
		}
		if s.billing != nil {
			if status, planCode, endsOn, graceUntil, features, found, berr := s.billing.SubscriptionForMe(ctx, sch.ID); berr == nil && found {
				sv := &SubscriptionView{Status: status, PlanCode: planCode, EndsOn: endsOn}
				if status == "grace" || status == "readonly" {
					g := graceUntil
					sv.GraceUntil = &g
				}
				view.Subscription = sv
				view.Features = features
			}
		}
	}
	return view, nil
}

func (s *Service) CookieSecure() bool { return s.cookieSecure }
