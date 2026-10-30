package auth

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

var (
	// ErrSetupDone means an account already exists, so first-run setup is closed.
	ErrSetupDone = errors.New("setup is already complete")
	// ErrSetupExpired means the setup window after startup has passed; restarting
	// grower opens a new one.
	ErrSetupExpired = errors.New("setup window has expired")
)

// SetupWindow is how long after startup the first account can be created from the
// web. A fresh install is often reachable through the reverse proxy before anyone
// has set it up; the window limits how long a stranger could claim it first.
const SetupWindow = 10 * time.Minute

// setupState holds when first-run setup closes. The zero deadline means setup was
// never opened (PrepareSetup not called, as in the CLI).
type setupState struct {
	mu       sync.Mutex
	deadline time.Time
}

// SetupStatus describes first-run setup: Needed while no account exists, Open while
// it can still be completed from the web.
type SetupStatus struct {
	Needed bool
	Open   bool
}

// Setup reports whether the first account still needs creating and whether the
// window to do so is open.
func (s *Service) Setup(ctx context.Context) (SetupStatus, error) {
	n, err := s.db.Reader().CountUsers(ctx)
	if err != nil {
		return SetupStatus{}, fmt.Errorf("count users: %w", err)
	}
	if n > 0 {
		return SetupStatus{}, nil
	}
	s.setup.mu.Lock()
	deadline := s.setup.deadline
	s.setup.mu.Unlock()
	return SetupStatus{Needed: true, Open: !deadline.IsZero() && s.now().Before(deadline)}, nil
}

// PrepareSetup opens the setup window if there are no users yet, and reports
// whether it did.
func (s *Service) PrepareSetup(ctx context.Context) (bool, error) {
	status, err := s.Setup(ctx)
	if err != nil || !status.Needed {
		return false, err
	}
	s.setup.mu.Lock()
	s.setup.deadline = s.now().Add(SetupWindow)
	s.setup.mu.Unlock()
	return true, nil
}

// CompleteSetup creates the first account while the setup window is open, then
// signs it in like Login. Attempts count against the login rate limit.
func (s *Service) CompleteSetup(ctx context.Context, username, password, timezone, clientIP, userAgent string) (Principal, string, error) {
	if !s.limiter.Allow(clientIP) {
		return Principal{}, "", ErrRateLimited
	}
	status, err := s.Setup(ctx)
	switch {
	case err != nil:
		return Principal{}, "", err
	case !status.Needed:
		return Principal{}, "", ErrSetupDone
	case !status.Open:
		return Principal{}, "", ErrSetupExpired
	}

	user, err := s.createUser(ctx, username, password, timezone, true)
	if err != nil {
		return Principal{}, "", err
	}
	token, expires, err := s.sessions.Create(ctx, user.ID, userAgent)
	if err != nil {
		return Principal{}, "", err
	}
	return Principal{UserID: user.ID, Username: user.Username, SessionHash: hashToken(token), ExpiresAt: expires}, token, nil
}
