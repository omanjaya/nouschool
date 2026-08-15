package tenant

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"regexp"
	"strings"

	"github.com/omanjaya/nouschool/internal/platform/httpx"
)

// Berkas ini mengurus custom domain end-to-end (Fase 11, docs/01-tenant.md
// "Custom domain & Caddy"): admin sekolah isi domain -> disimpan PENDING ->
// verifikasi DNS (bandingkan A record dengan SERVER_IP) -> pindah jadi
// custom_domain aktif. `/internal/check-domain` (dipakai Caddy On-Demand TLS)
// TIDAK berubah — sudah benar sejak Fase 1 (hanya domain verified yang lolos,
// lihat Handler.CheckDomain di handler.go).

// domainHostRe — format hostname umum: label alfanumerik+hubung dipisah
// titik, TLD minimal 2 huruf. Sengaja longgar (tidak validasi TLD spesifik)
// supaya domain custom apa pun (.sch.id, .id, .com, dst) diterima.
var domainHostRe = regexp.MustCompile(`^([a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z]{2,63}$`)

func validDomainFormat(domain string) bool {
	if len(domain) < 4 || len(domain) > 253 {
		return false
	}
	return domainHostRe.MatchString(domain)
}

// errDomainTaken — 409 (bukan 422 validasi biasa): domain sudah dipakai
// sekolah lain, klien perlu tahu ini konflik data, bukan salah format.
var errDomainTaken = &httpx.Error{Status: http.StatusConflict, Code: "domain_taken", Message: "Domain ini sudah dipakai sekolah lain."}

// CustomDomainStatus — shape response GET/PUT/POST verify /api/custom-domain.
type CustomDomainStatus struct {
	Domain       string `json:"domain"` // "" bila sekolah belum pernah isi domain
	Verified     bool   `json:"verified"`
	ServerIP     string `json:"server_ip"`
	Instructions string `json:"instructions"`
}

// domainRepository adalah kontrak yang dibutuhkan DomainService dari
// Repository — dideklarasikan sebagai interface (dipenuhi *Repository secara
// struktural) supaya DomainService bisa dites dengan fake in-memory, tanpa
// DB (lihat domain_test.go), sama seperti pola attendanceRepository/
// leaveRepository/studentRepository di modul lain.
type domainRepository interface {
	SchoolByID(ctx context.Context, id int64) (School, error)
	DomainUsedByOtherSchool(ctx context.Context, domain string, excludeSchoolID int64) (bool, error)
	SetPendingDomain(ctx context.Context, schoolID int64, domain string) (School, error)
	VerifyPendingDomain(ctx context.Context, schoolID int64) (School, error)
	ClearDomain(ctx context.Context, schoolID int64) (School, error)
}

var _ domainRepository = (*Repository)(nil)

// DomainService mengurus custom domain milik sekolah. Dipisah dari Service
// (CRUD sekolah/tahun ajaran) supaya berkas ini bisa dibaca berdiri sendiri —
// tetap memakai Repository & AuditLogger yang sama.
type DomainService struct {
	repo       domainRepository
	resolver   *HostResolver
	audit      AuditLogger
	serverIP   string
	baseDomain string
	// lookupHost — dibungkus supaya bisa diganti fake di test (verifikasi DNS
	// TANPA menyentuh jaringan asli). Default net.LookupHost lewat NewDomainService.
	lookupHost func(host string) ([]string, error)
}

func NewDomainService(repo *Repository, resolver *HostResolver, audit AuditLogger, serverIP, baseDomain string) *DomainService {
	return &DomainService{
		repo: repo, resolver: resolver, audit: audit,
		serverIP: strings.TrimSpace(serverIP), baseDomain: strings.ToLower(strings.TrimSpace(baseDomain)),
		lookupHost: net.LookupHost,
	}
}

// newDomainServiceForTest membangun DomainService dengan repository FAKE
// (in-memory, tanpa DB) & lookupHost bisa diganti — dipakai test di package
// ini saja (domain_test.go).
func newDomainServiceForTest(repo domainRepository, resolver *HostResolver, serverIP, baseDomain string, lookupHost func(string) ([]string, error)) *DomainService {
	if lookupHost == nil {
		lookupHost = net.LookupHost
	}
	return &DomainService{repo: repo, resolver: resolver, serverIP: serverIP, baseDomain: baseDomain, lookupHost: lookupHost}
}

func (d *DomainService) instructions() string {
	if d.serverIP == "" {
		return "SERVER_IP belum diset di server platform — verifikasi domain tidak bisa dilakukan sampai dikonfigurasi admin platform."
	}
	return fmt.Sprintf("Arahkan DNS A record domain Anda ke IP %s, lalu klik \"Verifikasi\". Perubahan DNS bisa butuh waktu hingga 24 jam untuk merambat sebelum terdeteksi.", d.serverIP)
}

func (d *DomainService) statusFor(sch School) CustomDomainStatus {
	st := CustomDomainStatus{ServerIP: d.serverIP, Instructions: d.instructions()}
	switch {
	case sch.CustomDomain != "":
		st.Domain, st.Verified = sch.CustomDomain, true
	case sch.PendingDomain != "":
		st.Domain, st.Verified = sch.PendingDomain, false
	}
	return st
}

// GetStatus — GET /api/custom-domain.
func (d *DomainService) GetStatus(ctx context.Context, schoolID int64) (CustomDomainStatus, error) {
	sch, err := d.repo.SchoolByID(ctx, schoolID)
	if err != nil {
		return CustomDomainStatus{}, err
	}
	return d.statusFor(sch), nil
}

