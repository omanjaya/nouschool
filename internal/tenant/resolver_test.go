package tenant

import (
	"context"
	"testing"
	"time"
)

// fakeSchoolLookup implementasi schoolLookup tanpa database, untuk test.
type fakeSchoolLookup struct {
	bySlug         map[string]School
	byCustomDomain map[string]School
	calls          int
}

func (f *fakeSchoolLookup) SchoolBySlug(ctx context.Context, slug string) (School, error) {
	f.calls++
	if s, ok := f.bySlug[slug]; ok {
		return s, nil
	}
	return School{}, ErrNotFound
}

func (f *fakeSchoolLookup) SchoolByCustomDomain(ctx context.Context, domain string) (School, error) {
	f.calls++
	if s, ok := f.byCustomDomain[domain]; ok {
		return s, nil
	}
	return School{}, ErrNotFound
}

func newTestResolver() (*HostResolver, *fakeSchoolLookup) {
	fake := &fakeSchoolLookup{
		bySlug: map[string]School{
			"demo": {ID: 1, Name: "Sekolah Demo", Slug: "demo", Timezone: "Asia/Jakarta", Status: "active"},
		},
		byCustomDomain: map[string]School{
			"sekolahku.id": {ID: 2, Name: "Sekolahku", Slug: "sekolahku", Timezone: "Asia/Jakarta", Status: "active"},
		},
	}
	r := NewHostResolver(fake, "localhost")
	return r, fake
}

func TestResolvePlatformHost(t *testing.T) {
	r, _ := newTestResolver()
	resolved, err := r.Resolve(context.Background(), "localhost")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resolved.IsPlatform {
		t.Fatal("host base domain seharusnya konteks platform")
	}
}

func TestResolveAdminSubdomainIsPlatform(t *testing.T) {
	r, _ := newTestResolver()
	resolved, err := r.Resolve(context.Background(), "admin.localhost")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resolved.IsPlatform {
		t.Fatal("admin.<base_domain> seharusnya konteks platform, bukan tenant")
	}
}

func TestResolveTenantSubdomain(t *testing.T) {
	r, _ := newTestResolver()
	resolved, err := r.Resolve(context.Background(), "demo.localhost")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resolved.IsPlatform {
		t.Fatal("subdomain sekolah tidak boleh dianggap platform")
	}
	if resolved.School.Slug != "demo" || resolved.School.ID != 1 {
		t.Fatalf("sekolah salah: %+v", resolved.School)
	}
}

func TestResolveStripsPort(t *testing.T) {
	r, _ := newTestResolver()
	resolved, err := r.Resolve(context.Background(), StripPort("demo.localhost:8210"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resolved.School.Slug != "demo" {
		t.Fatalf("resolusi dengan port seharusnya tetap ketemu slug demo, dapat: %+v", resolved.School)
	}

	if got := StripPort("Demo.Localhost:8210"); got != "demo.localhost" {
		t.Fatalf("StripPort seharusnya lowercase & buang port, dapat %q", got)
	}
}

func TestResolveCustomDomain(t *testing.T) {
	r, _ := newTestResolver()
	resolved, err := r.Resolve(context.Background(), "sekolahku.id")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resolved.School.ID != 2 {
		t.Fatalf("custom domain seharusnya resolve ke sekolah id 2, dapat: %+v", resolved.School)
	}
}

func TestResolveNotFound(t *testing.T) {
	r, _ := newTestResolver()
	_, err := r.Resolve(context.Background(), "tidak-ada.localhost")
	if err != ErrSchoolNotFound {
		t.Fatalf("expected ErrSchoolNotFound, dapat: %v", err)
	}
}

func TestResolveCachesLookup(t *testing.T) {
	r, fake := newTestResolver()
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	r.now = func() time.Time { return now }

	if _, err := r.Resolve(context.Background(), "demo.localhost"); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Resolve(context.Background(), "demo.localhost"); err != nil {
		t.Fatal(err)
	}
	if fake.calls != 1 {
		t.Fatalf("lookup kedua seharusnya dari cache, calls = %d", fake.calls)
	}

	// Geser waktu melewati TTL 60 detik -> cache kedaluwarsa, lookup ulang.
	now = now.Add(61 * time.Second)
	if _, err := r.Resolve(context.Background(), "demo.localhost"); err != nil {
		t.Fatal(err)
	}
	if fake.calls != 2 {
		t.Fatalf("setelah TTL lewat seharusnya lookup ulang, calls = %d", fake.calls)
	}
}

func TestResolveCachesNotFound(t *testing.T) {
	r, fake := newTestResolver()
	if _, err := r.Resolve(context.Background(), "tidak-ada.localhost"); err != ErrSchoolNotFound {
		t.Fatal(err)
	}
	if _, err := r.Resolve(context.Background(), "tidak-ada.localhost"); err != ErrSchoolNotFound {
		t.Fatal(err)
	}
	if fake.calls != 1 {
		t.Fatalf("hasil not-found seharusnya ikut di-cache, calls = %d", fake.calls)
	}
}
