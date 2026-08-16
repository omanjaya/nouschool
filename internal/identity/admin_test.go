package identity

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/omanjaya/nouschool/internal/platform/httpx"
)

// -- fakeAdminResetRepo: implementasi adminResetRepo in-memory (tanpa DB) --

type fakeAdminResetRepo struct {
	users             map[int64]User
	memberships       map[int64][]Membership // key: userID
	passwordHashes    map[int64]string
	sessionsDeleted   map[int64]int // key: userID -> berapa kali DeleteSessionsByUser dipanggil
	updatePasswordErr error
}

func newFakeAdminResetRepo() *fakeAdminResetRepo {
	return &fakeAdminResetRepo{
		users: map[int64]User{}, memberships: map[int64][]Membership{},
		passwordHashes: map[int64]string{}, sessionsDeleted: map[int64]int{},
	}
}

func (f *fakeAdminResetRepo) UserByID(ctx context.Context, id int64) (User, error) {
	u, ok := f.users[id]
	if !ok {
		return User{}, ErrNotFound
	}
	return u, nil
}

func (f *fakeAdminResetRepo) ListActiveMemberships(ctx context.Context, userID, schoolID int64) ([]Membership, error) {
	var out []Membership
	for _, m := range f.memberships[userID] {
		if m.SchoolID == schoolID && m.Status == "active" {
			out = append(out, m)
		}
	}
	return out, nil
}

func (f *fakeAdminResetRepo) UpdateUserPassword(ctx context.Context, id int64, hash string) error {
	if f.updatePasswordErr != nil {
		return f.updatePasswordErr
	}
	f.passwordHashes[id] = hash
	return nil
}

func (f *fakeAdminResetRepo) DeleteSessionsByUser(ctx context.Context, userID int64) error {
	f.sessionsDeleted[userID]++
	return nil
}

var _ adminResetRepo = (*fakeAdminResetRepo)(nil)

func fakeHash(password string) (string, error) { return "hashed:" + password, nil }
func fakeGenFixed() (string, error)            { return "abfixed23g", nil }

// -- adminResetPassword: member valid --

