package realtime

import (
	"context"
	"net/http"

	"github.com/omanjaya/nouschool/internal/platform/httpx"
	"github.com/omanjaya/nouschool/internal/platform/reqctx"
)

// Fase 15 Gap 6 (docs/12-sion-parity.md "presence online (WS hub)"):
// GET /api/presence menyusun ulang Hub.OnlineUsers (hub.go) menjadi ringkasan
// {total, by_role, users} — ditempatkan DI MODUL REALTIME (bukan identity)
// karena state koneksi (Hub) memang tinggal di sini; identity hanya
// dibutuhkan untuk SATU hal kecil (nama user) lewat consumer-side interface
// primitif di bawah, TIDAK butuh adapter cross-module (beda dari
// teaching/attendance yang butuh scheduleadapter.go, dsb — shape data di
// sini sudah primitif dari awal).

// IdentityGateway adalah kebutuhan modul realtime dari modul identity (nama
// user untuk presence) — consumer-side interface kecil dideklarasikan di
// sisi PEMAKAI (lihat CLAUDE.md). Dipenuhi *identity.Service secara
// struktural lewat method UserName (signature primitif) — realtime TIDAK
// mengimpor identity untuk tipe apa pun.
type IdentityGateway interface {
	UserName(ctx context.Context, userID int64) (name string, found bool, err error)
}

// SetIdentityGateway menyuntikkan IdentityGateway SETELAH konstruksi
// (opsional, disuntik main.go — pola sama SetNotifier/SetRealtime di modul
// lain). nil aman: Presence tetap jalan dengan field "name" kosong.
func (h *Handler) SetIdentityGateway(g IdentityGateway) { h.identity = g }

// PresenceUserView adalah satu baris "users" pada GET /api/presence.
type PresenceUserView struct {
	UserID int64  `json:"user_id"`
	Name   string `json:"name"`
	Role   string `json:"role"`
}

// PresenceView adalah shape response GET /api/presence.
type PresenceView struct {
	Total  int                `json:"total"`
	ByRole map[string]int     `json:"by_role"`
	Users  []PresenceUserView `json:"users"`
}

// Presence — GET /api/presence (perm teaching:monitor, dipasang routes.go —
// SAMA gerbang dengan monitoring mengajar/dashboard TV, konsisten dgn
// "siapa yang online" adalah bagian dari monitoring). schoolID SELALU dari
// reqctx (hasil resolusi tenant), TIDAK PERNAH dari query/path — Hub sudah
// mengelompokkan koneksi per sekolah sendiri (isolasi tenant otomatis).
func (h *Handler) Presence(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	schoolID := reqctx.SchoolID(ctx)
	online := h.hub.OnlineUsers(schoolID)

	view := PresenceView{ByRole: map[string]int{}, Users: make([]PresenceUserView, 0, len(online))}
	for _, u := range online {
		name := ""
		if h.identity != nil {
			if n, found, err := h.identity.UserName(ctx, u.UserID); err == nil && found {
				name = n
			}
		}
		view.Users = append(view.Users, PresenceUserView{UserID: u.UserID, Name: name, Role: u.Role})
		view.ByRole[u.Role]++
	}
	view.Total = len(view.Users)
	httpx.JSON(w, http.StatusOK, view)
}
