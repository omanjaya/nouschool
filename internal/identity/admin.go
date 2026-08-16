package identity

import (
	"context"
	"crypto/rand"
	"errors"
	"net/http"
	"time"

	"github.com/omanjaya/nouschool/internal/platform/httpx"
)

// Fase 13 — Panel super admin P4 "Operasional" (docs/11-superadmin.md).
// P4.1 (daftar anggota sekolah), P4.2 (reset password user), P4.3 (audit log
// viewer per sekolah) ditempatkan DI SINI (perluasan modul identity) — bukan
// modul agregator baru — karena KETIGANYA hanya menyentuh tabel yang sudah
// dimiliki modul identity sendiri (users/memberships/sessions/audit_log).
// Bandingkan dengan internal/platformadmin (modul BARU fase 13 ini) yang
// menampung P1/P3/P4.4 — SEMUA butuh join lintas tabel milik modul LAIN
// (billing/student/attendance/teaching/notification) sehingga layak jadi
// modul agregator tersendiri (lihat catatan desain lengkap di
// internal/platformadmin/service.go).

// -- P4.1: GET /api/admin/schools/{id}/members --

// MemberView adalah satu baris response GET /api/admin/schools/{id}/members.
type MemberView struct {
	UserID    int64      `json:"user_id"`
	Name      string     `json:"name"`
	Email     string     `json:"email,omitempty"`
	Username  string     `json:"username,omitempty"`
	Role      string     `json:"role"`
	Status    string     `json:"status"`
	LastLogin *time.Time `json:"last_login"`
}

// ListMembers — daftar anggota sekolah (satu baris per membership; user bisa
// muncul >1 kali bila punya >1 role) untuk panel super admin.
func (s *Service) ListMembers(ctx context.Context, schoolID int64) ([]MemberView, error) {
	rows, err := s.repo.ListMembersForSchool(ctx, schoolID)
	if err != nil {
		return nil, err
	}
	out := make([]MemberView, 0, len(rows))
	for _, r := range rows {
		out = append(out, MemberView{
			UserID: r.UserID, Name: r.Name, Email: r.Email, Username: r.Username,
			Role: r.Role, Status: r.Status, LastLogin: r.LastLogin,
		})
	}
	return out, nil
}

// -- P4.2: POST /api/admin/users/{id}/reset-password --

// tempPasswordChars — a-z0-9 TANPA 0/o/1/l (mudah dibaca, sesuai instruksi
// tugas fase 13 P4.2).
const tempPasswordChars = "abcdefghijkmnpqrstuvwxyz23456789"
const tempPasswordLength = 10

// generateTempPassword menghasilkan password sementara acak (crypto/rand).
// Bias modulo (256 % 32 == 0, jadi SEBENARNYA tidak ada bias di sini karena
// len(tempPasswordChars)=32 habis membagi 256) — dicatat karena pola byte%len
// ini bisa bias untuk panjang charset lain, TIDAK untuk kasus 32 ini.
func generateTempPassword() (string, error) {
	b := make([]byte, tempPasswordLength)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	out := make([]byte, tempPasswordLength)
	for i, v := range b {
		out[i] = tempPasswordChars[int(v)%len(tempPasswordChars)]
	}
	return string(out), nil
}

// errCannotResetSuperAdmin — task fase 13 P4.2: "TOLAK bila target is_super_admin".
var errCannotResetSuperAdmin = &httpx.Error{
	Status:  http.StatusForbidden,
	Code:    "forbidden",
	Message: "Tidak bisa mereset password sesama super admin lewat panel ini.",
}

// adminResetAuditLogger & adminResetRepo — interface kecil dipenuhi *Service/
// *Repository secara struktural, dideklarasikan supaya adminResetPassword
// (fungsi murni di bawah) bisa dites dengan fake repo TANPA DB — pola sama
// impersonationAuditLogger/impersonationUserRepo di impersonation.go.
type adminResetAuditLogger interface {
	Log(ctx context.Context, schoolID, userID *int64, action, entity string, entityID *int64, oldValue, newValue any) error
}

type adminResetRepo interface {
	UserByID(ctx context.Context, id int64) (User, error)
	ListActiveMemberships(ctx context.Context, userID, schoolID int64) ([]Membership, error)
	UpdateUserPassword(ctx context.Context, id int64, hash string) error
	DeleteSessionsByUser(ctx context.Context, userID int64) error
}

