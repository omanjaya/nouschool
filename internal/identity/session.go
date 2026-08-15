package identity

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net/http"
	"time"
)

const (
	sessionCookieName  = "ns_session"
	sessionTTL         = 30 * 24 * time.Hour // umur cookie sliding (role selain display)
	sessionRenewWindow = 15 * 24 * time.Hour // perpanjang bila sisa < ini

	// displaySessionTTL — akun display (TV ruang guru) login sekali, dipasang
	// di mini-PC/TV, dan TIDAK boleh sering ter-logout (docs/02-identity.md:
	// "role display 1 tahun", docs/06-teaching.md "session panjang").
	displaySessionTTL         = 365 * 24 * time.Hour
	displaySessionRenewWindow = 60 * 24 * time.Hour // perpanjang bila sisa < ini
)

// sessionTTLForRole & sessionRenewWindowForRole — TTL sesi PER ROLE
// (docs/02-identity.md "Cookie: ... umur 30 hari (sliding), role display 1
// tahun"). Dipakai Login, IssueSession (gateway.go), dan sliding renewal
// RequireAuth (middleware.go) supaya sesi role display TIDAK diperpendek
// diam-diam jadi 30 hari saat renewal (bug yang mudah lolos kalau lupa —
// makanya ketiga tempat WAJIB pakai fungsi ini, bukan konstanta sessionTTL
// langsung).
func sessionTTLForRole(role string) time.Duration {
	if role == RoleDisplay {
		return displaySessionTTL
	}
	return sessionTTL
}

func sessionRenewWindowForRole(role string) time.Duration {
	if role == RoleDisplay {
		return displaySessionRenewWindow
	}
	return sessionRenewWindow
}

var ErrInvalidSession = errors.New("identity: token sesi tidak valid")

// newSessionToken menghasilkan token acak 32 byte. raw dikirim ke klien lewat
// cookie; hash (SHA-256 dari byte mentah) yang disimpan di DB.
func newSessionToken() (raw string, hash []byte, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return "", nil, err
	}
	raw = base64.RawURLEncoding.EncodeToString(b)
	sum := sha256.Sum256(b)
	return raw, sum[:], nil
}

// hashToken menghitung ulang hash dari token mentah yang dikirim klien
// (cookie), untuk dicocokkan dengan token_hash tersimpan di DB.
func hashToken(raw string) ([]byte, error) {
	b, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return nil, ErrInvalidSession
	}
	sum := sha256.Sum256(b)
	return sum[:], nil
}

func setSessionCookie(w http.ResponseWriter, token string, expiresAt time.Time, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		Expires:  expiresAt,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}

func clearSessionCookie(w http.ResponseWriter, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}
