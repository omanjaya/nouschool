package identity

import (
	"context"
	"fmt"
	"testing"
)

type fakeSetMemberStatusRepo struct {
	users              map[int64]User
	memberships        map[string]Membership // key: fmt userID/schoolID/role
	deletedSessionsFor []int64                // userID yang sesinya dihapus di sekolah ini
	setStatusCalls     int
}

func membershipKey(userID, schoolID int64, role string) string {
	return fmt.Sprintf("%d|%d|%s", userID, schoolID, role)
}

func newFakeSetMemberStatusRepo() *fakeSetMemberStatusRepo {
	return &fakeSetMemberStatusRepo{users: map[int64]User{}, memberships: map[string]Membership{}}
}

func (f *fakeSetMemberStatusRepo) UserByID(ctx context.Context, id int64) (User, error) {
	u, ok := f.users[id]
	if !ok {
		return User{}, ErrNotFound
	}
	return u, nil
}

func (f *fakeSetMemberStatusRepo) GetMembership(ctx context.Context, userID, schoolID int64, role string) (Membership, error) {
	m, ok := f.memberships[membershipKey(userID, schoolID, role)]
	if !ok {
		return Membership{}, ErrNotFound
	}
	return m, nil
}

func (f *fakeSetMemberStatusRepo) SetMembershipStatus(ctx context.Context, userID, schoolID int64, role, status string) error {
	f.setStatusCalls++
	key := membershipKey(userID, schoolID, role)
	m := f.memberships[key]
	m.Status = status
	f.memberships[key] = m
	return nil
}

func (f *fakeSetMemberStatusRepo) DeleteSessionsByUserSchool(ctx context.Context, userID, schoolID int64) error {
	f.deletedSessionsFor = append(f.deletedSessionsFor, userID)
	return nil
}

func TestSetMemberStatus_RejectsSelf(t *testing.T) {
	repo := newFakeSetMemberStatusRepo()
	repo.users[5] = User{ID: 5, Name: "Guru A"}
	repo.memberships[membershipKey(5, 1, RoleGuru)] = Membership{UserID: 5, SchoolID: 1, Role: RoleGuru, Status: "active"}

	_, err := setMemberStatus(context.Background(), repo, nil, 1, 5, 5, RoleGuru, "inactive")
	if err == nil {
		t.Fatal("expected error: tidak bisa mengubah status diri sendiri")
	}
}

func TestSetMemberStatus_RejectsAdminSekolahRole(t *testing.T) {
	repo := newFakeSetMemberStatusRepo()
	repo.users[7] = User{ID: 7, Name: "Admin Lain"}
	repo.memberships[membershipKey(7, 1, RoleAdminSekolah)] = Membership{UserID: 7, SchoolID: 1, Role: RoleAdminSekolah, Status: "active"}

	_, err := setMemberStatus(context.Background(), repo, nil, 1, 1, 7, RoleAdminSekolah, "inactive")
	if err == nil {
		t.Fatal("expected error: tidak bisa menonaktifkan role admin_sekolah")
	}
}

func TestSetMemberStatus_RejectsSuperAdminTarget(t *testing.T) {
	repo := newFakeSetMemberStatusRepo()
	repo.users[9] = User{ID: 9, Name: "Super Admin", IsSuperAdmin: true}
	repo.memberships[membershipKey(9, 1, RoleGuru)] = Membership{UserID: 9, SchoolID: 1, Role: RoleGuru, Status: "active"}

	_, err := setMemberStatus(context.Background(), repo, nil, 1, 1, 9, RoleGuru, "inactive")
	if err == nil {
		t.Fatal("expected error: tidak bisa menonaktifkan super admin")
	}
}

func TestSetMemberStatus_RejectsUnknownStatus(t *testing.T) {
	repo := newFakeSetMemberStatusRepo()
	repo.users[5] = User{ID: 5}
	repo.memberships[membershipKey(5, 1, RoleGuru)] = Membership{UserID: 5, SchoolID: 1, Role: RoleGuru, Status: "active"}

	_, err := setMemberStatus(context.Background(), repo, nil, 1, 1, 5, RoleGuru, "banned")
	if err == nil {
		t.Fatal("expected error: status tidak dikenal")
	}
}

func TestSetMemberStatus_NotFoundMembership(t *testing.T) {
	repo := newFakeSetMemberStatusRepo()
	repo.users[5] = User{ID: 5}
	// TIDAK ada membership (5,1,guru) tersimpan.

	_, err := setMemberStatus(context.Background(), repo, nil, 1, 1, 5, RoleGuru, "inactive")
	if err == nil {
		t.Fatal("expected 404: membership tidak ditemukan")
	}
}

func TestSetMemberStatus_InactiveDeletesSessionsInThisSchoolOnly(t *testing.T) {
	repo := newFakeSetMemberStatusRepo()
	repo.users[5] = User{ID: 5, Name: "Guru A"}
	repo.memberships[membershipKey(5, 1, RoleGuru)] = Membership{UserID: 5, SchoolID: 1, Role: RoleGuru, Status: "active"}

	m, err := setMemberStatus(context.Background(), repo, nil, 1, 1, 5, RoleGuru, "inactive")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if m.Status != "inactive" {
		t.Fatalf("status hasil salah: %+v", m)
	}
	if repo.memberships[membershipKey(5, 1, RoleGuru)].Status != "inactive" {
		t.Fatal("status membership di repo belum berubah")
	}
	if len(repo.deletedSessionsFor) != 1 || repo.deletedSessionsFor[0] != 5 {
		t.Fatalf("seharusnya menghapus sesi user 5 di sekolah ini: %v", repo.deletedSessionsFor)
	}
}

func TestSetMemberStatus_ActiveDoesNotDeleteSessions(t *testing.T) {
	repo := newFakeSetMemberStatusRepo()
	repo.users[5] = User{ID: 5, Name: "Guru A"}
	repo.memberships[membershipKey(5, 1, RoleGuru)] = Membership{UserID: 5, SchoolID: 1, Role: RoleGuru, Status: "inactive"}

	_, err := setMemberStatus(context.Background(), repo, nil, 1, 1, 5, RoleGuru, "active")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(repo.deletedSessionsFor) != 0 {
		t.Fatalf("mengaktifkan kembali TIDAK seharusnya menghapus sesi: %v", repo.deletedSessionsFor)
	}
	if repo.memberships[membershipKey(5, 1, RoleGuru)].Status != "active" {
		t.Fatal("status membership seharusnya jadi active")
	}
}
