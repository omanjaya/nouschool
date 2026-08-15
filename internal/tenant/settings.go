package tenant

import (
	"context"
	"encoding/json"
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
	default:
		return nil, false
	}
}
