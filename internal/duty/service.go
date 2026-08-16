package duty

import (
	"context"
	"errors"
	"sort"
	"strings"

	"github.com/omanjaya/nouschool/internal/platform/httpx"
	"github.com/omanjaya/nouschool/internal/platform/reqctx"
)

// -- consumer-side interface (lihat CLAUDE.md) --

// IdentityGateway adalah kebutuhan modul duty dari modul identity: gerbang
// permission, audit log, dan resolusi user_id per role (validasi assignment).
type IdentityGateway interface {
	HasPermission(role, perm string) bool
	Log(ctx context.Context, schoolID, userID *int64, action, entity string, entityID *int64, oldValue, newValue any) error
	UsersWithRole(ctx context.Context, schoolID int64, role string) ([]int64, error)
}

// AcademicYearLookup adalah kebutuhan modul duty dari modul tenant: tahun
// ajaran aktif (assignment SELALU per TA aktif, docs tugas Fase 14 B1).
type AcademicYearLookup interface {
	ActiveAcademicYearID(ctx context.Context, schoolID int64) (id int64, ok bool, err error)
}

// ErrNoActiveAcademicYear — sekolah belum mengaktifkan tahun ajaran (pesan
// sama seperti modul lain, mis. internal/discipline).
var ErrNoActiveAcademicYear = httpx.Validation("Sekolah belum punya tahun ajaran aktif. Aktifkan dulu di menu tahun ajaran.")

// Service berisi aturan bisnis modul duty.
type Service struct {
	repo     dutyRepository
	identity IdentityGateway
	years    AcademicYearLookup
}

func NewService(repo *Repository, identity IdentityGateway, years AcademicYearLookup) *Service {
	return &Service{repo: repo, identity: identity, years: years}
}

// newServiceForTest membangun Service dengan repository FAKE (in-memory,
// tanpa DB) — dipakai test di package ini saja.
func newServiceForTest(repo dutyRepository, identity IdentityGateway, years AcademicYearLookup) *Service {
	return &Service{repo: repo, identity: identity, years: years}
}

func (s *Service) audit(ctx context.Context, schoolID, actorUserID int64, action, entity string, entityID int64, oldValue, newValue any) {
	sid, uid, eid := schoolID, actorUserID, entityID
	_ = s.identity.Log(ctx, &sid, &uid, action, entity, &eid, oldValue, newValue)
}

func (s *Service) requireManage(ctx context.Context) error {
	if !s.identity.HasPermission(reqctx.Role(ctx), PermDutyManage) {
		return httpx.ErrForbidden
	}
	return nil
}

func (s *Service) activeYear(ctx context.Context, schoolID int64) (int64, error) {
	id, ok, err := s.years.ActiveAcademicYearID(ctx, schoolID)
	if err != nil {
		return 0, err
	}
	if !ok {
		return 0, ErrNoActiveAcademicYear
	}
	return id, nil
}

// -- GET /api/duties/flags --

// ListFlags — statis, TANPA DB (perm duty:manage — semua endpoint duties
// pakai gerbang yang sama, kepsek TIDAK butuh kelola tugas, sesuai instruksi
// tugas).
func (s *Service) ListFlags(ctx context.Context) ([]FlagInfo, error) {
	if err := s.requireManage(ctx); err != nil {
		return nil, err
	}
	out := make([]FlagInfo, 0, len(allFlags))
	for _, f := range allFlags {
		out = append(out, FlagInfo{Key: f, Label: flagLabels[f], ForRoleHint: flagForRoleHint[f]})
	}
	return out, nil
}

// -- GET/POST/PATCH/DELETE /api/duties --

func normalizeFlags(flags []string) ([]string, error) {
	out := make([]string, 0, len(flags))
	seen := make(map[string]bool, len(flags))
	for _, f := range flags {
		f = strings.TrimSpace(f)
		if f == "" || seen[f] {
			continue
		}
		if !validFlag(f) {
			return nil, httpx.Validation("Flag tidak dikenal: " + f)
		}
		seen[f] = true
		out = append(out, f)
	}
	sort.Strings(out)
	return out, nil
}

func (s *Service) ListDuties(ctx context.Context, schoolID int64) ([]DutyView, error) {
	if err := s.requireManage(ctx); err != nil {
		return nil, err
	}
	yearID, _, err := s.years.ActiveAcademicYearID(ctx, schoolID)
	if err != nil {
		return nil, err
	}
	rows, err := s.repo.ListDutiesWithAssigneeCount(ctx, schoolID, yearID)
	if err != nil {
		return nil, err
	}
	out := make([]DutyView, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.DutyRecord.view(r.AssigneeCount))
	}
	return out, nil
}

