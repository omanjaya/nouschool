package realtime

import (
	"encoding/json"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func decode(t *testing.T, payload []byte) Message {
	t.Helper()
	var m Message
	if err := json.Unmarshal(payload, &m); err != nil {
		t.Fatalf("gagal decode payload: %v", err)
	}
	return m
}

func recvOrTimeout(t *testing.T, ch <-chan []byte) []byte {
	t.Helper()
	select {
	case payload := <-ch:
		return payload
	case <-time.After(2 * time.Second):
		t.Fatal("timeout menunggu payload")
		return nil
	}
}

func assertNoMessage(t *testing.T, ch <-chan []byte) {
	t.Helper()
	select {
	case payload := <-ch:
		t.Fatalf("tidak seharusnya menerima payload, dapat: %s", payload)
	case <-time.After(100 * time.Millisecond):
	}
}

// TestPublishBroadcast — Roles & UserIDs kosong = broadcast ke SEMUA
// koneksi sekolah ini, TIDAK ke sekolah lain.
func TestPublishBroadcast(t *testing.T) {
	h := NewHub()
	a := h.Register(1, 10, "admin_sekolah", nil)
	b := h.Register(1, 20, "guru", nil)
	other := h.Register(2, 30, "admin_sekolah", nil)

	h.Publish(1, Event{Type: "attendance.session", Data: map[string]any{"class_id": 5}})

	for _, c := range []*Client{a, b} {
		msg := decode(t, recvOrTimeout(t, c.Send()))
		if msg.Type != "attendance.session" {
			t.Fatalf("type salah: %s", msg.Type)
		}
	}
	assertNoMessage(t, other.Send())
}

// TestPublishByRole — hanya koneksi dengan role di Roles yang menerima.
func TestPublishByRole(t *testing.T) {
	h := NewHub()
	admin := h.Register(1, 10, "admin_sekolah", nil)
	guru := h.Register(1, 20, "guru", nil)
	kepsek := h.Register(1, 30, "kepala_sekolah", nil)

	h.Publish(1, Event{Type: "attendance.summary", Roles: []string{"admin_sekolah", "kepala_sekolah"}})

	recvOrTimeout(t, admin.Send())
	recvOrTimeout(t, kepsek.Send())
	assertNoMessage(t, guru.Send())
}

// TestPublishByUserID — hanya koneksi dengan user_id di UserIDs yang menerima.
func TestPublishByUserID(t *testing.T) {
	h := NewHub()
	c1 := h.Register(1, 10, "guru", nil)
	c2 := h.Register(1, 20, "guru", nil)

	h.Publish(1, Event{Type: "notification", UserIDs: []int64{20}})

	assertNoMessage(t, c1.Send())
	recvOrTimeout(t, c2.Send())
}

// TestPublishUnionRoleAndUserID — Roles & UserIDs keduanya terisi = union
// (cocok salah satu).
func TestPublishUnionRoleAndUserID(t *testing.T) {
	h := NewHub()
	byRole := h.Register(1, 10, "admin_sekolah", nil)
	byUser := h.Register(1, 99, "guru", nil)
	neither := h.Register(1, 20, "guru", nil)

	h.Publish(1, Event{Type: "leave", Roles: []string{"admin_sekolah"}, UserIDs: []int64{99}})

	recvOrTimeout(t, byRole.Send())
	recvOrTimeout(t, byUser.Send())
	assertNoMessage(t, neither.Send())
}

// TestPublishBufferFullDropsConnection — koneksi dengan buffer penuh
// diputus (Drop dipanggil) TANPA Publish blocking/deadlock, klien lain di
// sekolah yang sama tetap menerima event.
func TestPublishBufferFullDropsConnection(t *testing.T) {
	h := NewHub()
	var dropped int32
	h.Register(1, 10, "guru", func() { atomic.AddInt32(&dropped, 1) })
	fast := h.Register(1, 20, "guru", nil)

	// Penuhi buffer slow (tanpa pernah dibaca) — sendBuffer = 32.
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < sendBuffer+5; i++ {
			h.Publish(1, Event{Type: "attendance.session", Data: map[string]any{"i": i}})
		}
	}()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("Publish deadlock saat buffer klien lambat penuh")
	}

	if atomic.LoadInt32(&dropped) == 0 {
		t.Fatal("klien dengan buffer penuh seharusnya di-drop")
	}
	// fast harus tetap kebanjiran pesan tanpa pernah macet menahan publish
	// klien lain (goroutine di atas sudah selesai membuktikan itu).
	recvOrTimeout(t, fast.Send())
}

