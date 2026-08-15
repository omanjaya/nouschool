package schedule

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

// importTTL — hasil parse import (preview) disimpan in-memory selama ini
// sebelum commit; pola sama dengan internal/student (ImportStore).
const importTTL = 15 * time.Minute

type importEntry struct {
	rows      []slotImportRow
	yearID    int64
	expiresAt time.Time
}

// importStore adalah penyimpanan sementara hasil parse import jadwal (map +
// mutex, key upload_id acak, TTL 15 menit) — sengaja in-memory (bukan DB)
// karena hanya menjembatani preview -> commit dalam satu sesi kerja admin.
type importStore struct {
	mu    sync.Mutex
	items map[string]importEntry
	now   func() time.Time
}

func newImportStore() *importStore {
	return &importStore{items: make(map[string]importEntry), now: time.Now}
}

func randomUploadID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func (st *importStore) prune() {
	now := st.now()
	for id, e := range st.items {
		if now.After(e.expiresAt) {
			delete(st.items, id)
		}
	}
}

func (st *importStore) put(rows []slotImportRow, yearID int64) (string, error) {
	id, err := randomUploadID()
	if err != nil {
		return "", err
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	st.prune()
	st.items[id] = importEntry{rows: rows, yearID: yearID, expiresAt: st.now().Add(importTTL)}
	return id, nil
}

func (st *importStore) get(id string) ([]slotImportRow, int64, bool) {
	st.mu.Lock()
	defer st.mu.Unlock()
	e, ok := st.items[id]
	if !ok || st.now().After(e.expiresAt) {
		delete(st.items, id)
		return nil, 0, false
	}
	return e.rows, e.yearID, true
}
