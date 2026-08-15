package tenant

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/omanjaya/nouschool/internal/platform/httpx"
	"github.com/omanjaya/nouschool/internal/platform/reqctx"
	"github.com/omanjaya/nouschool/internal/platform/storage"
)

// Handler menerjemahkan HTTP <-> Service. Tidak menyusun SQL, tidak
// memutuskan aturan bisnis — hanya decode/validate bentuk + panggil service.
type Handler struct {
	svc      *Service
	settings *SettingsService
	resolver *HostResolver
	domain   *DomainService
	interest *InterestService
	files    *storage.Store
}

func NewHandler(svc *Service, settings *SettingsService, resolver *HostResolver, domain *DomainService, interest *InterestService, files *storage.Store) *Handler {
	return &Handler{svc: svc, settings: settings, resolver: resolver, domain: domain, interest: interest, files: files}
}

// CheckDomain dipanggil Caddy (On-Demand TLS) untuk memvalidasi domain.
// 200 hanya jika domain dikenal (subdomain slug atau custom domain) dan
// sekolahnya berstatus active.
func (h *Handler) CheckDomain(w http.ResponseWriter, r *http.Request) {
	domain := StripPort(r.URL.Query().Get("domain"))
	if domain == "" {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	resolved, err := h.resolver.Resolve(r.Context(), domain)
	if err != nil || resolved.IsPlatform {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	if resolved.School.Status != "active" {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusOK)
}

type createSchoolRequest struct {
	Name     string `json:"name"`
	Slug     string `json:"slug"`
	Timezone string `json:"timezone"`
}

func (h *Handler) ListSchools(w http.ResponseWriter, r *http.Request) {
	schools, err := h.svc.ListSchools(r.Context())
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, schools)
}

func (h *Handler) CreateSchool(w http.ResponseWriter, r *http.Request) {
	var req createSchoolRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.WriteError(w, err)
		return
	}
	sch, err := h.svc.CreateSchool(r.Context(), reqctx.UserID(r.Context()), req.Name, req.Slug, req.Timezone)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, sch)
}

type updateSchoolRequest struct {
	Name         *string `json:"name"`
	CustomDomain *string `json:"custom_domain"`
	Timezone     *string `json:"timezone"`
	Status       *string `json:"status"`
}

func pathInt64(r *http.Request, name string) (int64, error) {
	return strconv.ParseInt(r.PathValue(name), 10, 64)
}

func (h *Handler) UpdateSchool(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		httpx.WriteError(w, httpx.Validation("ID sekolah tidak valid."))
		return
	}
	var req updateSchoolRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.WriteError(w, err)
		return
	}
	in := UpdateSchoolInput{}
	if req.Name != nil {
		in.Name = *req.Name
	}
	if req.CustomDomain != nil {
		in.CustomDomain = *req.CustomDomain
	}
	if req.Timezone != nil {
		in.Timezone = *req.Timezone
	}
	if req.Status != nil {
		in.Status = *req.Status
	}
	sch, err := h.svc.UpdateSchool(r.Context(), reqctx.UserID(r.Context()), id, in)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, sch)
}

type createAcademicYearRequest struct {
	Name     string `json:"name"`
	StartsOn Date   `json:"starts_on"`
	EndsOn   Date   `json:"ends_on"`
}

func (h *Handler) ListAcademicYears(w http.ResponseWriter, r *http.Request) {
	schoolID, err := pathInt64(r, "id")
	if err != nil {
		httpx.WriteError(w, httpx.Validation("ID sekolah tidak valid."))
		return
	}
	years, err := h.svc.ListAcademicYears(r.Context(), schoolID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, years)
}

func (h *Handler) CreateAcademicYear(w http.ResponseWriter, r *http.Request) {
	schoolID, err := pathInt64(r, "id")
	if err != nil {
		httpx.WriteError(w, httpx.Validation("ID sekolah tidak valid."))
		return
	}
	var req createAcademicYearRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.WriteError(w, err)
		return
	}
	startsOn := req.StartsOn.Time
	endsOn := req.EndsOn.Time
	if startsOn.IsZero() || endsOn.IsZero() {
		httpx.WriteError(w, httpx.Validation("starts_on dan ends_on wajib diisi format YYYY-MM-DD."))
		return
	}
	ay, err := h.svc.CreateAcademicYear(r.Context(), reqctx.UserID(r.Context()), schoolID, req.Name, startsOn, endsOn)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, ay)
}

