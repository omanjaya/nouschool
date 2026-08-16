package employee

import (
	"context"
	"crypto/rand"
	"errors"
	"strings"

	"github.com/omanjaya/nouschool/internal/platform/httpx"
	"github.com/omanjaya/nouschool/internal/platform/reqctx"
)

// -- consumer-side interface (lihat CLAUDE.md) --

// IdentityGateway adalah kebutuhan modul employee dari modul identity —
// signature primitif, dipenuhi *identity.Service secara STRUKTURAL (pola
// yang sama dengan internal/student.IdentityGateway & internal/identity's
// P2.1/P4.2 temp-password).
type IdentityGateway interface {
	HasPermission(role, perm string) bool
	Log(ctx context.Context, schoolID, userID *int64, action, entity string, entityID *int64, oldValue, newValue any) error
	UserIDByEmail(ctx context.Context, email string) (int64, bool, error)
	UserIDByUsername(ctx context.Context, username string) (int64, bool, error)
	CreateAccount(ctx context.Context, email, username, passwordHash, name string) (int64, error)
	CreateMembership(ctx context.Context, userID, schoolID int64, role string) error
	HashPassword(password string) (string, error)
}

// Service berisi aturan bisnis modul employee.
type Service struct {
	repo     employeeRepository
	identity IdentityGateway
}

func NewService(repo *Repository, identity IdentityGateway) *Service {
	return &Service{repo: repo, identity: identity}
}

// newServiceForTest membangun Service dengan repository FAKE (in-memory,
// tanpa DB) — dipakai test di package ini saja.
func newServiceForTest(repo employeeRepository, identity IdentityGateway) *Service {
	return &Service{repo: repo, identity: identity}
}

func (s *Service) audit(ctx context.Context, schoolID, actorUserID int64, action, entity string, entityID int64, oldValue, newValue any) {
	sid, uid, eid := schoolID, actorUserID, entityID
	_ = s.identity.Log(ctx, &sid, &uid, action, entity, &eid, oldValue, newValue)
}

func (s *Service) requireManage(ctx context.Context) error {
	if !s.identity.HasPermission(reqctx.Role(ctx), PermManage) {
		return httpx.ErrForbidden
	}
	return nil
}

// tempPasswordChars/Length — pola IDENTIK internal/identity/admin.go
// generateTempPassword (a-z0-9 TANPA 0/o/1/l, mudah dibaca) — didefinisikan
// ulang LOKAL karena employee TIDAK boleh mengimpor identity (lihat CLAUDE.md).
const tempPasswordChars = "abcdefghijkmnpqrstuvwxyz23456789"
const tempPasswordLength = 10

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

var errNameRequired = httpx.Validation("Nama pegawai wajib diisi.")
var errIdentifierRequired = httpx.Validation("Isi minimal salah satu: email atau username.")

func conflictIdentifier(field string) *httpx.Error {
	return httpx.Conflict(field + " sudah dipakai akun lain.")
}

// CreateEmployeeInput adalah parameter Service.CreateEmployee.
type CreateEmployeeInput struct {
	Name     string
	Email    string
	Username string
	NIP      string
}

// CreateEmployee — POST /api/employees: buat user + membership pegawai +
// profil employees, mengembalikan temp_password SEKALI TAMPIL (pola
// internal/identity/admin.go createSchoolAdmin/adminResetPassword).
func (s *Service) CreateEmployee(ctx context.Context, actorUserID, schoolID int64, in CreateEmployeeInput) (CreatedEmployee, error) {
	if err := s.requireManage(ctx); err != nil {
		return CreatedEmployee{}, err
	}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return CreatedEmployee{}, errNameRequired
	}
	email := strings.ToLower(strings.TrimSpace(in.Email))
	username := strings.TrimSpace(in.Username)
	if email == "" && username == "" {
		return CreatedEmployee{}, errIdentifierRequired
	}

	if email != "" {
		if _, ok, err := s.identity.UserIDByEmail(ctx, email); err != nil {
			return CreatedEmployee{}, err
		} else if ok {
			return CreatedEmployee{}, conflictIdentifier("Email")
		}
	}
	if username != "" {
		if _, ok, err := s.identity.UserIDByUsername(ctx, username); err != nil {
			return CreatedEmployee{}, err
		} else if ok {
			return CreatedEmployee{}, conflictIdentifier("Username")
		}
	}

	tempPassword, err := generateTempPassword()
	if err != nil {
		return CreatedEmployee{}, err
	}
	hash, err := s.identity.HashPassword(tempPassword)
	if err != nil {
		return CreatedEmployee{}, err
	}
	userID, err := s.identity.CreateAccount(ctx, email, username, hash, name)
	if err != nil {
		return CreatedEmployee{}, err
	}
	if err := s.identity.CreateMembership(ctx, userID, schoolID, RolePegawai); err != nil {
		return CreatedEmployee{}, err
	}
	nip := strings.TrimSpace(in.NIP)
	rec, err := s.repo.CreateEmployee(ctx, schoolID, userID, nip)
	if err != nil {
		if errors.Is(err, ErrConflict) {
			return CreatedEmployee{}, httpx.Conflict("User ini sudah punya profil pegawai.")
		}
		return CreatedEmployee{}, err
	}

	s.audit(ctx, schoolID, actorUserID, "employee.create", "employee", rec.ID, nil,
		map[string]any{"name": name, "email": email, "username": username, "nip": nip})

	return CreatedEmployee{Employee: rec.view(), TempPassword: tempPassword}, nil
}

// UpdateEmployee — PATCH /api/employees/{id}: saat ini hanya NIP yang bisa
// diubah (pola sama internal/student.UpdateTeacher — profil tipis).
func (s *Service) UpdateEmployee(ctx context.Context, actorUserID, schoolID, id int64, nip string) (Employee, error) {
	if err := s.requireManage(ctx); err != nil {
		return Employee{}, err
	}
	rec, err := s.repo.UpdateEmployeeNIP(ctx, schoolID, id, strings.TrimSpace(nip))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return Employee{}, httpx.ErrNotFound
		}
		return Employee{}, err
	}
	s.audit(ctx, schoolID, actorUserID, "employee.update", "employee", id, nil, map[string]any{"nip": rec.NIP})
	return rec.view(), nil
}

func (s *Service) ListEmployees(ctx context.Context, schoolID int64) ([]Employee, error) {
	if err := s.requireManage(ctx); err != nil {
		return nil, err
	}
	rows, err := s.repo.ListEmployees(ctx, schoolID)
	if err != nil {
		return nil, err
	}
	out := make([]Employee, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.view())
	}
	return out, nil
}
