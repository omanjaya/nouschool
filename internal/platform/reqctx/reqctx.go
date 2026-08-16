// Package reqctx menyimpan data hasil resolusi tenant & sesi login di
// request context. Ini satu-satunya cara modul lain membaca school_id /
// user_id / role — TIDAK PERNAH dari body, query param, atau path
// (lihat aturan multi-tenant di CLAUDE.md).
package reqctx

import "context"

// School adalah data sekolah minimal yang dibutuhkan lintas modul,
// hasil resolusi Host header oleh middleware ResolveTenant.
type School struct {
	ID   int64
	Name string
	Slug string
	// Timezone dipakai logika "hari absensi" (lihat platform/clock);
	// Status dipakai enforcement langganan (billing, fase lanjut).
	Timezone string
	Status   string
}

type ctxKey int

const (
	keySchool ctxKey = iota
	keyPlatform
	keyUserID
	keyRole
	keySuperAdmin
	keyImpersonatorUserID
)

// WithSchool menandai request sebagai konteks tenant (host sekolah).
func WithSchool(ctx context.Context, school School) context.Context {
	return context.WithValue(ctx, keySchool, school)
}

// SchoolFromContext mengembalikan data sekolah dari context, jika ada.
func SchoolFromContext(ctx context.Context) (School, bool) {
	s, ok := ctx.Value(keySchool).(School)
	return s, ok
}

// SchoolID mengembalikan school_id dari context, atau 0 bila konteks platform.
func SchoolID(ctx context.Context) int64 {
	s, ok := SchoolFromContext(ctx)
	if !ok {
		return 0
	}
	return s.ID
}

// WithPlatform menandai request sebagai konteks platform (host base domain
// atau admin.<base_domain>) — tanpa sekolah spesifik.
func WithPlatform(ctx context.Context) context.Context {
	return context.WithValue(ctx, keyPlatform, true)
}

// IsPlatform melaporkan apakah request ini berada di host platform.
func IsPlatform(ctx context.Context) bool {
	v, _ := ctx.Value(keyPlatform).(bool)
	return v
}

// WithUser menandai request dengan identitas user hasil sesi login.
func WithUser(ctx context.Context, userID int64, role string, isSuperAdmin bool) context.Context {
	ctx = context.WithValue(ctx, keyUserID, userID)
	ctx = context.WithValue(ctx, keyRole, role)
	ctx = context.WithValue(ctx, keySuperAdmin, isSuperAdmin)
	return ctx
}

// UserID mengembalikan id user yang sedang login, atau 0 bila tidak login.
func UserID(ctx context.Context) int64 {
	v, _ := ctx.Value(keyUserID).(int64)
	return v
}

// Role mengembalikan role aktif sesi (role membership, atau "super_admin").
func Role(ctx context.Context) string {
	v, _ := ctx.Value(keyRole).(string)
	return v
}

// IsSuperAdmin melaporkan apakah user yang login adalah super admin.
func IsSuperAdmin(ctx context.Context) bool {
	v, _ := ctx.Value(keySuperAdmin).(bool)
	return v
}

// WithImpersonator menandai request sebagai sesi impersonasi USER (Fase 14
// Gelombang D, docs/12-sion-parity.md) — impersonatorUserID adalah admin
// sekolah yang sedang "menyamar" sebagai user login saat ini (lihat
// internal/identity/impersonation_user.go). TIDAK dipanggil sama sekali utk
// sesi normal (lihat ImpersonatorUserID di bawah: ok=false berarti bukan
// sesi impersonasi).
func WithImpersonator(ctx context.Context, impersonatorUserID int64) context.Context {
	return context.WithValue(ctx, keyImpersonatorUserID, impersonatorUserID)
}

// ImpersonatorUserID mengembalikan id admin yang sedang mengimpersonasi sesi
// ini, ok=false bila sesi ini BUKAN sesi impersonasi (mayoritas request).
func ImpersonatorUserID(ctx context.Context) (int64, bool) {
	v, ok := ctx.Value(keyImpersonatorUserID).(int64)
	return v, ok
}