func (h *Handler) ActivateAcademicYear(w http.ResponseWriter, r *http.Request) {
	schoolID, err := pathInt64(r, "id")
	if err != nil {
		httpx.WriteError(w, httpx.Validation("ID sekolah tidak valid."))
		return
	}
	ayID, err := pathInt64(r, "ayID")
	if err != nil {
		httpx.WriteError(w, httpx.Validation("ID tahun ajaran tidak valid."))
		return
	}
	ay, err := h.svc.ActivateAcademicYear(r.Context(), reqctx.UserID(r.Context()), schoolID, ayID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, ay)
}

// ListAcademicYearsForSchool — GET /api/academic-years. Daftar tahun ajaran
// SEKOLAH SENDIRI (school_id dari reqctx, host tenant), dipakai UI untuk
// filter (bukan panel admin lintas sekolah — beda dari ListAcademicYears
// yang dipakai /api/admin/schools/{id}/academic-years).
func (h *Handler) ListAcademicYearsForSchool(w http.ResponseWriter, r *http.Request) {
	years, err := h.svc.ListAcademicYears(r.Context(), reqctx.SchoolID(r.Context()))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, years)
}

// GetSettings — GET /api/settings/{module}.
func (h *Handler) GetSettings(w http.ResponseWriter, r *http.Request) {
	module := r.PathValue("module")
	dst, ok := NewModuleSettings(module)
	if !ok {
		httpx.WriteError(w, httpx.Validation("Module settings tidak dikenal: "+module))
		return
	}
	if err := h.settings.Get(r.Context(), reqctx.SchoolID(r.Context()), module, dst); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, dst)
}

// PutSettings — PUT /api/settings/{module}. Butuh role admin_sekolah
// (dijaga middleware requirePerm("settings:manage") di routes.go). Module
// yang ditandai IsSuperAdminOnlyModule (mis. "notification") DITOLAK di sini
// apa pun rolenya — mutasi module itu HANYA lewat
// GET/PUT /api/admin/schools/{id}/settings/{module} (lihat AdminPutSettings
// di bawah), sesuai docs/08-notification.md.
func (h *Handler) PutSettings(w http.ResponseWriter, r *http.Request) {
	module := r.PathValue("module")
	if IsSuperAdminOnlyModule(module) {
		httpx.WriteError(w, httpx.ErrForbidden)
		return
	}
	dst, ok := NewModuleSettings(module)
	if !ok {
		httpx.WriteError(w, httpx.Validation("Module settings tidak dikenal: "+module))
		return
	}
	if err := httpx.Decode(r, dst); err != nil {
		httpx.WriteError(w, err)
		return
	}
	ctx := r.Context()
	if err := h.settings.Put(ctx, reqctx.SchoolID(ctx), reqctx.UserID(ctx), module, dst); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, dst)
}

// AdminGetSettings — GET /api/admin/schools/{id}/settings/{module}. Host
// PLATFORM, super admin saja (dijaga middleware requireSuperAdmin di
// routes.go) — dipakai super admin mengelola settings sekolah MANAPUN, mis.
// module "notification" yang tidak bisa diubah admin sekolah sendiri
// (docs/08-notification.md).
func (h *Handler) AdminGetSettings(w http.ResponseWriter, r *http.Request) {
	schoolID, err := pathInt64(r, "id")
	if err != nil {
		httpx.WriteError(w, httpx.Validation("ID sekolah tidak valid."))
		return
	}
	module := r.PathValue("module")
	dst, ok := NewModuleSettings(module)
	if !ok {
		httpx.WriteError(w, httpx.Validation("Module settings tidak dikenal: "+module))
		return
	}
	if err := h.settings.Get(r.Context(), schoolID, module, dst); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, dst)
}