// PutDomain — PUT /api/custom-domain {domain}: validasi format + unik lintas
// sekolah, simpan sebagai PENDING (belum aktif sampai POST verify sukses).
func (d *DomainService) PutDomain(ctx context.Context, actorUserID, schoolID int64, domain string) (CustomDomainStatus, error) {
	domain = strings.ToLower(strings.TrimSpace(domain))
	if domain == "" {
		return CustomDomainStatus{}, httpx.Validation("Domain wajib diisi.")
	}
	if !validDomainFormat(domain) {
		return CustomDomainStatus{}, httpx.Validation("Format domain tidak valid. Contoh: sekolahku.sch.id")
	}
	if domain == d.baseDomain || strings.HasSuffix(domain, "."+d.baseDomain) {
		return CustomDomainStatus{}, httpx.Validation("Domain tidak boleh memakai subdomain platform NouSchool.")
	}
	used, err := d.repo.DomainUsedByOtherSchool(ctx, domain, schoolID)
	if err != nil {
		return CustomDomainStatus{}, err
	}
	if used {
		return CustomDomainStatus{}, errDomainTaken
	}

	sch, err := d.repo.SetPendingDomain(ctx, schoolID, domain)
	if err != nil {
		return CustomDomainStatus{}, err
	}
	if d.audit != nil {
		sid, uid := schoolID, actorUserID
		_ = d.audit.Log(ctx, &sid, &uid, "school.domain_set_pending", "school", &schoolID, nil, map[string]any{"pending_domain": domain})
	}
	return d.statusFor(sch), nil
}

// VerifyDomain — POST /api/custom-domain/verify: resolve DNS domain pending,
// bandingkan dengan SERVER_IP. SERVER_IP kosong -> gagal SEGERA dengan pesan
// jelas (TANPA mencoba resolve DNS), sesuai keputusan scope tugas ("dev
// default kosong = verifikasi selalu gagal dengan pesan jelas").
func (d *DomainService) VerifyDomain(ctx context.Context, actorUserID, schoolID int64) (CustomDomainStatus, error) {
	sch, err := d.repo.SchoolByID(ctx, schoolID)
	if err != nil {
		return CustomDomainStatus{}, err
	}
	if sch.PendingDomain == "" {
		return CustomDomainStatus{}, httpx.Validation("Belum ada domain yang menunggu verifikasi. Isi domain dulu lewat PUT /api/custom-domain.")
	}
	if d.serverIP == "" {
		return CustomDomainStatus{}, httpx.Validation("SERVER_IP belum diset di server platform — verifikasi tidak bisa dilakukan. Hubungi admin platform untuk mengonfigurasi SERVER_IP.")
	}

	ips, err := d.lookupHost(sch.PendingDomain)
	if err != nil {
		return CustomDomainStatus{}, httpx.Validation(fmt.Sprintf(
			"Gagal me-resolve DNS domain %q: %s. Pastikan A record domain sudah diarahkan ke %s, lalu coba lagi.",
			sch.PendingDomain, err.Error(), d.serverIP))
	}
	matched := false
	for _, ip := range ips {
		if ip == d.serverIP {
			matched = true
			break
		}
	}
	if !matched {
		return CustomDomainStatus{}, httpx.Validation(fmt.Sprintf(
			"Domain %q belum mengarah ke IP %s (DNS saat ini menunjuk ke: %s). Perubahan DNS bisa butuh waktu hingga 24 jam — coba lagi setelah itu.",
			sch.PendingDomain, d.serverIP, strings.Join(ips, ", ")))
	}

	verifiedDomain := sch.PendingDomain
	updated, err := d.repo.VerifyPendingDomain(ctx, schoolID)
	if err != nil {
		return CustomDomainStatus{}, err
	}
	// Invalidate cache resolver supaya domain langsung bisa diakses tanpa
	// menunggu TTL 60 detik habis (docs/01: "perubahan domain TIDAK butuh
	// restart Caddy" — resolver in-process kita juga harus segera update).
	d.resolver.Invalidate(verifiedDomain)
	if d.audit != nil {
		sid, uid := schoolID, actorUserID
		_ = d.audit.Log(ctx, &sid, &uid, "school.domain_verified", "school", &schoolID, nil, map[string]any{"custom_domain": verifiedDomain})
	}
	return d.statusFor(updated), nil
}

// DeleteDomain — DELETE /api/custom-domain: lepas domain sendiri (aktif
// maupun yang masih pending sekaligus).
func (d *DomainService) DeleteDomain(ctx context.Context, actorUserID, schoolID int64) error {
	sch, err := d.repo.SchoolByID(ctx, schoolID)
	if err != nil {
		return err
	}
	if sch.CustomDomain == "" && sch.PendingDomain == "" {
		return httpx.Validation("Sekolah ini belum punya custom domain.")
	}
	if _, err := d.repo.ClearDomain(ctx, schoolID); err != nil {
		return err
	}
	d.resolver.Invalidate(sch.CustomDomain)
	d.resolver.Invalidate(sch.PendingDomain)
	if d.audit != nil {
		sid, uid := schoolID, actorUserID
		_ = d.audit.Log(ctx, &sid, &uid, "school.domain_removed", "school", &schoolID,
			map[string]any{"custom_domain": sch.CustomDomain, "pending_domain": sch.PendingDomain}, nil)
	}
	return nil
}
