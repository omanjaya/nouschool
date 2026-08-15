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

// Service berisi aturan bisnis modul identity: login/logout, sesi, RBAC.
type Service struct {
	repo         *Repository
	rateLimiter  *RateLimiter
	cookieSecure bool
}

func NewService(repo *Repository, rateLimiter *RateLimiter, cookieSecure bool) *Service {
	return &Service{repo: repo, rateLimiter: rateLimiter, cookieSecure: cookieSecure}
}

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
	}

	token, tokenHash, terr := newSessionToken()
	if terr != nil {
		return LoginResult{}, terr
	}
	expiresAt := time.Now().Add(sessionTTL)

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
	}
	return view, nil
}

func (s *Service) CookieSecure() bool { return s.cookieSecure }