// CreateDutyInput adalah parameter Service.CreateDuty.
type CreateDutyInput struct {
	Name    string
	ForRole string
	Flags   []string
}

func (s *Service) CreateDuty(ctx context.Context, actorUserID, schoolID int64, in CreateDutyInput) (DutyView, error) {
	if err := s.requireManage(ctx); err != nil {
		return DutyView{}, err
	}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return DutyView{}, httpx.Validation("Nama tugas wajib diisi.")
	}
	if !validForRole(in.ForRole) {
		return DutyView{}, httpx.Validation("for_role harus 'guru' atau 'pegawai'.")
	}
	flags, err := normalizeFlags(in.Flags)
	if err != nil {
		return DutyView{}, err
	}
	rec, err := s.repo.CreateDuty(ctx, schoolID, name, in.ForRole, flags)
	if err != nil {
		if errors.Is(err, ErrConflict) {
			return DutyView{}, httpx.Conflict("Tugas dengan nama itu sudah ada.")
		}
		return DutyView{}, err
	}
	s.audit(ctx, schoolID, actorUserID, "duty.create", "duty", rec.ID, nil,
		map[string]any{"name": rec.Name, "for_role": rec.ForRole, "flags": rec.Flags})
	return rec.view(0), nil
}

// PatchDutyInput adalah parameter Service.UpdateDuty — nil = field tidak diubah.
type PatchDutyInput struct {
	Name    *string
	ForRole *string
	Flags   []string // nil = tidak diubah; non-nil (termasuk kosong) = diganti
	Active  *bool
}

func (s *Service) UpdateDuty(ctx context.Context, actorUserID, schoolID, id int64, in PatchDutyInput) (DutyView, error) {
	if err := s.requireManage(ctx); err != nil {
		return DutyView{}, err
	}
	old, err := s.repo.GetDutyByID(ctx, schoolID, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return DutyView{}, httpx.ErrNotFound
		}
		return DutyView{}, err
	}
	if in.Name != nil {
		trimmed := strings.TrimSpace(*in.Name)
		if trimmed == "" {
			return DutyView{}, httpx.Validation("Nama tugas wajib diisi.")
		}
		in.Name = &trimmed
	}
	if in.ForRole != nil && !validForRole(*in.ForRole) {
		return DutyView{}, httpx.Validation("for_role harus 'guru' atau 'pegawai'.")
	}
	var flags []string
	if in.Flags != nil {
		flags, err = normalizeFlags(in.Flags)
		if err != nil {
			return DutyView{}, err
		}
	}
	rec, err := s.repo.UpdateDuty(ctx, schoolID, id, in.Name, in.ForRole, flags, in.Active)
	if err != nil {
		if errors.Is(err, ErrConflict) {
			return DutyView{}, httpx.Conflict("Tugas dengan nama itu sudah ada.")
		}
		if errors.Is(err, ErrNotFound) {
			return DutyView{}, httpx.ErrNotFound
		}
		return DutyView{}, err
	}
	s.audit(ctx, schoolID, actorUserID, "duty.update", "duty", id,
		map[string]any{"name": old.Name, "for_role": old.ForRole, "flags": old.Flags, "active": old.Active},
		map[string]any{"name": rec.Name, "for_role": rec.ForRole, "flags": rec.Flags, "active": rec.Active})
	return rec.view(0), nil
}

var errDutyInUse = httpx.Conflict("Tugas ini masih punya pemegang. Nonaktifkan (active=false) alih-alih menghapus.")

func (s *Service) DeleteDuty(ctx context.Context, actorUserID, schoolID, id int64) error {
	if err := s.requireManage(ctx); err != nil {
		return err
	}
	old, err := s.repo.GetDutyByID(ctx, schoolID, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return httpx.ErrNotFound
		}
		return err
	}
	count, err := s.repo.CountAssignmentsForDuty(ctx, schoolID, id)
	if err != nil {
		return err
	}
	if count > 0 {
		return errDutyInUse
	}
	rows, err := s.repo.DeleteDuty(ctx, schoolID, id)
	if err != nil {
		return err
	}
	if rows == 0 {
		return httpx.ErrNotFound
	}
	s.audit(ctx, schoolID, actorUserID, "duty.delete", "duty", id, map[string]any{"name": old.Name}, nil)
	return nil
}

