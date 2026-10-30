package server

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/reilandeubank/grower/internal/auth"
)

type setupStatus struct{ Needed, Open bool }

func setupBody(username, password, timezone string) string {
	b, _ := json.Marshal(map[string]string{"username": username, "password": password, "timezone": timezone})
	return string(b)
}

func (e *env) setup(username, password, timezone string) *http.Response {
	e.t.Helper()
	return e.do(request{method: http.MethodPost, path: "/api/v1/setup", body: setupBody(username, password, timezone)})
}

func TestSetupStatus(t *testing.T) {
	fresh := newEnv(t, envOptions{firstRun: true})
	if got := expectJSON[setupStatus](t, fresh.do(request{path: "/api/v1/setup"}), http.StatusOK); got != (setupStatus{true, true}) {
		t.Errorf("fresh install: %+v", got)
	}
	fresh.clock.t = fresh.clock.t.Add(auth.SetupWindow)
	if got := expectJSON[setupStatus](t, fresh.do(request{path: "/api/v1/setup"}), http.StatusOK); got != (setupStatus{true, false}) {
		t.Errorf("after the window: %+v", got)
	}
	existing := newEnv(t, envOptions{})
	if got := expectJSON[setupStatus](t, existing.do(request{path: "/api/v1/setup"}), http.StatusOK); got != (setupStatus{}) {
		t.Errorf("has a user: %+v", got)
	}
}

func TestSetupCreatesAccountAndSignsIn(t *testing.T) {
	e := newEnv(t, envOptions{firstRun: true})
	resp := e.setup("fern", testPassword, "Europe/Berlin")
	cookie := sessionCookie(t, resp, e.cfg.SessionCookieName())
	me := expectJSON[struct {
		User     struct{ Username string }
		Settings struct{ Timezone string }
	}](t, resp, http.StatusOK)
	if me.User.Username != "fern" || me.Settings.Timezone != "Europe/Berlin" {
		t.Fatalf("me = %+v", me)
	}

	// The cookie works, setup is closed, and the account logs in normally.
	if resp := e.do(request{path: "/api/v1/me", cookie: cookie}); resp.StatusCode != http.StatusOK {
		t.Fatalf("/me with setup cookie: status %d", resp.StatusCode)
	}
	if got := expectJSON[setupStatus](t, e.do(request{path: "/api/v1/setup"}), http.StatusOK); got.Needed {
		t.Error("setup still needed after completing it")
	}
	expectProblem(t, e.setup("ivy", testPassword, ""), http.StatusConflict)
	if resp := e.login("fern", testPassword); resp.StatusCode != http.StatusOK {
		t.Fatalf("login after setup: status %d", resp.StatusCode)
	}
}

func TestSetupTimezoneFallsBackToServerZone(t *testing.T) {
	// Named subtests: an empty name becomes "#00", which breaks the temp dir's SQLite
	// URI (#20).
	for name, tz := range map[string]string{"missing": "", "local": "Local", "unknown": "Not/AZone"} {
		t.Run(name, func(t *testing.T) {
			e := newEnv(t, envOptions{firstRun: true})
			me := expectJSON[struct{ Settings struct{ Timezone string } }](t, e.setup("fern", testPassword, tz), http.StatusOK)
			if me.Settings.Timezone != "America/Chicago" {
				t.Errorf("timezone = %q, want GROWER_TZ", me.Settings.Timezone)
			}
		})
	}
}

func TestSetupFailures(t *testing.T) {
	e := newEnv(t, envOptions{firstRun: true})
	tests := []struct {
		name   string
		req    request
		status int
	}{
		{"short password", request{method: http.MethodPost, path: "/api/v1/setup", body: setupBody("fern", "short", "")}, http.StatusBadRequest},
		{"bad username", request{method: http.MethodPost, path: "/api/v1/setup", body: setupBody("two words", testPassword, "")}, http.StatusBadRequest},
		{"cross-origin", request{method: http.MethodPost, path: "/api/v1/setup", body: setupBody("fern", testPassword, ""), headers: map[string]string{"Origin": "https://evil.example"}}, http.StatusForbidden},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expectProblem(t, e.do(tt.req), tt.status)
		})
	}
	if got := expectJSON[setupStatus](t, e.do(request{path: "/api/v1/setup"}), http.StatusOK); !got.Needed {
		t.Error("a failed setup must not create an account")
	}
}

func TestSetupAfterWindow(t *testing.T) {
	e := newEnv(t, envOptions{firstRun: true})
	e.clock.t = e.clock.t.Add(auth.SetupWindow)
	expectProblem(t, e.setup("fern", testPassword, ""), http.StatusForbidden)
}

func TestSetupClosedWhenUsersExist(t *testing.T) {
	e := newEnv(t, envOptions{})
	expectProblem(t, e.setup("ivy", testPassword, ""), http.StatusConflict)
}
