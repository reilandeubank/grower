package server

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/reilandeubank/grower/internal/api"
	"github.com/reilandeubank/grower/internal/apigen"
	"github.com/reilandeubank/grower/internal/species"
)

func (e *env) identify(c *http.Cookie, file []byte, fields map[string]string) *http.Response {
	e.t.Helper()
	body, ct := multipartBody(e.t, file, fields)
	return e.do(request{method: http.MethodPost, path: "/api/v1/identify", body: body, cookie: c, headers: map[string]string{"Content-Type": ct}})
}

func TestIdentifyNotConfigured(t *testing.T) {
	e := newEnv(t, envOptions{})
	resp := e.identify(e.loginCookie(), testPNG(t, 10, 10, 1), nil)
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK || strings.TrimSpace(string(b)) != `{"candidates":[],"configured":false,"detail":""}` {
		t.Errorf("%d %s", resp.StatusCode, b)
	}
}

func TestIdentify(t *testing.T) {
	var calls atomic.Int32
	status, organ := http.StatusOK, ""
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		f, h, err := r.FormFile("images")
		if err != nil {
			t.Errorf("images: %v", err)
			return
		}
		b, _ := io.ReadAll(f)
		if !bytes.HasPrefix(b, []byte("\x89PNG")) || h.Header.Get("Content-Type") != "image/png" || r.URL.Query().Get("api-key") != "k" {
			t.Errorf("forwarded %d bytes as %q", len(b), h.Header.Get("Content-Type"))
		}
		organ = r.FormValue("organs")
		w.WriteHeader(status)
		if status == http.StatusOK {
			_, _ = w.Write([]byte(`{"bestMatch":"Monstera deliciosa Liebm.","results":[
				{"score":0.88,"species":{"scientificNameWithoutAuthor":"Monstera deliciosa","scientificNameAuthorship":"Liebm.","commonNames":["Swiss cheese plant"]}},
				{"score":0.05,"species":{"scientificNameWithoutAuthor":"Monstera adansonii","commonNames":[]}}],
				"remainingIdentificationRequests":499}`))
		}
	}))
	defer fake.Close()
	pn := species.NewPlantNet("k")
	pn.BaseURL, pn.Budget = fake.URL, 3
	e := newEnv(t, envOptions{plantnet: pn})
	c := e.loginCookie()
	saved := e.newSpecies(c, `{"scientific_name":"monstera deliciosa"}`)
	photo := testPNG(t, 40, 30, 1)

	got := expectJSON[apigen.Identification](t, e.identify(c, photo, map[string]string{"organ": "leaf"}), http.StatusOK)
	if !got.Configured || got.Detail != "" || len(got.Candidates) != 2 || organ != "leaf" {
		t.Fatalf("identification = %+v", got)
	}
	if c0 := got.Candidates[0]; c0.ScientificName != "Monstera deliciosa" || c0.CommonName != "Swiss cheese plant" || c0.Score != 0.88 ||
		c0.SavedId == nil || *c0.SavedId != saved.Id {
		t.Errorf("first = %+v", c0)
	}
	if c1 := got.Candidates[1]; c1.CommonName != "" || c1.SavedId != nil {
		t.Errorf("second = %+v", c1)
	}

	status = http.StatusNotFound
	resp := e.identify(c, photo, nil) // organ defaults to auto
	if organ != "auto" {
		t.Errorf("organ = %q, want auto", organ)
	}
	if got := expectJSON[apigen.Identification](t, resp, http.StatusOK); got.Detail != "No match found. Try a clearer photo of a leaf or flower." || len(got.Candidates) != 0 {
		t.Errorf("no match = %+v", got)
	}
	status = http.StatusBadGateway
	if d := expectProblem(t, e.identify(c, photo, nil), http.StatusBadGateway); d != "Couldn't reach Pl@ntNet. Try again later." {
		t.Errorf("failure detail = %q", d)
	}
	// The budget (3) is spent: no call.
	got = expectJSON[apigen.Identification](t, e.identify(c, photo, nil), http.StatusOK)
	if got.Detail != "Plant identification's daily limit is used up. Try again tomorrow." || len(got.Candidates) != 0 || calls.Load() != 3 {
		t.Errorf("over budget = %+v after %d calls", got, calls.Load())
	}

	for _, tc := range []struct {
		name   string
		file   []byte
		fields map[string]string
		want   string
	}{
		{"no file", nil, nil, "Choose a photo to identify."},
		{"bad organ", photo, map[string]string{"organ": "root"}, "Organ must be auto, leaf, flower, fruit or bark."},
		{"not an image", []byte("GIF89a not really"), nil, "Only JPEG and PNG photos are supported."},
	} {
		if d := expectProblem(t, e.identify(c, tc.file, tc.fields), http.StatusBadRequest); d != tc.want {
			t.Errorf("%s: detail %q", tc.name, d)
		}
	}
	big := bytes.Repeat([]byte{0}, api.MaxUploadBytes+1)
	if d := expectProblem(t, e.identify(c, big, nil), http.StatusRequestEntityTooLarge); d != "Photos must be 15 MB or smaller." {
		t.Errorf("too large: %q", d)
	}
	if calls.Load() != 3 {
		t.Errorf("%d calls, want 3", calls.Load())
	}

	// CSRF: multipart is allowed here, but only same-origin; JSON is refused.
	body, ct := multipartBody(t, photo, nil)
	expectProblem(t, e.do(request{method: http.MethodPost, path: "/api/v1/identify", body: body, cookie: c, noOrigin: true,
		headers: map[string]string{"Content-Type": ct}}), http.StatusForbidden)
	expectProblem(t, e.do(request{method: http.MethodPost, path: "/api/v1/identify", body: body, cookie: c,
		headers: map[string]string{"Content-Type": ct, "Origin": "http://evil.test"}}), http.StatusForbidden)
	expectProblem(t, e.call(c, http.MethodPost, "/api/v1/identify", `{"organ":"leaf"}`), http.StatusUnsupportedMediaType)
	expectProblem(t, e.identify(nil, photo, nil), http.StatusUnauthorized)
}
