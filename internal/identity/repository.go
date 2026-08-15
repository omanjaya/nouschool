package identity

import (
	"context"
	"errors"
	"net/netip"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/omanjaya/nouschool/internal/identity/identitydb"
)

// ErrNotFound menandai baris tidak ditemukan di repository modul identity.
var ErrNotFound = errors.New("identity: data tidak ditemukan")

// Repository membungkus akses data modul identity (sqlc + pgx).
type Repository struct {
	pool *pgxpool.Pool
	q    *identitydb.Queries
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool, q: identitydb.New(pool)}
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

func textOrNil(s string) pgtype.Text {
	if s == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: s, Valid: true}
}

func int8OrNil(v *int64) pgtype.Int8 {
	if v == nil {
		return pgtype.Int8{}
	}
	return pgtype.Int8{Int64: *v, Valid: true}
}

func int8ToPtr(v pgtype.Int8) *int64 {
	if !v.Valid {
		return nil
	}
	x := v.Int64
	return &x
}

func userFromDB(u identitydb.User) User {
	return User{
		ID:           u.ID,
		Email:        u.Email.String,
		Username:     u.Username.String,
		PasswordHash: u.PasswordHash,
		Name:         u.Name,
		Phone:        u.Phone.String,
		IsSuperAdmin: u.IsSuperAdmin,
		CreatedAt:    u.CreatedAt.Time,
	}
}

func membershipFromDB(m identitydb.Membership) Membership {
	return Membership{ID: m.ID, UserID: m.UserID, SchoolID: m.SchoolID, Role: m.Role, Status: m.Status}
}

// -- users --

func (r *Repository) UserByEmail(ctx context.Context, email string) (User, error) {
	row, err := r.q.GetUserByEmail(ctx, textOrNil(email))
	if err != nil {
		return User{}, mapNoRows(err)
	}
	return userFromDB(row), nil
}

func (r *Repository) UserByUsername(ctx context.Context, username string) (User, error) {
	row, err := r.q.GetUserByUsername(ctx, textOrNil(username))
	if err != nil {
		return User{}, mapNoRows(err)
	}
	return userFromDB(row), nil
}

func (r *Repository) UserByID(ctx context.Context, id int64) (User, error) {
	row, err := r.q.GetUserByID(ctx, id)
	if err != nil {
		return User{}, mapNoRows(err)
	}
	return userFromDB(row), nil
}

type CreateUserInput struct {
	Email        string
	Username     string
	PasswordHash string
	Name         string
	Phone        string
	IsSuperAdmin bool
}

func (r *Repository) CreateUser(ctx context.Context, in CreateUserInput) (User, error) {
	row, err := r.q.CreateUser(ctx, identitydb.CreateUserParams{
		Email:        textOrNil(in.Email),
		Username:     textOrNil(in.Username),
		PasswordHash: in.PasswordHash,
		Name:         in.Name,
		Phone:        textOrNil(in.Phone),
		IsSuperAdmin: in.IsSuperAdmin,
	})
	if err != nil {
		return User{}, err
	}
	return userFromDB(row), nil
}

func (r *Repository) UpdateUserPassword(ctx context.Context, id int64, hash string) error {
	return r.q.UpdateUserPassword(ctx, identitydb.UpdateUserPasswordParams{ID: id, PasswordHash: hash})
}

func (r *Repository) SetUserName(ctx context.Context, id int64, name string) error {
	return r.q.SetUserName(ctx, identitydb.SetUserNameParams{ID: id, Name: name})
}

func (r *Repository) SetSuperAdmin(ctx context.Context, id int64, v bool) error {
	return r.q.SetSuperAdmin(ctx, identitydb.SetSuperAdminParams{ID: id, IsSuperAdmin: v})
}

// -- memberships --

