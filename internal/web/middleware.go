package web

import (
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"runtime/debug"
	"strings"
	"time"
)

const csp = "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; " +
	"connect-src 'self'; form-action 'self'; frame-ancestors 'none'; base-uri 'none'; object-src 'none'"

type statusWriter struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (w *statusWriter) WriteHeader(code int) {
	if w.status == 0 {
		w.status = code
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	n, err := w.ResponseWriter.Write(b)
	w.bytes += n
	return n, err
}

func (w *statusWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func newRequestID() string {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// middleware wraps every request with, from outermost to innermost: request
// logging, panic recovery, security headers, CSRF protection and a body limit.
func (s *Server) middleware(next http.Handler) http.Handler {
	csrf := http.NewCrossOriginProtection()
	inner := csrf.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		limit := int64(maxFormBytes)
		if r.URL.Path == "/import" {
			limit = maxImportBytes
		}
		// FormValue swallows MaxBytesError, so a declared oversized body is
		// rejected here with a proper 413 instead of a misleading 400.
		if r.ContentLength > limit {
			if r.Header.Get("HX-Request") == "true" {
				w.Header().Set("HX-Reswap", "none")
				toast(w, "error", "Conteúdo grande demais.")
			}
			http.Error(w, "Conteúdo grande demais.", http.StatusRequestEntityTooLarge)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, limit)
		next.ServeHTTP(w, r)
	}))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		id := newRequestID()
		sw := &statusWriter{ResponseWriter: w}
		h := sw.Header()
		h.Set("X-Request-Id", id)
		h.Set("Content-Security-Policy", csp)
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("X-Frame-Options", "DENY")

		defer func() {
			if rec := recover(); rec != nil {
				if rec == http.ErrAbortHandler { //nolint:errorlint // sentinel compared as documented by net/http
					panic(rec)
				}
				s.log.Error("panic", "request_id", id, "panic", rec, "stack", shortStack())
				if sw.status == 0 {
					http.Error(sw, "erro interno", http.StatusInternalServerError)
				}
			}
			level := slog.LevelInfo
			switch {
			case sw.status >= 500:
				level = slog.LevelError
			case r.URL.Path == "/healthz" || strings.HasPrefix(r.URL.Path, "/static/"):
				level = slog.LevelDebug
			}
			s.log.Log(r.Context(), level, "request",
				"request_id", id,
				"method", r.Method,
				"path", r.URL.Path,
				"status", sw.status,
				"bytes", sw.bytes,
				"duration_ms", float64(time.Since(start).Microseconds())/1000,
				"htmx", r.Header.Get("HX-Request") == "true",
			)
		}()
		inner.ServeHTTP(sw, r)
	})
}

// shortStack keeps the first frames of the panic stack, enough to locate the
// fault without flooding the log.
func shortStack() string {
	lines := strings.Split(string(debug.Stack()), "\n")
	if len(lines) > 24 {
		lines = lines[:24]
	}
	return strings.Join(lines, "\n")
}
