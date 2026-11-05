package auth

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/reilandeubank/grower/internal/db"
	"github.com/reilandeubank/grower/internal/db/dbgen"
)

const (
	// TokenPrefix starts every API token secret, so a leaked one is recognisable
	// (secret scanners, log reviews) and can't be mistaken for a session cookie.
	TokenPrefix = "grower_"
	// ScopeRead lets a token read; it is the only scope handed out for now.
	ScopeRead = "read"
	// tokenTouchAfter limits last-used writes: Home Assistant polls every 30 s by default.
	tokenTouchAfter = time.Hour
)

// ErrNoToken means the API token is unknown or revoked.
var ErrNoToken = errors.New("no valid api token")

// APIToken is a token as listed in Settings. The secret is never part of it.
type APIToken struct {
	ID         string
	Label      string
	Scope      string
	CreatedAt  time.Time
	LastUsedAt *time.Time
}

// APITokens creates, looks up and revokes per-user API tokens: long-lived bearer
// credentials for clients that can't keep the session cookie, such as Home Assistant.
type APITokens struct {
	db  *db.DB
	now Clock
}

// NewAPITokens returns a token store backed by d.
func NewAPITokens(d *db.DB, now Clock) *APITokens {
	return &APITokens{db: d, now: now}
}

// Create makes a read-only token for userID and returns it with its secret. The
// secret can't be recovered later: only its SHA-256 is stored, as for sessions.
func (t *APITokens) Create(ctx context.Context, userID, label string) (APIToken, string, error) {
	raw := make([]byte, tokenBytes)
	if _, err := rand.Read(raw); err != nil {
		return APIToken{}, "", fmt.Errorf("generate api token: %w", err)
	}
	secret := TokenPrefix + base64.RawURLEncoding.EncodeToString(raw)
	tok := APIToken{ID: db.NewID(), Label: label, Scope: ScopeRead, CreatedAt: t.now().UTC().Truncate(time.Second)}
	err := t.db.Writer().CreateAPIToken(ctx, dbgen.CreateAPITokenParams{
		ID:         tok.ID,
		UserID:     userID,
		Label:      label,
		SecretHash: hashToken(secret),
		Scope:      tok.Scope,
		CreatedAt:  db.FormatTime(tok.CreatedAt),
	})
	if err != nil {
		return APIToken{}, "", fmt.Errorf("insert api token: %w", err)
	}
	return tok, secret, nil
}

// List returns userID's tokens, oldest first.
func (t *APITokens) List(ctx context.Context, userID string) ([]APIToken, error) {
	rows, err := t.db.Reader().ListUserAPITokens(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list api tokens: %w", err)
	}
	out := make([]APIToken, len(rows))
	for i, r := range rows {
		created, err := db.ParseTime(r.CreatedAt)
		if err != nil {
			return nil, err
		}
		out[i] = APIToken{ID: r.ID, Label: r.Label, Scope: r.Scope, CreatedAt: created}
		if r.LastUsedAt.Valid {
			used, err := db.ParseTime(r.LastUsedAt.String)
			if err != nil {
				return nil, err
			}
			out[i].LastUsedAt = &used
		}
	}
	return out, nil
}

// Lookup resolves a secret into its owner. The token's last use is recorded at most
// once an hour.
func (t *APITokens) Lookup(ctx context.Context, secret string) (Principal, error) {
	if !strings.HasPrefix(secret, TokenPrefix) {
		return Principal{}, ErrNoToken
	}
	row, err := t.db.Reader().GetAPITokenWithUser(ctx, hashToken(secret))
	if errors.Is(err, sql.ErrNoRows) {
		return Principal{}, ErrNoToken
	}
	if err != nil {
		return Principal{}, fmt.Errorf("load api token: %w", err)
	}
	now := t.now()
	stale := !row.LastUsedAt.Valid
	if !stale {
		used, err := db.ParseTime(row.LastUsedAt.String)
		if err != nil {
			return Principal{}, err
		}
		stale = now.Sub(used) >= tokenTouchAfter
	}
	if stale {
		if err := t.db.Writer().TouchAPIToken(ctx, dbgen.TouchAPITokenParams{LastUsedAt: sql.NullString{String: db.FormatTime(now), Valid: true}, ID: row.ID}); err != nil {
			return Principal{}, fmt.Errorf("record api token use: %w", err)
		}
	}
	return Principal{UserID: row.UserID, Username: row.Username, TokenID: row.ID, TokenScope: row.Scope}, nil
}

// Revoke deletes one of userID's tokens. found is false when it isn't theirs or
// doesn't exist, so one user can't learn another's token IDs.
func (t *APITokens) Revoke(ctx context.Context, userID, id string) (found bool, err error) {
	n, err := t.db.Writer().DeleteUserAPIToken(ctx, dbgen.DeleteUserAPITokenParams{ID: id, UserID: userID})
	if err != nil {
		return false, fmt.Errorf("delete api token: %w", err)
	}
	return n > 0, nil
}
