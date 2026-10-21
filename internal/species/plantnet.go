package species

// Pl@ntNet adapter: plant identification from a photo, not care data. Shapes relied on,
// from https://my.plantnet.org/doc/api/identify and /doc/api/quota (checked 2026-10-21):
//
//   - POST https://my-api.plantnet.org/v2/identify/all?api-key=<key>&nb-results=5&lang=en,
//     multipart/form-data with one or more "images" (JPEG or PNG, at most 5, 50 MB in
//     all) and one "organs" value per image: leaf, flower, fruit, bark or auto.
//   - 200 -> {"query": {...}, "bestMatch": "Ajuga genevensis L.", "results": [{"score":
//     0.90734, "species": {"scientificNameWithoutAuthor": "Ajuga genevensis",
//     "scientificNameAuthorship": "L.", "scientificName": "Ajuga genevensis L.", "genus":
//     {...}, "family": {...}, "commonNames": ["Blue bugle", ...]}, "gbif": {"id": ...},
//     "powo": {"id": ...}}], "version": "...", "remainingIdentificationRequests": 498},
//     results best first.
//   - 404 {"statusCode": 404, "error": "Not Found", "message": "Species not found"} when
//     nothing matches; 429 with Retry-After when the daily quota is spent. Quotas reset
//     at 00:00 UTC.
//
// The key travels in the query string, so URLs are never logged (see do). Pl@ntNet's
// terms say submitted images are not kept; grower doesn't keep them either.

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"strings"
	"time"
)

// PlantNetDailyBudget stays under the free tier's 500 identifications a day.
const PlantNetDailyBudget = 450

// ErrNoMatch means Pl@ntNet found no species in the photo.
var ErrNoMatch = errors.New("no species matched")

// Identification is one suggested species, best first.
type Identification struct {
	ScientificName string
	CommonName     string
	Score          float64 // 0 to 1
}

// PlantNet identifies plants from photos with an API key, within a daily call budget.
type PlantNet struct {
	Key     string
	BaseURL string // without a trailing slash
	Client  *http.Client
	Now     func() time.Time
	Budget  int // calls per UTC day

	spent dailyCount
}

// NewPlantNet returns the adapter for the public service. Identification takes longer
// than a search, so its timeout is longer too.
func NewPlantNet(key string) *PlantNet {
	return &PlantNet{
		Key: key, BaseURL: "https://my-api.plantnet.org", Client: &http.Client{Timeout: 30 * time.Second},
		Now: time.Now, Budget: PlantNetDailyBudget,
	}
}

// maxIdentifications is how many suggestions are asked for and returned.
const maxIdentifications = 5

// Identify sends one JPEG or PNG photo (mime is its type) showing organ (leaf, flower,
// fruit, bark or auto). It returns ErrDailyLimit without calling when the budget is
// spent, and ErrNoMatch when Pl@ntNet recognises nothing.
func (p *PlantNet) Identify(ctx context.Context, image []byte, mime, organ string) ([]Identification, error) {
	if !p.spent.spend(p.Now(), p.Budget) {
		return nil, ErrDailyLimit
	}
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	filename := "photo.jpg"
	if mime == "image/png" {
		filename = "photo.png"
	}
	h := textproto.MIMEHeader{}
	h.Set("Content-Disposition", `form-data; name="images"; filename="`+filename+`"`)
	h.Set("Content-Type", mime)
	part, err := mw.CreatePart(h)
	if err == nil {
		_, err = part.Write(image)
	}
	if err == nil {
		err = mw.WriteField("organs", organ)
	}
	if err == nil {
		err = mw.Close()
	}
	if err != nil {
		return nil, fmt.Errorf("plantnet: build request: %w", err)
	}

	query := url.Values{"api-key": {p.Key}, "nb-results": {fmt.Sprint(maxIdentifications)}, "lang": {"en"}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.BaseURL+"/v2/identify/all?"+query.Encode(), &body) //nolint:gosec // G704: the URL is fixed; only the body carries the user's photo
	if err != nil {
		return nil, fmt.Errorf("plantnet: %w", redactKey(err, p.Key))
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	var res struct {
		Results []struct {
			Score   float64 `json:"score"`
			Species struct {
				ScientificNameWithoutAuthor string   `json:"scientificNameWithoutAuthor"`
				CommonNames                 []string `json:"commonNames"`
			} `json:"species"`
		} `json:"results"`
	}
	_, err = getJSON(p.Client, req, "Pl@ntNet", &res)
	var se *StatusError
	switch {
	case errors.As(err, &se) && se.Code == http.StatusNotFound:
		return nil, ErrNoMatch
	case errors.As(err, &se) && se.Code == http.StatusTooManyRequests:
		return nil, ErrDailyLimit // the key's own quota, perhaps spent elsewhere
	case err != nil:
		return nil, err
	}
	out := []Identification{}
	for _, r := range res.Results {
		name := strings.TrimSpace(r.Species.ScientificNameWithoutAuthor)
		if name == "" || len(out) == maxIdentifications {
			continue
		}
		id := Identification{ScientificName: name, Score: min(max(r.Score, 0), 1)}
		for _, c := range r.Species.CommonNames {
			if c = strings.TrimSpace(c); c != "" {
				id.CommonName = c
				break
			}
		}
		out = append(out, id)
	}
	if len(out) == 0 {
		return nil, ErrNoMatch
	}
	return out, nil
}

// redactKey hides the API key in an error that may quote the request URL.
func redactKey(err error, key string) error {
	if key == "" || !strings.Contains(err.Error(), key) {
		return err
	}
	return errors.New(strings.ReplaceAll(err.Error(), key, "REDACTED"))
}
