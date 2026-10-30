package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/reilandeubank/grower/internal/db/dbtest"
)

// newEmptyService is a service over a database with no users.
func newEmptyService(t *testing.T, limits RateLimits) *Service {
	t.Helper()
	clock := &fakeClock{t: time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)}
	svc, err := NewService(dbtest.Open(t), Options{HashParams: cheapParams, RateLimits: limits, Now: clock.Now})
	if err != nil {
		t.Fatal(err)
	}
	return svc
}

func TestNewSetupCode(t *testing.T) {
	seen := map[string]bool{}
	for range 50 {
		code, err := newSetupCode()
		if err != nil {
			t.Fatal(err)
		}
		if len(code) != setupCodeLen+1 || code[setupCodeLen/2] != '-' {
			t.Fatalf("code %q is not two groups of %d", code, setupCodeLen/2)
		}
		if strings.ContainsAny(code, "ILOU") {
			t.Fatalf("code %q has a Crockford-excluded letter", code)
		}
		if normalizeSetupCode(code) != strings.ReplaceAll(code, "-", "") {
			t.Fatalf("a generated code must already be normalized, got %q", normalizeSetupCode(code))
		}
		seen[code] = true
	}
	if len(seen) < 50 {
		t.Errorf("only %d distinct codes in 50", len(seen))
	}
}

