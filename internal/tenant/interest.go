package tenant

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/omanjaya/nouschool/internal/platform/clock"
	"github.com/omanjaya/nouschool/internal/platform/httpx"
)

// Berkas ini mengurus registrasi minat sekolah (Fase 11, landing page host
// platform): POST /api/public/interest (PUBLIK, rate-limit per IP) dan
// GET /api/admin/interest (super admin). interest_leads TANPA school_id
// (platform-level, calon sekolah belum jadi tenant — lihat migrations/00012).

// interestRateLimiter adalah rate limiter kecil per-IP, in-memory (mis N
// percobaan per jendela waktu). Duplikat MINIMAL dari pola yang sama dengan
// internal/identity.RateLimiter (login) — sengaja tidak diimpor lintas modul
// (identity adalah modul bisnis, bukan platform/, lihat aturan wiring di
// CLAUDE.md "antar-modul lewat interface kecil / platform/ diimpor
// langsung") — duplikasi kecil ini dianggap lebih murah daripada memindahkan
// RateLimiter ke platform/ hanya untuk satu pemakai baru.
type interestRateLimiter struct {
	mu       sync.Mutex
	attempts map[string][]time.Time
	max      int
	window   time.Duration
	clock    clock.Clock
}

func newInterestRateLimiter(max int, window time.Duration, c clock.Clock) *interestRateLimiter {
	if c == nil {
		c = clock.System{}
	}
	return &interestRateLimiter{attempts: make(map[string][]time.Time), max: max, window: window, clock: c}
}

func (r *interestRateLimiter) prune(key string) []time.Time {
	cutoff := r.clock.Now().Add(-r.window)
	kept := r.attempts[key][:0]
	for _, t := range r.attempts[key] {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	r.attempts[key] = kept
	return kept
}

// Allow melaporkan apakah key (biasanya IP) masih di bawah batas, DAN
// langsung mencatat percobaan ini bila diizinkan (check-and-record atomik di
// bawah satu lock, mencegah race dua request bersamaan sama-sama lolos pada
// percobaan terakhir yang tersisa).
func (r *interestRateLimiter) Allow(key string) bool {
	if key == "" {
		return true
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.prune(key)) >= r.max {
		return false
	}
	r.attempts[key] = append(r.attempts[key], r.clock.Now())
	return true
}

// ErrInterestRateLimited — batas registrasi minat per IP tercapai.
var errInterestRateLimited = &httpx.Error{Status: 429, Code: "rate_limited", Message: "Terlalu banyak percobaan dari alamat ini. Coba lagi dalam satu jam."}

// interestRepository adalah kontrak yang dibutuhkan InterestService dari
// Repository — dipenuhi *Repository secara struktural, memudahkan test tanpa
// DB (lihat interest_test.go).
type interestRepository interface {
	CreateInterestLead(ctx context.Context, in CreateInterestLeadInput) (InterestLead, error)
	ListInterestLeads(ctx context.Context) ([]InterestLead, error)
}

var _ interestRepository = (*Repository)(nil)

// InterestService mengurus registrasi minat sekolah dari landing page.
type InterestService struct {
	repo    interestRepository
	limiter *interestRateLimiter
}

// NewInterestService — limit 3 pengajuan per IP per jam (docs tugas Fase 11).
func NewInterestService(repo *Repository, clk clock.Clock) *InterestService {
	return &InterestService{repo: repo, limiter: newInterestRateLimiter(3, time.Hour, clk)}
}

// newInterestServiceForTest membangun InterestService dengan repository FAKE
// (in-memory, tanpa DB) — dipakai test di package ini saja (interest_test.go).
func newInterestServiceForTest(repo interestRepository, max int, window time.Duration, clk clock.Clock) *InterestService {
	return &InterestService{repo: repo, limiter: newInterestRateLimiter(max, window, clk)}
}

type SubmitInterestInput struct {
	SchoolName  string
	ContactName string
	Phone       string
	Email       string
	Note        string
	ClientIP    string
}

// SubmitInterest — POST /api/public/interest (publik, host platform).
func (s *InterestService) SubmitInterest(ctx context.Context, in SubmitInterestInput) (InterestLead, error) {
	if !s.limiter.Allow(in.ClientIP) {
		return InterestLead{}, errInterestRateLimited
	}

	schoolName := strings.TrimSpace(in.SchoolName)
	contactName := strings.TrimSpace(in.ContactName)
	phone := strings.TrimSpace(in.Phone)
	if schoolName == "" {
		return InterestLead{}, httpx.Validation("Nama sekolah wajib diisi.")
	}
	if contactName == "" {
		return InterestLead{}, httpx.Validation("Nama kontak wajib diisi.")
	}
	if phone == "" {
		return InterestLead{}, httpx.Validation("Nomor telepon/WhatsApp wajib diisi.")
	}

	return s.repo.CreateInterestLead(ctx, CreateInterestLeadInput{
		SchoolName: schoolName, ContactName: contactName, Phone: phone,
		Email: strings.TrimSpace(in.Email), Note: strings.TrimSpace(in.Note),
	})
}

// ListInterest — GET /api/admin/interest (super admin).
func (s *InterestService) ListInterest(ctx context.Context) ([]InterestLead, error) {
	return s.repo.ListInterestLeads(ctx)
}
