package teacherqr

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/omanjaya/nouschool/internal/teacherqr/teacherqrdb"
)

// teacherQRRepository adalah kontrak yang dibutuhkan Service dari repository
// — dipenuhi *Repository secara struktural, supaya Service bisa dites dengan
// fake repository in-memory, tanpa DB (pola sama modul lain).
type teacherQRRepository interface {
	DeleteExpiredForUser(ctx context.Context, schoolID, userID int64, now time.Time) error
	CreateToken(ctx context.Context, schoolID, userID int64, token string, expiresAt time.Time) (int64, error)
	// ConsumeToken menandai token terpakai secara ATOMIK (UPDATE ... WHERE
	// consumed_at IS NULL AND expires_at > now RETURNING user_id) — dua
	// pemanggil bersamaan pada token yang sama, HANYA SATU yang berhasil
	// (row lock implisit UPDATE Postgres). ok=false bila token tidak
	// dikenal/sudah dipakai/sudah kedaluwarsa.
	ConsumeToken(ctx context.Context, schoolID int64, token string, now time.Time) (userID int64, ok bool, err error)
}

var _ teacherQRRepository = (*Repository)(nil)

// Repository membungkus akses data modul teacherqr (sqlc + pgx).
type Repository struct {
	pool *pgxpool.Pool
	q    *teacherqrdb.Queries
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool, q: teacherqrdb.New(pool)}
}

func (r *Repository) DeleteExpiredForUser(ctx context.Context, schoolID, userID int64, now time.Time) error {
	return r.q.DeleteExpiredTeacherQRTokensForUser(ctx, teacherqrdb.DeleteExpiredTeacherQRTokensForUserParams{
		SchoolID: schoolID, UserID: userID, Now: tsOf(now),
	})
}

func (r *Repository) CreateToken(ctx context.Context, schoolID, userID int64, token string, expiresAt time.Time) (int64, error) {
	return r.q.CreateTeacherQRToken(ctx, teacherqrdb.CreateTeacherQRTokenParams{
		SchoolID: schoolID, UserID: userID, Token: token, ExpiresAt: tsOf(expiresAt),
	})
}

func (r *Repository) ConsumeToken(ctx context.Context, schoolID int64, token string, now time.Time) (int64, bool, error) {
	userID, err := r.q.ConsumeTeacherQRToken(ctx, teacherqrdb.ConsumeTeacherQRTokenParams{
		Token: token, SchoolID: schoolID, Now: tsOf(now),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, false, nil
		}
		return 0, false, err
	}
	return userID, true, nil
}
