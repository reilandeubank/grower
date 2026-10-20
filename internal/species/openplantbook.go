package species

// Open Plantbook adapter. Shapes relied on, from the live OpenAPI schema at
// https://open.plantbook.io/api/schema/ (API version 202606, checked 2026-10-19):
//
//   - POST /api/v1/token/, form-encoded grant_type=client_credentials, client_id,
//     client_secret -> {"access_token", "expires_in" (seconds, e.g. 86400), "token_type": "Bearer"}.
//     Requests then send "Authorization: Bearer <access_token>". (A permanent
//     "Authorization: Token <key>" also exists but only "when API-key authentication is
//     enabled", so client credentials are used: they work on every endpoint.)
//   - GET /api/v1/plant/search?alias=<q>&limit=<n> (alias: at least 3 characters, matched
//     against display_pid, alias and common names) -> {"count", "next", "previous",
//     "results": [{"pid", "display_pid", "alias" (nullable), "category"}]}.
//   - GET /api/v1/plant/detail/{pid}/ -> {"pid", "display_pid", "alias", "min_light_lux",
//     "max_light_lux", "min_soil_moist", "max_soil_moist", ..., "common_names":
//     [{"name", "language_code"}]}. Numeric fields are nullable integers; soil moisture
//     is a percentage. pid is the lower-cased display_pid and may contain spaces.

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// OpenPlantbook searches open.plantbook.io with OAuth2 client credentials.
type OpenPlantbook struct {
	ClientID, ClientSecret string
	BaseURL                string // without a trailing slash
	Client                 *http.Client
	Now                    func() time.Time

	mu      sync.Mutex
	token   string
	expires time.Time
}

// NewOpenPlantbook returns the adapter for the public service.
func NewOpenPlantbook(clientID, clientSecret string) *OpenPlantbook {
	return &OpenPlantbook{
		ClientID: clientID, ClientSecret: clientSecret,
		BaseURL: "https://open.plantbook.io", Client: NewHTTPClient(), Now: time.Now,
	}
}

func (p *OpenPlantbook) Source() string { return "openplantbook" }
func (p *OpenPlantbook) Name() string   { return "Open Plantbook" }

// plantbookSearchLimit keeps a lookup's list short enough to scan on a phone.
const plantbookSearchLimit = 20

func (p *OpenPlantbook) Search(ctx context.Context, q string) ([]Candidate, error) {
	// The API rejects shorter searches with a 400; there is nothing to find.
	if len([]rune(strings.TrimSpace(q))) < 3 {
		return []Candidate{}, nil
	}
	var res struct {
		Results []struct {
			PID        string  `json:"pid"`
			DisplayPID string  `json:"display_pid"`
			Alias      *string `json:"alias"`
		} `json:"results"`
	}
	query := url.Values{"alias": {q}, "limit": {fmt.Sprint(plantbookSearchLimit)}}
	if _, err := p.get(ctx, "/api/v1/plant/search?"+query.Encode(), &res); err != nil {
		return nil, err
	}
	out := make([]Candidate, 0, len(res.Results))
	for _, r := range res.Results {
		if r.PID == "" || r.DisplayPID == "" {
			continue
		}
		out = append(out, Candidate{Source: p.Source(), Ref: r.PID, ScientificName: r.DisplayPID, CommonName: deref(r.Alias)})
	}
	return out, nil
}

func (p *OpenPlantbook) Details(ctx context.Context, ref string) (Care, error) {
	var d struct {
		DisplayPID   string  `json:"display_pid"`
		Alias        *string `json:"alias"`
		MaxLightLux  *int64  `json:"max_light_lux"`
		MinSoilMoist *int64  `json:"min_soil_moist"`
		CommonNames  []struct {
			Name         string `json:"name"`
			LanguageCode string `json:"language_code"`
		} `json:"common_names"`
	}
	raw, err := p.get(ctx, "/api/v1/plant/detail/"+url.PathEscape(ref)+"/", &d)
	if err != nil {
		return Care{}, err
	}
	common := deref(d.Alias)
	for _, n := range d.CommonNames {
		if common == "" && strings.HasPrefix(strings.ToLower(n.LanguageCode), "en") {
			common = n.Name
		}
	}
	// Open Plantbook has moisture ranges but no watering frequency.
	return Care{
		ScientificName: d.DisplayPID, CommonName: common,
		Light: plantbookLight(d.MaxLightLux), WaterAt: plantbookWaterAt(d.MinSoilMoist), Raw: raw,
	}, nil
}

// get calls an API path with a bearer token. A 401 drops the cached token so the next
// call fetches a fresh one (it may have been revoked early).
func (p *OpenPlantbook) get(ctx context.Context, path string, out any) ([]byte, error) {
	token, err := p.accessToken(ctx)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.BaseURL+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	raw, err := getJSON(p.Client, req, p.Name(), out)
	if se := (*StatusError)(nil); errors.As(err, &se) && se.Code == http.StatusUnauthorized {
		p.mu.Lock()
		p.token = ""
		p.mu.Unlock()
	}
	return raw, err
}

// tokenMargin renews a token a little before it expires, so it can't lapse mid-request.
const tokenMargin = time.Minute

// accessToken returns the cached token, fetching a new one when it is missing or
// about to expire. The lock is held while fetching so concurrent calls share one.
func (p *OpenPlantbook) accessToken(ctx context.Context) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.token != "" && p.Now().Before(p.expires) {
		return p.token, nil
	}
	form := url.Values{"grant_type": {"client_credentials"}, "client_id": {p.ClientID}, "client_secret": {p.ClientSecret}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.BaseURL+"/api/v1/token/", strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	var tok struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int64  `json:"expires_in"`
	}
	if _, err := getJSON(p.Client, req, p.Name(), &tok); err != nil {
		return "", err
	}
	if tok.AccessToken == "" {
		return "", fmt.Errorf("%s token: no access_token in response", p.Name())
	}
	p.token = tok.AccessToken
	p.expires = p.Now().Add(time.Duration(tok.ExpiresIn)*time.Second - tokenMargin)
	return p.token, nil
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
