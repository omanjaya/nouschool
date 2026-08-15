package identity

import (
	"sync"
	"time"

	"github.com/omanjaya/nouschool/internal/platform/clock"
)

// RateLimiter membatasi percobaan gagal per key (per IP maupun per
// identifier akun) secara in-memory: N gagal dalam satu jendela waktu ->
// diblokir sampai jendela bergeser (lihat CLAUDE.md & docs/02-identity.md).
type RateLimiter struct {
	mu       sync.Mutex
	attempts map[string][]time.Time
	max      int
	window   time.Duration
	clock    clock.Clock
}

// NewRateLimiter membuat rate limiter. c boleh nil (pakai clock.System{}).
func NewRateLimiter(max int, window time.Duration, c clock.Clock) *RateLimiter {
	if c == nil {
		c = clock.System{}
	}
	return &RateLimiter{
		attempts: make(map[string][]time.Time),
		max:      max,
		window:   window,
		clock:    c,
	}
}

// Blocked melaporkan apakah key sudah mencapai batas gagal dalam jendela waktu.
func (r *RateLimiter) Blocked(key string) bool {
	if key == "" {
		return false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.prune(key)) >= r.max
}

// RecordFailure mencatat satu percobaan gagal untuk key.
func (r *RateLimiter) RecordFailure(key string) {
	if key == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	times := r.prune(key)
	r.attempts[key] = append(times, r.clock.Now())
}

// Reset menghapus riwayat gagal untuk key (dipanggil setelah login sukses).
func (r *RateLimiter) Reset(key string) {
	if key == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.attempts, key)
}

// prune membuang catatan gagal di luar jendela waktu. Dipanggil dengan mu terkunci.
func (r *RateLimiter) prune(key string) []time.Time {
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
