// Package httpx adalah satu-satunya tempat penerjemahan error domain → HTTP
// dan helper response JSON. Handler modul TIDAK menulis http.Error sendiri.
package httpx

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
)

// Error adalah error domain yang dikembalikan service layer.
type Error struct {
	Status  int    `json:"-"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *Error) Error() string { return e.Code + ": " + e.Message }

var (
	ErrNotFound     = &Error{Status: http.StatusNotFound, Code: "not_found", Message: "Data tidak ditemukan."}
	ErrForbidden    = &Error{Status: http.StatusForbidden, Code: "forbidden", Message: "Anda tidak punya akses untuk aksi ini."}
	ErrUnauthorized = &Error{Status: http.StatusUnauthorized, Code: "unauthorized", Message: "Silakan masuk terlebih dahulu."}
)

// Validation membuat error 422 dengan pesan yang aman ditampilkan ke pengguna.
func Validation(message string) *Error {
	return &Error{Status: http.StatusUnprocessableEntity, Code: "validation", Message: message}
}

type envelope struct {
	Error *Error `json:"error,omitempty"`
	Data  any    `json:"data,omitempty"`
}

// JSON menulis response sukses berbentuk {"data": ...}.
func JSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(envelope{Data: data})
}

// WriteError menerjemahkan error apa pun menjadi response JSON konsisten.
// Error non-domain tidak pernah membocorkan detail internal ke klien.
func WriteError(w http.ResponseWriter, err error) {
	var de *Error
	if !errors.As(err, &de) {
		slog.Error("internal error", "err", err)
		de = &Error{Status: http.StatusInternalServerError, Code: "internal", Message: "Terjadi kesalahan pada server. Coba lagi sebentar lagi."}
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(de.Status)
	_ = json.NewEncoder(w).Encode(envelope{Error: de})
}

// Decode membaca body JSON dengan batas ukuran wajar.
func Decode(r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(nil, r.Body, 1<<20) // 1 MB
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return Validation("Format data tidak valid.")
	}
	return nil
}
