package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/reilandeubank/grower/internal/db"
	"github.com/reilandeubank/grower/internal/db/dbtest"
)

// fakeClock is a settable clock for tests.
type fakeClock struct{ t time.Time }

func (c *fakeClock) Now() time.Time          { return c.t }
func (c *fakeClock) Advance(d time.Duration) { c.t = c.t.Add(d) }

func newTestService(t *testing.T) (*Service, *db.DB, *fakeClock) {
	t.Helper()
	d := dbtest.Open(t)
	clock := &fakeClock{t: time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)}
	svc, err := NewService(d, Options{HashParams: cheapParams, Now: clock.Now})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CreateUser(context.Background(), "fern", "monstera-deliciosa", "Europe/Berlin"); err != nil {
		t.Fatal(err)
	}
	return svc, d, clock
}

func TestLogin(t *testing.T) {
	svc, _, _ := newTestService(t)
	ctx := context.Background()
	tests := []struct {
		name, user, pass string
		wantErr          error
	}{
		{"ok", "fern", "monstera-deliciosa", nil},
		{"wrong password", "fern", "monstera", ErrInvalidCredentials},
		{"unknown user", "ivy", "monstera-deliciosa", ErrInvalidCredentials},
		{"case sensitive", "Fern", "monstera-deliciosa", ErrInvalidCredentials},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, token, err := svc.Login(ctx, tt.user, tt.pass, "192.0.2.1", "test")
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if err == nil && (token == "" || p.Username != "fern") {
				t.Errorf("unexpected result %+v %q", p, token)
			}
		})
	}
}

func TestSessionSlidingExpiry(t *testing.T) {
	svc, _, clock := newTestService(t)
	ctx := context.Background()
	_, token, err := svc.Login(ctx, "fern", "monstera-deliciosa", "192.0.2.1", "test")
	if err != nil {
		t.Fatal(err)
	}
	start := clock.t
	sessions := svc.Sessions()

	// Within a day: valid, not refreshed.
	clock.Advance(23 * time.Hour)
	p, refreshed, err := sessions.Lookup(ctx, token)
	if err != nil || refreshed || !p.ExpiresAt.Equal(start.Add(SessionTTL)) {
		t.Fatalf("lookup at 23h: %+v refreshed=%v err=%v", p, refreshed, err)
	}

	// After a day: refreshed, expiry slides to now+TTL.
	clock.Advance(2 * time.Hour)
	p, refreshed, err = sessions.Lookup(ctx, token)
	if err != nil || !refreshed || !p.ExpiresAt.Equal(clock.t.Add(SessionTTL)) {
		t.Fatalf("lookup at 25h: %+v refreshed=%v err=%v", p, refreshed, err)
	}

	// Just before the slid expiry it still works (and refreshes again).
	clock.Advance(SessionTTL - time.Second)
	if _, _, err := sessions.Lookup(ctx, token); err != nil {
		t.Fatalf("lookup before expiry: %v", err)
	}

	// Idle past expiry: gone, and the row is deleted.
	clock.Advance(SessionTTL)
	if _, _, err := sessions.Lookup(ctx, token); !errors.Is(err, ErrNoSession) {
		t.Fatalf("lookup after expiry: %v", err)
	}
}

func TestSessionLookupUnknownAndRevoke(t *testing.T) {
	svc, _, _ := newTestService(t)
	ctx := context.Background()
	sessions := svc.Sessions()
	if _, _, err := sessions.Lookup(ctx, ""); !errors.Is(err, ErrNoSession) {
		t.Errorf("empty token: %v", err)
	}
	if _, _, err := sessions.Lookup(ctx, "bogus"); !errors.Is(err, ErrNoSession) {
		t.Errorf("unknown token: %v", err)
	}
	p, token, err := svc.Login(ctx, "fern", "monstera-deliciosa", "192.0.2.1", "test")
	if err != nil {
		t.Fatal(err)
	}
	if err := sessions.Revoke(ctx, p.SessionHash); err != nil {
		t.Fatal(err)
	}
	if _, _, err := sessions.Lookup(ctx, token); !errors.Is(err, ErrNoSession) {
		t.Errorf("revoked token: %v", err)
	}
}

func TestSetPasswordRevokesSessions(t *testing.T) {
	svc, _, _ := newTestService(t)
	ctx := context.Background()
	_, token, err := svc.Login(ctx, "fern", "monstera-deliciosa", "192.0.2.1", "test")
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.SetPassword(ctx, "fern", "short"); !errors.Is(err, ErrPasswordTooShort) {
		t.Errorf("short password: %v", err)
	}
	if err := svc.SetPassword(ctx, "nobody", "a-new-long-password"); !errors.Is(err, ErrUserNotFound) {
		t.Errorf("unknown user: %v", err)
	}
	if err := svc.SetPassword(ctx, "fern", "a-new-long-password"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.Sessions().Lookup(ctx, token); !errors.Is(err, ErrNoSession) {
		t.Errorf("old session survived password change: %v", err)
	}
	if _, _, err := svc.Login(ctx, "fern", "a-new-long-password", "192.0.2.1", "test"); err != nil {
		t.Errorf("login with new password: %v", err)
	}
}

func TestCreateUserValidation(t *testing.T) {
	svc, d, _ := newTestService(t)
	ctx := context.Background()
	tests := []struct {
		name, user, pass string
		wantErr          error
	}{
		{"duplicate", "fern", "long-enough-pw", ErrUserExists},
		{"short password", "ivy", "123456789", ErrPasswordTooShort},
		{"space in name", "ivy leaf", "long-enough-pw", ErrInvalidUsername},
		{"empty name", "", "long-enough-pw", ErrInvalidUsername},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := svc.CreateUser(ctx, tt.user, tt.pass, "UTC"); !errors.Is(err, tt.wantErr) {
				t.Errorf("err = %v, want %v", err, tt.wantErr)
			}
		})
	}
	user, err := d.Reader().GetUserByUsername(ctx, "fern")
	if err != nil {
		t.Fatal(err)
	}
	settings, err := d.Reader().GetSettings(ctx, user.ID)
	if err != nil || settings.Timezone != "Europe/Berlin" {
		t.Errorf("settings = %+v, %v", settings, err)
	}
}

func TestLoginLimiter(t *testing.T) {
	clock := &fakeClock{t: time.Unix(0, 0)}
	l := NewLoginLimiter(RateLimits{PerIPEvery: 10 * time.Second, PerIPBurst: 2, GlobalEvery: time.Second, GlobalBurst: 3}, clock.Now)

	steps := []struct {
		ip      string
		advance time.Duration
		want    bool
	}{
		{"a", 0, true},
		{"a", 0, true},
		{"a", 0, false}, // per-IP burst spent
		{"b", 0, true},  // other IP unaffected; global now at 3
		{"c", 0, false}, // global burst spent
		{"c", time.Second, true},
		{"a", 9 * time.Second, true}, // 10 s after start: one per-IP token back
		{"a", 0, false},
	}
	for i, s := range steps {
		clock.Advance(s.advance)
		if got := l.Allow(s.ip); got != s.want {
			t.Fatalf("step %d (%s): Allow = %v, want %v", i, s.ip, got, s.want)
		}
	}
}
