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
	sessionTTL         = 30 * 24 * time.Hour // umur cookie sliding
	sessionRenewWindow = 15 * 24 * time.Hour // perpanjang bila sisa < ini
)

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
