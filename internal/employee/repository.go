package employee

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/omanjaya/nouschool/internal/employee/employeedb"
)

var ErrNotFound = errors.New("employee: data tidak ditemukan")
var ErrConflict = errors.New("employee: data sudah ada/bentrok")

// employeeRepository adalah kontrak yang dibutuhkan Service dari repository
// — dipenuhi *Repository secara struktural, supaya Service bisa dites dengan
// fake repository in-memory, tanpa DB.
type employeeRepository interface {
	CreateEmployee(ctx context.Context, schoolID, userID int64, nip string) (Record, error)
	UpdateEmployeeNIP(ctx context.Context, schoolID, id int64, nip string) (Record, error)
	ListEmployees(ctx context.Context, schoolID int64) ([]Record, error)
}

var _ employeeRepository = (*Repository)(nil)

// Repository membungkus akses data modul employee (sqlc + pgx).
type Repository struct {
	pool *pgxpool.Pool
	q    *employeedb.Queries
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool, q: employeedb.New(pool)}
}

func mapNoRows(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	return false
}

// Record adalah representasi domain satu baris employees + identitas user.
type Record struct {
	ID       int64
	UserID   int64
	Name     string
	Email    string
	Username string
	NIP      string
}

func (r Record) view() Employee {
	return Employee{ID: r.ID, UserID: r.UserID, Name: r.Name, Email: r.Email, Username: r.Username, NIP: r.NIP}
}

func (r *Repository) CreateEmployee(ctx context.Context, schoolID, userID int64, nip string) (Record, error) {
	row, err := r.q.CreateEmployee(ctx, employeedb.CreateEmployeeParams{SchoolID: schoolID, UserID: userID, Nip: nip})
	if err != nil {
		if isUniqueViolation(err) {
			return Record{}, ErrConflict
		}
		return Record{}, err
	}
	full, err := r.q.GetEmployeeByID(ctx, employeedb.GetEmployeeByIDParams{ID: row.ID, SchoolID: schoolID})
	if err != nil {
		return Record{}, mapNoRows(err)
	}
	return Record{ID: full.ID, UserID: full.UserID, Name: full.Name, Email: full.Email.String, Username: full.Username.String, NIP: full.Nip}, nil
}

func (r *Repository) UpdateEmployeeNIP(ctx context.Context, schoolID, id int64, nip string) (Record, error) {
	if _, err := r.q.UpdateEmployeeNIP(ctx, employeedb.UpdateEmployeeNIPParams{ID: id, SchoolID: schoolID, Nip: nip}); err != nil {
		return Record{}, mapNoRows(err)
	}
	full, err := r.q.GetEmployeeByID(ctx, employeedb.GetEmployeeByIDParams{ID: id, SchoolID: schoolID})
	if err != nil {
		return Record{}, mapNoRows(err)
	}
	return Record{ID: full.ID, UserID: full.UserID, Name: full.Name, Email: full.Email.String, Username: full.Username.String, NIP: full.Nip}, nil
}

func (r *Repository) ListEmployees(ctx context.Context, schoolID int64) ([]Record, error) {
	rows, err := r.q.ListEmployees(ctx, schoolID)
	if err != nil {
		return nil, err
	}
	out := make([]Record, 0, len(rows))
	for _, row := range rows {
		out = append(out, Record{ID: row.ID, UserID: row.UserID, Name: row.Name, Email: row.Email.String, Username: row.Username.String, NIP: row.Nip})
	}
	return out, nil
}
