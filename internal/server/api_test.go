package server

import (
	"net/http"
	"testing"
	"time"

	"github.com/reilandeubank/grower/internal/apigen"
	"github.com/reilandeubank/grower/internal/auth"
	"github.com/reilandeubank/grower/internal/buildinfo"
	"github.com/reilandeubank/grower/internal/config"
)

func TestLoginSetsCookie(t *testing.T) {
	tests := []struct {
		baseURL    string
		cookieName string
		secure     bool
	}{
		{"http://grower.test", config.InsecureSessionCookie, false},
		{"https://plants.example.com", config.SecureSessionCookie, true},
	}
	for _, tt := range tests {
		t.Run(tt.baseURL, func(t *testing.T) {
			e := newEnv(t, envOptions{env: map[string]string{"GROWER_BASE_URL": tt.baseURL}})
			resp := e.login(testUser, testPassword)
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("status = %d", resp.StatusCode)
			}
			c := sessionCookie(t, resp, tt.cookieName)
			if !c.HttpOnly || c.Secure != tt.secure || c.SameSite != http.SameSiteLaxMode || c.Path != "/" || c.Domain != "" {
				t.Errorf("cookie attributes = %+v", c)
			}
			if c.MaxAge != int(auth.SessionTTL.Seconds()) || len(c.Value) < 40 {
				t.Errorf("cookie MaxAge = %d, value len %d", c.MaxAge, len(c.Value))
			}
			if cc := resp.Header.Get("Cache-Control"); cc != "no-store" {
				t.Errorf("Cache-Control = %q", cc)
			}
			me := decode[apigen.Me](t, resp)
			if me.User.Username != testUser || me.Settings.Timezone != "America/Chicago" {
				t.Errorf("me = %+v", me)
			}
		})
	}
}

func TestLoginFailures(t *testing.T) {
	e := newEnv(t, envOptions{})
	tests := []struct {
		name   string
		body   string
		status int
	}{
		{"wrong password", `{"username":"fern","password":"wrong-password"}`, http.StatusUnauthorized},
		{"unknown user", `{"username":"ivy","password":"monstera-deliciosa"}`, http.StatusUnauthorized},
		{"empty username", `{"username":"","password":"x"}`, http.StatusBadRequest},
		{"malformed json", `{"username":`, http.StatusBadRequest},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := e.do(request{method: http.MethodPost, path: "/api/v1/auth/login", body: tt.body})
			for _, c := range resp.Cookies() {
				if c.Value != "" {
					t.Errorf("unexpected cookie %s", c.Name)
				}
			}
			expectProblem(t, resp, tt.status)
		})
	}
}

func TestLoginRateLimit(t *testing.T) {
	e := newEnv(t, envOptions{limits: auth.RateLimits{PerIPEvery: time.Minute, PerIPBurst: 2, GlobalEvery: time.Second, GlobalBurst: 100}})
	for i := range 2 {
		if resp := e.login(testUser, "wrong-password"); resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("attempt %d: status %d", i, resp.StatusCode)
		}
	}
	// Even the right password is refused once the bucket is empty.
	expectProblem(t, e.login(testUser, testPassword), http.StatusTooManyRequests)

	e.clock.t = e.clock.t.Add(time.Minute)
	if resp := e.login(testUser, testPassword); resp.StatusCode != http.StatusOK {
		t.Fatalf("after refill: status %d", resp.StatusCode)
	}
}

func TestMeRequiresAuth(t *testing.T) {
	e := newEnv(t, envOptions{})
	expectProblem(t, e.do(request{path: "/api/v1/me"}), http.StatusUnauthorized)

	stale := &http.Cookie{Name: e.cfg.SessionCookieName(), Value: "not-a-session"}
	resp := e.do(request{path: "/api/v1/me", cookie: stale})
	if c := sessionCookie(t, resp, e.cfg.SessionCookieName()); c.MaxAge >= 0 {
		t.Errorf("stale cookie not cleared: %+v", c)
	}
	expectProblem(t, resp, http.StatusUnauthorized)

	resp = e.do(request{path: "/api/v1/me", cookie: e.loginCookie()})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	me := decode[apigen.Me](t, resp)
	if me.User.Username != testUser || me.User.Id == "" || me.Settings.Timezone != "America/Chicago" || me.Version != buildinfo.Version {
		t.Errorf("me = %+v", me)
	}
}

