package species

import (
	"context"
	"strings"
	"sync"
	"time"
)

// Result is one provider's answer to a search.
type Result struct {
	Provider   Provider
	Candidates []Candidate
	Err        error
}

// Lookup searches every configured provider at once and caches their answers, so
// search-as-you-type and repeat searches don't spend provider calls (Perenual allows
// 100 a day).
type Lookup struct {
	providers []Provider
	now       func() time.Time

	mu    sync.Mutex
	cache map[string]cached
}

type cached struct {
	at         time.Time
	candidates []Candidate
}

const (
	cacheTTL  = 24 * time.Hour
	cacheSize = 200
)

// NewLookup searches the given providers, in that order. None is fine: the app works
// without species data.
func NewLookup(now func() time.Time, providers ...Provider) *Lookup {
	return &Lookup{providers: providers, now: now, cache: map[string]cached{}}
}

// Providers are the configured providers.
func (l *Lookup) Providers() []Provider { return l.providers }

// Provider returns the configured provider for a source, or nil.
func (l *Lookup) Provider(source string) Provider {
	for _, p := range l.providers {
		if p.Source() == source {
			return p
		}
	}
	return nil
}

// Cached reports whether every provider would answer q from the cache, so Search would
// call none of them.
func (l *Lookup) Cached(q string) bool {
	q = normalize(q)
	for _, p := range l.providers {
		if _, ok := l.get(cacheKey(p, q)); !ok {
			return false
		}
	}
	return true
}

// Search asks every provider concurrently and returns their results in provider
// order. Only successful answers are cached.
func (l *Lookup) Search(ctx context.Context, q string) []Result {
	q = normalize(q)
	results := make([]Result, len(l.providers))
	var wg sync.WaitGroup
	for i, p := range l.providers {
		results[i].Provider = p
		key := cacheKey(p, q)
		if c, ok := l.get(key); ok {
			results[i].Candidates = c
			continue
		}
		wg.Go(func() {
			c, err := p.Search(ctx, q)
			results[i].Candidates, results[i].Err = c, err
			if err == nil {
				l.put(key, c)
			}
		})
	}
	wg.Wait()
	return results
}

func normalize(q string) string { return strings.Join(strings.Fields(strings.ToLower(q)), " ") }

func cacheKey(p Provider, q string) string { return p.Source() + "\x00" + q }

func (l *Lookup) get(key string) ([]Candidate, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	c, ok := l.cache[key]
	if !ok || l.now().Sub(c.at) >= cacheTTL {
		return nil, false
	}
	return c.candidates, true
}

// put stores an answer. When full it drops expired entries, then the oldest one.
func (l *Lookup) put(key string, candidates []Candidate) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	if _, ok := l.cache[key]; !ok && len(l.cache) >= cacheSize {
		oldest := ""
		for k, c := range l.cache {
			if now.Sub(c.at) >= cacheTTL {
				delete(l.cache, k)
			} else if oldest == "" || c.at.Before(l.cache[oldest].at) {
				oldest = k
			}
		}
		if len(l.cache) >= cacheSize {
			delete(l.cache, oldest)
		}
	}
	l.cache[key] = cached{at: now, candidates: candidates}
}
