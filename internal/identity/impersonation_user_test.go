package identity

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/omanjaya/nouschool/internal/platform/httpx"
)

// -- fake repo (in-memory, tanpa DB) — memenuhi impersonateUserRepo DAN
// stopImpersonationRepo sekaligus, pola sama fakeImpersonationUserRepo di
// impersonation_test.go --

type fakeUserImpersonationRepo struct {
	users       map[int64]User
	memberships map[int64][]Membership // userID -> memberships (lintas sekolah)
	studentIDs  map[int64]int64        // userID -> studentID

	sessions  []CreateSessionInput
	createErr error

	sessionByHash map[string]SessionRow
	deletedHashes [][]byte
}

func newFakeUserImpersonationRepo() *fakeUserImpersonationRepo {
	return &fakeUserImpersonationRepo{
		users: map[int64]User{}, memberships: map[int64][]Membership{}, studentIDs: map[int64]int64{},
		sessionByHash: map[string]SessionRow{},
	}
}

func (f *fakeUserImpersonationRepo) UserByID(ctx context.Context, id int64) (User, error) {
	u, ok := f.users[id]
	if !ok {
		return User{}, ErrNotFound
	}
	return u, nil
}

func (f *fakeUserImpersonationRepo) ListActiveMemberships(ctx context.Context, userID, schoolID int64) ([]Membership, error) {
	var out []Membership
	for _, m := range f.memberships[userID] {
		if m.SchoolID == schoolID {
			out = append(out, m)
		}
	}
	return out, nil
}

func (f *fakeUserImpersonationRepo) StudentIDByUser(ctx context.Context, userID, schoolID int64) (int64, error) {
	id, ok := f.studentIDs[userID]
	if !ok {
		return 0, ErrNotFound
	}
	return id, nil
}

func (f *fakeUserImpersonationRepo) CreateSession(ctx context.Context, in CreateSessionInput) error {
	if f.createErr != nil {
		return f.createErr
	}
	f.sessions = append(f.sessions, in)
	// Simulasikan lookup balik lewat token_hash (dipakai StopImpersonation
	// pada test yang mensimulasikan alur "buat sesi impersonasi -> pakai utk
	// stop").
	row := SessionRow{
		ID: int64(len(f.sessions)), UserID: in.UserID, SchoolID: in.SchoolID, Role: in.Role,
		ExpiresAt: in.ExpiresAt, ImpersonatorUserID: in.ImpersonatorUserID,
	}
	if u, ok := f.users[in.UserID]; ok {
		row.UserName = u.Name
		row.IsSuperAdmin = u.IsSuperAdmin
	}
	f.sessionByHash[string(in.TokenHash)] = row
	return nil
}

func (f *fakeUserImpersonationRepo) SessionByTokenHash(ctx context.Context, hash []byte) (SessionRow, error) {
	row, ok := f.sessionByHash[string(hash)]
	if !ok {
		return SessionRow{}, ErrNotFound
	}
	return row, nil
}

func (f *fakeUserImpersonationRepo) DeleteSessionByTokenHash(ctx context.Context, hash []byte) error {
	f.deletedHashes = append(f.deletedHashes, hash)
	delete(f.sessionByHash, string(hash))
	return nil
}

// -- impersonateUser (murni) --

func TestImpersonateUser_TargetAdminSekolahForbidden(t *testing.T) {
	repo := newFakeUserImpersonationRepo()
	repo.users[20] = User{ID: 20, Name: "Admin Lain"}
	repo.memberships[20] = []Membership{{UserID: 20, SchoolID: 1, Role: RoleAdminSekolah, Status: "active"}}
	audit := &fakeAudit{}
	hostSchool := SchoolView{ID: 1, Name: "Demo", Slug: "demo"}

	_, err := impersonateUser(context.Background(), repo, audit, 10, 1, 20, hostSchool, "", "", time.Now())
	var de *httpx.Error
	if !errors.As(err, &de) || de.Status != 422 {
		t.Fatalf("target admin_sekolah lain harus ditolak, got: %v", err)
	}
	if len(repo.sessions) != 0 {
		t.Fatal("TIDAK boleh membuat sesi apa pun saat target terlarang")
	}
}

func TestImpersonateUser_TargetSuperAdminForbidden(t *testing.T) {
	repo := newFakeUserImpersonationRepo()
	repo.users[21] = User{ID: 21, Name: "Super Admin", IsSuperAdmin: true}
	repo.memberships[21] = []Membership{{UserID: 21, SchoolID: 1, Role: RoleGuru, Status: "active"}}
	audit := &fakeAudit{}
	hostSchool := SchoolView{ID: 1}

	if _, err := impersonateUser(context.Background(), repo, audit, 10, 1, 21, hostSchool, "", "", time.Now()); err == nil {
		t.Fatal("target super admin harus ditolak walau punya membership guru")
	}
}

