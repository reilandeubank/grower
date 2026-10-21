package species

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// plantnetResults follows the documented response (see plantnet.go), trimmed. The
// third result has no common names; the fourth no name at all.
const plantnetResults = `{"query":{"project":"all","images":["abc"],"organs":["leaf"],"includeRelatedImages":false,"noReject":false},
	"predictedOrgans":[{"image":"abc","filename":"photo.jpg","organ":"leaf","score":0.92}],
	"language":"en","preferedReferential":"k-world-flora","bestMatch":"Monstera deliciosa Liebm.",
	"results":[
	 {"score":0.91504,"species":{"scientificNameWithoutAuthor":"Monstera deliciosa","scientificNameAuthorship":"Liebm.",
	  "genus":{"scientificNameWithoutAuthor":"Monstera","scientificNameAuthorship":"","scientificName":"Monstera"},
	  "family":{"scientificNameWithoutAuthor":"Araceae","scientificNameAuthorship":"","scientificName":"Araceae"},
	  "commonNames":["Swiss cheese plant","Ceriman"],"scientificName":"Monstera deliciosa Liebm."},"gbif":{"id":"2868241"},"powo":{"id":"87430-1"}},
	 {"score":0.03712,"species":{"scientificNameWithoutAuthor":"Monstera adansonii","commonNames":["Adanson's monstera"]}},
	 {"score":0.01,"species":{"scientificNameWithoutAuthor":"Rhaphidophora tetrasperma","commonNames":[]}},
	 {"score":0.005,"species":{"scientificNameWithoutAuthor":""}}],
	"version":"2025-01-17 (7.3)","remainingIdentificationRequests":498}`

func TestPlantNetIdentify(t *testing.T) {
	var calls atomic.Int32
	status := http.StatusOK
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		q := r.URL.Query()
		if r.Method != http.MethodPost || r.URL.Path != "/v2/identify/all" || q.Get("api-key") != "k3y" || q.Get("nb-results") != "5" {
			t.Errorf("request %s %s", r.Method, r.URL.Path)
		}
		f, h, err := r.FormFile("images")
		if err != nil {
			t.Fatalf("images: %v", err)
		}
		b, _ := io.ReadAll(f)
		if string(b) != "jpeg bytes" || h.Header.Get("Content-Type") != "image/jpeg" || r.FormValue("organs") != "leaf" {
			t.Errorf("form: %q %q %q", b, h.Header.Get("Content-Type"), r.FormValue("organs"))
		}
		w.WriteHeader(status)
		switch status {
		case http.StatusOK:
			_, _ = w.Write([]byte(plantnetResults))
		case http.StatusNotFound:
			_, _ = w.Write([]byte(`{"statusCode":404,"error":"Not Found","message":"Species not found"}`))
		}
	}))
	defer srv.Close()
	clk := &clock{t: time.Date(2026, 10, 21, 23, 0, 0, 0, time.UTC)}
	p := NewPlantNet("k3y")
	p.BaseURL, p.Now, p.Budget = srv.URL, clk.Now, 4
	ctx := context.Background()
	identify := func() ([]Identification, error) { return p.Identify(ctx, []byte("jpeg bytes"), "image/jpeg", "leaf") }

	got, err := identify()
	if err != nil {
		t.Fatal(err)
	}
	want := []Identification{
		{ScientificName: "Monstera deliciosa", CommonName: "Swiss cheese plant", Score: 0.91504},
		{ScientificName: "Monstera adansonii", CommonName: "Adanson's monstera", Score: 0.03712},
		{ScientificName: "Rhaphidophora tetrasperma", Score: 0.01},
	}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] || got[2] != want[2] {
		t.Errorf("identify = %+v", got)
	}

	status = http.StatusNotFound
	if _, err := identify(); !errors.Is(err, ErrNoMatch) {
		t.Errorf("404 err = %v", err)
	}
	status = http.StatusTooManyRequests
	if _, err := identify(); !errors.Is(err, ErrDailyLimit) {
		t.Errorf("429 err = %v", err)
	}
	status = http.StatusInternalServerError
	var se *StatusError
	if _, err := identify(); !errors.As(err, &se) || se.Code != http.StatusInternalServerError {
		t.Errorf("500 err = %v", err)
	}

	// The budget (4) is spent: no call until the next UTC day.
	if _, err := identify(); !errors.Is(err, ErrDailyLimit) || calls.Load() != 4 {
		t.Errorf("over budget: err %v after %d calls", err, calls.Load())
	}
	clk.t = clk.t.Add(time.Hour)
	status = http.StatusOK
	if _, err := identify(); err != nil || calls.Load() != 5 {
		t.Errorf("next day: err %v after %d calls", err, calls.Load())
	}

	// Transport errors must not leak the key from the URL.
	srv.Close()
	if _, err := identify(); err == nil || strings.Contains(err.Error(), "k3y") {
		t.Errorf("closed server err = %v", err)
	}
}