// AdminPutSettings — PUT /api/admin/schools/{id}/settings/{module}. Lihat AdminGetSettings.
func (h *Handler) AdminPutSettings(w http.ResponseWriter, r *http.Request) {
	schoolID, err := pathInt64(r, "id")
	if err != nil {
		httpx.WriteError(w, httpx.Validation("ID sekolah tidak valid."))
		return
	}
	module := r.PathValue("module")
	dst, ok := NewModuleSettings(module)
	if !ok {
		httpx.WriteError(w, httpx.Validation("Module settings tidak dikenal: "+module))
		return
	}
	if err := httpx.Decode(r, dst); err != nil {
		httpx.WriteError(w, err)
		return
	}
	ctx := r.Context()
	if err := h.settings.Put(ctx, schoolID, reqctx.UserID(ctx), module, dst); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, dst)
}

// -- branding: upload logo (Fase 11, docs/01-tenant.md "Branding per sekolah") --

const maxBrandingLogoBytes = 2 << 20 // 2 MB

var brandingLogoExtByMime = map[string]string{
	"image/png":  "png",
	"image/jpeg": "jpg",
}

// UploadBrandingLogo — POST /api/settings/branding/logo (perm settings:manage,
// multipart field "file", png/jpg <= 2MB). Menyimpan file via platform/storage
// di "{school_id}/branding/logo.{ext}", lalu memuat settings branding
// TERSIMPAN SAAT INI dan menimpa HANYA field logo_path — app_name/
// primary_color yang sudah tersimpan TIDAK ikut ter-reset (beda dari
// PutSettings generik yang replace-all, lihat catatan di branding.go).
func (h *Handler) UploadBrandingLogo(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(maxBrandingLogoBytes); err != nil {
		httpx.WriteError(w, httpx.Validation("Gagal membaca form upload."))
		return
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		httpx.WriteError(w, httpx.Validation("File wajib diunggah (field 'file')."))
		return
	}
	defer file.Close()
	content, err := io.ReadAll(io.LimitReader(file, maxBrandingLogoBytes+1))
	if err != nil {
		httpx.WriteError(w, httpx.Validation("Gagal membaca isi file."))
		return
	}
	if len(content) > maxBrandingLogoBytes {
		httpx.WriteError(w, httpx.Validation("Ukuran logo maksimal 2MB."))
		return
	}
	mime := http.DetectContentType(content)
	ext, ok := brandingLogoExtByMime[mime]
	if !ok {
		httpx.WriteError(w, httpx.Validation("Logo harus berformat PNG atau JPG."))
		return
	}

	ctx := r.Context()
	schoolID := reqctx.SchoolID(ctx)

	var branding BrandingSettings
	if err := h.settings.Get(ctx, schoolID, "branding", &branding); err != nil {
		httpx.WriteError(w, err)
		return
	}
	if branding.AppName == "" {
		def := DefaultBrandingSettings()
		branding = def
	}

	relPath := logoRelPath(schoolID, ext)
	if err := h.files.Save(relPath, bytes.NewReader(content)); err != nil {
		httpx.WriteError(w, err)
		return
	}
	branding.LogoPath = relPath

	if err := h.settings.Put(ctx, schoolID, reqctx.UserID(ctx), "branding", &branding); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, branding)
}

// -- endpoint publik (TANPA auth) — Fase 11 --

// brandingPublicView adalah potongan branding dikirim ke klien publik (logo
// SEBAGAI URL endpoint stream, bukan path disk internal — lihat catatan di
// branding.go).
type brandingPublicView struct {
	AppName      string  `json:"app_name"`
	PrimaryColor string  `json:"primary_color"`
	LogoURL      *string `json:"logo_url"`
}

func logoRelPath(schoolID int64, ext string) string {
	return strconv.FormatInt(schoolID, 10) + "/branding/logo." + ext
}

const publicLogoURL = "/api/public/branding/logo"

// brandingLogoURLFor mengembalikan URL endpoint stream logo (bukan path disk
// internal) bila branding.LogoPath terisi, else nil (dipakai encode JSON
// "logo_url": null). Fungsi MURNI — dites langsung tanpa DB/HTTP (lihat
// handler_test.go).
func brandingLogoURLFor(branding BrandingSettings) *string {
	if branding.LogoPath == "" {
		return nil
	}
	u := publicLogoURL
	return &u
}