func TestSessionSlidingRefreshResendsCookie(t *testing.T) {
	e := newEnv(t, envOptions{})
	cookie := e.loginCookie()

	e.clock.t = e.clock.t.Add(time.Hour)
	if resp := e.do(request{path: "/api/v1/me", cookie: cookie}); len(resp.Cookies()) != 0 {
		t.Errorf("cookie re-sent before refresh interval")
	}

	e.clock.t = e.clock.t.Add(24 * time.Hour)
	resp := e.do(request{path: "/api/v1/me", cookie: cookie})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if c := sessionCookie(t, resp, e.cfg.SessionCookieName()); c.Value != cookie.Value || c.MaxAge != int(auth.SessionTTL.Seconds()) {
		t.Errorf("refreshed cookie = %+v", c)
	}

	e.clock.t = e.clock.t.Add(auth.SessionTTL + time.Second)
	expectProblem(t, e.do(request{path: "/api/v1/me", cookie: cookie}), http.StatusUnauthorized)
}

func TestLogout(t *testing.T) {
	e := newEnv(t, envOptions{})
	cookie := e.loginCookie()

	resp := e.do(request{method: http.MethodPost, path: "/api/v1/auth/logout", cookie: cookie})
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if c := sessionCookie(t, resp, e.cfg.SessionCookieName()); c.MaxAge >= 0 || c.Value != "" {
		t.Errorf("logout cookie not cleared: %+v", c)
	}
	expectProblem(t, e.do(request{path: "/api/v1/me", cookie: cookie}), http.StatusUnauthorized)

	// Logging out without a session is fine too.
	if resp := e.do(request{method: http.MethodPost, path: "/api/v1/auth/logout"}); resp.StatusCode != http.StatusNoContent {
		t.Errorf("anonymous logout status = %d", resp.StatusCode)
	}
}

func TestCSRF(t *testing.T) {
	e := newEnv(t, envOptions{})
	body := `{"username":"fern","password":"monstera-deliciosa"}`
	tests := []struct {
		name   string
		req    request
		status int
	}{
		{"foreign origin", request{headers: map[string]string{"Origin": "https://evil.example"}}, http.StatusForbidden},
		{"origin differs by port", request{headers: map[string]string{"Origin": "http://grower.test:8080"}}, http.StatusForbidden},
		{"no origin, cross-site", request{noOrigin: true, headers: map[string]string{"Sec-Fetch-Site": "cross-site"}}, http.StatusForbidden},
		{"no origin, no fetch metadata", request{noOrigin: true}, http.StatusForbidden},
		{"no origin, same-origin", request{noOrigin: true, headers: map[string]string{"Sec-Fetch-Site": "same-origin"}}, http.StatusOK},
		{"form content type", request{headers: map[string]string{"Content-Type": "application/x-www-form-urlencoded"}}, http.StatusUnsupportedMediaType},
		{"json with charset", request{headers: map[string]string{"Content-Type": "application/json; charset=utf-8"}}, http.StatusOK},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.req.method, tt.req.path, tt.req.body = http.MethodPost, "/api/v1/auth/login", body
			resp := e.do(tt.req)
			if tt.status == http.StatusOK {
				if resp.StatusCode != http.StatusOK {
					t.Fatalf("status = %d", resp.StatusCode)
				}
				return
			}
			expectProblem(t, resp, tt.status)
		})
	}
}

func TestUnknownAPIPath(t *testing.T) {
	e := newEnv(t, envOptions{spa: spaFS()})
	for _, path := range []string{"/api/v1/nope", "/api/", "/api/v2/me"} {
		t.Run(path, func(t *testing.T) {
			expectProblem(t, e.do(request{path: path}), http.StatusNotFound)
		})
	}
}

func TestHealthz(t *testing.T) {
	e := newEnv(t, envOptions{})
	resp := e.do(request{path: "/healthz"})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if got := decode[map[string]string](t, resp); got["status"] != "ok" || got["version"] != buildinfo.Version {
		t.Errorf("body = %v", got)
	}

	_ = e.db.Close()
	if resp := e.do(request{path: "/healthz"}); resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("closed DB: status = %d", resp.StatusCode)
	}
}

func TestSecurityHeaders(t *testing.T) {
	e := newEnv(t, envOptions{spa: spaFS()})
	for _, path := range []string{"/", "/api/v1/me", "/healthz"} {
		resp := e.do(request{path: path})
		h := resp.Header
		if h.Get("Content-Security-Policy") != contentSecurityPolicy || h.Get("X-Content-Type-Options") != "nosniff" ||
			h.Get("Referrer-Policy") != "same-origin" || h.Get("Permissions-Policy") == "" || h.Get("X-Request-Id") == "" {
			t.Errorf("%s: missing security headers: %v", path, h)
		}
	}
}
