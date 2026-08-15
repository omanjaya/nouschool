package tenant

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/omanjaya/nouschool/internal/platform/httpx"
)

// fakeDomainRepo implementasi domainRepository in-memory (tanpa DB) — dipakai
// menguji DomainService.PutDomain/VerifyDomain/DeleteDomain (docs tugas Fase
// 11: "custom domain (format, unik, verify tanpa SERVER_IP gagal jelas)").
type fakeDomainRepo struct {
	schools map[int64]School
}

func newFakeDomainRepo(schools ...School) *fakeDomainRepo {
	m := make(map[int64]School, len(schools))
	for _, s := range schools {
		m[s.ID] = s
	}
	return &fakeDomainRepo{schools: m}
}

// SchoolBySlug/SchoolByCustomDomain satisfy schoolLookup so fakeDomainRepo
// can also be passed to NewHostResolver in these tests (DomainService calls
// resolver.Invalidate, which never queries the repo — these stubs are unused
// but required to satisfy the interface).
func (f *fakeDomainRepo) SchoolBySlug(ctx context.Context, slug string) (School, error) {
	return School{}, ErrNotFound
}

func (f *fakeDomainRepo) SchoolByCustomDomain(ctx context.Context, domain string) (School, error) {
	return School{}, ErrNotFound
}

func (f *fakeDomainRepo) SchoolByID(ctx context.Context, id int64) (School, error) {
	s, ok := f.schools[id]
	if !ok {
		return School{}, ErrNotFound
	}
	return s, nil
}

func (f *fakeDomainRepo) DomainUsedByOtherSchool(ctx context.Context, domain string, excludeSchoolID int64) (bool, error) {
	for id, s := range f.schools {
		if id == excludeSchoolID {
			continue
		}
		if s.CustomDomain == domain || s.PendingDomain == domain {
			return true, nil
		}
	}
	return false, nil
}

func (f *fakeDomainRepo) SetPendingDomain(ctx context.Context, schoolID int64, domain string) (School, error) {
	s, ok := f.schools[schoolID]
	if !ok {
		return School{}, ErrNotFound
	}
	s.PendingDomain = domain
	f.schools[schoolID] = s
	return s, nil
}

func (f *fakeDomainRepo) VerifyPendingDomain(ctx context.Context, schoolID int64) (School, error) {
	s, ok := f.schools[schoolID]
	if !ok {
		return School{}, ErrNotFound
	}
	s.CustomDomain = s.PendingDomain
	s.PendingDomain = ""
	f.schools[schoolID] = s
	return s, nil
}

func (f *fakeDomainRepo) ClearDomain(ctx context.Context, schoolID int64) (School, error) {
	s, ok := f.schools[schoolID]
	if !ok {
		return School{}, ErrNotFound
	}
	s.CustomDomain, s.PendingDomain = "", ""
	f.schools[schoolID] = s
	return s, nil
}

func errAsHTTPX(t *testing.T, err error) *httpx.Error {
	t.Helper()
	var de *httpx.Error
	if !errors.As(err, &de) {
		t.Fatalf("expected *httpx.Error, dapat %T: %v", err, err)
	}
	return de
}

func TestValidDomainFormat(t *testing.T) {
	cases := map[string]bool{
		"sekolahdemo.sch.id":  true,
		"sekolahku.id":        true,
		"a.co":                true,
		"tanpa-titik":         false,
		"":                    false,
		".leading.dot":        false,
		"trailing.dot.":       false,
		"spasi di domain.com": false,
		"-mulai-hubung.com":   false,
	}
	for domain, want := range cases {
		if got := validDomainFormat(domain); got != want {
			t.Errorf("validDomainFormat(%q) = %v, ingin %v", domain, got, want)
		}
	}
}

func TestPutDomainRejectsInvalidFormat(t *testing.T) {
	repo := newFakeDomainRepo(School{ID: 1})
	svc := newDomainServiceForTest(repo, NewHostResolver(repo, "localhost"), "", "localhost", nil)
	_, err := svc.PutDomain(context.Background(), 1, 1, "bukan domain valid")
	if err == nil {
		t.Fatal("expected error format domain tidak valid")
	}
	de := errAsHTTPX(t, err)
	if de.Code != "validation" {
		t.Fatalf("expected validation error, dapat %q", de.Code)
	}
}

func TestPutDomainRejectsDuplicateAcrossSchools(t *testing.T) {
	repo := newFakeDomainRepo(
		School{ID: 1, CustomDomain: "sudahdipakai.sch.id"},
		School{ID: 2},
	)
	svc := newDomainServiceForTest(repo, NewHostResolver(repo, "localhost"), "", "localhost", nil)
	_, err := svc.PutDomain(context.Background(), 1, 2, "sudahdipakai.sch.id")
	if err == nil {
		t.Fatal("expected error domain sudah dipakai")
	}
	de := errAsHTTPX(t, err)
	if de.Status != 409 || de.Code != "domain_taken" {
		t.Fatalf("expected 409 domain_taken, dapat status=%d code=%q", de.Status, de.Code)
	}
}

func TestPutDomainAllowsResavingOwnDomain(t *testing.T) {
	repo := newFakeDomainRepo(School{ID: 1, PendingDomain: "punyaku.sch.id"})
	svc := newDomainServiceForTest(repo, NewHostResolver(repo, "localhost"), "", "localhost", nil)
	status, err := svc.PutDomain(context.Background(), 1, 1, "punyaku.sch.id")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status.Verified {
		t.Fatal("domain baru diisi seharusnya belum verified")
	}
	if status.Domain != "punyaku.sch.id" {
		t.Fatalf("domain salah: %+v", status)
	}
}

