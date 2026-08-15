package tenant

import (
	"context"
	"errors"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/omanjaya/nouschool/internal/platform/httpx"
	"github.com/omanjaya/nouschool/internal/platform/reqctx"
)

// ErrSchoolNotFound — host tidak cocok sekolah manapun (subdomain / custom domain).
var ErrSchoolNotFound = &httpx.Error{
	Status:  http.StatusNotFound,
	Code:    "school_not_found",
	Message: "Sekolah tidak ditemukan untuk domain ini.",
}

type cacheEntry struct {
	resolved  ResolvedHost
	notFound  bool
	expiresAt time.Time
}

// schoolLookup adalah interface kecil yang dibutuhkan HostResolver — cukup
// dipenuhi *Repository, tapi dideklarasikan sebagai interface supaya
// HostResolver bisa dites tanpa database (fake di resolver_test.go).
type schoolLookup interface {
	SchoolBySlug(ctx context.Context, slug string) (School, error)
	SchoolByCustomDomain(ctx context.Context, domain string) (School, error)
}

// HostResolver menerjemahkan Host header ke ResolvedHost (platform atau
// sekolah tertentu), dengan cache in-memory TTL 60 detik (lihat docs/01-tenant.md
// bagian "Resolusi tenant").
type HostResolver struct {
	repo       schoolLookup
	baseDomain string
	ttl        time.Duration
	now        func() time.Time

	mu    sync.RWMutex
	cache map[string]cacheEntry
}

func NewHostResolver(repo schoolLookup, baseDomain string) *HostResolver {
	return &HostResolver{
		repo:       repo,
		baseDomain: strings.ToLower(strings.TrimSpace(baseDomain)),
		ttl:        60 * time.Second,
		now:        time.Now,
		cache:      make(map[string]cacheEntry),
	}
}

// StripPort membuang ":port" dari Host header — Host header di dev/prod bisa
// mengandung port (mis. "demo.localhost:8210").
func StripPort(host string) string {
	host = strings.ToLower(strings.TrimSpace(host))
	if h, _, err := net.SplitHostPort(host); err == nil {
		return h
	}
	return host
}

// Resolve menentukan konteks (platform / sekolah) dari host (tanpa port).
// Urutan aturan sesuai docs/01-tenant.md:
//  1. host == BASE_DOMAIN atau "admin."+BASE_DOMAIN -> konteks platform
//  2. host == {slug}.BASE_DOMAIN -> lookup slug
//  3. selain itu -> lookup custom_domain
func (h *HostResolver) Resolve(ctx context.Context, host string) (ResolvedHost, error) {
	host = strings.ToLower(strings.TrimSpace(host))

	if host == h.baseDomain || host == "admin."+h.baseDomain {
		return ResolvedHost{IsPlatform: true}, nil
	}

	if cached, ok := h.fromCache(host); ok {
		if cached.notFound {
			return ResolvedHost{}, ErrSchoolNotFound
		}
		return cached.resolved, nil
	}

	var school School
	var err error
	if strings.HasSuffix(host, "."+h.baseDomain) {
		slug := strings.TrimSuffix(host, "."+h.baseDomain)
		school, err = h.repo.SchoolBySlug(ctx, slug)
	} else {
		school, err = h.repo.SchoolByCustomDomain(ctx, host)
	}
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			h.storeCache(host, cacheEntry{notFound: true})
			return ResolvedHost{}, ErrSchoolNotFound
		}
		return ResolvedHost{}, err
	}

	resolved := ResolvedHost{School: school}
	h.storeCache(host, cacheEntry{resolved: resolved})
	return resolved, nil
}

func (h *HostResolver) fromCache(host string) (cacheEntry, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	e, ok := h.cache[host]
	if !ok || h.now().After(e.expiresAt) {
		return cacheEntry{}, false
	}
	return e, true
}

func (h *HostResolver) storeCache(host string, e cacheEntry) {
	e.expiresAt = h.now().Add(h.ttl)
	h.mu.Lock()
	h.cache[host] = e
	h.mu.Unlock()
}

// Middleware memasang hasil resolusi ke request context. Tidak ketemu -> 404 JSON.
// Ini middleware PERTAMA setelah Recover/Logger/SecurityHeaders (lihat cmd/server/main.go).
func (h *HostResolver) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host := StripPort(r.Host)
		resolved, err := h.Resolve(r.Context(), host)
		if err != nil {
			httpx.WriteError(w, err)
			return
		}

		ctx := r.Context()
		if resolved.IsPlatform {
			ctx = reqctx.WithPlatform(ctx)
		} else {
			ctx = reqctx.WithSchool(ctx, reqctx.School{
				ID:       resolved.School.ID,
				Name:     resolved.School.Name,
				Slug:     resolved.School.Slug,
				Timezone: resolved.School.Timezone,
				Status:   resolved.School.Status,
			})
		}
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
