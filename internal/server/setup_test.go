package server

import (
	"encoding/json"
	"net/http"
	"testing"
)

func setupBody(code, username, password, timezone string) string {
	b, _ := json.Marshal(map[string]string{"code": code, "username": username, "password": password, "timezone": timezone})
	return string(b)
}

func TestSetupStatus(t *testing.T) {
	for _, tt := range []struct {
		name     string
		firstRun bool
		want     bool
	}{
		{"fresh install", true, true},
		{"has a user", false, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			e := newEnv(t, envOptions{firstRun: tt.firstRun})
			got := expectJSON[struct{ Needed bool }](t, e.do(request{path: "/api/v1/setup"}), http.StatusOK)
			if got.Needed != tt.want {
				t.Errorf("needed = %v, want %v", got.Needed, tt.want)
			}
		})
	}
}

func TestSetupCreatesAccountAndSignsIn(t *testing.T) {
	e := newEnv(t, envOptions{firstRun: true})
	resp := e.do(request{method: http.MethodPost, path: "/api/v1/setup", body: setupBody(e.setupCode, "fern", testPassword, "Europe/Berlin")})
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
	if got := expectJSON[struct{ Needed bool }](t, e.do(request{path: "/api/v1/setup"}), http.StatusOK); got.Needed {
		t.Error("setup still needed after completing it")
	}
	expectProblem(t, e.do(request{method: http.MethodPost, path: "/api/v1/setup", body: setupBody(e.setupCode, "ivy", testPassword, "")}), http.StatusConflict)
	if resp := e.login("fern", testPassword); resp.StatusCode != http.StatusOK {
		t.Fatalf("login after setup: status %d", resp.StatusCode)
	}
}

func TestSetupTimezoneFallsBackToServerZone(t *testing.T) {
	// Named subtests: an empty name becomes "#00", and the temp dir path would then carry
	// a "#" into the SQLite file URI.
	for name, tz := range map[string]string{"missing": "", "local": "Local", "unknown": "Not/AZone"} {
		t.Run(name, func(t *testing.T) {
			e := newEnv(t, envOptions{firstRun: true})
			resp := e.do(request{method: http.MethodPost, path: "/api/v1/setup", body: setupBody(e.setupCode, "fern", testPassword, tz)})
			me := expectJSON[struct{ Settings struct{ Timezone string } }](t, resp, http.StatusOK)
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
		{"wrong code", request{method: http.MethodPost, path: "/api/v1/setup", body: setupBody("WRONG-CODE0", "fern", testPassword, "")}, http.StatusForbidden},
		{"missing code", request{method: http.MethodPost, path: "/api/v1/setup", body: setupBody("", "fern", testPassword, "")}, http.StatusBadRequest},
		{"short password", request{method: http.MethodPost, path: "/api/v1/setup", body: setupBody(e.setupCode, "fern", "short", "")}, http.StatusBadRequest},
		{"bad username", request{method: http.MethodPost, path: "/api/v1/setup", body: setupBody(e.setupCode, "two words", testPassword, "")}, http.StatusBadRequest},
		{"cross-origin", request{method: http.MethodPost, path: "/api/v1/setup", body: setupBody(e.setupCode, "fern", testPassword, ""), headers: map[string]string{"Origin": "https://evil.example"}}, http.StatusForbidden},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expectProblem(t, e.do(tt.req), tt.status)
		})
	}
	if got := expectJSON[struct{ Needed bool }](t, e.do(request{path: "/api/v1/setup"}), http.StatusOK); !got.Needed {
		t.Error("a failed setup must not create an account")
	}
}

func TestSetupWithConfiguredCode(t *testing.T) {
	e := newEnv(t, envOptions{firstRun: true, env: map[string]string{"GROWER_SETUP_CODE": "our-secret-code"}})
	resp := e.do(request{method: http.MethodPost, path: "/api/v1/setup", body: setupBody("OUR SECRET CODE", "fern", testPassword, "")})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
}

func TestSetupClosedWhenUsersExist(t *testing.T) {
	e := newEnv(t, envOptions{})
	expectProblem(t, e.do(request{method: http.MethodPost, path: "/api/v1/setup", body: setupBody("ANY-CODE00", "ivy", testPassword, "")}), http.StatusConflict)
}