func TestImpersonateUser_TargetDisplayForbidden(t *testing.T) {
	repo := newFakeUserImpersonationRepo()
	repo.users[22] = User{ID: 22, Name: "TV Ruang Guru"}
	repo.memberships[22] = []Membership{{UserID: 22, SchoolID: 1, Role: RoleDisplay, Status: "active"}}
	audit := &fakeAudit{}
	hostSchool := SchoolView{ID: 1}

	if _, err := impersonateUser(context.Background(), repo, audit, 10, 1, 22, hostSchool, "", "", time.Now()); err == nil {
		t.Fatal("target role display harus ditolak")
	}
}

func TestImpersonateUser_TargetNotMemberForbidden(t *testing.T) {
	repo := newFakeUserImpersonationRepo()
	repo.users[23] = User{ID: 23, Name: "Bukan Anggota"}
	// TIDAK ada membership di sekolah 1.
	audit := &fakeAudit{}
	hostSchool := SchoolView{ID: 1}

	if _, err := impersonateUser(context.Background(), repo, audit, 10, 1, 23, hostSchool, "", "", time.Now()); err == nil {
		t.Fatal("target bukan member aktif sekolah ini harus ditolak")
	}
}

func TestImpersonateUser_SelfForbidden(t *testing.T) {
	repo := newFakeUserImpersonationRepo()
	audit := &fakeAudit{}
	hostSchool := SchoolView{ID: 1}

	if _, err := impersonateUser(context.Background(), repo, audit, 10, 1, 10, hostSchool, "", "", time.Now()); err == nil {
		t.Fatal("mengimpersonasi diri sendiri harus ditolak")
	}
}

func TestImpersonateUser_GuruAllowed_TTLOneHourNoRenewal(t *testing.T) {
	repo := newFakeUserImpersonationRepo()
	repo.users[30] = User{ID: 30, Name: "Rendi Saputra"}
	repo.memberships[30] = []Membership{{UserID: 30, SchoolID: 1, Role: RoleGuru, Status: "active"}}
	audit := &fakeAudit{}
	hostSchool := SchoolView{ID: 1, Name: "Demo", Slug: "demo"}
	now := time.Date(2026, 8, 16, 3, 0, 0, 0, time.UTC)

	result, err := impersonateUser(context.Background(), repo, audit, 10, 1, 30, hostSchool, "127.0.0.1", "curl", now)
	if err != nil {
		t.Fatalf("guru aktif harus boleh diimpersonasi: %v", err)
	}
	if result.View.Role != RoleGuru || result.View.Name != "Rendi Saputra" || result.View.IsSuperAdmin {
		t.Fatalf("view target salah: %+v", result.View)
	}
	if !result.ExpiresAt.Equal(now.Add(1 * time.Hour)) {
		t.Fatalf("TTL sesi impersonasi USER harus PERSIS 1 jam, got %v (selisih %v)", result.ExpiresAt, result.ExpiresAt.Sub(now))
	}
	if len(repo.sessions) != 1 {
		t.Fatalf("harus membuat tepat 1 sesi, got %d", len(repo.sessions))
	}
	sess := repo.sessions[0]
	if sess.Role != RoleGuru {
		t.Fatalf("role sesi DI DB harus role ASLI target (guru), BUKAN sentinel, got %q", sess.Role)
	}
	if sess.ImpersonatorUserID == nil || *sess.ImpersonatorUserID != 10 {
		t.Fatalf("impersonator_user_id harus terisi = admin (10), got %+v", sess.ImpersonatorUserID)
	}
	if len(audit.calls) != 1 || audit.calls[0].action != "admin.user_impersonate_started" {
		t.Fatalf("audit admin.user_impersonate_started harus terpanggil tepat sekali, got %+v", audit.calls)
	}
	if *audit.calls[0].userID != 10 {
		t.Fatalf("audit actor harus admin (10), got %+v", audit.calls[0].userID)
	}
}

