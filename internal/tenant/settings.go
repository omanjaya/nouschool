package tenant

import (
	"context"
	"encoding/json"

	"github.com/omanjaya/nouschool/internal/attendance"
	"github.com/omanjaya/nouschool/internal/leave"
	"github.com/omanjaya/nouschool/internal/notification"
	"github.com/omanjaya/nouschool/internal/teaching"
)

// Settings adalah kontrak yang wajib dipenuhi tiap struct settings modul
// (satu pola dipakai semua modul — lihat CLAUDE.md & docs/01-tenant.md).
type Settings interface {
	Validate() error
}

// SettingsService adalah implementasi generik pola school_settings:
// baca (dengan default bila belum ada baris) dan tulis (validasi + audit).
type SettingsService struct {
	repo  *Repository
	audit AuditLogger
}

func NewSettingsService(repo *Repository, audit AuditLogger) *SettingsService {
	return &SettingsService{repo: repo, audit: audit}
}

// Get memuat settings modul ke dst (harus pointer, sudah diisi default oleh
// pemanggil sebelum dipanggil). Bila belum ada baris tersimpan, dst tetap
// berisi default — sekolah baru tidak perlu seeding settings.
func (s *SettingsService) Get(ctx context.Context, schoolID int64, module string, dst Settings) error {
	raw, found, err := s.repo.GetSetting(ctx, schoolID, module)
	if err != nil {
		return err
	}
	if !found {
		return nil
	}
	return json.Unmarshal(raw, dst)
}

// Put memvalidasi lalu menyimpan settings modul, dan mencatat audit_log.
func (s *SettingsService) Put(ctx context.Context, schoolID, actorUserID int64, module string, v Settings) error {
	if err := v.Validate(); err != nil {
		return err
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return err
	}
	if err := s.repo.PutSetting(ctx, schoolID, module, raw, actorUserID); err != nil {
		return err
	}
	if s.audit != nil {
		sid, uid := schoolID, actorUserID
		_ = s.audit.Log(ctx, &sid, &uid, "settings.save", "school_settings", nil, nil, v)
	}
	return nil
}

// NewModuleSettings mengembalikan instance default suatu module settings,
// atau ok=false bila module tidak dikenal. Daftar module bertambah seiring
// modul lain (attendance, leave, notification, ...) menambah settings-nya.
func NewModuleSettings(module string) (Settings, bool) {
	switch module {
	case "branding":
		b := DefaultBrandingSettings()
		return &b, true
	case "attendance":
		a := attendance.DefaultSettings()
		return &a, true
	case "leave":
		l := leave.DefaultSettings()
		return &l, true
	case "teaching":
		t := teaching.DefaultSettings()
		return &t, true
	case "notification":
		n := notification.DefaultSettings()
		return &n, true
	default:
		return nil, false
	}
}

// superAdminOnlyModules — module school_settings yang HANYA boleh diubah
// super admin (host platform), BUKAN admin sekolah lewat endpoint tenant
// umum PUT /api/settings/{module} (docs/08-notification.md "notification:
// DIKELOLA SUPER ADMIN" — WhatsApp/email memakai kredensial & biaya
// platform). Mutasi module ini HANYA lewat
// GET/PUT /api/admin/schools/{id}/settings/{module} (lihat handler.go/routes.go).
var superAdminOnlyModules = map[string]bool{
	"notification": true,
}

// IsSuperAdminOnlyModule melaporkan apakah module settings tsb hanya boleh
// diubah super admin.
func IsSuperAdminOnlyModule(module string) bool {
	return superAdminOnlyModules[module]
}
