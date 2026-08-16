package teacherqr

import (
	"context"
	"crypto/rand"
	"net/http"
	"strings"

	"github.com/omanjaya/nouschool/internal/platform/clock"
	"github.com/omanjaya/nouschool/internal/platform/httpx"
	"github.com/omanjaya/nouschool/internal/platform/reqctx"
)

// Realtime adalah kebutuhan modul teacherqr dari modul realtime — dipenuhi
// adapter cmd/server (pola sama modul lain, lihat CLAUDE.md).
type Realtime interface {
	PublishTo(schoolID int64, eventType string, data map[string]any, roles []string, userIDs []int64)
}

// Service berisi aturan bisnis modul teacherqr. SENGAJA TANPA
// IdentityGateway/audit — token ephemeral (TTL 60 detik, sekali pakai) bukan
// "mutasi penting" seperti izin/absensi/setting (lihat CLAUDE.md aturan
// audit_log).
type Service struct {
	repo     teacherQRRepository
	clock    clock.Clock
	realtime Realtime
}

func NewService(repo *Repository, clk clock.Clock) *Service {
	if clk == nil {
		clk = clock.System{}
	}
	return &Service{repo: repo, clock: clk}
}

func (s *Service) SetRealtime(r Realtime) { s.realtime = r }

// newServiceForTest membangun Service dengan repository FAKE (in-memory,
// tanpa DB) — dipakai test di package ini saja.
func newServiceForTest(repo teacherQRRepository, clk clock.Clock) *Service {
	return &Service{repo: repo, clock: clk}
}

func generateToken() (string, error) {
	b := make([]byte, tokenLength)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	out := make([]byte, tokenLength)
	for i, v := range b {
		out[i] = tokenChars[int(v)%len(tokenChars)]
	}
	return string(out), nil
}

func normalizeToken(raw string) string {
	return strings.TrimPrefix(strings.TrimSpace(raw), tokenPrefix)
}

// -- POST /api/teacher-qr --

// GenerateToken — HANYA role guru (docs tugas: "pegawai TIDAK perlu").
// Cleanup lazy: token kedaluwarsa milik user ini dihapus sebelum membuat
// yang baru.
func (s *Service) GenerateToken(ctx context.Context, actorUserID, schoolID int64) (TokenView, error) {
	if reqctx.Role(ctx) != RoleGuru {
		return TokenView{}, httpx.ErrForbidden
	}
	now := s.clock.Now()
	if err := s.repo.DeleteExpiredForUser(ctx, schoolID, actorUserID, now); err != nil {
		return TokenView{}, err
	}
	token, err := generateToken()
	if err != nil {
		return TokenView{}, err
	}
	expiresAt := now.Add(tokenTTL)
	if _, err := s.repo.CreateToken(ctx, schoolID, actorUserID, token, expiresAt); err != nil {
		return TokenView{}, err
	}
	return TokenView{Token: token, ExpiresAt: expiresAt}, nil
}

// ErrInvalidToken — token tidak dikenal, sudah dipakai, atau sudah
// kedaluwarsa (docs tugas: pesan persis "QR tidak berlaku. Minta guru
// menampilkan QR baru.", HTTP 410).
var ErrInvalidToken = &httpx.Error{Status: http.StatusGone, Code: "qr_expired", Message: "QR tidak berlaku. Minta guru menampilkan QR baru."}

// ConsumeToken — API PUBLIK (dipakai modul lain lewat consumer-side
// interface, mis. internal/exitpermit & internal/latearrival — BUKAN
// endpoint HTTP tersendiri): valid+belum expired+belum consumed -> tandai
// consumed_at (atomik) -> publish realtime "teacherqr" {} ke pemilik token
// (frontend guru auto-regenerate QR baru) -> kembalikan user_id pemilik.
func (s *Service) ConsumeToken(ctx context.Context, schoolID int64, raw string) (int64, error) {
	token := normalizeToken(raw)
	if token == "" {
		return 0, httpx.Validation("Token QR wajib diisi.")
	}
	userID, ok, err := s.repo.ConsumeToken(ctx, schoolID, token, s.clock.Now())
	if err != nil {
		return 0, err
	}
	if !ok {
		return 0, ErrInvalidToken
	}
	if s.realtime != nil {
		s.realtime.PublishTo(schoolID, "teacherqr", map[string]any{}, nil, []int64{userID})
	}
	return userID, nil
}
