package identity

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"net/http"
	"time"
)

// Berkas ini berisi method Service yang dipakai modul LAIN (mis. student)
// lewat consumer-side interface kecil yang dideklarasikan di sisi pemakai
// (lihat CLAUDE.md). Semua signature sengaja hanya memakai tipe primitif
// supaya modul pemakai TIDAK perlu mengimpor package identity untuk
// memenuhi interface-nya secara struktural.

// HashPassword membungkus fungsi paket HashPassword sebagai method Service.
func (s *Service) HashPassword(password string) (string, error) {
	return HashPassword(password)
}

// HasPermission membungkus fungsi paket HasPermission (rbac.go) sebagai
// method Service, supaya modul lain bisa melakukan gerbang permission yang
// SAMA dari layer service-nya sendiri untuk endpoint dengan otorisasi
// campuran (permission ATAU object-level), tanpa mengimpor identity.
func (s *Service) HasPermission(role, perm string) bool {
	return HasPermission(role, perm)
}

// CreateAccount membuat user baru (email dan/atau username, minimal salah
// satu wajib diisi — ditegakkan CHECK constraint di DB). Dipakai modul
// student untuk membuat akun guru & aktivasi undangan siswa/wali.
func (s *Service) CreateAccount(ctx context.Context, email, username, passwordHash, name string) (int64, error) {
	u, err := s.repo.CreateUser(ctx, CreateUserInput{
		Email: email, Username: username, PasswordHash: passwordHash, Name: name,
	})
	if err != nil {
		return 0, err
	}
	return u.ID, nil
}

// UserIDByEmail mengembalikan (id, true, nil) bila ditemukan, (0, false, nil)
// bila tidak — TIDAK mengembalikan error domain ErrNotFound (modul pemakai
// tidak perlu tahu tipe error identity).
func (s *Service) UserIDByEmail(ctx context.Context, email string) (int64, bool, error) {
	if email == "" {
		return 0, false, nil
	}
	u, err := s.repo.UserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return 0, false, nil
		}
		return 0, false, err
	}
	return u.ID, true, nil
}

// UserIDByUsername — lihat UserIDByEmail.
func (s *Service) UserIDByUsername(ctx context.Context, username string) (int64, bool, error) {
	if username == "" {
		return 0, false, nil
	}
	u, err := s.repo.UserByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return 0, false, nil
		}
		return 0, false, err
	}
	return u.ID, true, nil
}

// UsersWithRole mengembalikan user_id seluruh user dengan membership AKTIF
// role tsb di sekolah ini — dipakai modul leave (fase 9,
// docs/08-notification.md) lewat consumer-side interface untuk resolve
// penerima notifikasi step approval role-only (approver_user_id NULL:
// "siapa pun yang sedang memegang role ini").
func (s *Service) UsersWithRole(ctx context.Context, schoolID int64, role string) ([]int64, error) {
	return s.repo.UserIDsByRole(ctx, schoolID, role)
}

// CreateMembership membuat/mengaktifkan membership (upsert, idempoten).
func (s *Service) CreateMembership(ctx context.Context, userID, schoolID int64, role string) error {
	_, err := s.repo.CreateMembership(ctx, userID, schoolID, role)
	return err
}

// IssueSession membuat sesi baru untuk (userID, schoolID, role) dan
// mengembalikan token mentah (untuk cookie) + waktu kedaluwarsa — logika sama
// dengan ekor Login, dipakai modul lain untuk auto-login (mis. setelah
// aktivasi undangan siswa/wali).
func (s *Service) IssueSession(ctx context.Context, userID, schoolID int64, role, ip, userAgent string) (string, time.Time, error) {
	token, tokenHash, err := newSessionToken()
	if err != nil {
		return "", time.Time{}, err
	}
	expiresAt := time.Now().Add(sessionTTLForRole(role))
	var schoolIDPtr *int64
	if schoolID != 0 {
		schoolIDPtr = &schoolID
	}
	if err := s.repo.CreateSession(ctx, CreateSessionInput{
		UserID: userID, SchoolID: schoolIDPtr, TokenHash: tokenHash, Role: role,
		ExpiresAt: expiresAt, IP: ip, UserAgent: userAgent,
	}); err != nil {
		return "", time.Time{}, err
	}
	return token, expiresAt, nil
}

