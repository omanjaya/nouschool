package realtime

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"

	"github.com/omanjaya/nouschool/internal/platform/reqctx"
)

const (
	// pingInterval — ping level-WebSocket server->klien (docs tugas: "server
	// juga kirim ping websocket-level tiap 30 dtk").
	pingInterval = 30 * time.Second
	// readTimeout — batas diam klien (tidak kirim apa pun, termasuk pong
	// level-WebSocket balasan Ping di atas maupun {"type":"ping"} aplikasi)
	// sebelum koneksi ditutup (docs tugas: "timeout baca 90 dtk → tutup").
	readTimeout = 90 * time.Second
	// writeTimeout — batas tiap operasi tulis tunggal (event/ping/pong)
	// supaya klien macet tidak menggantung goroutine penulis selamanya.
	writeTimeout = 10 * time.Second
	// maxClientMessageBytes — klien HANYA pernah mengirim heartbeat kecil
	// ({"type":"ping"}), jadi batas kecil sudah cukup & mencegah penyalahgunaan.
	maxClientMessageBytes = 4096
)

// Handler menerjemahkan HTTP -> upgrade WebSocket -> Hub. Route dipasang
// lewat RegisterRoutes (routes.go) di belakang RequireAuth SAJA (semua role
// termasuk display boleh konek — bus ini read-only, tidak ada aksi yang
// perlu digerbang izin lebih lanjut).
type Handler struct {
	hub *Hub
}

// NewHandler membangun Handler baru di atas hub.
func NewHandler(hub *Hub) *Handler { return &Handler{hub: hub} }

// clientMessage adalah SATU-SATUNYA bentuk pesan yang diterima dari klien
// (docs tugas: "baca pesan klien hanya untuk {"type":"ping"}").
type clientMessage struct {
	Type string `json:"type"`
}

// Serve — GET /api/ws. Upgrade ke WebSocket, daftarkan ke hub, kirim hello,
// lalu jalankan read-loop (heartbeat klien) & write-loop (event dari hub +
// ping berkala) sampai salah satu berhenti (error/timeout/putus paksa).
func (h *Handler) Serve(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	schoolID := reqctx.SchoolID(ctx)
	userID := reqctx.UserID(ctx)
	role := reqctx.Role(ctx)

	conn, err := websocket.Accept(w, r, nil)
	if err != nil {
		// Accept sudah menulis response error ke w sendiri.
		return
	}
	conn.SetReadLimit(maxClientMessageBytes)

	var closeOnce sync.Once
	stop := make(chan struct{})
	closeAll := func() {
		closeOnce.Do(func() {
			close(stop)
			conn.CloseNow()
		})
	}

	client := h.hub.Register(schoolID, userID, role, closeAll)
	defer h.hub.Unregister(client)
	defer closeAll()

	// hello — dikirim SEBELUM writePump jalan (satu-satunya penulis pada
	// titik ini, tidak perlu lewat channel Send()).
	{
		wctx, cancel := context.WithTimeout(context.Background(), writeTimeout)
		_ = wsjson.Write(wctx, conn, Message{Type: "hello", Data: map[string]any{"user_id": userID}})
		cancel()
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		writePump(conn, client, stop, closeAll)
	}()

	readPump(conn) // blok sampai error/timeout/conn ditutup paksa

	closeAll()
	<-done // pastikan writePump sudah berhenti sebelum handler return (tanpa goroutine leak)
}

// readPump membaca pesan klien satu per satu, masing-masing dengan batas
// waktu readTimeout (diam lebih lama -> dianggap koneksi mati). Satu-satunya
// pesan yang ditindaklanjuti adalah {"type":"ping"} -> balas {"type":"pong"}
// langsung (Write aman dipanggil konkuren dengan writePump — lihat catatan
// konkurensi di conn.go package coder/websocket: "All methods may be called
// concurrently except for Reader and Read"). Pesan lain/tidak valid
// diabaikan (JSON tidak valid akan membuat wsjson.Read menutup koneksi
// sendiri dengan status yang sesuai).
func readPump(conn *websocket.Conn) {
	for {
		ctx, cancel := context.WithTimeout(context.Background(), readTimeout)
		var msg clientMessage
		err := wsjson.Read(ctx, conn, &msg)
		cancel()
		if err != nil {
			return
		}
		if msg.Type == "ping" {
			wctx, wcancel := context.WithTimeout(context.Background(), writeTimeout)
			_ = wsjson.Write(wctx, conn, Message{Type: "pong"})
			wcancel()
		}
	}
}

// writePump adalah satu-satunya goroutine yang mengirim event dari
// Hub.Publish (lewat client.Send()) + ping level-WebSocket berkala. Berhenti
// saat stop ditutup (closeAll dipanggil dari mana pun: readPump berakhir,
// atau Hub men-drop koneksi ini karena buffer penuh) atau saat penulisan
// gagal (koneksi mati) — pada kasus kedua closeAll dipanggil sendiri supaya
// readPump yang sedang menunggu Read() ikut terbangun.
func writePump(conn *websocket.Conn, client *Client, stop <-chan struct{}, closeAll func()) {
	defer closeAll()

	ticker := time.NewTicker(pingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-stop:
			return
		case payload, ok := <-client.Send():
			if !ok {
				return
			}
			wctx, cancel := context.WithTimeout(context.Background(), writeTimeout)
			err := conn.Write(wctx, websocket.MessageText, payload)
			cancel()
			if err != nil {
				return
			}
		case <-ticker.C:
			wctx, cancel := context.WithTimeout(context.Background(), writeTimeout)
			err := conn.Ping(wctx)
			cancel()
			if err != nil {
				return
			}
		}
	}
}
