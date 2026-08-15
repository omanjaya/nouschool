// Package realtime adalah bus event read-only server→client lewat WebSocket
// (Fase 12). Klien TIDAK mengirim perintah — hanya heartbeat
// ({"type":"ping"} → server balas {"type":"pong"}). Event berisi TIPE + data
// MINIMAL (id/tanggal, dst) — TIDAK PERNAH data sensitif; klien selalu
// refetch lewat REST yang sudah ber-authz saat menerima event (lihat
// docs/ROADMAP.md Fase 12: "server→client ... data minimal ... klien
// refetch via REST").
//
// Hub menyimpan koneksi per sekolah (school_id dari reqctx, hasil resolusi
// tenant — TIDAK PERNAH dari body/query/path, sama seperti modul lain,
// lihat CLAUDE.md aturan multi-tenant) dengan metadata {userID, role} untuk
// targeting: broadcast satu sekolah (Roles & UserIDs kosong), role tertentu,
// user tertentu, atau union keduanya.
package realtime

// Event adalah satu pesan yang dipublikasikan ke Hub. Type + Data dikirim
// APA ADANYA ke klien (lihat Message di ws.go) — Data WAJIB primitif/kecil
// (id, tanggal, dst), BUKAN payload penuh (klien refetch via REST).
type Event struct {
	Type string
	Data map[string]any

	// Roles & UserIDs KOSONG DUA-DUANYA = broadcast ke SEMUA koneksi sekolah
	// ini. Salah satu/keduanya terisi = union (koneksi cocok bila role-nya
	// ada di Roles ATAU user_id-nya ada di UserIDs) — lihat matchesTarget di
	// hub.go.
	Roles   []string
	UserIDs []int64
}

// Message adalah bentuk wire JSON yang dikirim ke klien lewat WebSocket —
// dipakai baik untuk event hasil Publish (hub.go) maupun pesan kontrol
// hello/pong (ws.go).
type Message struct {
	Type string         `json:"type"`
	Data map[string]any `json:"data,omitempty"`
}