func TestPutDomainSetsPending(t *testing.T) {
	repo := newFakeDomainRepo(School{ID: 1})
	svc := newDomainServiceForTest(repo, NewHostResolver(repo, "localhost"), "203.0.113.10", "localhost", nil)
	status, err := svc.PutDomain(context.Background(), 1, 1, "SekolahDemo.sch.id")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status.Verified {
		t.Fatal("baru PUT domain seharusnya belum verified")
	}
	if status.Domain != "sekolahdemo.sch.id" {
		t.Fatalf("domain seharusnya di-lowercase, dapat %q", status.Domain)
	}
	if status.ServerIP != "203.0.113.10" {
		t.Fatalf("server_ip salah: %q", status.ServerIP)
	}
	if status.Instructions == "" {
		t.Fatal("instructions seharusnya terisi")
	}
}

// TestVerifyDomainFailsClearlyWithoutServerIP — docs tugas: "verify (custom
// domain) gagal dengan pesan SERVER_IP belum diset".
func TestVerifyDomainFailsClearlyWithoutServerIP(t *testing.T) {
	repo := newFakeDomainRepo(School{ID: 1, PendingDomain: "sekolahdemo.sch.id"})
	// serverIP kosong (default dev, docs tugas) — lookupHost TIDAK BOLEH
	// dipanggil sama sekali (gagal sebelum menyentuh jaringan).
	lookupCalled := false
	svc := newDomainServiceForTest(repo, NewHostResolver(repo, "localhost"), "", "localhost", func(string) ([]string, error) {
		lookupCalled = true
		return nil, errors.New("tidak seharusnya dipanggil")
	})
	_, err := svc.VerifyDomain(context.Background(), 1, 1)
	if err == nil {
		t.Fatal("expected error karena SERVER_IP kosong")
	}
	de := errAsHTTPX(t, err)
	if de.Code != "validation" {
		t.Fatalf("expected validation error, dapat %q", de.Code)
	}
	if !strings.Contains(strings.ToUpper(de.Message), "SERVER_IP") {
		t.Fatalf("pesan error seharusnya menyebut SERVER_IP dengan jelas, dapat: %q", de.Message)
	}
	if lookupCalled {
		t.Fatal("lookupHost TIDAK BOLEH dipanggil saat SERVER_IP kosong")
	}
}

func TestVerifyDomainWithoutPendingDomainFails(t *testing.T) {
	repo := newFakeDomainRepo(School{ID: 1})
	svc := newDomainServiceForTest(repo, NewHostResolver(repo, "localhost"), "203.0.113.10", "localhost", nil)
	if _, err := svc.VerifyDomain(context.Background(), 1, 1); err == nil {
		t.Fatal("expected error karena belum ada pending_domain")
	}
}

func TestVerifyDomainSucceedsWhenIPMatches(t *testing.T) {
	repo := newFakeDomainRepo(School{ID: 1, PendingDomain: "sekolahdemo.sch.id"})
	resolver := NewHostResolver(repo, "localhost")
	svc := newDomainServiceForTest(repo, resolver, "203.0.113.10", "localhost", func(host string) ([]string, error) {
		if host != "sekolahdemo.sch.id" {
			t.Fatalf("lookupHost dipanggil dengan host salah: %q", host)
		}
		return []string{"203.0.113.10"}, nil
	})
	status, err := svc.VerifyDomain(context.Background(), 1, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !status.Verified || status.Domain != "sekolahdemo.sch.id" {
		t.Fatalf("status seharusnya verified: %+v", status)
	}
	sch, _ := repo.SchoolByID(context.Background(), 1)
	if sch.CustomDomain != "sekolahdemo.sch.id" || sch.PendingDomain != "" {
		t.Fatalf("pending_domain seharusnya pindah ke custom_domain: %+v", sch)
	}
}

func TestVerifyDomainFailsWhenIPMismatch(t *testing.T) {
	repo := newFakeDomainRepo(School{ID: 1, PendingDomain: "sekolahdemo.sch.id"})
	svc := newDomainServiceForTest(repo, NewHostResolver(repo, "localhost"), "203.0.113.10", "localhost", func(host string) ([]string, error) {
		return []string{"198.51.100.1"}, nil
	})
	_, err := svc.VerifyDomain(context.Background(), 1, 1)
	if err == nil {
		t.Fatal("expected error IP tidak cocok")
	}
	sch, _ := repo.SchoolByID(context.Background(), 1)
	if sch.CustomDomain != "" || sch.PendingDomain != "sekolahdemo.sch.id" {
		t.Fatalf("domain TIDAK BOLEH berubah saat verifikasi gagal: %+v", sch)
	}
}

func TestDeleteDomainClearsBoth(t *testing.T) {
	repo := newFakeDomainRepo(School{ID: 1, CustomDomain: "aktif.sch.id"})
	svc := newDomainServiceForTest(repo, NewHostResolver(repo, "localhost"), "203.0.113.10", "localhost", nil)
	if err := svc.DeleteDomain(context.Background(), 1, 1); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	sch, _ := repo.SchoolByID(context.Background(), 1)
	if sch.CustomDomain != "" {
		t.Fatalf("custom_domain seharusnya kosong: %+v", sch)
	}
}

func TestDeleteDomainWithoutDomainFails(t *testing.T) {
	repo := newFakeDomainRepo(School{ID: 1})
	svc := newDomainServiceForTest(repo, NewHostResolver(repo, "localhost"), "", "localhost", nil)
	if err := svc.DeleteDomain(context.Background(), 1, 1); err == nil {
		t.Fatal("expected error karena belum punya domain")
	}
}