func TestImpersonateUser_StudentIDPopulatedForSiswa(t *testing.T) {
	repo := newFakeUserImpersonationRepo()
	repo.users[40] = User{ID: 40, Name: "Budi Santoso"}
	repo.memberships[40] = []Membership{{UserID: 40, SchoolID: 1, Role: RoleSiswa, Status: "active"}}
	repo.studentIDs[40] = 999
	audit := &fakeAudit{}
	hostSchool := SchoolView{ID: 1}

	result, err := impersonateUser(context.Background(), repo, audit, 10, 1, 40, hostSchool, "", "", time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.View.StudentID != 999 {
		t.Fatalf("StudentID harus terisi utk role siswa, got %d", result.View.StudentID)
	}
}

// -- stopImpersonation (murni) --

func TestStopImpersonation_NotImpersonatingRejected(t *testing.T) {
	repo := newFakeUserImpersonationRepo()
	audit := &fakeAudit{}
	hostSchool := SchoolView{ID: 1}

	// Sesi normal (BUKAN hasil ImpersonateUser) -- token asal-asalan, tidak
	// dikenal repo -> ditolak dengan pesan yang SAMA (errNotImpersonating).
	_, err := stopImpersonation(context.Background(), repo, audit, "token-tidak-dikenal", hostSchool, "", "", time.Now())
	if err != errNotImpersonating {
		t.Fatalf("token tidak dikenal harus errNotImpersonating, got: %v", err)
	}
}

func TestStopImpersonation_EmptyTokenRejected(t *testing.T) {
	repo := newFakeUserImpersonationRepo()
	audit := &fakeAudit{}
	if _, err := stopImpersonation(context.Background(), repo, audit, "", SchoolView{ID: 1}, "", "", time.Now()); err != errNotImpersonating {
		t.Fatalf("token kosong harus errNotImpersonating, got: %v", err)
	}
}

func TestStopImpersonation_RestoresAdminWithNormalTTL(t *testing.T) {
	repo := newFakeUserImpersonationRepo()
	repo.users[30] = User{ID: 30, Name: "Rendi Saputra"}
	repo.memberships[30] = []Membership{{UserID: 30, SchoolID: 1, Role: RoleGuru, Status: "active"}}
	repo.users[10] = User{ID: 10, Name: "Admin Sekolah"}
	audit := &fakeAudit{}
	hostSchool := SchoolView{ID: 1, Name: "Demo", Slug: "demo"}
	now := time.Date(2026, 8, 16, 3, 0, 0, 0, time.UTC)

	// Mulai impersonasi -> dapatkan token sesi impersonasi.
	started, err := impersonateUser(context.Background(), repo, audit, 10, 1, 30, hostSchool, "", "", now)
	if err != nil {
		t.Fatalf("gagal memulai impersonasi: %v", err)
	}

	// Stop pakai token sesi impersonasi itu.
	stopNow := now.Add(20 * time.Minute)
	result, err := stopImpersonation(context.Background(), repo, audit, started.Token, hostSchool, "127.0.0.1", "curl", stopNow)
	if err != nil {
		t.Fatalf("stop harus berhasil: %v", err)
	}
	if result.View.Role != RoleAdminSekolah || result.View.Name != "Admin Sekolah" {
		t.Fatalf("stop harus mengembalikan view ADMIN, got %+v", result.View)
	}
	if !result.ExpiresAt.Equal(stopNow.Add(sessionTTLForRole(RoleAdminSekolah))) {
		t.Fatalf("sesi admin baru harus TTL NORMAL (30 hari), got selisih %v", result.ExpiresAt.Sub(stopNow))
	}
	if len(repo.deletedHashes) != 1 {
		t.Fatalf("sesi impersonasi lama harus dihapus tepat sekali, got %d", len(repo.deletedHashes))
	}
	// 2 sesi total dibuat: 1 saat impersonate, 1 saat stop.
	if len(repo.sessions) != 2 {
		t.Fatalf("expected 2 sesi dibuat total (impersonate+stop), got %d", len(repo.sessions))
	}
	adminSession := repo.sessions[1]
	if adminSession.UserID != 10 || adminSession.Role != RoleAdminSekolah || adminSession.ImpersonatorUserID != nil {
		t.Fatalf("sesi admin baru salah: %+v", adminSession)
	}

	var stoppedCall *auditCall
	for i := range audit.calls {
		if audit.calls[i].action == "admin.user_impersonate_stopped" {
			stoppedCall = &audit.calls[i]
		}
	}
	if stoppedCall == nil {
		t.Fatal("audit admin.user_impersonate_stopped harus terpanggil")
	}
	if *stoppedCall.userID != 10 {
		t.Fatalf("audit actor stop harus admin (10), got %+v", stoppedCall.userID)
	}
}

func TestStopImpersonation_CannotStopTwice(t *testing.T) {
	repo := newFakeUserImpersonationRepo()
	repo.users[30] = User{ID: 30, Name: "Rendi Saputra"}
	repo.memberships[30] = []Membership{{UserID: 30, SchoolID: 1, Role: RoleGuru, Status: "active"}}
	repo.users[10] = User{ID: 10, Name: "Admin Sekolah"}
	audit := &fakeAudit{}
	hostSchool := SchoolView{ID: 1}
	now := time.Now()

	started, err := impersonateUser(context.Background(), repo, audit, 10, 1, 30, hostSchool, "", "", now)
	if err != nil {
		t.Fatalf("gagal memulai impersonasi: %v", err)
	}
	if _, err := stopImpersonation(context.Background(), repo, audit, started.Token, hostSchool, "", "", now); err != nil {
		t.Fatalf("stop pertama harus berhasil: %v", err)
	}
	// Token sesi impersonasi sudah dihapus -> stop lagi dengan token yang
	// sama harus gagal (bukan sesi yang valid lagi).
	if _, err := stopImpersonation(context.Background(), repo, audit, started.Token, hostSchool, "", "", now); err != errNotImpersonating {
		t.Fatalf("stop kedua kali dengan token sama harus errNotImpersonating, got: %v", err)
	}
}
