package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/reilandeubank/grower/internal/db"
	"github.com/reilandeubank/grower/internal/db/dbgen"
)

var (
	ErrUserExists       = errors.New("user already exists")
	ErrUserNotFound     = errors.New("user not found")
	ErrPasswordTooShort = fmt.Errorf("password must be at least %d characters", MinPasswordLength)
	ErrInvalidUsername  = errors.New("username must be 1-64 characters with no spaces or control characters")
)

// ValidateUsername matches the API's LoginRequest limits and keeps names printable.
func ValidateUsername(name string) error {
	if n := utf8.RuneCountInString(name); n < 1 || n > 64 {
		return ErrInvalidUsername
	}
	if strings.IndexFunc(name, func(r rune) bool { return unicode.IsSpace(r) || !unicode.IsPrint(r) }) >= 0 {
		return ErrInvalidUsername
	}
	return nil
}

// ValidatePassword enforces the minimum length (in characters, not bytes).
func ValidatePassword(password string) error {
	if utf8.RuneCountInString(password) < MinPasswordLength {
		return ErrPasswordTooShort
	}
	return nil
}

// CreateUser adds a user and their settings, with timezone as the initial IANA zone.
func (s *Service) CreateUser(ctx context.Context, username, password, timezone string) (dbgen.User, error) {
	if err := ValidateUsername(username); err != nil {
		return dbgen.User{}, err
	}
	if err := ValidatePassword(password); err != nil {
		return dbgen.User{}, err
	}
	hash, err := HashPassword(password, s.hashParams)
	if err != nil {
		return dbgen.User{}, err
	}
	now := db.FormatTime(s.now())
	user := dbgen.User{ID: db.NewID(), Username: username, PasswordHash: hash, CreatedAt: now, UpdatedAt: now}

	err = s.db.InTx(ctx, func(q *dbgen.Queries) error {
		if _, err := q.GetUserByUsername(ctx, username); err == nil {
			return ErrUserExists
		} else if !errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("check username: %w", err)
		}
		if err := q.CreateUser(ctx, dbgen.CreateUserParams(user)); err != nil {
			return fmt.Errorf("insert user: %w", err)
		}
		err := q.CreateSettings(ctx, dbgen.CreateSettingsParams{
			UserID: user.ID, Timezone: timezone, CreatedAt: now, UpdatedAt: now,
		})
		if err != nil {
			return fmt.Errorf("insert settings: %w", err)
		}
		return nil
	})
	if err != nil {
		return dbgen.User{}, err
	}
	return user, nil
}

// SetPassword replaces a user's password and signs them out everywhere.
func (s *Service) SetPassword(ctx context.Context, username, password string) error {
	if err := ValidatePassword(password); err != nil {
		return err
	}
	hash, err := HashPassword(password, s.hashParams)
	if err != nil {
		return err
	}
	return s.db.InTx(ctx, func(q *dbgen.Queries) error {
		user, err := q.GetUserByUsername(ctx, username)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrUserNotFound
		} else if err != nil {
			return fmt.Errorf("load user: %w", err)
		}
		_, err = q.UpdateUserPassword(ctx, dbgen.UpdateUserPasswordParams{
			PasswordHash: hash, UpdatedAt: db.FormatTime(s.now()), ID: user.ID,
		})
		if err != nil {
			return fmt.Errorf("update password: %w", err)
		}
		if err := q.DeleteUserSessions(ctx, user.ID); err != nil {
			return fmt.Errorf("revoke sessions: %w", err)
		}
		return nil
	})
}
