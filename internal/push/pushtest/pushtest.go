// Package pushtest fakes a web push service for tests.
package pushtest

import (
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

// Service is an https push service that records deliveries and answers each endpoint
// path with its Status (201 Created by default).
type Service struct {
	*httptest.Server
	mu     sync.Mutex
	status map[string]int
	got    []string
}

// New starts a fake push service, closed when the test ends. Senders must use its
// Client(), which trusts its certificate.
func New(t testing.TB) *Service {
	t.Helper()
	s := &Service{status: map[string]int{}}
	s.Server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		if r.Header.Get("Authorization") == "" || r.Header.Get("Content-Encoding") != "aes128gcm" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		s.got = append(s.got, r.URL.Path)
		code, ok := s.status[r.URL.Path]
		if !ok {
			code = http.StatusCreated
		}
		w.WriteHeader(code)
	}))
	t.Cleanup(s.Close)
	return s
}

// SetStatus makes the service answer path with code.
func (s *Service) SetStatus(path string, code int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.status[path] = code
}

// Requests are the endpoint paths posted to so far, in order.
func (s *Service) Requests() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.got...)
}

// Keys returns valid browser subscription keys (p256dh, auth), as PushSubscription.toJSON gives them.
func Keys(t testing.TB) (p256dh, auth string) {
	t.Helper()
	k, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	secret := make([]byte, 16)
	_, _ = rand.Read(secret)
	return base64.RawURLEncoding.EncodeToString(k.PublicKey().Bytes()), base64.RawURLEncoding.EncodeToString(secret)
}