func TestAdminResetPasswordSuccess(t *testing.T) {
	repo := newFakeAdminResetRepo()
	repo.users[10] = User{ID: 10, Name: "Rendi", IsSuperAdmin: false}
	repo.memberships[10] = []Membership{{ID: 1, UserID: 10, SchoolID: 7, Role: RoleGuru, Status: "active"}}
	audit := &fakeAudit{}

	temp, err := adminResetPassword(context.Background(), repo, audit, fakeGenFixed, fakeHash, 42, 7, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if temp != "abfixed23g" {
		t.Errorf("temp password = %q, ingin abfixed23g", temp)
	}
	if repo.passwordHashes[10] != "hashed:abfixed23g" {
		t.Errorf("password hash tidak tersimpan dengan benar: %q", repo.passwordHashes[10])
	}
	if repo.sessionsDeleted[10] != 1 {
		t.Errorf("DeleteSessionsByUser seharusnya dipanggil TEPAT sekali, got %d", repo.sessionsDeleted[10])
	}
	if len(audit.calls) != 1 || audit.calls[0].action != "admin.reset_password" {
		t.Fatalf("audit admin.reset_password seharusnya tercatat, got %+v", audit.calls)
	}
	if *audit.calls[0].schoolID != 7 || *audit.calls[0].userID != 42 {
		t.Errorf("audit school_id/user_id salah: %+v", audit.calls[0])
	}
}

// -- adminResetPassword: tolak super admin --

func TestAdminResetPasswordRejectsSuperAdmin(t *testing.T) {
	repo := newFakeAdminResetRepo()
	repo.users[10] = User{ID: 10, Name: "Super", IsSuperAdmin: true}
	audit := &fakeAudit{}

	_, err := adminResetPassword(context.Background(), repo, audit, fakeGenFixed, fakeHash, 42, 7, 10)
	if !errors.Is(err, error(errCannotResetSuperAdmin)) {
		t.Fatalf("ingin errCannotResetSuperAdmin, got %v", err)
	}
	if repo.sessionsDeleted[10] != 0 {
		t.Error("TIDAK boleh menghapus sesi saat ditolak")
	}
	if len(audit.calls) != 0 {
		t.Error("TIDAK boleh audit saat ditolak")
	}
}

// -- adminResetPassword: bukan anggota sekolah ini --

func TestAdminResetPasswordRejectsNonMember(t *testing.T) {
	repo := newFakeAdminResetRepo()
	repo.users[10] = User{ID: 10, Name: "Rendi", IsSuperAdmin: false}
	// Member di sekolah LAIN (8), bukan sekolah target (7).
	repo.memberships[10] = []Membership{{ID: 1, UserID: 10, SchoolID: 8, Role: RoleGuru, Status: "active"}}
	audit := &fakeAudit{}

	_, err := adminResetPassword(context.Background(), repo, audit, fakeGenFixed, fakeHash, 42, 7, 10)
	if err == nil {
		t.Fatal("ingin error (bukan anggota sekolah 7), dapat nil")
	}
	if repo.sessionsDeleted[10] != 0 {
		t.Error("TIDAK boleh menghapus sesi saat ditolak")
	}
}

// -- adminResetPassword: user tidak ditemukan --

func TestAdminResetPasswordUserNotFound(t *testing.T) {
	repo := newFakeAdminResetRepo()
	audit := &fakeAudit{}

	_, err := adminResetPassword(context.Background(), repo, audit, fakeGenFixed, fakeHash, 42, 7, 999)
	if err == nil {
		t.Fatal("ingin error not found")
	}
}

// -- generateTempPassword: charset & panjang --

func TestGenerateTempPasswordCharsetAndLength(t *testing.T) {
	for i := 0; i < 50; i++ {
		pw, err := generateTempPassword()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(pw) != tempPasswordLength {
			t.Fatalf("panjang = %d, ingin %d (pw=%q)", len(pw), tempPasswordLength, pw)
		}
		if strings.ContainsAny(pw, "0o1l") {
			t.Fatalf("password sementara mengandung karakter terlarang (0/o/1/l): %q", pw)
		}
		for _, c := range pw {
			if !strings.ContainsRune(tempPasswordChars, c) {
				t.Fatalf("karakter %q di luar charset yang diizinkan (pw=%q)", c, pw)
			}
		}
	}
}

// -- createSchoolAdmin: P2.1 (fase 13 Gelombang 2) --

// fakeCreateSchoolAdminRepo implementasi createSchoolAdminRepo in-memory.
type fakeCreateSchoolAdminRepo struct {
	byEmail     map[string]User
	byUsername  map[string]User
	nextID      int64
	created     []CreateUserInput
	memberships []Membership
}

func newFakeCreateSchoolAdminRepo() *fakeCreateSchoolAdminRepo {
	return &fakeCreateSchoolAdminRepo{byEmail: map[string]User{}, byUsername: map[string]User{}}
}

func (f *fakeCreateSchoolAdminRepo) UserByEmail(ctx context.Context, email string) (User, error) {
	u, ok := f.byEmail[email]
	if !ok {
		return User{}, ErrNotFound
	}
	return u, nil
}

func (f *fakeCreateSchoolAdminRepo) UserByUsername(ctx context.Context, username string) (User, error) {
	u, ok := f.byUsername[username]
	if !ok {
		return User{}, ErrNotFound
	}
	return u, nil
}

func (f *fakeCreateSchoolAdminRepo) CreateUser(ctx context.Context, in CreateUserInput) (User, error) {
	f.nextID++
	f.created = append(f.created, in)
	u := User{ID: f.nextID, Email: in.Email, Username: in.Username, Name: in.Name, PasswordHash: in.PasswordHash}
	if in.Email != "" {
		f.byEmail[in.Email] = u
	}
	if in.Username != "" {
		f.byUsername[in.Username] = u
	}
	return u, nil
}

func (f *fakeCreateSchoolAdminRepo) CreateMembership(ctx context.Context, userID, schoolID int64, role string) (Membership, error) {
	m := Membership{ID: int64(len(f.memberships) + 1), UserID: userID, SchoolID: schoolID, Role: role, Status: "active"}
	f.memberships = append(f.memberships, m)
	return m, nil
}

var _ createSchoolAdminRepo = (*fakeCreateSchoolAdminRepo)(nil)

func TestCreateSchoolAdminSuccess(t *testing.T) {
	repo := newFakeCreateSchoolAdminRepo()
	audit := &fakeAudit{}

	userID, tempPassword, err := createSchoolAdmin(context.Background(), repo, nil, audit, fakeGenFixed, fakeHash, 42, 7, "Budi", "budi@sekolah.test", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if userID != 1 {
		t.Errorf("userID = %d, ingin 1", userID)
	}
	if tempPassword != "abfixed23g" {
		t.Errorf("temp password = %q, ingin abfixed23g", tempPassword)
	}
	if len(repo.memberships) != 1 || repo.memberships[0].Role != RoleAdminSekolah || repo.memberships[0].SchoolID != 7 {
		t.Fatalf("membership admin_sekolah tidak dibuat dengan benar: %+v", repo.memberships)
	}
	if len(audit.calls) != 1 || audit.calls[0].action != "admin.create_school_admin" {
		t.Fatalf("audit admin.create_school_admin seharusnya tercatat, got %+v", audit.calls)
	}
	if *audit.calls[0].schoolID != 7 || *audit.calls[0].userID != 42 {
		t.Errorf("audit school_id/user_id salah: %+v", audit.calls[0])
	}
}

func TestCreateSchoolAdminRequiresName(t *testing.T) {
	repo := newFakeCreateSchoolAdminRepo()
	_, _, err := createSchoolAdmin(context.Background(), repo, nil, nil, fakeGenFixed, fakeHash, 42, 7, "  ", "budi@sekolah.test", "")
	if err != errSchoolAdminNameRequired {
		t.Fatalf("error = %v, ingin errSchoolAdminNameRequired", err)
	}
}

func TestCreateSchoolAdminRequiresIdentifier(t *testing.T) {
	repo := newFakeCreateSchoolAdminRepo()
	_, _, err := createSchoolAdmin(context.Background(), repo, nil, nil, fakeGenFixed, fakeHash, 42, 7, "Budi", "", "")
	if err != errSchoolAdminIdentifierRequired {
		t.Fatalf("error = %v, ingin errSchoolAdminIdentifierRequired", err)
	}
}

func TestCreateSchoolAdminRejectsDuplicateEmail(t *testing.T) {
	repo := newFakeCreateSchoolAdminRepo()
	repo.byEmail["budi@sekolah.test"] = User{ID: 99, Email: "budi@sekolah.test"}

	_, _, err := createSchoolAdmin(context.Background(), repo, nil, nil, fakeGenFixed, fakeHash, 42, 7, "Budi", "budi@sekolah.test", "")
	var de *httpx.Error
	if !errors.As(err, &de) || de.Status != 409 {
		t.Fatalf("ingin error 409 conflict, got %v", err)
	}
	if len(repo.created) != 0 {
		t.Error("TIDAK boleh membuat user baru saat email bentrok")
	}
}

func TestCreateSchoolAdminRejectsDuplicateUsername(t *testing.T) {
	repo := newFakeCreateSchoolAdminRepo()
	repo.byUsername["budi"] = User{ID: 99, Username: "budi"}

	_, _, err := createSchoolAdmin(context.Background(), repo, nil, nil, fakeGenFixed, fakeHash, 42, 7, "Budi", "", "budi")
	var de *httpx.Error
	if !errors.As(err, &de) || de.Status != 409 {
		t.Fatalf("ingin error 409 conflict, got %v", err)
	}
}

func TestCreateSchoolAdminRejectsMissingSchool(t *testing.T) {
	repo := newFakeCreateSchoolAdminRepo()
	schools := fakeSchoolGateway{status: "", slug: "", found: false}

	_, _, err := createSchoolAdmin(context.Background(), repo, schools, nil, fakeGenFixed, fakeHash, 42, 999, "Budi", "budi@sekolah.test", "")
	if !errors.Is(err, error(httpx.ErrNotFound)) {
		t.Fatalf("ingin httpx.ErrNotFound, got %v", err)
	}
}

// -- listAuditLogPage: pagination + filter --

type fakeAuditLogRepo struct {
	rows []AuditLogRow
}

func (f *fakeAuditLogRepo) matching(actionPrefix string) []AuditLogRow {
	if actionPrefix == "" {
		return f.rows
	}
	var out []AuditLogRow
	for _, r := range f.rows {
		if strings.HasPrefix(r.Action, actionPrefix) {
			out = append(out, r)
		}
	}
	return out
}

func (f *fakeAuditLogRepo) ListAuditLogForSchool(ctx context.Context, schoolID int64, actionPrefix string, limit, offset int32) ([]AuditLogRow, error) {
	matched := f.matching(actionPrefix)
	start := int(offset)
	if start > len(matched) {
		start = len(matched)
	}
	end := start + int(limit)
	if end > len(matched) {
		end = len(matched)
	}
	return matched[start:end], nil
}

func (f *fakeAuditLogRepo) CountAuditLogForSchool(ctx context.Context, schoolID int64, actionPrefix string) (int64, error) {
	return int64(len(f.matching(actionPrefix))), nil
}

var _ auditLogRepo = (*fakeAuditLogRepo)(nil)

func makeAuditRows(n int, action string) []AuditLogRow {
	out := make([]AuditLogRow, 0, n)
	base := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < n; i++ {
		out = append(out, AuditLogRow{ID: int64(i + 1), Action: action, Entity: "x", At: base.Add(time.Duration(i) * time.Hour)})
	}
	return out
}

func TestListAuditLogPagePagination(t *testing.T) {
	repo := &fakeAuditLogRepo{}
	repo.rows = append(repo.rows, makeAuditRows(5, "attendance.update")...)

	page, err := listAuditLogPage(context.Background(), repo, 1, 1, 2, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if page.Total != 5 {
		t.Errorf("total = %d, ingin 5", page.Total)
	}
	if len(page.Items) != 2 {
		t.Fatalf("items = %d, ingin 2 (per_page=2)", len(page.Items))
	}

	page2, err := listAuditLogPage(context.Background(), repo, 1, 3, 2, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(page2.Items) != 1 {
		t.Fatalf("halaman terakhir seharusnya sisa 1 item, got %d", len(page2.Items))
	}
}

func TestListAuditLogPageFilterByActionPrefix(t *testing.T) {
	repo := &fakeAuditLogRepo{}
	repo.rows = append(repo.rows, makeAuditRows(3, "attendance.update")...)
	repo.rows = append(repo.rows, makeAuditRows(2, "leave.decide")...)

	page, err := listAuditLogPage(context.Background(), repo, 1, 1, 50, "attendance.")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if page.Total != 3 {
		t.Errorf("total dgn filter attendance. = %d, ingin 3", page.Total)
	}
	for _, it := range page.Items {
		if !strings.HasPrefix(it.Action, "attendance.") {
			t.Errorf("item lolos filter salah: %+v", it)
		}
	}
}

func TestListAuditLogPageDefaultsAndClamp(t *testing.T) {
	repo := &fakeAuditLogRepo{}
	repo.rows = append(repo.rows, makeAuditRows(1, "x")...)

	// page=0/perPage=0 -> default 1/50.
	if _, err := listAuditLogPage(context.Background(), repo, 1, 0, 0, ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// perPage > 200 -> clamp 200 (tidak error).
	if _, err := listAuditLogPage(context.Background(), repo, 1, 1, 9999, ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
