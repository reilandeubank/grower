package server

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/reilandeubank/grower/internal/auth"
	"github.com/reilandeubank/grower/internal/config"
	"github.com/reilandeubank/grower/internal/db"
	"github.com/reilandeubank/grower/internal/db/dbtest"
	"github.com/reilandeubank/grower/internal/push"
	"github.com/reilandeubank/grower/internal/push/pushtest"
)

const (
	testUser     = "fern"
	testPassword = "monstera-deliciosa"
)

type fakeClock struct{ t time.Time }

func (c *fakeClock) Now() time.Time { return c.t }

type env struct {
	t       *testing.T
	handler http.Handler
	db      *db.DB
	clock   *fakeClock
	cfg     config.Config
	// pushService receives every notification the server sends.
	pushService *pushtest.Service
}

type envOptions struct {
	env    map[string]string
	limits auth.RateLimits
	spa    fstest.MapFS
}

func newEnv(t *testing.T, opts envOptions) *env {
	t.Helper()
	vars := map[string]string{"GROWER_BASE_URL": "http://grower.test", "GROWER_TZ": "America/Chicago", "GROWER_DATA_DIR": t.TempDir()}
	for k, v := range opts.env {
		vars[k] = v
	}
	cfg, err := config.Load(func(k string) string { return vars[k] })
	if err != nil {
		t.Fatal(err)
	}
	d := dbtest.Open(t)
	clock := &fakeClock{t: time.Date(2026, 5, 1, 9, 0, 0, 0, time.UTC)}
	svc, err := auth.NewService(d, auth.Options{
		HashParams: auth.HashParams{MemoryKiB: 64, Iterations: 1, Parallelism: 1, SaltLen: 16, KeyLen: 32},
		RateLimits: opts.limits,
		Now:        clock.Now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CreateUser(context.Background(), testUser, testPassword, cfg.Location.String()); err != nil {
		t.Fatal(err)
	}
	spa := opts.spa
	if spa == nil {
		spa = fstest.MapFS{}
	}
	log := slog.New(slog.DiscardHandler)
	keys, err := push.LoadKeys(cfg.DataDir)
	if err != nil {
		t.Fatal(err)
	}
	fake := pushtest.New(t)
	pusher := &push.Sender{DB: d, Keys: keys, Subject: "http://grower.test", Client: fake.Client(), Now: clock.Now, Log: log}
	h := New(Deps{Config: cfg, DB: d, Auth: svc, Push: pusher, SPA: spa, Log: log})
	return &env{t: t, handler: h, db: d, clock: clock, cfg: cfg, pushService: fake}
}

// request describes one call; zero values give a same-origin JSON request.
type request struct {
	method, path, body string
	headers            map[string]string
	cookie             *http.Cookie
	remoteAddr         string
	noOrigin           bool
}

func (e *env) do(r request) *http.Response {
	e.t.Helper()
	if r.method == "" {
		r.method = http.MethodGet
	}
	var body io.Reader
	if r.body != "" {
		body = strings.NewReader(r.body)
	}
	req := httptest.NewRequest(r.method, r.path, body)
	if r.body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if r.method != http.MethodGet && !r.noOrigin {
		req.Header.Set("Origin", e.cfg.Origin())
	}
	for k, v := range r.headers {
		req.Header.Set(k, v)
	}
	if r.cookie != nil {
		req.AddCookie(r.cookie)
	}
	if r.remoteAddr != "" {
		req.RemoteAddr = r.remoteAddr
	}
	rec := httptest.NewRecorder()
	e.handler.ServeHTTP(rec, req)
	return rec.Result()
}

func (e *env) login(user, password string) *http.Response {
	e.t.Helper()
	b, _ := json.Marshal(map[string]string{"username": user, "password": password})
	return e.do(request{method: http.MethodPost, path: "/api/v1/auth/login", body: string(b)})
}

// loginCookie logs in as the test user and returns the session cookie.
func (e *env) loginCookie() *http.Cookie {
	e.t.Helper()
	resp := e.login(testUser, testPassword)
	if resp.StatusCode != http.StatusOK {
		e.t.Fatalf("login: status %d", resp.StatusCode)
	}
	return sessionCookie(e.t, resp, e.cfg.SessionCookieName())
}

func sessionCookie(t *testing.T, resp *http.Response, name string) *http.Cookie {
	t.Helper()
	for _, c := range resp.Cookies() {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("no %s cookie in response", name)
	return nil
}

func decode[T any](t *testing.T, resp *http.Response) T {
	t.Helper()
	defer resp.Body.Close()
	var v T
	if err := json.NewDecoder(resp.Body).Decode(&v); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	return v
}

// expectProblem asserts an RFC 9457 response with the given status and returns its detail.
func expectProblem(t *testing.T, resp *http.Response, status int) string {
	t.Helper()
	if resp.StatusCode != status {
		t.Fatalf("status = %d, want %d", resp.StatusCode, status)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/problem+json" {
		t.Errorf("Content-Type = %q, want application/problem+json", ct)
	}
	p := decode[struct {
		Title  string `json:"title"`
		Status int    `json:"status"`
		Detail string `json:"detail"`
	}](t, resp)
	if p.Status != status || p.Title == "" {
		t.Errorf("problem body = %+v", p)
	}
	return p.Detail
}

// call makes a same-origin JSON request with the given session cookie (nil for none).
func (e *env) call(cookie *http.Cookie, method, path, body string) *http.Response {
	e.t.Helper()
	return e.do(request{method: method, path: path, body: body, cookie: cookie})
}

// expectJSON asserts the status and decodes the body.
func expectJSON[T any](t *testing.T, resp *http.Response, status int) T {
	t.Helper()
	if resp.StatusCode != status {
		t.Fatalf("status = %d, want %d", resp.StatusCode, status)
	}
	return decode[T](t, resp)
}
