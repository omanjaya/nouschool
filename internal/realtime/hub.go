package realtime

import (
	"encoding/json"
	"log/slog"
	"sync"
)

// sendBuffer adalah kapasitas channel outbox per koneksi. Publish TIDAK
// PERNAH blocking menunggu klien lambat: bila buffer penuh, koneksi
// dianggap macet dan DIPUTUS (klien reconnect otomatis, lihat docs
// tugas Fase 12 "Kirim non-blocking ... penuh → drop conn").
const sendBuffer = 32

// Client adalah satu koneksi WebSocket terdaftar di Hub, dengan metadata
// hasil sesi login (school_id/user_id/role — SELALU dari reqctx via ws.go,
// TIDAK PERNAH dari pesan klien). Field tidak diekspor: modul lain hanya
// berinteraksi lewat Hub.Register/Unregister/Publish.
type Client struct {
	hub      *Hub
	schoolID int64
	userID   int64
	role     string

	send chan []byte

	closeOnce sync.Once
	// closer menutup transport WebSocket sesungguhnya (disuntik ws.go lewat
	// Hub.Register) — nil-safe: Hub sendiri TIDAK tahu apa pun soal
	// websocket.Conn (pemisahan transport vs fan-out, memudahkan test Hub
	// murni tanpa server HTTP nyata, lihat hub_test.go).
	closer func()
}

// Send mengembalikan channel baca-saja tempat ws.go (writePump) mengambil
// payload JSON yang harus dikirim ke klien ini.
func (c *Client) Send() <-chan []byte { return c.send }

// Drop menutup transport koneksi ini (dipanggil Hub saat buffer penuh, atau
// ws.go sendiri saat ingin memutus paksa). Idempoten & nil-safe.
func (c *Client) Drop() {
	c.closeOnce.Do(func() {
		if c.closer != nil {
			c.closer()
		}
	})
}

// Hub menyimpan seluruh koneksi WebSocket aktif, dikelompokkan per sekolah —
// satu peta per school_id supaya Publish tidak pernah menyapu koneksi
// sekolah lain (isolasi tenant, CLAUDE.md). Aman dipakai konkuren dari
// banyak goroutine (satu per koneksi HTTP) lewat RWMutex.
type Hub struct {
	mu      sync.RWMutex
	schools map[int64]map[*Client]struct{}
}

// NewHub membuat Hub kosong.
func NewHub() *Hub {
	return &Hub{schools: make(map[int64]map[*Client]struct{})}
}

// Register mendaftarkan koneksi baru ke sekolah schoolID dengan metadata
// userID/role hasil sesi login. closer (opsional, boleh nil — dipakai test)
// dipanggil Hub.Publish saat buffer koneksi ini penuh, untuk memutus paksa
// transport-nya; ws.go menyuntik `func() { wsConn.CloseNow() }`.
func (h *Hub) Register(schoolID, userID int64, role string, closer func()) *Client {
	c := &Client{hub: h, schoolID: schoolID, userID: userID, role: role, send: make(chan []byte, sendBuffer), closer: closer}
	h.mu.Lock()
	set, ok := h.schools[schoolID]
	if !ok {
		set = make(map[*Client]struct{})
		h.schools[schoolID] = set
	}
	set[c] = struct{}{}
	h.mu.Unlock()
	return c
}

// Unregister melepas koneksi dari Hub (dipanggil ws.go lewat defer saat
// koneksi berakhir, apa pun sebabnya). Aman dipanggil lebih dari sekali /
// untuk klien yang sudah tidak terdaftar (no-op).
func (h *Hub) Unregister(c *Client) {
	h.mu.Lock()
	if set, ok := h.schools[c.schoolID]; ok {
		delete(set, c)
		if len(set) == 0 {
			delete(h.schools, c.schoolID)
		}
	}
	h.mu.Unlock()
}

// matchesTarget menentukan apakah koneksi dengan role/userID ini termasuk
// sasaran ev (lihat dok Event.Roles/UserIDs di model.go).
func matchesTarget(role string, userID int64, ev Event) bool {
	if len(ev.Roles) == 0 && len(ev.UserIDs) == 0 {
		return true
	}
	for _, r := range ev.Roles {
		if r == role {
			return true
		}
	}
	for _, id := range ev.UserIDs {
		if id == userID {
			return true
		}
	}
	return false
}

// Publish mengirim ev ke seluruh koneksi sekolah schoolID yang cocok dengan
// target (lihat matchesTarget). Non-blocking per koneksi: bila buffer
// channel-nya penuh, koneksi itu DIPUTUS (Client.Drop) alih-alih menahan
// Publish menunggu — satu klien lambat/macet TIDAK PERNAH memperlambat event
// utk klien lain maupun goroutine pemanggil (biasanya request HTTP modul
// lain, lihat SetRealtime di masing-masing modul). Aman dipanggil dari
// banyak goroutine sekaligus.
func (h *Hub) Publish(schoolID int64, ev Event) {
	payload, err := json.Marshal(Message{Type: ev.Type, Data: ev.Data})
	if err != nil {
		slog.Error("realtime: gagal marshal event", "type", ev.Type, "err", err)
		return
	}

	h.mu.RLock()
	set := h.schools[schoolID]
	targets := make([]*Client, 0, len(set))
	for c := range set {
		if matchesTarget(c.role, c.userID, ev) {
			targets = append(targets, c)
		}
	}
	h.mu.RUnlock()

	for _, c := range targets {
		select {
		case c.send <- payload:
		default:
			// Buffer penuh -> koneksi dianggap macet, putus paksa (klien
			// reconnect sendiri lewat exponential backoff di frontend).
			// Unregister TIDAK dipanggil di sini (Hub tidak menahan lock
			// tulis di sini) — ws.go akan memanggilnya sendiri lewat defer
			// begitu Drop membuat pembacaan/penulisan transport error.
			c.Drop()
		}
	}
}
