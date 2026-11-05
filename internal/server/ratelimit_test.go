package server

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"time"

	"github.com/reilandeubank/grower/internal/species"
)

// tooMany checks resp is the 429 for a per-user limit, retrying after retryAfter.
func tooMany(t *testing.T, resp *http.Response, retryAfter, detail string) {
	t.Helper()
	if got := resp.Header.Get("Retry-After"); got != retryAfter {
		t.Errorf("Retry-After = %q, want %q", got, retryAfter)
	}
	if got := expectProblem(t, resp, http.StatusTooManyRequests); got != detail {
		t.Errorf("detail = %q, want %q", got, detail)
	}
}

// otherUser signs in a second user, whose limits are their own.
func (e *env) otherUser() *http.Cookie {
	e.t.Helper()
	if _, err := e.auth.CreateUser(context.Background(), "ivy", testPassword, "UTC"); err != nil {
		e.t.Fatal(err)
	}
	return sessionCookie(e.t, e.login("ivy", testPassword), e.cfg.SessionCookieName())
}

func TestSpeciesLookupRateLimit(t *testing.T) {
	const detail = "Too many species lookups. Try again in a few seconds."
	p := &fakeProvider{source: "perenual", name: "Perenual", candidates: []species.Candidate{{Source: "perenual", Ref: "1", ScientificName: "Ficus"}}}
	e := newEnv(t, envOptions{species: []species.Provider{p}})
	c := e.loginCookie()
	lookup := func(c *http.Cookie, q string) *http.Response {
		return e.call(c, http.MethodGet, "/api/v1/species/lookup?q="+url.QueryEscape(q), "")
	}
	for i := range 10 {
		expectJSON[any](t, lookup(c, fmt.Sprint("ficus", i)), http.StatusOK)
	}
	tooMany(t, lookup(c, "ficus new"), "1", detail)
	if n := p.searches.Load(); n != 10 {
		t.Errorf("provider searched %d times, want 10", n)
	}

	// Cached answers spend no provider call, so they don't count.
	expectJSON[any](t, lookup(c, "FICUS3"), http.StatusOK)
	// Import shares the limit.
	tooMany(t, e.call(c, http.MethodPost, "/api/v1/species/import", `{"source":"perenual","ref":"1"}`), "1", detail)
	// Every user has their own.
	expectJSON[any](t, lookup(e.otherUser(), "ficus new"), http.StatusOK)

	// A second later there's one more (ivy's search cached "ficus new", so use new queries).
	e.clock.t = e.clock.t.Add(time.Second)
	expectJSON[any](t, lookup(c, "ficus a"), http.StatusOK)
	tooMany(t, lookup(c, "ficus b"), "1", detail)
}

func TestIdentifyRateLimit(t *testing.T) {
	const detail = "Too many identifications. Try again in a few seconds."
	var calls atomic.Int32
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		_, _ = w.Write([]byte(`{"results":[{"score":0.9,"species":{"scientificNameWithoutAuthor":"Ficus lyrata","commonNames":[]}}]}`))
	}))
	defer fake.Close()
	pn := species.NewPlantNet("k")
	pn.BaseURL = fake.URL
	e := newEnv(t, envOptions{plantnet: pn})
	c := e.loginCookie()
	photo := testPNG(t, 10, 10, 1)
	for range 5 {
		expectJSON[any](t, e.identify(c, photo, nil), http.StatusOK)
	}
	tooMany(t, e.identify(c, photo, nil), "5", detail)
	// A request that can't be sent isn't limited: it gets its own error.
	expectProblem(t, e.identify(c, nil, nil), http.StatusBadRequest)
	if n := calls.Load(); n != 5 {
		t.Errorf("Pl@ntNet called %d times, want 5", n)
	}

	e.clock.t = e.clock.t.Add(5 * time.Second)
	expectJSON[any](t, e.identify(c, photo, nil), http.StatusOK)
	tooMany(t, e.identify(c, photo, nil), "5", detail)
}
