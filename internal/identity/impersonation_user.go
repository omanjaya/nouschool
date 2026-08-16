package identity

import (
	"context"
	"errors"
	"time"

	"github.com/omanjaya/nouschool/internal/platform/httpx"
	"github.com/omanjaya/nouschool/internal/platform/reqctx"
)

// Fase 14 Gelombang D (docs/12-sion-parity.md "Impersonate USER oleh admin
// sekolah"): admin_sekolah bisa "masuk sebagai" member LAIN sekolahnya
// (guru/kepala sekolah/siswa/orang tua/pegawai) — reuse infra sesi/audit,
// TAPI SENGAJA modul terpisah dari impersonation.go (fase 13 "masuk sebagai
// sekolah" super admin, sentinel role admin_sekolah:impersonating): di sini
// admin & target berada di HOST TENANT YANG SAMA, jadi cookie ditukar
// LANGSUNG (bukan token sekali-pakai lintas tab), dan role sesi baru adalah
// ROLE ASLI target (bukan sentinel) — RBAC (RequirePerm/HasPermission)
// otomatis berlaku benar sesuai role target TANPA translasi apa pun.
//
// **Keputusan desain (dilaporkan)**: docs tugas menyebut "reuse mekanisme
// sentinel/renew-window 0" — DIIMPLEMENTASIKAN LEWAT KOLOM BARU
// sessions.impersonator_user_id (migrasi 00021), BUKAN sentinel role seperti
// fase 13. Alasan: sentinel role fase 13 bekerja karena sesi HASILNYA selalu
// admin_sekolah (peran generik tunggal) sehingga sentinel bisa "diterjemahkan
// kembali" satu arah di RequireAuth. Di sini target session role BISA salah
// satu dari 5 kemungkinan (guru/kepala_sekolah/siswa/orang_tua/pegawai) — 5
// sentinel terpisah tidak sepadan. Kolom impersonator_user_id sudah cukup:
// RequireAuth (middleware.go) mengecek keberadaannya utk (a) melewati sliding
// renewal SAMA SEKALI (TTL 1 jam keras) & (b) tetap membawa role ASLI target
// ke reqctx tanpa translasi. Pola pemisahan fungsi MURNI (testable tanpa DB,
// terima interface kecil) dari method Service (pegang *Repository nyata)
// SAMA PERSIS dengan impersonation.go fase 13 — lihat impersonationAuditLogger
// yang dipakai bersama.

// impersonationUserSessionTTL — sesi impersonasi USER HANYA 1 jam & TIDAK
// PERNAH diperpanjang sliding (lihat RequireAuth: sesi dengan
// impersonator_user_id terisi dikecualikan total dari renewal generik).
const impersonationUserSessionTTL = 1 * time.Hour

// errImpersonationTargetForbidden — pesan SATU macam (tidak membedakan alasan
// spesifik ke klien, sama prinsip dengan ErrImpersonationInvalid fase 13).
var errImpersonationTargetForbidden = httpx.Validation("Target tidak bisa diimpersonasi: harus anggota AKTIF sekolah ini, dan bukan admin sekolah lain, super admin, atau akun display.")

// errNotImpersonating — POST /api/auth/impersonation/stop dipanggil dari
// sesi yang BUKAN hasil ImpersonateUser.
var errNotImpersonating = httpx.Validation("Sesi ini bukan sesi impersonasi.")

// impersonateUserRepo adalah kontrak minimal dibutuhkan impersonateUser —
// dipenuhi *Repository secara struktural, atau fake di
// impersonation_user_test.go.
type impersonateUserRepo interface {
	UserByID(ctx context.Context, id int64) (User, error)
	ListActiveMemberships(ctx context.Context, userID, schoolID int64) ([]Membership, error)
	StudentIDByUser(ctx context.Context, userID, schoolID int64) (int64, error)
	CreateSession(ctx context.Context, in CreateSessionInput) error
}

// impersonateUser implementasi MURNI (testable) dari Service.ImpersonateUser.
// now dipakai menghitung expiresAt (bukan time.Now() langsung) supaya bisa
// dites deterministik.
func impersonateUser(ctx context.Context, repo impersonateUserRepo, audit impersonationAuditLogger, actorUserID, schoolID, targetUserID int64, hostSchool SchoolView, ip, userAgent string, now time.Time) (LoginResult, error) {
	if targetUserID == 0 {
		return LoginResult{}, httpx.Validation("ID target wajib diisi.")
	}
	if targetUserID == actorUserID {
		return LoginResult{}, httpx.Validation("Tidak bisa mengimpersonasi diri sendiri.")
	}
	target, err := repo.UserByID(ctx, targetUserID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return LoginResult{}, httpx.ErrNotFound
		}
		return LoginResult{}, err
	}
	if target.IsSuperAdmin {
		return LoginResult{}, errImpersonationTargetForbidden
	}

	memberships, err := repo.ListActiveMemberships(ctx, targetUserID, schoolID)
	if err != nil {
		return LoginResult{}, err
	}
	if len(memberships) == 0 {
		return LoginResult{}, errImpersonationTargetForbidden
	}
	roles := make([]string, 0, len(memberships))
	for _, m := range memberships {
		roles = append(roles, m.Role)
	}
	role := PickActiveRole(roles)
	// Terlarang: admin sekolah LAIN (diri sendiri sudah ditolak di atas via
	// cek ID) dan role display (akun TV bersama, bukan personal — docs tugas
	// eksplisit mengecualikan keduanya; super admin sudah dicek di atas via
	// target.IsSuperAdmin).
	if role == "" || role == RoleAdminSekolah || role == RoleDisplay {
		return LoginResult{}, errImpersonationTargetForbidden
	}

	token, tokenHash, terr := newSessionToken()
	if terr != nil {
		return LoginResult{}, terr
	}
	expiresAt := now.Add(impersonationUserSessionTTL)
	impersonator := actorUserID
	sid := schoolID
	if err := repo.CreateSession(ctx, CreateSessionInput{
		UserID: targetUserID, SchoolID: &sid, TokenHash: tokenHash, Role: role,
		ExpiresAt: expiresAt, IP: ip, UserAgent: userAgent, ImpersonatorUserID: &impersonator,
	}); err != nil {
		return LoginResult{}, err
	}

	view := UserView{ID: target.ID, Name: target.Name, Role: role, IsSuperAdmin: false, School: &hostSchool}
	if role == RoleSiswa {
		if studentID, serr := repo.StudentIDByUser(ctx, target.ID, hostSchool.ID); serr == nil {
			view.StudentID = studentID
		}
	}

	if audit != nil {
		schoolIDCopy, actorCopy, targetCopy := schoolID, actorUserID, targetUserID
		_ = audit.Log(ctx, &schoolIDCopy, &actorCopy, "admin.user_impersonate_started", "user", &targetCopy, nil, map[string]any{"role": role})
	}

	return LoginResult{View: view, Token: token, ExpiresAt: expiresAt}, nil
}

