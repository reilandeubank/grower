package species

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type clock struct{ t time.Time }

func (c *clock) Now() time.Time { return c.t }

// Fixtures follow the documented shapes (see the adapter comments), trimmed.
const (
	plantbookToken  = `{"access_token":"tok-1","expires_in":3600,"token_type":"Bearer","scope":"read"}`
	plantbookSearch = `{"count":2,"next":null,"previous":null,"results":[
		{"pid":"monstera deliciosa","display_pid":"Monstera deliciosa","alias":"Swiss cheese plant","category":"Araceae, Monstera"},
		{"pid":"monstera adansonii","display_pid":"Monstera adansonii","alias":null,"category":"Araceae, Monstera"}]}`
	plantbookDetail = `{"pid":"monstera deliciosa","display_pid":"Monstera deliciosa","alias":null,"category":"Araceae, Monstera",
		"max_light_mmol":2500,"min_light_mmol":1200,"max_light_lux":30000,"min_light_lux":5000,"max_temp":32,"min_temp":12,
		"min_temp_survival":null,"max_env_humid":80,"min_env_humid":20,"max_soil_moist":60,"min_soil_moist":15,
		"max_soil_ec":2000,"min_soil_ec":350,"origin":null,"image_url":"https://opb-img.plantbook.io/monstera%20deliciosa.jpg",
		"image_source":null,"image_license":null,"common_names":[{"name":"Fensterblatt","language_code":"de"},{"name":"Swiss cheese plant","language_code":"en"}]}`
	perenualSearch = `{"data":[
		{"id":2961,"common_name":"Swiss cheese plant","scientific_name":["Monstera deliciosa"],"other_name":["Ceriman"],"family":null,"genus":"Monstera","default_image":null},
		{"id":5257,"common_name":"Upgrade Plans To Premium/Supreme - https://perenual.com/subscription-api-pricing. I'm sorry","scientific_name":"Upgrade Plans To Premium/Supreme - https://perenual.com/subscription-api-pricing. I'm sorry","other_name":[]},
		{"id":7,"common_name":"no name","scientific_name":[]}],
		"to":3,"per_page":30,"current_page":1,"from":1,"last_page":1,"total":3}`
	perenualDetail = `{"id":2961,"common_name":"Swiss cheese plant","scientific_name":["Monstera deliciosa"],"other_name":[],
		"watering":"Average","watering_general_benchmark":{"value":"\"7-10\"","unit":"days"},
		"sunlight":["part shade","filtered shade"],"indoor":true}`
)

// plantbookServer fakes Open Plantbook, counting token requests.
func plantbookServer(t *testing.T, tokens *atomic.Int32) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.UserAgent(), "grower/") {
			t.Errorf("User-Agent = %q", r.UserAgent())
		}
		if r.URL.Path == "/api/v1/token/" {
			if r.Method != http.MethodPost || r.PostFormValue("grant_type") != "client_credentials" ||
				r.PostFormValue("client_id") != "id" || r.PostFormValue("client_secret") != "secret" {
				t.Errorf("token request: %s %v", r.Method, r.PostForm)
			}
			n := tokens.Add(1)
			_, _ = w.Write([]byte(strings.Replace(plantbookToken, "tok-1", fmt.Sprint("tok-", n), 1)))
			return
		}
		if r.Header.Get("Authorization") != fmt.Sprint("Bearer tok-", tokens.Load()) {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch {
		case r.URL.Path == "/api/v1/plant/search" && r.URL.Query().Get("alias") == "monstera":
			_, _ = w.Write([]byte(plantbookSearch))
		case r.URL.EscapedPath() == "/api/v1/plant/detail/monstera%20deliciosa/":
			_, _ = w.Write([]byte(plantbookDetail))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestOpenPlantbook(t *testing.T) {
	var tokens atomic.Int32
	srv := plantbookServer(t, &tokens)
	clk := &clock{t: time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)}
	p := NewOpenPlantbook("id", "secret")
	p.BaseURL, p.Now = srv.URL, clk.Now
	ctx := context.Background()

	got, err := p.Search(ctx, "monstera")
	if err != nil {
		t.Fatal(err)
	}
	want := []Candidate{
		{Source: "openplantbook", Ref: "monstera deliciosa", ScientificName: "Monstera deliciosa", CommonName: "Swiss cheese plant"},
		{Source: "openplantbook", Ref: "monstera adansonii", ScientificName: "Monstera adansonii"},
	}
	if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("search = %+v", got)
	}

	care, err := p.Details(ctx, "monstera deliciosa")
	if err != nil {
		t.Fatal(err)
	}
	if care.ScientificName != "Monstera deliciosa" || care.CommonName != "Swiss cheese plant" || care.WaterIntervalDays != nil ||
		text(care.Light) != "bright_indirect" || text(care.WaterAt) != "2" || !strings.Contains(string(care.Raw), `"min_soil_ec":350`) {
		t.Errorf("details = %+v", care)
	}
	if n := tokens.Load(); n != 1 {
		t.Errorf("token fetched %d times, want once (cached)", n)
	}

	// Close to expiry (3600 s minus the margin) a new token is fetched.
	clk.t = clk.t.Add(59*time.Minute + time.Second)
	if _, err := p.Search(ctx, "monstera"); err != nil || tokens.Load() != 2 {
		t.Errorf("after expiry: err %v, %d tokens", err, tokens.Load())
	}

	// Too short to search: no call at all.
	if got, err := p.Search(ctx, "mo"); err != nil || len(got) != 0 {
		t.Errorf("short search = %v, %v", got, err)
	}

	var se *StatusError
	if _, err := p.Details(ctx, "nope"); !errors.As(err, &se) || se.Code != http.StatusNotFound {
		t.Errorf("unknown pid err = %v", err)
	}
}