// -- GET/PUT /api/duties/{id}/assignments --

func (s *Service) GetAssignments(ctx context.Context, schoolID, dutyID int64) ([]AssignmentUserView, error) {
	if err := s.requireManage(ctx); err != nil {
		return nil, err
	}
	duty, err := s.repo.GetDutyByID(ctx, schoolID, dutyID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, httpx.ErrNotFound
		}
		return nil, err
	}
	yearID, err := s.activeYear(ctx, schoolID)
	if err != nil {
		return nil, err
	}
	rows, err := s.repo.ListAssignmentsForDutyYear(ctx, schoolID, dutyID, yearID)
	if err != nil {
		return nil, err
	}
	out := make([]AssignmentUserView, 0, len(rows))
	for _, r := range rows {
		out = append(out, AssignmentUserView{UserID: r.UserID, Name: r.Name, Role: duty.ForRole})
	}
	return out, nil
}

// PutAssignments mengganti seluruh pemegang tugas duty pada TA AKTIF —
// validasi setiap user_id adalah anggota AKTIF sekolah dengan role SESUAI
// duty.for_role (lewat IdentityGateway.UsersWithRole, tanpa query membership
// baru di modul ini).
func (s *Service) PutAssignments(ctx context.Context, actorUserID, schoolID, dutyID int64, userIDs []int64) ([]AssignmentUserView, error) {
	if err := s.requireManage(ctx); err != nil {
		return nil, err
	}
	duty, err := s.repo.GetDutyByID(ctx, schoolID, dutyID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, httpx.ErrNotFound
		}
		return nil, err
	}
	yearID, err := s.activeYear(ctx, schoolID)
	if err != nil {
		return nil, err
	}

	eligible, err := s.identity.UsersWithRole(ctx, schoolID, duty.ForRole)
	if err != nil {
		return nil, err
	}
	eligibleSet := make(map[int64]bool, len(eligible))
	for _, id := range eligible {
		eligibleSet[id] = true
	}
	unique := make([]int64, 0, len(userIDs))
	seen := make(map[int64]bool, len(userIDs))
	for _, uid := range userIDs {
		if uid == 0 || seen[uid] {
			continue
		}
		if !eligibleSet[uid] {
			return nil, httpx.Validation("Salah satu user bukan anggota aktif sekolah dengan role " + duty.ForRole + ".")
		}
		seen[uid] = true
		unique = append(unique, uid)
	}

	if err := s.repo.ReplaceAssignments(ctx, schoolID, dutyID, yearID, unique); err != nil {
		return nil, err
	}
	s.audit(ctx, schoolID, actorUserID, "duty.assignments_replace", "duty", dutyID, nil,
		map[string]any{"user_ids": unique, "academic_year_id": yearID})

	return s.GetAssignments(ctx, schoolID, dutyID)
}

// -- interface publik (dipakai modul lain, mis. studentleave, via
// consumer-side interface — lihat CLAUDE.md) --

// UserHasFlag melaporkan apakah user memegang SETIDAKNYA SATU tugas AKTIF
// pada TA AKTIF sekolah yang membawa flag tsb. ok=false (tanpa error) bila
// sekolah belum punya TA aktif (tidak ada assignment yang bisa cocok).
func (s *Service) UserHasFlag(ctx context.Context, schoolID, userID int64, flag string) (bool, error) {
	yearID, ok, err := s.years.ActiveAcademicYearID(ctx, schoolID)
	if err != nil {
		return false, err
	}
	if !ok {
		return false, nil
	}
	return s.repo.UserHasFlag(ctx, schoolID, userID, yearID, flag)
}

// UserIDsWithFlag mengembalikan seluruh user_id pemegang tugas AKTIF pada TA
// AKTIF sekolah yang membawa flag tsb (dipakai resolusi target notifikasi/
// antrian modul lain). [] (TANPA error) bila sekolah belum punya TA aktif.
func (s *Service) UserIDsWithFlag(ctx context.Context, schoolID int64, flag string) ([]int64, error) {
	yearID, ok, err := s.years.ActiveAcademicYearID(ctx, schoolID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, nil
	}
	return s.repo.UserIDsWithFlag(ctx, schoolID, yearID, flag)
}
