package exitpermit

import (
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// Helper konversi pgtype <-> tipe Go primitif — pola SAMA
// internal/studentleave/pgtype_helpers.go (didefinisikan ulang di sini
// karena exitpermit tidak mengimpor studentleave).

func textOrNilFrom(s string) pgtype.Text {
	if s == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: s, Valid: true}
}

func int8OrNil(v int64) pgtype.Int8 {
	if v == 0 {
		return pgtype.Int8{}
	}
	return pgtype.Int8{Int64: v, Valid: true}
}

func int8ToInt64(v pgtype.Int8) int64 {
	if !v.Valid {
		return 0
	}
	return v.Int64
}

func tsToTime(t pgtype.Timestamptz) time.Time {
	if !t.Valid {
		return time.Time{}
	}
	return t.Time
}

func tsOrNil(t time.Time) pgtype.Timestamptz {
	if t.IsZero() {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: t, Valid: true}
}
