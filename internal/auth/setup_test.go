package auth

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/reilandeubank/grower/internal/db/dbtest"
)

// newEmptyService is a service over a database with no users.
func newEmptyService(t *testing.T, limits RateLimits) (*Service, *fakeClock) {
	t.Helper()
	clock := &fakeClock{t: time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)}
	svc, err := NewService(dbtest.Open(t), Options{HashParams: cheapParams, RateLimits: limits, Now: clock.Now})
	if err != nil {
		t.Fatal(err)
	}
	return svc, clock
}

func TestPrepareSetup(t *testing.T) {
	ctx := context.Background()
	t.Run("opens on an empty database", func(t *testing.T) {
		svc, _ := newEmptyService(t, RateLimits{})
		if got, _ := svc.Setup(ctx); got != (SetupStatus{Needed: true}) {
			t.Fatalf("before PrepareSetup: %+v, want needed but closed", got)
		}
		if open, err := svc.PrepareSetup(ctx); err != nil || !open {
			t.Fatalf("PrepareSetup = %v, %v", open, err)
		}
		if got, _ := svc.Setup(ctx); got != (SetupStatus{Needed: true, Open: true}) {
			t.Fatalf("after PrepareSetup: %+v", got)
		}
	})
	t.Run("does nothing once a user exists", func(t *testing.T) {
		svc, _, _ := newTestService(t)
		if open, err := svc.PrepareSetup(ctx); err != nil || open {
			t.Fatalf("PrepareSetup = %v, %v", open, err)
		}
		if got, _ := svc.Setup(ctx); got != (SetupStatus{}) {
			t.Fatalf("Setup = %+v, want not needed", got)
		}
	})
}

func TestCompleteSetup(t *testing.T) {
	ctx := context.Background()
	svc, _ := newEmptyService(t, RateLimits{})
	if _, err := svc.PrepareSetup(ctx); err != nil {
		t.Fatal(err)
	}

	if _, _, err := svc.CompleteSetup(ctx, "fern", "short", "UTC", "192.0.2.1", "test"); !errors.Is(err, ErrPasswordTooShort) {
		t.Fatalf("short password: err = %v", err)
	}
	if got, _ := svc.Setup(ctx); !got.Needed {
		t.Fatal("a failed attempt must not create a user")
	}

	p, token, err := svc.CompleteSetup(ctx, "fern", "monstera-deliciosa", "Europe/Berlin", "192.0.2.1", "test")
	if err != nil {
		t.Fatalf("CompleteSetup: %v", err)
	}
	if got, _, err := svc.Sessions().Lookup(ctx, token); err != nil || got.UserID != p.UserID || got.Username != "fern" {
		t.Fatalf("session lookup = %+v, %v", got, err)
	}
	settings, err := svc.db.Reader().GetSettings(ctx, p.UserID)
	if err != nil || settings.Timezone != "Europe/Berlin" {
		t.Fatalf("settings = %+v, %v", settings, err)
	}

	if _, _, err := svc.CompleteSetup(ctx, "ivy", "monstera-deliciosa", "UTC", "192.0.2.1", "test"); !errors.Is(err, ErrSetupDone) {
		t.Fatalf("second setup: err = %v, want ErrSetupDone", err)
	}
}

func TestCompleteSetupWindow(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name    string
		prepare bool
		after   time.Duration
		wantErr error
	}{
		{"never opened (CLI)", false, 0, ErrSetupExpired},
		{"just before closing", true, SetupWindow - time.Second, nil},
		{"after closing", true, SetupWindow, ErrSetupExpired},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, clock := newEmptyService(t, RateLimits{})
			if tt.prepare {
				if _, err := svc.PrepareSetup(ctx); err != nil {
					t.Fatal(err)
				}
			}
			clock.Advance(tt.after)
			_, _, err := svc.CompleteSetup(ctx, "fern", "monstera-deliciosa", "UTC", "192.0.2.1", "test")
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestCompleteSetupRateLimited(t *testing.T) {
	ctx := context.Background()
	svc, _ := newEmptyService(t, RateLimits{PerIPEvery: time.Hour, PerIPBurst: 3, GlobalEvery: time.Second, GlobalBurst: 100})
	if _, err := svc.PrepareSetup(ctx); err != nil {
		t.Fatal(err)
	}
	for i := range 3 {
		if _, _, err := svc.CompleteSetup(ctx, "fern", "short", "UTC", "192.0.2.1", "test"); !errors.Is(err, ErrPasswordTooShort) {
			t.Fatalf("attempt %d: err = %v", i, err)
		}
	}
	if _, _, err := svc.CompleteSetup(ctx, "fern", "monstera-deliciosa", "UTC", "192.0.2.1", "test"); !errors.Is(err, ErrRateLimited) {
		t.Fatalf("err = %v, want ErrRateLimited", err)
	}
}

func TestCompleteSetupConcurrent(t *testing.T) {
	ctx := context.Background()
	svc, _ := newEmptyService(t, RateLimits{PerIPEvery: time.Millisecond, PerIPBurst: 100, GlobalEvery: time.Millisecond, GlobalBurst: 100})
	if _, err := svc.PrepareSetup(ctx); err != nil {
		t.Fatal(err)
	}

	const n = 8
	var wg sync.WaitGroup
	errs := make([]error, n)
	start := make(chan struct{})
	for i := range n {
		wg.Go(func() {
			<-start
			_, _, errs[i] = svc.CompleteSetup(ctx, fmt.Sprintf("user%d", i), "monstera-deliciosa", "UTC", fmt.Sprintf("192.0.2.%d", i+1), "test")
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
			// Lost the race.
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