// brandingPublicViewFor mengisi default bila branding belum pernah disimpan
// (AppName kosong = sekolah belum pernah PUT settings branding). Fungsi
// MURNI — dites langsung tanpa DB/HTTP.
func brandingPublicViewFor(branding BrandingSettings) brandingPublicView {
	if branding.AppName == "" {
		branding = DefaultBrandingSettings()
	}
	return brandingPublicView{AppName: branding.AppName, PrimaryColor: branding.PrimaryColor, LogoURL: brandingLogoURLFor(branding)}
}

// PublicContext — GET /api/public/context (PUBLIK). Host platform ->
// {"platform":true}; host tenant -> {"platform":false,"school":{...},
// "branding":{...}} (dipakai landing/branding client sebelum login — lihat
// docs/01-tenant.md & docs tugas Fase 11).
func (h *Handler) PublicContext(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if reqctx.IsPlatform(ctx) {
		httpx.JSON(w, http.StatusOK, map[string]any{"platform": true})
		return
	}
	sch, ok := reqctx.SchoolFromContext(ctx)
	if !ok {
		httpx.JSON(w, http.StatusOK, map[string]any{"platform": true})
		return
	}
	var branding BrandingSettings
	if err := h.settings.Get(ctx, sch.ID, "branding", &branding); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{
		"platform": false,
		"school":   map[string]any{"name": sch.Name, "slug": sch.Slug},
		"branding": brandingPublicViewFor(branding),
	})
}

// contentTypeForExt — logo hanya disimpan sebagai png/jpg (lihat
// brandingLogoExtByMime), jadi cukup pemetaan sederhana dari ekstensi.
func contentTypeForExt(relPath string) string {
	switch {
	case strings.HasSuffix(relPath, ".png"):
		return "image/png"
	case strings.HasSuffix(relPath, ".jpg"), strings.HasSuffix(relPath, ".jpeg"):
		return "image/jpeg"
	default:
		return "application/octet-stream"
	}
}

// PublicBrandingLogo — GET /api/public/branding/logo (PUBLIK, host tenant).
// 404 bila host platform, atau sekolah belum pernah upload logo.
func (h *Handler) PublicBrandingLogo(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sch, ok := reqctx.SchoolFromContext(ctx)
	if !ok {
		httpx.WriteError(w, httpx.ErrNotFound)
		return
	}
	var branding BrandingSettings
	if err := h.settings.Get(ctx, sch.ID, "branding", &branding); err != nil {
		httpx.WriteError(w, err)
		return
	}
	if branding.LogoPath == "" {
		httpx.WriteError(w, httpx.ErrNotFound)
		return
	}
	f, err := h.files.Open(branding.LogoPath)
	if err != nil {
		httpx.WriteError(w, httpx.ErrNotFound)
		return
	}
	defer f.Close()
	w.Header().Set("Content-Type", contentTypeForExt(branding.LogoPath))
	w.Header().Set("Cache-Control", "public, max-age=300")
	_, _ = io.Copy(w, f)
}

// manifestIcon & webManifest — shape JSON standar web app manifest (subset
// yang dipakai vite-plugin-pwa/browser, docs/01-tenant.md "PWA manifest dinamis").
type manifestIcon struct {
	Src   string `json:"src"`
	Sizes string `json:"sizes"`
	Type  string `json:"type"`
}

type webManifest struct {
	Name       string         `json:"name"`
	ShortName  string         `json:"short_name"`
	ThemeColor string         `json:"theme_color"`
	Icons      []manifestIcon `json:"icons"`
	Display    string         `json:"display"`
	StartURL   string         `json:"start_url"`
}

const defaultManifestIcon = "/pwa-icon.svg"

// defaultManifest — manifest platform NouSchool (host platform, atau
// fallback bila konteks tenant tidak tersedia).
func defaultManifest() webManifest {
	return webManifest{
		Name: "NouSchool", ShortName: "NouSchool", ThemeColor: DefaultBrandingSettings().PrimaryColor,
		Icons:   []manifestIcon{{Src: defaultManifestIcon, Sizes: "any", Type: "image/svg+xml"}},
		Display: "standalone", StartURL: "/",
	}
}