// ImpersonateUser — POST /api/users/{id}/impersonate (host tenant,
// RequirePerm user:impersonate ditegakkan di routes.go). actorUserID = admin
// sekolah yang sedang login (reqctx.UserID). schoolID = host tenant saat ini.
func (s *Service) ImpersonateUser(ctx context.Context, actorUserID, schoolID, targetUserID int64, ip, userAgent string) (LoginResult, error) {
	sch, ok := reqctx.SchoolFromContext(ctx)
	if !ok {
		return LoginResult{}, httpx.ErrForbidden
	}
	hostSchool := SchoolView{ID: sch.ID, Name: sch.Name, Slug: sch.Slug}
	return impersonateUser(ctx, s.repo, s, actorUserID, schoolID, targetUserID, hostSchool, ip, userAgent, time.Now())
}

// stopImpersonationRepo adalah kontrak minimal dibutuhkan stopImpersonation.
type stopImpersonationRepo interface {
	SessionByTokenHash(ctx context.Context, hash []byte) (SessionRow, error)
	DeleteSessionByTokenHash(ctx context.Context, hash []byte) error
	UserByID(ctx context.Context, id int64) (User, error)
	CreateSession(ctx context.Context, in CreateSessionInput) error
}

// stopImpersonation implementasi MURNI (testable) dari
// Service.StopImpersonation: token = nilai cookie sesi impersonasi SAAT INI.
// Menghapus sesi impersonasi lalu menerbitkan sesi BARU utk admin
// (impersonator_user_id sesi ini) dengan role admin_sekolah & TTL NORMAL
// (sessionTTLForRole — sliding renewal berlaku lagi seperti sesi biasa).
func stopImpersonation(ctx context.Context, repo stopImpersonationRepo, audit impersonationAuditLogger, token string, hostSchool SchoolView, ip, userAgent string, now time.Time) (LoginResult, error) {
	if token == "" {
		return LoginResult{}, errNotImpersonating
	}
	hash, herr := hashToken(token)
	if herr != nil {
		return LoginResult{}, errNotImpersonating
	}
	sess, err := repo.SessionByTokenHash(ctx, hash)
	if err != nil {
		return LoginResult{}, errNotImpersonating
	}
	if sess.ImpersonatorUserID == nil {
		return LoginResult{}, errNotImpersonating
	}
	adminUserID := *sess.ImpersonatorUserID
	targetUserID := sess.UserID

	if err := repo.DeleteSessionByTokenHash(ctx, hash); err != nil {
		return LoginResult{}, err
	}

	admin, err := repo.UserByID(ctx, adminUserID)
	if err != nil {
		return LoginResult{}, err
	}

	newToken, tokenHash, terr := newSessionToken()
	if terr != nil {
		return LoginResult{}, terr
	}
	schoolID := hostSchool.ID
	expiresAt := now.Add(sessionTTLForRole(RoleAdminSekolah))
	if err := repo.CreateSession(ctx, CreateSessionInput{
		UserID: adminUserID, SchoolID: &schoolID, TokenHash: tokenHash, Role: RoleAdminSekolah,
		ExpiresAt: expiresAt, IP: ip, UserAgent: userAgent,
	}); err != nil {
		return LoginResult{}, err
	}

	view := UserView{ID: admin.ID, Name: admin.Name, Role: RoleAdminSekolah, IsSuperAdmin: admin.IsSuperAdmin, School: &hostSchool}

	if audit != nil {
		schoolIDCopy, adminCopy, targetCopy := schoolID, adminUserID, targetUserID
		_ = audit.Log(ctx, &schoolIDCopy, &adminCopy, "admin.user_impersonate_stopped", "user", &targetCopy, nil, nil)
	}

	return LoginResult{View: view, Token: newToken, ExpiresAt: expiresAt}, nil
}

// StopImpersonation — POST /api/auth/impersonation/stop.
func (s *Service) StopImpersonation(ctx context.Context, token, ip, userAgent string) (LoginResult, error) {
	sch, ok := reqctx.SchoolFromContext(ctx)
	if !ok {
		return LoginResult{}, httpx.ErrForbidden
	}
	hostSchool := SchoolView{ID: sch.ID, Name: sch.Name, Slug: sch.Slug}
	return stopImpersonation(ctx, s.repo, s, token, hostSchool, ip, userAgent, time.Now())
}
