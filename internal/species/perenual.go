package species

// Perenual adapter. Shapes relied on, from https://perenual.com/docs/api (checked
// 2026-10-19):
//
//   - GET /api/v2/species-list?key=<key>&q=<q> -> {"data": [{"id", "common_name",
//     "scientific_name": ["Abies alba"], "other_name": [...], ...}], "to", "per_page",
//     "current_page", "from", "last_page", "total"}.
//   - GET /api/v2/species/details/{id}?key=<key> -> {"id", "common_name",
//     "scientific_name": [...], "watering": "Frequent" | "Average" | "Minimum" | "None",
//     "watering_general_benchmark": {"value": ..., "unit": "days"}, "sunlight": ["full sun",
//     "part shade"], ...}.
//
// Ambiguities, handled defensively: the docs print the benchmark value as a bare 5-7
// (not valid JSON), so it is accepted as a string (with or without extra quotes) or a
// number. The free tier is known (not documented) to answer some fields of premium
// species with an "Upgrade Plans ..." string instead of the usual type, so every field
// is decoded loosely, that notice counts as blank, and anything unexpected maps to
// "unknown". Error statuses for a bad key or a spent quota are not
// documented; any non-2xx is a StatusError.

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// PerenualDailyBudget stays under the free tier's 100 calls a day, leaving room for
// calls made elsewhere with the same key.
const PerenualDailyBudget = 90

// Perenual searches perenual.com with an API key, within a daily call budget.
type Perenual struct {
	Key     string
	BaseURL string // without a trailing slash
	Client  *http.Client
	Now     func() time.Time
	Budget  int // calls per UTC day

	spent dailyCount
}

// NewPerenual returns the adapter for the public service.
func NewPerenual(key string) *Perenual {
	return &Perenual{Key: key, BaseURL: "https://perenual.com", Client: NewHTTPClient(), Now: time.Now, Budget: PerenualDailyBudget}
}

func (p *Perenual) Source() string { return "perenual" }
func (p *Perenual) Name() string   { return "Perenual" }

// freeTierMaxID is the highest species id Perenual's free plan serves details for: "Access
// To Species Data(1-3000)" (https://perenual.com/subscription-api-pricing). Higher ids answer
// 429, so they are left out of search results rather than offered and then refused.
const freeTierMaxID = 3000

func (p *Perenual) Search(ctx context.Context, q string) ([]Candidate, error) {
	var res struct {
		Data []struct {
			ID             json.Number     `json:"id"`
			CommonName     json.RawMessage `json:"common_name"`
			ScientificName json.RawMessage `json:"scientific_name"`
		} `json:"data"`
	}
	if _, err := p.get(ctx, "/api/v2/species-list", url.Values{"q": {q}}, &res); err != nil {
		return nil, err
	}
	out := make([]Candidate, 0, len(res.Data))
	for _, r := range res.Data {
		name := firstString(r.ScientificName)
		id, err := strconv.ParseUint(r.ID.String(), 10, 64)
		if err != nil || id > freeTierMaxID || name == "" {
			continue
		}
		out = append(out, Candidate{Source: p.Source(), Ref: r.ID.String(), ScientificName: name, CommonName: firstString(r.CommonName)})
	}
	return out, nil
}

func (p *Perenual) Details(ctx context.Context, ref string) (Care, error) {
	if _, err := strconv.ParseUint(ref, 10, 64); err != nil {
		return Care{}, fmt.Errorf("perenual: species id %q is not a number", ref)
	}
	var d struct {
		CommonName     json.RawMessage `json:"common_name"`
		ScientificName json.RawMessage `json:"scientific_name"`
		Watering       json.RawMessage `json:"watering"`
		Benchmark      json.RawMessage `json:"watering_general_benchmark"`
		Sunlight       json.RawMessage `json:"sunlight"`
	}
	raw, err := p.get(ctx, "/api/v2/species/details/"+ref, nil, &d)
	if err != nil {
		return Care{}, err
	}
	var bench struct {
		Value json.RawMessage `json:"value"`
		Unit  string          `json:"unit"`
	}
	_ = json.Unmarshal(d.Benchmark, &bench) // not an object: no benchmark
	value := firstString(bench.Value)
	var n json.Number
	if value == "" && json.Unmarshal(bench.Value, &n) == nil {
		value = n.String()
	}
	return Care{
		ScientificName:    firstString(d.ScientificName),
		CommonName:        firstString(d.CommonName),
		WaterIntervalDays: perenualInterval(firstString(d.Watering), value, bench.Unit),
		Light:             perenualLight(stringsOf(d.Sunlight)),
		Raw:               raw,
	}, nil
}

// get spends one call of today's budget, then calls the API. A spent budget returns
// ErrDailyLimit without calling.
func (p *Perenual) get(ctx context.Context, path string, query url.Values, out any) ([]byte, error) {
	if !p.spent.spend(p.Now(), p.Budget) {
		return nil, ErrDailyLimit
	}
	if query == nil {
		query = url.Values{}
	}
	query.Set("key", p.Key)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.BaseURL+path+"?"+query.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("perenual %s: %w", path, err)
	}
	return getJSON(p.Client, req, p.Name(), out)
}

// stringsOf reads a JSON array of strings, or a single string as a list of one.
func stringsOf(raw json.RawMessage) []string {
	var list []string
	if json.Unmarshal(raw, &list) == nil {
		return list
	}
	var s string
	if json.Unmarshal(raw, &s) == nil && s != "" {
		return []string{s}
	}
	return nil
}

// firstString is the first non-blank string of stringsOf, trimmed. Free-tier answers
// for premium species put an "Upgrade Plans To Premium/Supreme ..." notice in text
// fields; that counts as blank.
func firstString(raw json.RawMessage) string {
	for _, s := range stringsOf(raw) {
		if s = strings.TrimSpace(s); s != "" && !strings.HasPrefix(s, "Upgrade Plans") {
			return s
		}
	}
	return ""
}
