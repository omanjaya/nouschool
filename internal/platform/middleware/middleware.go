// Package middleware berisi middleware lintas modul.
// Fase 1 menambahkan: ResolveTenant (Host → school_id), RequireAuth, RequirePerm.
package middleware

import (
	"log/slog"
	"net/http"
	"time"
)

// Chain menerapkan middleware berurutan: Chain(h, a, b) => a(b(h)).
func Chain(h http.Handler, mws ...func(http.Handler) http.Handler) http.Handler {
	for i := len(mws) - 1; i >= 0; i-- {
		h = mws[i](h)
	}
	return h
}

// Recover mencegah panic satu request mematikan server.
func Recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				slog.Error("panic", "err", rec, "path", r.URL.Path)
				http.Error(w, `{"error":{"code":"internal","message":"Terjadi kesalahan pada server."}}`, http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// Logger mencatat setiap request: method, path, status, durasi.
func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)
		slog.Info("http",
			"method", r.Method,
			"path", r.URL.Path,
			"status", sw.status,
			"dur", time.Since(start).Round(time.Millisecond).String(),
		)
	})
}

// SecurityHeaders memasang header keamanan dasar (CSP detail diatur di Caddy).
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "same-origin")
		next.ServeHTTP(w, r)
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}
