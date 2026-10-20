package species

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"
)

// fake is a provider answering every search with one candidate named after q.
type fake struct {
	source string
	err    error
	calls  atomic.Int32
}

func (f *fake) Source() string { return f.source }
func (f *fake) Name() string   { return f.source }
func (f *fake) Search(_ context.Context, q string) ([]Candidate, error) {
	f.calls.Add(1)
	if f.err != nil {
		return nil, f.err
	}
	return []Candidate{{Source: f.source, Ref: q, ScientificName: q}}, nil
}
func (f *fake) Details(context.Context, string) (Care, error) { return Care{}, nil }

func TestLookup(t *testing.T) {
	clk := &clock{t: time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)}
	a, b := &fake{source: "a"}, &fake{source: "b", err: errors.New("down")}
	l := NewLookup(clk.Now, a, b)
	ctx := context.Background()

	res := l.Search(ctx, "  Monstera   Deliciosa ")
	if len(res) != 2 || res[0].Provider != a || res[0].Err != nil || res[0].Candidates[0].Ref != "monstera deliciosa" ||
		res[1].Provider != b || res[1].Err == nil {
		t.Fatalf("results = %+v", res)
	}
	// Same normalized query: a's answer comes from the cache; b's failure was not cached.
	l.Search(ctx, "monstera deliciosa")
	if a.calls.Load() != 1 || b.calls.Load() != 2 {
		t.Errorf("calls a=%d b=%d", a.calls.Load(), b.calls.Load())
	}
	clk.t = clk.t.Add(cacheTTL)
	l.Search(ctx, "monstera deliciosa")
	if a.calls.Load() != 2 {
		t.Errorf("expired entry not refetched: %d calls", a.calls.Load())
	}

	if l.Provider("b") != b || l.Provider("c") != nil || len(l.Providers()) != 2 {
		t.Error("provider lookup")
	}
	if got := NewLookup(clk.Now).Search(ctx, "x"); len(got) != 0 {
		t.Errorf("no providers = %+v", got)
	}
}

func TestLookupCacheIsBounded(t *testing.T) {
	clk := &clock{t: time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)}
	a := &fake{source: "a"}
	l := NewLookup(clk.Now, a)
	for i := range cacheSize + 10 {
		clk.t = clk.t.Add(time.Second)
		l.Search(context.Background(), fmt.Sprint("q", i))
	}
	if len(l.cache) != cacheSize {
		t.Errorf("cache holds %d entries", len(l.cache))
	}
	// The oldest went first; the newest is still cached.
	l.Search(context.Background(), "q0")
	l.Search(context.Background(), fmt.Sprint("q", cacheSize+9))
	if a.calls.Load() != cacheSize+11 {
		t.Errorf("calls = %d", a.calls.Load())
	}
}