// TestUnregisterClean — setelah Unregister, klien tidak lagi menerima event
// & tidak tersisa entri kosong di peta sekolah (unregister bersih).
func TestUnregisterClean(t *testing.T) {
	h := NewHub()
	c := h.Register(1, 10, "guru", nil)
	h.Unregister(c)

	h.mu.RLock()
	_, stillThere := h.schools[1]
	h.mu.RUnlock()
	if stillThere {
		t.Fatal("peta sekolah seharusnya kosong & terhapus setelah klien terakhir unregister")
	}

	// Publish setelah unregister tidak boleh panic/deadlock walau tidak ada
	// yang menerima.
	h.Publish(1, Event{Type: "attendance.session"})
	assertNoMessage(t, c.Send())

	// Unregister dua kali (mis. defer ganda) harus aman (no-op).
	h.Unregister(c)
}

// TestConcurrentRegisterPublishUnregister — dites dengan -race: banyak
// goroutine register/publish/unregister sekaligus tidak boleh membuat race
// pada peta internal Hub.
func TestConcurrentRegisterPublishUnregister(t *testing.T) {
	h := NewHub()
	var wg sync.WaitGroup
	const n = 50

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			c := h.Register(int64(i%3), int64(i), "guru", nil)
			// Consumer supaya buffer tidak penuh & Drop tidak dipicu di sini.
			stop := make(chan struct{})
			go func() {
				for {
					select {
					case <-c.Send():
					case <-stop:
						return
					}
				}
			}()
			for j := 0; j < 10; j++ {
				h.Publish(int64(i%3), Event{Type: "x"})
			}
			close(stop)
			h.Unregister(c)
		}(i)
	}
	wg.Wait()
}

// TestPublishAllBroadcastsAllSchools — PublishAll (fase 13 Gelombang 2,
// docs/11-superadmin.md P5) menjangkau SEMUA sekolah yang punya koneksi
// terdaftar, TIDAK hanya satu sekolah seperti Publish biasa.
func TestPublishAllBroadcastsAllSchools(t *testing.T) {
	h := NewHub()
	a := h.Register(1, 10, "admin_sekolah", nil)
	b := h.Register(2, 20, "guru", nil)
	c := h.Register(3, 30, "kepala_sekolah", nil)

	h.PublishAll(Event{Type: "announcement", Data: map[string]any{}})

	for _, cl := range []*Client{a, b, c} {
		msg := decode(t, recvOrTimeout(t, cl.Send()))
		if msg.Type != "announcement" {
			t.Fatalf("type salah: %s", msg.Type)
		}
	}
}

// TestPublishAllNoConnectionsNoop — PublishAll pada Hub tanpa koneksi sama
// sekali tidak panic/deadlock (no-op bersih).
func TestPublishAllNoConnectionsNoop(t *testing.T) {
	h := NewHub()
	h.PublishAll(Event{Type: "announcement"})
}

// TestPublisherNilSafe — Client dengan closer nil (pola Register(... , nil)
// dipakai test lain di file ini) tidak panic saat di-drop (buffer penuh).
func TestPublisherNilSafe(t *testing.T) {
	h := NewHub()
	c := h.Register(1, 10, "guru", nil)
	for i := 0; i < sendBuffer+5; i++ {
		h.Publish(1, Event{Type: "x"})
	}
	// Drop manual kedua kali juga tidak boleh panic (idempoten).
	c.Drop()
	c.Drop()
}
