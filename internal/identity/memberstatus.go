package identity

import (
	"context"
	"errors"

	"github.com/omanjaya/nouschool/internal/platform/httpx"
)

// Fase 15 Gap 6 (docs/12-sion-parity.md "Nonaktifkan/aktifkan user"):
// PATCH /api/members/{userId}/status — host TENANT, perm student:manage
// (SAMA gerbang dgn CRUD siswa/pegawai — keputusan sendiri, konsisten dgn
// presedan employee.RegisterRoutes yang juga memakai student:manage, lihat
// docs/02-identity.md). Target adalah SATU baris membership spesifik
// (userId, school, role dari body) — user bisa punya >1 role di sekolah yang
// sama, hanya role yang disebut yang terdampak.
//
// **Enforcement login sudah OTOMATIS, TIDAK butuh kode tambahan**: Login
// (service.go) memanggil ListActiveMemberships, query SQL-nya memfilter
// `status = 'active'` (queries.sql ListActiveMembershipsByUserSchool) —
// membership berstatus inactive TIDAK PERNAH ikut PickActiveRole. Bila
// SEMUA membership user itu di sekolah ini nonaktif, memberships kosong ->
// Login gagal generik ErrInvalidCredentials — diverifikasi e2e (lihat
// laporan tugas), bukan sekadar diasumsikan.

var (
	errMemberStatusSelfForbidden  = httpx.Validation("Tidak bisa mengubah status keanggotaan diri sendiri.")
	errMemberStatusAdminForbidden = httpx.Validation("Tidak bisa menonaktifkan/mengaktifkan admin sekolah atau super admin lewat endpoint ini.")
	errMemberStatusInvalid        = httpx.Validation("Status harus 'active' atau 'inactive'.")
)

// setMemberStatusRepo adalah kontrak minimal dibutuhkan setMemberStatus —
// dipenuhi *Repository secara struktural, dideklarasikan supaya bisa dites
// dengan fake in-memory TANPA DB (pola sama adminResetRepo di admin.go).
type setMemberStatusRepo interface {
	UserByID(ctx context.Context, id int64) (User, error)
	GetMembership(ctx context.Context, userID, schoolID int64, role string) (Membership, error)
	SetMembershipStatus(ctx context.Context, userID, schoolID int64, role, status string) error
	DeleteSessionsByUserSchool(ctx context.Context, userID, schoolID int64) error
}

// setMemberStatus implementasi MURNI (testable) dari Service.SetMemberStatus:
//  1. status harus 'active'/'inactive'
//  2. TOLAK bila target == aktor (diri sendiri)
//  3. TOLAK bila role target == admin_sekolah
//  4. TOLAK bila target adalah super admin (users.is_super_admin)
//  5. membership (userID, schoolID, role) HARUS ada -> 404 bila tidak
//  6. update status; bila jadi inactive -> hapus SEMUA sesi user itu DI
//     SEKOLAH INI (bukan lintas sekolah, beda dari AdminResetPassword)
//  7. audit admin.member_status
func setMemberStatus(
	ctx context.Context, repo setMemberStatusRepo, audit adminResetAuditLogger,
	schoolID, actorUserID, targetUserID int64, role, status string,
) (Membership, error) {
	if status != "active" && status != "inactive" {
		return Membership{}, errMemberStatusInvalid
	}
	if targetUserID == actorUserID {
		return Membership{}, errMemberStatusSelfForbidden
	}
	if role == RoleAdminSekolah {
		return Membership{}, errMemberStatusAdminForbidden
	}

	target, err := repo.UserByID(ctx, targetUserID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return Membership{}, httpx.ErrNotFound
		}
		return Membership{}, err
	}
	if target.IsSuperAdmin {
		return Membership{}, errMemberStatusAdminForbidden
	}

	if _, err := repo.GetMembership(ctx, targetUserID, schoolID, role); err != nil {
		if errors.Is(err, ErrNotFound) {
			return Membership{}, httpx.ErrNotFound
		}
		return Membership{}, err
	}

	if err := repo.SetMembershipStatus(ctx, targetUserID, schoolID, role, status); err != nil {
		return Membership{}, err
	}
	if status == "inactive" {
		if err := repo.DeleteSessionsByUserSchool(ctx, targetUserID, schoolID); err != nil {
			return Membership{}, err
		}
	}

	if audit != nil {
		sid, uid := schoolID, actorUserID
		_ = audit.Log(ctx, &sid, &uid, "admin.member_status", "membership", &targetUserID, nil,
			map[string]any{"role": role, "status": status})
	}

	return Membership{UserID: targetUserID, SchoolID: schoolID, Role: role, Status: status}, nil
}

// SetMemberStatus — PATCH /api/members/{userId}/status.
func (s *Service) SetMemberStatus(ctx context.Context, schoolID, actorUserID, targetUserID int64, role, status string) (Membership, error) {
	return setMemberStatus(ctx, s.repo, s, schoolID, actorUserID, targetUserID, role, status)
}