func TestOpenPlantbookDropsRejectedToken(t *testing.T) {
	var tokens atomic.Int32
	srv := plantbookServer(t, &tokens)
	p := NewOpenPlantbook("id", "secret")
	p.BaseURL = srv.URL
	p.token, p.expires = "stale", time.Now().Add(time.Hour)

	var se *StatusError
	if _, err := p.Search(context.Background(), "monstera"); !errors.As(err, &se) || se.Code != http.StatusUnauthorized {
		t.Fatalf("stale token err = %v", err)
	}
	if _, err := p.Search(context.Background(), "monstera"); err != nil || tokens.Load() != 1 {
		t.Errorf("retry: err %v, %d tokens", err, tokens.Load())
	}
}

func TestPerenual(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.URL.Query().Get("key") != "k3y" || !strings.HasPrefix(r.UserAgent(), "grower/") {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch r.URL.Path {
		case "/api/v2/species-list":
			if r.URL.Query().Get("q") != "monstera" {
				t.Errorf("q = %q", r.URL.Query().Get("q"))
			}
			_, _ = w.Write([]byte(perenualSearch))
		case "/api/v2/species/details/2961":
			_, _ = w.Write([]byte(perenualDetail))
		default:
			w.WriteHeader(http.StatusTooManyRequests)
		}
	}))
	defer srv.Close()
	clk := &clock{t: time.Date(2026, 10, 19, 23, 0, 0, 0, time.UTC)}
	p := NewPerenual("k3y")
	p.BaseURL, p.Now, p.Budget = srv.URL, clk.Now, 3
	ctx := context.Background()

	got, err := p.Search(ctx, "monstera")
	if err != nil {
		t.Fatal(err)
	}
	// The free tier's upsell row and the row without a name are dropped.
	if len(got) != 1 || got[0] != (Candidate{Source: "perenual", Ref: "2961", ScientificName: "Monstera deliciosa", CommonName: "Swiss cheese plant"}) {
		t.Errorf("search = %+v", got)
	}

	care, err := p.Details(ctx, "2961")
	if err != nil {
		t.Fatal(err)
	}
	if care.ScientificName != "Monstera deliciosa" || care.CommonName != "Swiss cheese plant" || text(care.WaterIntervalDays) != "9" ||
		text(care.Light) != "bright_indirect" || care.WaterAt != nil || !strings.Contains(string(care.Raw), `"indoor":true`) {
		t.Errorf("details = %+v", care)
	}

	var se *StatusError
	if _, err := p.Details(ctx, "1"); !errors.As(err, &se) || se.Code != http.StatusTooManyRequests {
		t.Errorf("non-2xx err = %v", err)
	}
	if _, err := p.Details(ctx, "../x"); err == nil || calls.Load() != 3 {
		t.Errorf("bad id: err %v after %d calls", err, calls.Load())
	}

	// The budget (3) is spent: no call is made until the next UTC day.
	if _, err := p.Search(ctx, "monstera"); !errors.Is(err, ErrDailyLimit) || calls.Load() != 3 {
		t.Errorf("over budget: err %v after %d calls", err, calls.Load())
	}
	clk.t = clk.t.Add(time.Hour)
	if _, err := p.Search(ctx, "monstera"); err != nil || calls.Load() != 4 {
		t.Errorf("next day: err %v after %d calls", err, calls.Load())
	}

	// Transport errors must not leak the key from the URL.
	srv.Close()
	if _, err := p.Search(ctx, "monstera"); err == nil || strings.Contains(err.Error(), "k3y") {
		t.Errorf("closed server err = %v", err)
	}
}

func TestPerenualLooseFields(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"id":5257,"common_name":"Upgrade Plans","scientific_name":"Upgrade Plans",
			"watering":"Upgrade Plans","watering_general_benchmark":"Upgrade Plans","sunlight":"Upgrade Plans"}`))
	}))
	defer srv.Close()
	p := NewPerenual("k")
	p.BaseURL = srv.URL
	care, err := p.Details(context.Background(), "5257")
	if err != nil || care.WaterIntervalDays != nil || care.Light != nil {
		t.Errorf("details = %+v, %v", care, err)
	}

	srv2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"scientific_name":["Ficus elastica"],"watering":"Minimum","watering_general_benchmark":{"value":2,"unit":"weeks"},"sunlight":["full shade"]}`))
	}))
	defer srv2.Close()
	p.BaseURL = srv2.URL
	care, err = p.Details(context.Background(), "1")
	if err != nil || text(care.WaterIntervalDays) != "14" || text(care.Light) != "low" {
		t.Errorf("numeric benchmark details = %+v, %v", care, err)
	}
}
