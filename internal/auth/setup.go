package auth

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"errors"
	"fmt"
	"strings"
	"sync"
)

var (
	// ErrSetupDone means an account already exists, so first-run setup is closed.
	ErrSetupDone = errors.New("setup is already complete")
	// ErrWrongSetupCode means the code doesn't match the one in the server log.
	ErrWrongSetupCode = errors.New("wrong setup code")
)

// setupAlphabet is Crockford's base32: no I, L, O or U, so a code read off a log
// can't be mistyped as a lookalike.
const setupAlphabet = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

// setupCodeLen gives 50 bits, far beyond what the login rate limit lets anyone guess.
const setupCodeLen = 10

// setupState holds the code that unlocks first-run setup. It lives only in memory: a
// restart makes a new one, and completing setup clears it.
type setupState struct {
	mu   sync.Mutex
	code string // normalized; "" when setup is not armed
}

// SetupNeeded reports whether no account exists yet.
func (s *Service) SetupNeeded(ctx context.Context) (bool, error) {
	n, err := s.db.Reader().CountUsers(ctx)
	if err != nil {
		return false, fmt.Errorf("count users: %w", err)
	}
	return n == 0, nil
}

// PrepareSetup arms first-run setup if there are no users yet. configured is the
// operator's code (GROWER_SETUP_CODE), or "" to generate one. It returns the code to
// show the operator, formatted for reading, and whether setup is needed at all; when
// it isn't, nothing is armed and the code is "".
func (s *Service) PrepareSetup(ctx context.Context, configured string) (code string, needed bool, err error) {
	needed, err = s.SetupNeeded(ctx)
	if err != nil || !needed {
		return "", false, err
	}
	code = configured
	if code == "" {
		if code, err = newSetupCode(); err != nil {
			return "", false, err
		}
	}
	s.setup.mu.Lock()
	s.setup.code = normalizeSetupCode(code)
	s.setup.mu.Unlock()
	return code, true, nil
}

// CompleteSetup creates the first account when code matches the armed setup code,
// then signs it in like Login. Attempts count against the login rate limit.
func (s *Service) CompleteSetup(ctx context.Context, code, username, password, timezone, clientIP, userAgent string) (Principal, string, error) {
	if !s.limiter.Allow(clientIP) {
		return Principal{}, "", ErrRateLimited
	}
	needed, err := s.SetupNeeded(ctx)
	if err != nil {
		return Principal{}, "", err
	}
	if !needed {
		return Principal{}, "", ErrSetupDone
	}
	if !s.setupCodeMatches(code) {
		// A racing setup may have just finished and cleared the code: say so rather
		// than blame the code.
		if needed, err := s.SetupNeeded(ctx); err == nil && !needed {
			return Principal{}, "", ErrSetupDone
		}
		return Principal{}, "", ErrWrongSetupCode
	}

	user, err := s.createUser(ctx, username, password, timezone, true)
	if err != nil {
		return Principal{}, "", err
	}
	s.setup.mu.Lock()
	s.setup.code = ""
	s.setup.mu.Unlock()

	token, expires, err := s.sessions.Create(ctx, user.ID, userAgent)
	if err != nil {
		return Principal{}, "", err
	}
	return Principal{UserID: user.ID, Username: user.Username, SessionHash: hashToken(token), ExpiresAt: expires}, token, nil
}

func (s *Service) setupCodeMatches(code string) bool {
	s.setup.mu.Lock()
	want := s.setup.code
	s.setup.mu.Unlock()
	got := normalizeSetupCode(code)
	return want != "" && subtle.ConstantTimeCompare([]byte(got), []byte(want)) == 1
}

// newSetupCode returns a random code formatted as two groups, e.g. "K7Q2M-X9PB4".
func newSetupCode() (string, error) {
	raw := make([]byte, setupCodeLen)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate setup code: %w", err)
	}
	var b strings.Builder
	for i, v := range raw {
		if i == setupCodeLen/2 {
			b.WriteByte('-')
		}
		b.WriteByte(setupAlphabet[v&31]) // 256 is a multiple of 32, so this is unbiased
	}
	return b.String(), nil
}

// normalizeSetupCode ignores case, spaces and dashes, and reads the Crockford
// lookalikes (O for 0, I and L for 1) the way the alphabet intends.
func normalizeSetupCode(code string) string {
	return strings.Map(func(r rune) rune {
		switch r {
		case ' ', '\t', '-':
			return -1
		case 'o', 'O':
			return '0'
		case 'i', 'I', 'l', 'L':
			return '1'
		}
		if 'a' <= r && r <= 'z' {
			return r - 'a' + 'A'
		}
		return r
	}, strings.TrimSpace(code))
}
