package student

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

// importTTL — hasil parse import (preview) disimpan in-memory selama ini
// sebelum commit; lihat docs/03-student.md.
const importTTL = 15 * time.Minute

type importEntry struct {
	kind      string // "students" | "teachers"
	students  []studentImportRow
	teachers  []teacherImportRow
	expiresAt time.Time
}

// ImportStore adalah penyimpanan sementara hasil parse import (map + mutex,
// key upload_id acak, TTL 15 menit) — sengaja in-memory (bukan DB) karena
// hanya menjembatani preview -> commit dalam satu sesi kerja admin.
type ImportStore struct {
	mu    sync.Mutex
	items map[string]importEntry
	now   func() time.Time
}

func NewImportStore() *ImportStore {
	return &ImportStore{items: make(map[string]importEntry), now: time.Now}
}

func randomUploadID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// prune membuang entri kedaluwarsa. Dipanggil dengan mu terkunci.
func (st *ImportStore) prune() {
	now := st.now()
	for id, e := range st.items {
		if now.After(e.expiresAt) {
			delete(st.items, id)
		}
	}
}

func (st *ImportStore) putStudents(rows []studentImportRow) (string, error) {
	id, err := randomUploadID()
	if err != nil {
		return "", err
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	st.prune()
	st.items[id] = importEntry{kind: "students", students: rows, expiresAt: st.now().Add(importTTL)}
	return id, nil
}

func (st *ImportStore) getStudents(id string) ([]studentImportRow, bool) {
	st.mu.Lock()
	defer st.mu.Unlock()
	e, ok := st.items[id]
	if !ok || e.kind != "students" || st.now().After(e.expiresAt) {
		delete(st.items, id)
		return nil, false
	}
	return e.students, true
}

func (st *ImportStore) putTeachers(rows []teacherImportRow) (string, error) {
	id, err := randomUploadID()
	if err != nil {
		return "", err
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	st.prune()
	st.items[id] = importEntry{kind: "teachers", teachers: rows, expiresAt: st.now().Add(importTTL)}
	return id, nil
}

func (st *ImportStore) getTeachers(id string) ([]teacherImportRow, bool) {
	st.mu.Lock()
	defer st.mu.Unlock()
	e, ok := st.items[id]
	if !ok || e.kind != "teachers" || st.now().After(e.expiresAt) {
		delete(st.items, id)
		return nil, false
	}
	return e.teachers, true
}