func TestNormalizeSetupCode(t *testing.T) {
	tests := []struct{ in, want string }{
		{"K7Q2M-X9PB4", "K7Q2MX9PB4"},
		{" k7q2m x9pb4 ", "K7Q2MX9PB4"},
		{"OIL", "011"},
		{"our-secret-code", "0URSECRETC0DE"},
	}
	for _, tt := range tests {
		if got := normalizeSetupCode(tt.in); got != tt.want {
			t.Errorf("normalizeSetupCode(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestPrepareSetup(t *testing.T) {
	ctx := context.Background()
	t.Run("generates a code when empty", func(t *testing.T) {
		svc := newEmptyService(t, RateLimits{})
		code, needed, err := svc.PrepareSetup(ctx, "")
		if err != nil || !needed || len(code) != setupCodeLen+1 {
			t.Fatalf("PrepareSetup = %q, %v, %v", code, needed, err)
		}
	})
	t.Run("uses the configured code", func(t *testing.T) {
		svc := newEmptyService(t, RateLimits{})
		code, needed, err := svc.PrepareSetup(ctx, "our-secret-code")
		if err != nil || !needed || code != "our-secret-code" {
			t.Fatalf("PrepareSetup = %q, %v, %v", code, needed, err)
		}
	})
	t.Run("arms nothing once a user exists", func(t *testing.T) {
		svc, _, _ := newTestService(t)
		code, needed, err := svc.PrepareSetup(ctx, "our-secret-code")
		if err != nil || needed || code != "" {
			t.Fatalf("PrepareSetup = %q, %v, %v", code, needed, err)
		}
		if svc.setupCodeMatches("our-secret-code") {
			t.Error("setup must not be armed when users exist")
		}
	})
}

func TestCompleteSetup(t *testing.T) {
	ctx := context.Background()
	svc := newEmptyService(t, RateLimits{})
	code, _, err := svc.PrepareSetup(ctx, "")
	if err != nil {
		t.Fatal(err)
	}

	if _, _, err := svc.CompleteSetup(ctx, "WRONG-CODE0", "fern", "monstera-deliciosa", "UTC", "192.0.2.1", "test"); !errors.Is(err, ErrWrongSetupCode) {
		t.Fatalf("wrong code: err = %v", err)
	}
	if _, _, err := svc.CompleteSetup(ctx, code, "fern", "short", "UTC", "192.0.2.1", "test"); !errors.Is(err, ErrPasswordTooShort) {
		t.Fatalf("short password: err = %v", err)
	}
	if needed, _ := svc.SetupNeeded(ctx); !needed {
		t.Fatal("a failed attempt must not create a user")
	}

	// Typed in lower case without the dash, as someone reading it off a log might.
	typed := strings.ToLower(strings.ReplaceAll(code, "-", ""))
	p, token, err := svc.CompleteSetup(ctx, typed, "fern", "monstera-deliciosa", "Europe/Berlin", "192.0.2.1", "test")
	if err != nil {
		t.Fatalf("CompleteSetup: %v", err)
	}
	if p.Username != "fern" || token == "" {
		t.Fatalf("principal = %+v, token %q", p, token)
	}
	if got, _, err := svc.Sessions().Lookup(ctx, token); err != nil || got.UserID != p.UserID {
		t.Fatalf("session lookup = %+v, %v", got, err)
	}
	settings, err := svc.db.Reader().GetSettings(ctx, p.UserID)
	if err != nil || settings.Timezone != "Europe/Berlin" {
		t.Fatalf("settings = %+v, %v", settings, err)
	}

	if _, _, err := svc.CompleteSetup(ctx, code, "ivy", "monstera-deliciosa", "UTC", "192.0.2.1", "test"); !errors.Is(err, ErrSetupDone) {
		t.Fatalf("second setup: err = %v, want ErrSetupDone", err)
	}
	if svc.setupCodeMatches(code) {
		t.Error("the code must be cleared once setup completes")
	}
}

func TestCompleteSetupUnarmed(t *testing.T) {
	// No PrepareSetup (as for a service built by the CLI): no code is accepted.
	svc := newEmptyService(t, RateLimits{})
	_, _, err := svc.CompleteSetup(context.Background(), "", "fern", "monstera-deliciosa", "UTC", "192.0.2.1", "test")
	if !errors.Is(err, ErrWrongSetupCode) {
		t.Fatalf("err = %v, want ErrWrongSetupCode", err)
	}
}

func TestCompleteSetupRateLimited(t *testing.T) {
	ctx := context.Background()
	svc := newEmptyService(t, RateLimits{PerIPEvery: time.Hour, PerIPBurst: 3, GlobalEvery: time.Second, GlobalBurst: 100})
	code, _, err := svc.PrepareSetup(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	for i := range 3 {
		if _, _, err := svc.CompleteSetup(ctx, "WRONG-CODE0", "fern", "monstera-deliciosa", "UTC", "192.0.2.1", "test"); !errors.Is(err, ErrWrongSetupCode) {
			t.Fatalf("attempt %d: err = %v", i, err)
		}
	}
	// Even the right code waits once the bucket is empty.
	if _, _, err := svc.CompleteSetup(ctx, code, "fern", "monstera-deliciosa", "UTC", "192.0.2.1", "test"); !errors.Is(err, ErrRateLimited) {
		t.Fatalf("err = %v, want ErrRateLimited", err)
	}
}

func TestCompleteSetupConcurrent(t *testing.T) {
	ctx := context.Background()
	svc := newEmptyService(t, RateLimits{PerIPEvery: time.Millisecond, PerIPBurst: 100, GlobalEvery: time.Millisecond, GlobalBurst: 100})
	code, _, err := svc.PrepareSetup(ctx, "")
	if err != nil {
		t.Fatal(err)
	}

	const n = 8
	var wg sync.WaitGroup
	errs := make([]error, n)
	start := make(chan struct{})
	for i := range n {
		wg.Go(func() {
			<-start
			_, _, errs[i] = svc.CompleteSetup(ctx, code, fmt.Sprintf("user%d", i), "monstera-deliciosa", "UTC", fmt.Sprintf("192.0.2.%d", i+1), "test")
		})
	}
	close(start)
	wg.Wait()

	created := 0
	for i, err := range errs {
		switch {
		case err == nil:
			created++
		case errors.Is(err, ErrSetupDone):
			// Lost the race, inside the transaction or after the code was cleared.
		default:
			t.Errorf("request %d: unexpected error %v", i, err)
		}
	}
	if created != 1 {
		t.Fatalf("%d setups succeeded, want exactly 1", created)
	}
	if n, err := svc.db.Reader().CountUsers(ctx); err != nil || n != 1 {
		t.Fatalf("users = %d, %v; want 1", n, err)
	}
}
