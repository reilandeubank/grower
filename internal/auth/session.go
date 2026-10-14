package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/reilandeubank/grower/internal/db"
	"github.com/reilandeubank/grower/internal/db/dbgen"
)

const (
	// SessionTTL is the sliding lifetime: an installed PWA used at least once every
	// 90 days stays signed in.
	SessionTTL = 90 * 24 * time.Hour
	// sessionRefreshAfter limits session writes to about one per day per device.
	sessionRefreshAfter = 24 * time.Hour
	tokenBytes          = 32
	maxUserAgentLen     = 256
)

// ErrNoSession means the token is unknown or expired.
var ErrNoSession = errors.New("no valid session")

// Clock returns the current time; tests substitute a fake.
type Clock func() time.Time

// Principal is the signed-in user attached to a request.
type Principal struct {
	UserID   string
	Username string
	// SessionHash identifies the session (the stored SHA-256 of the cookie token).
	SessionHash string
	// ExpiresAt is the session expiry after any refresh.
	ExpiresAt time.Time
}

// Sessions creates, looks up and revokes server-side sessions.
type Sessions struct {
	db  *db.DB
	now Clock
}

// NewSessions returns a session store backed by d.
func NewSessions(d *db.DB, now Clock) *Sessions {
	return &Sessions{db: d, now: now}
}

// Create starts a session for userID and returns the cookie token and expiry.
func (s *Sessions) Create(ctx context.Context, userID, userAgent string) (string, time.Time, error) {
	raw := make([]byte, tokenBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", time.Time{}, fmt.Errorf("generate session token: %w", err)
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	now := s.now()
	expires := now.Add(SessionTTL)
	if len(userAgent) > maxUserAgentLen {
		userAgent = userAgent[:maxUserAgentLen]
	}
	err := s.db.Writer().CreateSession(ctx, dbgen.CreateSessionParams{
		IDHash:     hashToken(token),
		UserID:     userID,
		CreatedAt:  db.FormatTime(now),
		LastSeenAt: db.FormatTime(now),
		ExpiresAt:  db.FormatTime(expires),
		UserAgent:  userAgent,
	})
	if err != nil {
		return "", time.Time{}, fmt.Errorf("insert session: %w", err)
	}
	return token, expires, nil
}

// Lookup resolves a cookie token. When the session was last refreshed more than a day
// ago its expiry slides forward and refreshed is true, so the caller re-sends the cookie.
func (s *Sessions) Lookup(ctx context.Context, token string) (p Principal, refreshed bool, err error) {
	if token == "" {
		return Principal{}, false, ErrNoSession
	}
	idHash := hashToken(token)
	row, err := s.db.Reader().GetSessionWithUser(ctx, idHash)
	if errors.Is(err, sql.ErrNoRows) {
		return Principal{}, false, ErrNoSession
	}
	if err != nil {
		return Principal{}, false, fmt.Errorf("load session: %w", err)
	}
	expires, err := db.ParseTime(row.ExpiresAt)
	if err != nil {
		return Principal{}, false, err
	}
	lastSeen, err := db.ParseTime(row.LastSeenAt)
	if err != nil {
		return Principal{}, false, err
	}

	now := s.now()
	if !now.Before(expires) {
		if err := s.db.Writer().DeleteSession(ctx, idHash); err != nil {
			return Principal{}, false, fmt.Errorf("delete expired session: %w", err)
		}
		return Principal{}, false, ErrNoSession
	}

	p = Principal{UserID: row.UserID, Username: row.Username, SessionHash: idHash, ExpiresAt: expires}
	if now.Sub(lastSeen) >= sessionRefreshAfter {
		p.ExpiresAt = now.Add(SessionTTL)
		err := s.db.Writer().TouchSession(ctx, dbgen.TouchSessionParams{
			LastSeenAt: db.FormatTime(now),
			ExpiresAt:  db.FormatTime(p.ExpiresAt),
			IDHash:     idHash,
		})
		if err != nil {
			return Principal{}, false, fmt.Errorf("refresh session: %w", err)
		}
		refreshed = true
	}
	return p, refreshed, nil
}

// Revoke deletes one session by its stored hash. Unknown hashes are not an error.
func (s *Sessions) Revoke(ctx context.Context, sessionHash string) error {
	if err := s.db.Writer().DeleteSession(ctx, sessionHash); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

// RevokeAllForUser signs the user out everywhere.
func (s *Sessions) RevokeAllForUser(ctx context.Context, userID string) error {
	if err := s.db.Writer().DeleteUserSessions(ctx, userID); err != nil {
		return fmt.Errorf("delete user sessions: %w", err)
	}
	return nil
}

// DeleteExpired removes sessions past their expiry and returns how many went.
func (s *Sessions) DeleteExpired(ctx context.Context) (int64, error) {
	n, err := s.db.Writer().DeleteExpiredSessions(ctx, db.FormatTime(s.now()))
	if err != nil {
		return 0, fmt.Errorf("delete expired sessions: %w", err)
	}
	return n, nil
}

// hashToken is what the database stores, so a copy of the DB cannot be replayed as cookies.
func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
