package tenant

import (
	"net/http"
	"strconv"

	"github.com/omanjaya/nouschool/internal/platform/httpx"
	"github.com/omanjaya/nouschool/internal/platform/reqctx"
)

// Handler menerjemahkan HTTP <-> Service. Tidak menyusun SQL, tidak
// memutuskan aturan bisnis — hanya decode/validate bentuk + panggil service.
type Handler struct {
	svc      *Service
	settings *SettingsService
	resolver *HostResolver
}

func NewHandler(svc *Service, settings *SettingsService, resolver *HostResolver) *Handler {
	return &Handler{svc: svc, settings: settings, resolver: resolver}
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
// (dijaga middleware requirePerm("settings:manage") di routes.go).
func (h *Handler) PutSettings(w http.ResponseWriter, r *http.Request) {
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
	if err := h.settings.Put(ctx, reqctx.SchoolID(ctx), reqctx.UserID(ctx), module, dst); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, dst)
}
