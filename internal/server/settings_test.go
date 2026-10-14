package server

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/reilandeubank/grower/internal/apigen"
	"github.com/reilandeubank/grower/internal/auth"
)

func TestUpdateSettings(t *testing.T) {
	e := newEnv(t, envOptions{})
	e.clock.t = time.Date(2026, 5, 1, 3, 0, 0, 0, time.UTC) // April 30 in Chicago, May 1 in London
	c := e.loginCookie()
	if got := e.today(c).Date.String(); got != "2026-04-30" {
		t.Fatalf("today in Chicago = %s", got)
	}

	for _, tz := range []string{"", "Local", "Mars/Olympus_Mons", "../../etc/passwd", "/etc/localtime"} {
		resp := e.call(c, http.MethodPut, "/api/v1/settings", `{"timezone":"`+tz+`"}`)
		if got := expectProblem(t, resp, http.StatusBadRequest); got != "Pick a time zone from the list." {
			t.Errorf("%q: detail = %q", tz, got)
		}
	}

	got := expectJSON[apigen.Settings](t, e.call(c, http.MethodPut, "/api/v1/settings", `{"timezone":"Europe/London"}`), http.StatusOK)
	if got.Timezone != "Europe/London" {
		t.Errorf("settings = %+v", got)
	}
	if me := expectJSON[apigen.Me](t, e.call(c, http.MethodGet, "/api/v1/me", ""), http.StatusOK); me.Settings.Timezone != "Europe/London" {
		t.Errorf("me settings = %+v", me.Settings)
	}
	if got := e.today(c).Date.String(); got != "2026-05-01" {
		t.Errorf("today after switching to London = %s", got)
	}
	expectProblem(t, e.call(nil, http.MethodPut, "/api/v1/settings", `{"timezone":"UTC"}`), http.StatusUnauthorized)
}

func TestChangePassword(t *testing.T) {
	e := newEnv(t, envOptions{})
	other := e.loginCookie()
	c := e.loginCookie()
	change := func(current, next string) *http.Response {
		return e.call(c, http.MethodPost, "/api/v1/auth/password", `{"current_password":"`+current+`","new_password":"`+next+`"}`)
	}

	if got := expectProblem(t, change("wrong-password", "a-brand-new-password"), http.StatusBadRequest); got != "Your current password is wrong." {
		t.Errorf("wrong current: detail = %q", got)
	}
	if got := expectProblem(t, change(testPassword, "short"), http.StatusBadRequest); got != "Use at least 10 characters." {
		t.Errorf("short: detail = %q", got)
	}
	// Failed attempts change nothing.
	if resp := e.login(testUser, testPassword); resp.StatusCode != http.StatusOK {
		t.Fatalf("old password stopped working: %d", resp.StatusCode)
	}

	if resp := change(testPassword, "a-brand-new-password"); resp.StatusCode != http.StatusNoContent {
		t.Fatalf("change: status %d", resp.StatusCode)
	}
	if resp := e.call(c, http.MethodGet, "/api/v1/me", ""); resp.StatusCode != http.StatusOK {
		t.Errorf("this session was signed out: %d", resp.StatusCode)
	}
	expectProblem(t, e.call(other, http.MethodGet, "/api/v1/me", ""), http.StatusUnauthorized)
	expectProblem(t, e.login(testUser, testPassword), http.StatusUnauthorized)
	if resp := e.login(testUser, "a-brand-new-password"); resp.StatusCode != http.StatusOK {
		t.Errorf("new password: login status %d", resp.StatusCode)
	}
	expectProblem(t, e.call(nil, http.MethodPost, "/api/v1/auth/password", `{"current_password":"x","new_password":"y"}`), http.StatusUnauthorized)
}

func TestChangePasswordRateLimit(t *testing.T) {
	e := newEnv(t, envOptions{limits: auth.RateLimits{PerIPEvery: time.Minute, PerIPBurst: 3, GlobalEvery: time.Second, GlobalBurst: 100}})
	c := e.loginCookie() // uses one attempt
	body := func(current string) string {
		return `{"current_password":"` + current + `","new_password":"a-brand-new-password"}`
	}
	for range 2 {
		expectProblem(t, e.call(c, http.MethodPost, "/api/v1/auth/password", body("wrong-password")), http.StatusBadRequest)
	}
	// The bucket is shared with login and empty now, so even the right password waits.
	if got := expectProblem(t, e.call(c, http.MethodPost, "/api/v1/auth/password", body(testPassword)), http.StatusTooManyRequests); got != "Too many attempts. Wait a minute and try again." {
		t.Errorf("detail = %q", got)
	}
	e.clock.t = e.clock.t.Add(time.Minute)
	if resp := e.call(c, http.MethodPost, "/api/v1/auth/password", body(testPassword)); resp.StatusCode != http.StatusNoContent {
		t.Errorf("after refill: status %d", resp.StatusCode)
	}
}

func TestStorage(t *testing.T) {
	e := newEnv(t, envOptions{})
	c := e.loginCookie()
	empty := expectJSON[apigen.Storage](t, e.call(c, http.MethodGet, "/api/v1/storage", ""), http.StatusOK)
	if empty.PhotoCount != 0 || empty.PhotoBytes != 0 || empty.DatabaseBytes <= 0 || empty.LastBackupAt != nil {
		t.Errorf("empty storage = %+v", empty)
	}

	p := e.newPlant(c, "Fern")
	photos := []apigen.Photo{
		e.newPhoto(c, p.Id, testPNG(t, 300, 200, 1), "2026-04-01T10:00:00Z"),
		e.newPhoto(c, p.Id, testPNG(t, 300, 200, 2), "2026-04-02T10:00:00Z"),
	}
	var want int64
	for _, ph := range photos {
		for _, name := range []string{"original.png", "small.jpg", "medium.jpg"} {
			fi, err := os.Stat(filepath.Join(e.photoDir(ph.Id), name))
			if err != nil {
				t.Fatal(err)
			}
			want += fi.Size()
		}
	}

	backups := filepath.Join(e.cfg.DataDir, "backups")
	if err := os.MkdirAll(backups, 0o750); err != nil {
		t.Fatal(err)
	}
	newest := time.Date(2026, 4, 30, 3, 5, 0, 0, time.UTC)
	for name, mtime := range map[string]time.Time{
		"grower-2026-04-29.db": newest.AddDate(0, 0, -1),
		"grower-2026-04-30.db": newest,
		"notes.txt":            newest.Add(time.Hour), // not a backup
	} {
		path := filepath.Join(backups, name)
		if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(path, mtime, mtime); err != nil {
			t.Fatal(err)
		}
	}

	got := expectJSON[apigen.Storage](t, e.call(c, http.MethodGet, "/api/v1/storage", ""), http.StatusOK)
	if got.PhotoCount != 2 || got.PhotoBytes != want || got.DatabaseBytes <= 0 || got.LastBackupAt == nil || !got.LastBackupAt.Equal(newest) {
		t.Errorf("storage = %+v, want photo_bytes %d and last backup %s", got, want, newest)
	}
	expectProblem(t, e.call(nil, http.MethodGet, "/api/v1/storage", ""), http.StatusUnauthorized)
}