// manifestForBranding — manifest host tenant dari branding sekolah (Fase 11,
// docs/01-tenant.md "PWA manifest dinamis"): name/short_name = app_name,
// theme_color = primary_color, icons = logo (satu entri sizes "any") bila
// ada, else ikon default. Fungsi MURNI — dites langsung tanpa DB/HTTP (lihat
// handler_test.go).
func manifestForBranding(branding BrandingSettings) webManifest {
	if branding.AppName == "" {
		branding = DefaultBrandingSettings()
	}
	m := defaultManifest()
	m.Name, m.ShortName, m.ThemeColor = branding.AppName, branding.AppName, branding.PrimaryColor
	if branding.LogoPath != "" {
		m.Icons = []manifestIcon{{Src: publicLogoURL, Sizes: "any", Type: contentTypeForExt(branding.LogoPath)}}
	}
	return m
}

// ManifestWebmanifest — GET /manifest.webmanifest (PUBLIK). Host platform ->
// manifest default NouSchool; host tenant -> manifest dari branding sekolah.
func (h *Handler) ManifestWebmanifest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	m := defaultManifest()

	if sch, ok := reqctx.SchoolFromContext(ctx); ok && !reqctx.IsPlatform(ctx) {
		var branding BrandingSettings
		if err := h.settings.Get(ctx, sch.ID, "branding", &branding); err == nil {
			m = manifestForBranding(branding)
		}
	}

	w.Header().Set("Content-Type", "application/manifest+json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(m)
}

// -- custom domain (Fase 11, docs/01-tenant.md "Custom domain & Caddy") --

// GetCustomDomain — GET /api/custom-domain (perm settings:manage).
func (h *Handler) GetCustomDomain(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	status, err := h.domain.GetStatus(ctx, reqctx.SchoolID(ctx))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, status)
}

type putCustomDomainRequest struct {
	Domain string `json:"domain"`
}

// PutCustomDomain — PUT /api/custom-domain {domain} (perm settings:manage).
func (h *Handler) PutCustomDomain(w http.ResponseWriter, r *http.Request) {
	var req putCustomDomainRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.WriteError(w, err)
		return
	}
	ctx := r.Context()
	status, err := h.domain.PutDomain(ctx, reqctx.UserID(ctx), reqctx.SchoolID(ctx), req.Domain)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, status)
}

// VerifyCustomDomain — POST /api/custom-domain/verify (perm settings:manage).
func (h *Handler) VerifyCustomDomain(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	status, err := h.domain.VerifyDomain(ctx, reqctx.UserID(ctx), reqctx.SchoolID(ctx))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, status)
}

// DeleteCustomDomain — DELETE /api/custom-domain (perm settings:manage).
func (h *Handler) DeleteCustomDomain(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if err := h.domain.DeleteDomain(ctx, reqctx.UserID(ctx), reqctx.SchoolID(ctx)); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

// -- registrasi minat sekolah (Fase 11, landing page) --

type submitInterestRequest struct {
	SchoolName  string `json:"school_name"`
	ContactName string `json:"contact_name"`
	Phone       string `json:"phone"`
	Email       string `json:"email"`
	Note        string `json:"note"`
}

// clientIP mengambil IP klien dari RemoteAddr (host:port) — dipakai rate
// limit per IP (sama pola dengan internal/student.clientIP untuk aktivasi).
func clientIP(r *http.Request) string {
	host := r.RemoteAddr
	if idx := strings.LastIndex(host, ":"); idx > 0 {
		return host[:idx]
	}
	return host
}

// SubmitInterest — POST /api/public/interest (PUBLIK, host platform,
// rate-limit 3/jam per IP).
func (h *Handler) SubmitInterest(w http.ResponseWriter, r *http.Request) {
	var req submitInterestRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.WriteError(w, err)
		return
	}
	lead, err := h.interest.SubmitInterest(r.Context(), SubmitInterestInput{
		SchoolName: req.SchoolName, ContactName: req.ContactName, Phone: req.Phone,
		Email: req.Email, Note: req.Note, ClientIP: clientIP(r),
	})
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, lead)
}

// AdminListInterest — GET /api/admin/interest (super admin).
func (h *Handler) AdminListInterest(w http.ResponseWriter, r *http.Request) {
	items, err := h.interest.ListInterest(r.Context())
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, items)
}
