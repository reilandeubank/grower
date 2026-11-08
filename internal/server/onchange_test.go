package server

import (
	"net/http"
	"testing"
)

// API writes that succeed poke the MQTT publisher; reads and failures don't.
func TestOnChangeAfterWrites(t *testing.T) {
	calls := 0
	e := newEnv(t, envOptions{onChange: func() { calls++ }})
	c := e.loginCookie() // a successful POST
	calls = 0

	steps := []struct {
		name         string
		method, path string
		body         string
		wantCalls    int
	}{
		{"read", http.MethodGet, "/api/v1/today", "", 0},
		{"create", http.MethodPost, "/api/v1/plants", `{"name":"Fern"}`, 1},
		{"invalid", http.MethodPost, "/api/v1/plants", `{"name":""}`, 1},
		{"missing", http.MethodDelete, "/api/v1/plants/nope", "", 1},
	}
	for _, s := range steps {
		e.call(c, s.method, s.path, s.body)
		if calls != s.wantCalls {
			t.Errorf("after %s: %d calls, want %d", s.name, calls, s.wantCalls)
		}
	}
	// Without a cookie, a write is refused and changes nothing.
	e.call(nil, http.MethodPost, "/api/v1/plants", `{"name":"Ivy"}`)
	if calls != 1 {
		t.Errorf("after unauthenticated write: %d calls", calls)
	}
}