// SetSessionCookie memasang cookie sesi (nama & flag keamanan sama dengan Login).
func (s *Service) SetSessionCookie(w http.ResponseWriter, token string, expiresAt time.Time) {
	setSessionCookie(w, token, expiresAt, s.cookieSecure)
}

const (
	inviteCodeChars = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789" // tanpa 0/O/1/I — hindari salah baca
	inviteCodeLen   = 8
)

func randomInviteCode() (string, error) {
	b := make([]byte, inviteCodeLen)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	out := make([]byte, inviteCodeLen)
	for i, c := range b {
		out[i] = inviteCodeChars[int(c)%len(inviteCodeChars)]
	}
	return string(out), nil
}

// invitationCodeRepo adalah kontrak minimal yang dibutuhkan generateInvitationCode
// — dipenuhi *Repository secara struktural, dideklarasikan sebagai interface
// supaya logika idempoten/unik bisa dites dengan fake in-memory (lihat
// gateway_test.go), tanpa DB.
type invitationCodeRepo interface {
	ActiveInvitation(ctx context.Context, schoolID int64, role string, targetID int64, now time.Time) (Invitation, error)
	CreateInvitation(ctx context.Context, schoolID int64, code, role string, targetID int64, expiresAt time.Time) (Invitation, error)
}

// generateInvitationCode implementasi murni (testable) dari GenerateInvitationCode.
func generateInvitationCode(ctx context.Context, repo invitationCodeRepo, schoolID int64, role string, targetID int64, ttl time.Duration, now time.Time) (string, time.Time, error) {
	if existing, err := repo.ActiveInvitation(ctx, schoolID, role, targetID, now); err == nil {
		return existing.Code, existing.ExpiresAt, nil
	} else if !errors.Is(err, ErrNotFound) {
		return "", time.Time{}, err
	}

	expiresAt := now.Add(ttl)
	for attempt := 0; attempt < 5; attempt++ {
		code, err := randomInviteCode()
		if err != nil {
			return "", time.Time{}, err
		}
		inv, err := repo.CreateInvitation(ctx, schoolID, code, role, targetID, expiresAt)
		if err == nil {
			return inv.Code, inv.ExpiresAt, nil
		}
		// Kemungkinan tabrakan kode unik (peluang sangat kecil) — coba lagi.
	}
	return "", time.Time{}, fmt.Errorf("identity: gagal membuat kode undangan unik setelah beberapa percobaan")
}

// GenerateInvitationCode membuat kode undangan baru untuk (schoolID, role,
// targetID), atau memakai ulang kode yang masih aktif (belum dipakai & belum
// kedaluwarsa) — idempoten sesuai docs/03-student.md.
func (s *Service) GenerateInvitationCode(ctx context.Context, schoolID int64, role string, targetID int64, ttl time.Duration) (string, time.Time, error) {
	return generateInvitationCode(ctx, s.repo, schoolID, role, targetID, ttl, time.Now())
}

// ValidateInvitationCode mengembalikan (schoolID, role, targetID, true, nil)
// bila kode valid (ditemukan, belum dipakai, belum kedaluwarsa); ok=false
// bila tidak. schoolID disertakan supaya pemanggil bisa menegakkan bahwa kode
// ini memang milik sekolah pada host saat ini (bukan sekolah lain).
func (s *Service) ValidateInvitationCode(ctx context.Context, code string) (schoolID int64, role string, targetID int64, ok bool, err error) {
	inv, err := s.repo.InvitationByCode(ctx, code)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return 0, "", 0, false, nil
		}
		return 0, "", 0, false, err
	}
	if inv.UsedAt != nil || time.Now().After(inv.ExpiresAt) {
		return 0, "", 0, false, nil
	}
	return inv.SchoolID, inv.Role, inv.TargetID, true, nil
}

// MarkInvitationUsed menandai kode undangan sudah dipakai (waktu sekarang).
func (s *Service) MarkInvitationUsed(ctx context.Context, code string) error {
	return s.repo.MarkInvitationUsed(ctx, code, time.Now())
}
