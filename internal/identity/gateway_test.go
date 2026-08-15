package identity

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

// fakeInvitationRepo implementasi invitationCodeRepo in-memory (tanpa DB),
// dipakai menguji logika unik/idempoten generateInvitationCode.
type fakeInvitationRepo struct {
	byCode map[string]Invitation
	nextID int64
}

func newFakeInvitationRepo() *fakeInvitationRepo {
	return &fakeInvitationRepo{byCode: make(map[string]Invitation)}
}

func (f *fakeInvitationRepo) ActiveInvitation(ctx context.Context, schoolID int64, role string, targetID int64, now time.Time) (Invitation, error) {
	for _, inv := range f.byCode {
		if inv.SchoolID == schoolID && inv.Role == role && inv.TargetID == targetID &&
			inv.UsedAt == nil && inv.ExpiresAt.After(now) {
			return inv, nil
		}
	}
	return Invitation{}, ErrNotFound
}

func (f *fakeInvitationRepo) CreateInvitation(ctx context.Context, schoolID int64, code, role string, targetID int64, expiresAt time.Time) (Invitation, error) {
	if _, exists := f.byCode[code]; exists {
		return Invitation{}, fmt.Errorf("identity: kode %q sudah dipakai (unique violation)", code)
	}
	f.nextID++
	inv := Invitation{ID: f.nextID, SchoolID: schoolID, Code: code, Role: role, TargetID: targetID, ExpiresAt: expiresAt}
	f.byCode[code] = inv
	return inv, nil
}

func TestGenerateInvitationCodeCreatesValidUniqueCode(t *testing.T) {
	repo := newFakeInvitationRepo()
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	code1, exp1, err := generateInvitationCode(context.Background(), repo, 1, RoleSiswa, 100, 30*24*time.Hour, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(code1) != inviteCodeLen {
		t.Fatalf("kode harus %d karakter, dapat %q (%d)", inviteCodeLen, code1, len(code1))
	}
	for _, c := range code1 {
		if !strings.ContainsRune(inviteCodeChars, c) {
			t.Fatalf("karakter %q di luar charset undangan %q", c, inviteCodeChars)
		}
	}
	if !exp1.Equal(now.Add(30 * 24 * time.Hour)) {
		t.Fatalf("expiry salah: %v", exp1)
	}

	// Role berbeda untuk target yang sama -> kode berbeda (baris invitation berbeda).
	code2, _, err := generateInvitationCode(context.Background(), repo, 1, RoleOrangTua, 100, 30*24*time.Hour, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code2 == code1 {
		t.Fatal("kode untuk role berbeda seharusnya tidak sama")
	}
	if len(repo.byCode) != 2 {
		t.Fatalf("seharusnya ada 2 baris invitation, dapat %d", len(repo.byCode))
	}
}

func TestGenerateInvitationCodeReuseIdempotent(t *testing.T) {
	repo := newFakeInvitationRepo()
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	code1, exp1, err := generateInvitationCode(context.Background(), repo, 1, RoleSiswa, 100, 30*24*time.Hour, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Panggil lagi beberapa saat kemudian (masih dalam masa berlaku, belum
	// dipakai) -> kode SAMA dipakai ulang, TIDAK membuat baris baru.
	later := now.Add(time.Hour)
	code2, exp2, err := generateInvitationCode(context.Background(), repo, 1, RoleSiswa, 100, 30*24*time.Hour, later)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code2 != code1 {
		t.Fatalf("kode aktif seharusnya dipakai ulang: %q vs %q", code2, code1)
	}
	if !exp2.Equal(exp1) {
		t.Fatalf("expiry seharusnya tidak berubah saat reuse: %v vs %v", exp2, exp1)
	}
	if len(repo.byCode) != 1 {
		t.Fatalf("reuse seharusnya tidak menambah baris, dapat %d baris", len(repo.byCode))
	}
}

func TestGenerateInvitationCodeExpiredIsNotReused(t *testing.T) {
	repo := newFakeInvitationRepo()
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	code1, _, err := generateInvitationCode(context.Background(), repo, 1, RoleSiswa, 100, time.Hour, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	afterExpiry := now.Add(2 * time.Hour)
	code2, _, err := generateInvitationCode(context.Background(), repo, 1, RoleSiswa, 100, time.Hour, afterExpiry)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code2 == code1 {
		t.Fatal("kode yang sudah kedaluwarsa seharusnya TIDAK dipakai ulang")
	}
	if len(repo.byCode) != 2 {
		t.Fatalf("seharusnya ada 2 baris invitation setelah kode lama kedaluwarsa, dapat %d", len(repo.byCode))
	}
}

func TestGenerateInvitationCodeUsedIsNotReused(t *testing.T) {
	repo := newFakeInvitationRepo()
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	code1, _, err := generateInvitationCode(context.Background(), repo, 1, RoleSiswa, 100, 30*24*time.Hour, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	inv := repo.byCode[code1]
	used := now.Add(time.Minute)
	inv.UsedAt = &used
	repo.byCode[code1] = inv

	code2, _, err := generateInvitationCode(context.Background(), repo, 1, RoleSiswa, 100, 30*24*time.Hour, now.Add(2*time.Minute))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code2 == code1 {
		t.Fatal("kode yang sudah dipakai seharusnya TIDAK dipakai ulang")
	}
}
