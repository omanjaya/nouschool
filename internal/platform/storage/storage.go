// Package storage adalah helper kecil penyimpanan file di disk lokal (root
// bisa dikonfigurasi lewat env UPLOAD_DIR, default "./uploads"). Sama seperti
// platform/clock, package ini diimpor LANGSUNG oleh modul yang butuh (mis.
// leave, untuk lampiran) — bukan lewat consumer-side interface, karena ini
// infrastruktur bersama (platform/), bukan modul bisnis lain.
package storage

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ErrInvalidPath menandai relPath mencoba keluar dari root (mis. mengandung "..").
var ErrInvalidPath = errors.New("storage: path tidak valid")

// Store menyimpan & membaca file di bawah satu direktori root.
type Store struct {
	root string
}

// New membangun Store dengan root tertentu ("./uploads" bila kosong).
func New(root string) *Store {
	if root == "" {
		root = "./uploads"
	}
	return &Store{root: root}
}

// FromEnv membangun Store dari env UPLOAD_DIR (default "./uploads").
func FromEnv() *Store {
	return New(os.Getenv("UPLOAD_DIR"))
}

// Root mengembalikan direktori root penyimpanan.
func (s *Store) Root() string { return s.root }

// resolve menerjemahkan relPath (path relatif, dipakai apa adanya sebagai
// nilai yang disimpan di DB) menjadi path absolut di disk, menolak path yang
// mencoba keluar dari root.
func (s *Store) resolve(relPath string) (string, error) {
	relPath = strings.TrimLeft(filepath.ToSlash(relPath), "/")
	if relPath == "" || strings.Contains(relPath, "..") {
		return "", ErrInvalidPath
	}
	return filepath.Join(s.root, filepath.FromSlash(relPath)), nil
}

// Save menulis isi r ke relPath (relatif root), membuat direktori induk bila
// perlu. relPath yang sama dipakai pemanggil sebagai nilai yang disimpan di DB.
func (s *Store) Save(relPath string, r io.Reader) error {
	abs, err := s.resolve(relPath)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return err
	}
	f, err := os.Create(abs)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, r)
	return err
}

// Open membuka file relPath untuk dibaca — pemanggil wajib Close().
func (s *Store) Open(relPath string) (*os.File, error) {
	abs, err := s.resolve(relPath)
	if err != nil {
		return nil, err
	}
	return os.Open(abs)
}

// RandomFilename menghasilkan nama file acak (32 hex char) dengan ekstensi
// ext (tanpa titik; kosong = tanpa ekstensi).
func RandomFilename(ext string) (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	name := hex.EncodeToString(b)
	if ext != "" {
		name += "." + ext
	}
	return name, nil
}
