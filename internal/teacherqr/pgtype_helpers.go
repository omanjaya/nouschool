package teacherqr

import (
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// tsOf — konversi time.Time -> pgtype.Timestamptz NOT NULL (kolom
// teacher_qr_tokens.expires_at/consumed_at SELALU diisi nilai valid oleh
// modul ini, tidak pernah NULL saat ditulis).
func tsOf(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: true}
}
