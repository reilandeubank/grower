// Package species looks up care data for plant species from online providers
// (Open Plantbook and Perenual). The app keeps its own editable copy of what it
// imports, so providers are only called on demand and every one is optional.
package species

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/reilandeubank/grower/internal/buildinfo"
)

// Candidate is one search result: enough to show it and import it later.
type Candidate struct {
	Source         string // "openplantbook" or "perenual"
	Ref            string // the provider's own id
	ScientificName string
	CommonName     string
}

// Care is a species' care defaults as mapped from a provider. Nil fields are unknown.
type Care struct {
	ScientificName    string
	CommonName        string
	WaterIntervalDays *int
	Light             *string // low, medium, bright_indirect or direct
	WaterAt           *int    // 1–9 on the moisture meter's 1–10 scale
	Raw               json.RawMessage
}

// Provider is an online source of species data.
type Provider interface {
	Source() string // the id stored in species.source
	Name() string   // for people, e.g. "Open Plantbook"
	Search(ctx context.Context, q string) ([]Candidate, error)
	Details(ctx context.Context, ref string) (Care, error)
}

// ErrDailyLimit means a provider's own call budget for today is spent; no call was made.
var ErrDailyLimit = errors.New("daily call budget used up")

// StatusError is a non-2xx answer from a provider.
type StatusError struct {
	Provider string
	Code     int
}

func (e *StatusError) Error() string {
	return fmt.Sprintf("%s answered HTTP %d", e.Provider, e.Code)
}

// requestTimeout bounds each provider call so a slow provider can't hold up a lookup.
const requestTimeout = 10 * time.Second

// NewHTTPClient is the client the adapters use by default.
func NewHTTPClient() *http.Client { return &http.Client{Timeout: requestTimeout} }

// maxBody bounds a provider response; details payloads are a few kilobytes.
const maxBody = 1 << 20

// do sends req and returns the body of a 2xx response. Transport errors drop the URL,
// because Perenual's carries the API key in its query string.
func do(client *http.Client, req *http.Request, provider string) ([]byte, error) {
	req.Header.Set("User-Agent", "grower/"+buildinfo.Version)
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req) //nolint:gosec // G704: adapters build URLs from their own base URL; user input only fills the query or body
	if err != nil {
		var uerr *url.Error
		if errors.As(err, &uerr) {
			err = uerr.Err
		}
		return nil, fmt.Errorf("%s %s: %w", provider, req.URL.Path, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, &StatusError{Provider: provider, Code: resp.StatusCode}
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return nil, fmt.Errorf("%s %s: read body: %w", provider, req.URL.Path, err)
	}
	return body, nil
}

// getJSON decodes a 2xx body into out and returns the raw bytes too.
func getJSON(client *http.Client, req *http.Request, provider string, out any) ([]byte, error) {
	body, err := do(client, req, provider)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(body, out); err != nil {
		return nil, fmt.Errorf("%s %s: decode: %w", provider, req.URL.Path, err)
	}
	return body, nil
}

// dailyCount counts calls per UTC day, the period providers' free quotas reset on.
type dailyCount struct {
	mu    sync.Mutex
	day   string
	calls int
}

// spend counts a call against limit for now's UTC day, reporting whether one was left.
func (c *dailyCount) spend(now time.Time, limit int) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if day := now.UTC().Format(time.DateOnly); day != c.day {
		c.day, c.calls = day, 0
	}
	if c.calls >= limit {
		return false
	}
	c.calls++
	return true
}