// adminResetPassword implementasi murni (testable) dari Service.AdminResetPassword:
//  1. tolak bila target adalah super admin (task: "TOLAK bila target is_super_admin")
//  2. validasi target memang anggota AKTIF sekolah ini
//  3. generate password sementara → hash → simpan → hapus SEMUA sesi user itu
//     (lebih aman daripada membiarkan sesi lama tetap hidup)
//  4. audit admin.reset_password (school_id target, entity user)
func adminResetPassword(
	ctx context.Context, repo adminResetRepo, audit adminResetAuditLogger,
	genPassword func() (string, error), hashPassword func(string) (string, error),
	actorUserID, schoolID, targetUserID int64,
) (string, error) {
	target, err := repo.UserByID(ctx, targetUserID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return "", httpx.ErrNotFound
		}
		return "", err
	}
	if target.IsSuperAdmin {
		return "", errCannotResetSuperAdmin
	}

	memberships, err := repo.ListActiveMemberships(ctx, targetUserID, schoolID)
	if err != nil {
		return "", err
	}
	if len(memberships) == 0 {
		return "", httpx.Validation("User ini bukan anggota aktif sekolah ini.")
	}

	tempPassword, err := genPassword()
	if err != nil {
		return "", err
	}
	hash, err := hashPassword(tempPassword)
	if err != nil {
		return "", err
	}
	if err := repo.UpdateUserPassword(ctx, targetUserID, hash); err != nil {
		return "", err
	}
	if err := repo.DeleteSessionsByUser(ctx, targetUserID); err != nil {
		return "", err
	}

	if audit != nil {
		sid, uid := schoolID, actorUserID
		_ = audit.Log(ctx, &sid, &uid, "admin.reset_password", "user", &targetUserID, nil, nil)
	}
	return tempPassword, nil
}

// AdminResetPassword — POST /api/admin/users/{id}/reset-password (host
// platform, RequireSuperAdmin; body {"school_id"} dipakai konteks audit &
// validasi keanggotaan — lihat handler.go).
func (s *Service) AdminResetPassword(ctx context.Context, actorUserID, schoolID, targetUserID int64) (string, error) {
	return adminResetPassword(ctx, s.repo, s, generateTempPassword, HashPassword, actorUserID, schoolID, targetUserID)
}

// -- P4.3: GET /api/admin/schools/{id}/audit --

// AuditLogItem adalah satu baris pada AuditLogPage.
type AuditLogItem struct {
	ID       int64     `json:"id"`
	UserName *string   `json:"user_name"`
	Action   string    `json:"action"`
	Entity   string    `json:"entity"`
	EntityID *int64    `json:"entity_id"`
	At       time.Time `json:"at"`
}

// AuditLogPage adalah shape response GET /api/admin/schools/{id}/audit.
type AuditLogPage struct {
	Items []AuditLogItem `json:"items"`
	Total int64          `json:"total"`
}

// auditLogRepo — interface kecil dipenuhi *Repository secara struktural,
// dideklarasikan supaya listAuditLogPage (fungsi murni di bawah) bisa dites
// dengan fake repo TANPA DB (pola sama adminResetRepo di atas).
type auditLogRepo interface {
	ListAuditLogForSchool(ctx context.Context, schoolID int64, actionPrefix string, limit, offset int32) ([]AuditLogRow, error)
	CountAuditLogForSchool(ctx context.Context, schoolID int64, actionPrefix string) (int64, error)
}

// listAuditLogPage implementasi murni (testable) dari Service.ListAuditLog:
// page/perPage default 1/50, perPage maks 200 (task fase 13 P4.3).
// actionPrefix kosong = tanpa filter, selain itu prefix-match (mis. "attendance.").
func listAuditLogPage(ctx context.Context, repo auditLogRepo, schoolID int64, page, perPage int, actionPrefix string) (AuditLogPage, error) {
	if page < 1 {
		page = 1
	}
	if perPage <= 0 {
		perPage = 50
	}
	if perPage > 200 {
		perPage = 200
	}
	rows, err := repo.ListAuditLogForSchool(ctx, schoolID, actionPrefix, int32(perPage), int32((page-1)*perPage))
	if err != nil {
		return AuditLogPage{}, err
	}
	total, err := repo.CountAuditLogForSchool(ctx, schoolID, actionPrefix)
	if err != nil {
		return AuditLogPage{}, err
	}
	items := make([]AuditLogItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, AuditLogItem{ID: r.ID, UserName: r.UserName, Action: r.Action, Entity: r.Entity, EntityID: r.EntityID, At: r.At})
	}
	return AuditLogPage{Items: items, Total: total}, nil
}

// ListAuditLog — GET /api/admin/schools/{id}/audit.
func (s *Service) ListAuditLog(ctx context.Context, schoolID int64, page, perPage int, actionPrefix string) (AuditLogPage, error) {
	return listAuditLogPage(ctx, s.repo, schoolID, page, perPage, actionPrefix)
}
