package server

import (
	"net/http"
	"net/netip"
	"testing"
	"time"

	"github.com/reilandeubank/grower/internal/auth"
)

func TestClientFromXFF(t *testing.T) {
	trusted := []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")}
	isTrusted := func(a netip.Addr) bool { return trusted[0].Contains(a) }
	peer := netip.MustParseAddr("10.0.0.2")
	tests := []struct {
		name    string
		headers []string
		want    string
	}{
		{"no header", nil, "10.0.0.2"},
		{"single client", []string{"203.0.113.7"}, "203.0.113.7"},
		{"spoofed left hop ignored", []string{"1.2.3.4, 203.0.113.7"}, "203.0.113.7"},
		{"skips trusted hops", []string{"203.0.113.7, 10.0.0.9"}, "203.0.113.7"},
		{"multiple headers", []string{"1.2.3.4", "203.0.113.7"}, "203.0.113.7"},
		{"garbage stops the walk", []string{"203.0.113.7, junk, 10.0.0.9"}, "10.0.0.9"},
		{"ipv6", []string{"2001:db8::1"}, "2001:db8::1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := clientFromXFF(peer, tt.headers, isTrusted); got.String() != tt.want {
				t.Errorf("client = %s, want %s", got, tt.want)
			}
		})
	}
}

// The rate limiter is keyed by client IP, so it shows whose address the server believed.
func TestTrustedProxyHandling(t *testing.T) {
	limits := auth.RateLimits{PerIPEvery: time.Hour, PerIPBurst: 1, GlobalEvery: time.Second, GlobalBurst: 100}
	loginFrom := func(e *env, remote, xff string) int {
		return e.do(request{
			method: http.MethodPost, path: "/api/v1/auth/login", remoteAddr: remote,
			body:    `{"username":"fern","password":"wrong-password"}`,
			headers: map[string]string{"X-Forwarded-For": xff},
		}).StatusCode
	}

	t.Run("untrusted peer cannot spoof", func(t *testing.T) {
		e := newEnv(t, envOptions{limits: limits})
		if got := loginFrom(e, "198.51.100.1:5000", "203.0.113.1"); got != http.StatusUnauthorized {
			t.Fatalf("first attempt: %d", got)
		}
		// A new XFF value does not buy a fresh bucket: the peer address is used.
		if got := loginFrom(e, "198.51.100.1:5000", "203.0.113.2"); got != http.StatusTooManyRequests {
			t.Fatalf("spoofed attempt: %d", got)
		}
	})

	t.Run("trusted proxy forwards client IP", func(t *testing.T) {
		e := newEnv(t, envOptions{limits: limits, env: map[string]string{"GROWER_TRUSTED_PROXIES": "172.18.0.0/16"}})
		if got := loginFrom(e, "172.18.0.5:5000", "203.0.113.1"); got != http.StatusUnauthorized {
			t.Fatalf("client 1: %d", got)
		}
		if got := loginFrom(e, "172.18.0.5:5000", "203.0.113.2"); got != http.StatusUnauthorized {
			t.Fatalf("client 2 should have its own bucket: %d", got)
		}
		if got := loginFrom(e, "172.18.0.5:5000", "203.0.113.1"); got != http.StatusTooManyRequests {
			t.Fatalf("client 1 again: %d", got)
		}
	})
}
