package identity

import (
	"testing"
	"time"

	"github.com/omanjaya/nouschool/internal/platform/clock"
)

func TestRateLimiterBlocksAfterMaxFailures(t *testing.T) {
	fixed := &movableClock{t: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	rl := NewRateLimiter(5, 15*time.Minute, fixed)

	key := "1.2.3.4"
	if rl.Blocked(key) {
		t.Fatal("belum ada percobaan gagal, seharusnya tidak diblokir")
	}

	for i := 0; i < 4; i++ {
		rl.RecordFailure(key)
	}
	if rl.Blocked(key) {
		t.Fatal("4 gagal masih di bawah batas 5, seharusnya belum diblokir")
	}

	rl.RecordFailure(key) // gagal ke-5
	if !rl.Blocked(key) {
		t.Fatal("5 gagal dalam jendela seharusnya diblokir")
	}
}

func TestRateLimiterWindowExpires(t *testing.T) {
	fixed := &movableClock{t: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	rl := NewRateLimiter(5, 15*time.Minute, fixed)

	key := "user@demo.localhost"
	for i := 0; i < 5; i++ {
		rl.RecordFailure(key)
	}
	if !rl.Blocked(key) {
		t.Fatal("seharusnya diblokir setelah 5 gagal")
	}

	fixed.t = fixed.t.Add(16 * time.Minute) // jendela 15 menit lewat
	if rl.Blocked(key) {
		t.Fatal("setelah jendela lewat seharusnya tidak diblokir lagi")
	}
}

func TestRateLimiterResetClearsFailures(t *testing.T) {
	fixed := &movableClock{t: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	rl := NewRateLimiter(5, 15*time.Minute, fixed)

	key := "admin"
	for i := 0; i < 5; i++ {
		rl.RecordFailure(key)
	}
	rl.Reset(key)
	if rl.Blocked(key) {
		t.Fatal("setelah Reset seharusnya tidak diblokir")
	}
}

func TestRateLimiterPerKeyIndependent(t *testing.T) {
	fixed := &movableClock{t: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	rl := NewRateLimiter(5, 15*time.Minute, fixed)

	for i := 0; i < 5; i++ {
		rl.RecordFailure("ip-a")
	}
	if !rl.Blocked("ip-a") {
		t.Fatal("ip-a seharusnya diblokir")
	}
	if rl.Blocked("ip-b") {
		t.Fatal("ip-b tidak boleh ikut diblokir")
	}
}

// movableClock adalah clock.Clock yang waktunya bisa digeser manual di test.
type movableClock struct{ t time.Time }

func (m *movableClock) Now() time.Time { return m.t }

var _ clock.Clock = (*movableClock)(nil)
