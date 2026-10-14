package server

import (
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/reilandeubank/grower/internal/api"
)

// statusRecorder remembers the status code for the access log.
type statusRecorder struct {
	http.ResponseWriter
	status int
	bytes  int64
}

func (s *statusRecorder) WriteHeader(code int) {
	if s.status == 0 {
		s.status = code
	}
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusRecorder) Write(b []byte) (int, error) {
	if s.status == 0 {
		s.status = http.StatusOK
	}
	n, err := s.ResponseWriter.Write(b)
	s.bytes += int64(n)
	return n, err
}

// Unwrap lets http.ResponseController reach the underlying writer (flush, deadlines).
func (s *statusRecorder) Unwrap() http.ResponseWriter { return s.ResponseWriter }

// logRequests writes one access-log line per request. It deliberately logs no
// headers, cookies, query strings or bodies.
func logRequests(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			id := newRequestID()
			w.Header().Set("X-Request-Id", id)
			rec := &statusRecorder{ResponseWriter: w}
			next.ServeHTTP(rec, r)
			if rec.status == 0 {
				rec.status = http.StatusOK
			}
			level := slog.LevelInfo
			if rec.status >= 500 {
				level = slog.LevelError
			}
			log.LogAttrs(r.Context(), level, "request",
				slog.String("request_id", id),
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Int("status", rec.status),
				slog.Int64("bytes", rec.bytes),
				slog.Duration("duration", time.Since(start)),
				slog.String("client_ip", hostOnly(r.RemoteAddr)),
				slog.String("scheme", r.URL.Scheme),
			)
		})
	}
}

func newRequestID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b) // crypto/rand.Read never fails on supported platforms
	return hex.EncodeToString(b)
}

// recoverPanics turns a handler panic into a 500 instead of a dropped connection.
func recoverPanics(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				v := recover()
				if v == nil {
					return
				}
				if v == http.ErrAbortHandler { //nolint:errorlint // sentinel re-panic per net/http docs
					panic(v)
				}
				log.ErrorContext(r.Context(), "panic", "path", r.URL.Path, "panic", v, "stack", string(debug.Stack()))
				api.WriteProblem(w, http.StatusInternalServerError, "")
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// contentSecurityPolicy allows only same-origin resources. 'unsafe-inline' styles are
// needed by Radix/React style attributes; data:/blob: images for photo previews.
const contentSecurityPolicy = "default-src 'self'; img-src 'self' data: blob:; style-src 'self' 'unsafe-inline'; " +
	"font-src 'self'; connect-src 'self'; manifest-src 'self'; worker-src 'self'; " +
	"frame-ancestors 'none'; base-uri 'self'; form-action 'self'"

// permissionsPolicy keeps camera (photo capture) for ourselves and disables the rest.
const permissionsPolicy = "camera=(self), microphone=(), geolocation=(), payment=(), usb=()"

// securityHeaders sets headers on every response. HSTS is the reverse proxy's job.
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", contentSecurityPolicy)
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "same-origin")
		h.Set("Permissions-Policy", permissionsPolicy)
		h.Set("X-Frame-Options", "DENY")
		next.ServeHTTP(w, r)
	})
}

// noStore keeps API responses (personal data) out of browser and proxy caches.
func noStore(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

// limitBody caps request bodies so a client cannot stream unbounded data. Uploads get
// the larger cap.
func limitBody(isUpload func(*http.Request) bool, n, upload int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			limit := n
			if isUpload(r) {
				limit = upload
			}
			r.Body = http.MaxBytesReader(w, r.Body, limit)
			next.ServeHTTP(w, r)
		})
	}
}
