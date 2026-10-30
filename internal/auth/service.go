package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/reilandeubank/grower/internal/db"
)

var (
	// ErrInvalidCredentials covers both unknown users and wrong passwords, so the
	// response never reveals which usernames exist.
	ErrInvalidCredentials = errors.New("invalid username or password")
	// ErrRateLimited means too many login attempts; try again later.
	ErrRateLimited = errors.New("too many login attempts")
	// ErrWrongPassword means the current password given to ChangePassword is wrong.
	ErrWrongPassword = errors.New("wrong current password")
)

// maxConcurrentHashes caps memory: each argon2id run allocates HashParams.MemoryKiB.
const maxConcurrentHashes = 2

// Options configure a Service. Zero values select production defaults.
type Options struct {
	HashParams HashParams
	RateLimits RateLimits
	Now        Clock
}

// Service implements login and user management on top of the database.
type Service struct {
	db         *db.DB
	sessions   *Sessions
	limiter    *LoginLimiter
	hashParams HashParams
	now        Clock
	// dummyHash is verified against when the username is unknown so that response
	// time does not reveal whether an account exists.
	dummyHash string
	hashSlots chan struct{}
	setup     setupState
}

// NewService builds the auth service. It hashes once up front to prepare dummyHash.
func NewService(d *db.DB, opts Options) (*Service, error) {
	if opts.HashParams == (HashParams{}) {
		opts.HashParams = DefaultHashParams()
	}
	if opts.RateLimits == (RateLimits{}) {
		opts.RateLimits = DefaultRateLimits()
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	dummy, err := HashPassword("grower-dummy-password", opts.HashParams)
	if err != nil {
		return nil, err
	}
	return &Service{
		db:         d,
		sessions:   NewSessions(d, opts.Now),
		limiter:    NewLoginLimiter(opts.RateLimits, opts.Now),
		hashParams: opts.HashParams,
		now:        opts.Now,
		dummyHash:  dummy,
		hashSlots:  make(chan struct{}, maxConcurrentHashes),
	}, nil
}

// Sessions exposes the session store (used by the HTTP session middleware).
func (s *Service) Sessions() *Sessions { return s.sessions }

// Now returns the service clock's current time.
func (s *Service) Now() time.Time { return s.now() }

// Login checks credentials and starts a session, returning its cookie token.
func (s *Service) Login(ctx context.Context, username, password, clientIP, userAgent string) (Principal, string, error) {
	if !s.limiter.Allow(clientIP) {
		return Principal{}, "", ErrRateLimited
	}

	user, err := s.db.Reader().GetUserByUsername(ctx, username)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		_, _ = s.verify(ctx, password, s.dummyHash)
		return Principal{}, "", ErrInvalidCredentials
	case err != nil:
		return Principal{}, "", fmt.Errorf("load user: %w", err)
	}

	ok, err := s.verify(ctx, password, user.PasswordHash)
	if err != nil {
		return Principal{}, "", fmt.Errorf("verify password for user %s: %w", user.ID, err)
	}
	if !ok {
		return Principal{}, "", ErrInvalidCredentials
	}

	token, expires, err := s.sessions.Create(ctx, user.ID, userAgent)
	if err != nil {
		return Principal{}, "", err
	}
	// Opportunistic cleanup; logins are rare enough that this costs nothing.
	if _, err := s.sessions.DeleteExpired(ctx); err != nil {
		return Principal{}, "", err
	}
	return Principal{UserID: user.ID, Username: user.Username, SessionHash: hashToken(token), ExpiresAt: expires}, token, nil
}

// verify runs VerifyPassword while holding one of the limited hashing slots.
func (s *Service) verify(ctx context.Context, password, hash string) (bool, error) {
	select {
	case s.hashSlots <- struct{}{}:
		defer func() { <-s.hashSlots }()
	case <-ctx.Done():
		return false, ctx.Err()
	}
	return VerifyPassword(password, hash)
}
