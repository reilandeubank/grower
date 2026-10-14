package server

import (
	"net"
	"net/http"
	"net/netip"
	"strings"
)

// realIP rewrites r.RemoteAddr to the client's address and sets r.URL.Scheme.
// X-Forwarded-For/-Proto are believed only when the direct peer is a trusted proxy,
// otherwise any client could spoof its IP and dodge the login rate limit. The client
// is the rightmost XFF hop that is not itself a trusted proxy (hops to its left are
// client-supplied and unverifiable).
func realIP(trusted []netip.Prefix) func(http.Handler) http.Handler {
	isTrusted := func(a netip.Addr) bool {
		for _, p := range trusted {
			if p.Contains(a) {
				return true
			}
		}
		return false
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			scheme := "http"
			if r.TLS != nil {
				scheme = "https"
			}
			r = r.WithContext(r.Context()) // shallow copy; never mutate the caller's request
			u := *r.URL
			r.URL = &u
			peer, err := netip.ParseAddrPort(r.RemoteAddr)
			if err == nil && isTrusted(peer.Addr().Unmap()) {
				client := clientFromXFF(peer.Addr().Unmap(), r.Header.Values("X-Forwarded-For"), isTrusted)
				r.RemoteAddr = netip.AddrPortFrom(client, peer.Port()).String()
				if p := strings.ToLower(strings.TrimSpace(r.Header.Get("X-Forwarded-Proto"))); p == "http" || p == "https" {
					scheme = p
				}
			}
			r.URL.Scheme = scheme
			next.ServeHTTP(w, r)
		})
	}
}

// clientFromXFF walks the X-Forwarded-For chain from the right, skipping trusted proxies.
func clientFromXFF(peer netip.Addr, headers []string, isTrusted func(netip.Addr) bool) netip.Addr {
	var hops []string
	for _, h := range headers {
		hops = append(hops, strings.Split(h, ",")...)
	}
	client := peer
	for i := len(hops) - 1; i >= 0; i-- {
		addr, err := netip.ParseAddr(strings.TrimSpace(hops[i]))
		if err != nil {
			break // garbage from here leftwards; keep the last good hop
		}
		client = addr.Unmap()
		if !isTrusted(client) {
			break
		}
	}
	return client
}

// hostOnly strips the port from an address, tolerating addresses without one.
func hostOnly(addr string) string {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}
	return host
}