func (r *Repository) ListActiveMemberships(ctx context.Context, userID, schoolID int64) ([]Membership, error) {
	rows, err := r.q.ListActiveMembershipsByUserSchool(ctx, identitydb.ListActiveMembershipsByUserSchoolParams{
		UserID: userID, SchoolID: schoolID,
	})
	if err != nil {
		return nil, err
	}
	out := make([]Membership, 0, len(rows))
	for _, row := range rows {
		out = append(out, membershipFromDB(row))
	}
	return out, nil
}

// CreateMembership upsert membership (ON CONFLICT set status=active) —
// idempoten, dipakai bootstrap & undangan.
func (r *Repository) CreateMembership(ctx context.Context, userID, schoolID int64, role string) (Membership, error) {
	row, err := r.q.CreateMembership(ctx, identitydb.CreateMembershipParams{UserID: userID, SchoolID: schoolID, Role: role})
	if err != nil {
		return Membership{}, err
	}
	return membershipFromDB(row), nil
}

// -- sessions --

func parseIP(s string) *netip.Addr {
	if s == "" {
		return nil
	}
	addr, err := netip.ParseAddr(s)
	if err != nil {
		return nil
	}
	return &addr
}

type CreateSessionInput struct {
	UserID    int64
	SchoolID  *int64 // nil untuk sesi super admin di host platform
	TokenHash []byte
	Role      string
	ExpiresAt time.Time
	IP        string
	UserAgent string
}

func (r *Repository) CreateSession(ctx context.Context, in CreateSessionInput) error {
	_, err := r.q.CreateSession(ctx, identitydb.CreateSessionParams{
		UserID:    in.UserID,
		SchoolID:  int8OrNil(in.SchoolID),
		TokenHash: in.TokenHash,
		Role:      in.Role,
		ExpiresAt: pgtype.Timestamptz{Time: in.ExpiresAt, Valid: true},
		Ip:        parseIP(in.IP),
		UserAgent: textOrNil(in.UserAgent),
	})
	return err
}

// SessionRow adalah hasil join sessions+users dipakai RequireAuth.
type SessionRow struct {
	ID           int64
	UserID       int64
	SchoolID     *int64
	Role         string
	ExpiresAt    time.Time
	UserName     string
	IsSuperAdmin bool
}

func (r *Repository) SessionByTokenHash(ctx context.Context, hash []byte) (SessionRow, error) {
	row, err := r.q.GetSessionByTokenHash(ctx, hash)
	if err != nil {
		return SessionRow{}, mapNoRows(err)
	}
	return SessionRow{
		ID:           row.ID,
		UserID:       row.UserID,
		SchoolID:     int8ToPtr(row.SchoolID),
		Role:         row.Role,
		ExpiresAt:    row.ExpiresAt.Time,
		UserName:     row.UserName,
		IsSuperAdmin: row.UserIsSuperAdmin,
	}, nil
}

func (r *Repository) ExtendSession(ctx context.Context, id int64, expiresAt time.Time) error {
	return r.q.ExtendSession(ctx, identitydb.ExtendSessionParams{ID: id, ExpiresAt: pgtype.Timestamptz{Time: expiresAt, Valid: true}})
}

func (r *Repository) DeleteSessionByTokenHash(ctx context.Context, hash []byte) error {
	return r.q.DeleteSessionByTokenHash(ctx, hash)
}

// -- audit --

type InsertAuditLogInput struct {
	SchoolID *int64
	UserID   *int64
	Action   string
	Entity   string
	EntityID *int64
	OldValue []byte
	NewValue []byte
}

func (r *Repository) InsertAuditLog(ctx context.Context, in InsertAuditLogInput) error {
	return r.q.InsertAuditLog(ctx, identitydb.InsertAuditLogParams{
		SchoolID: int8OrNil(in.SchoolID),
		UserID:   int8OrNil(in.UserID),
		Action:   in.Action,
		Entity:   in.Entity,
		EntityID: int8OrNil(in.EntityID),
		OldValue: in.OldValue,
		NewValue: in.NewValue,
	})
}
