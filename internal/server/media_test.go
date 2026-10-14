package server

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestMediaHandler(t *testing.T) {
	e := newEnv(t, envOptions{spa: spaFS()})
	c := e.loginCookie()
	p := e.newPlant(c, "Monty")
	ph := e.newPhoto(c, p.Id, testPNG(t, 300, 200, 1), "2026-04-01T00:00:00Z")

	expectProblem(t, e.do(request{path: ph.Urls.Small}), http.StatusUnauthorized)

	for url, wantType := range map[string]string{ph.Urls.Small: "image/jpeg", ph.Urls.Medium: "image/jpeg", ph.Urls.Original: "image/png"} {
		resp := e.do(request{path: url, cookie: c})
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("%s: status %d", url, resp.StatusCode)
		}
		h := resp.Header
		if h.Get("Content-Type") != wantType || h.Get("Cache-Control") != "private, max-age=31536000, immutable" ||
			h.Get("X-Content-Type-Options") != "nosniff" || h.Get("Last-Modified") == "" {
			t.Errorf("%s: headers %v", url, h)
		}
		// Conditional requests are answered without the body.
		again := e.do(request{path: url, cookie: c, headers: map[string]string{"If-Modified-Since": h.Get("Last-Modified")}})
		if again.StatusCode != http.StatusNotModified {
			t.Errorf("%s: conditional status %d", url, again.StatusCode)
		}
	}

	base := "/media/photos/" + ph.Id
	for _, path := range []string{
		base + "/large",
		base + "/original.png",
		"/media/photos/01JABCDEFGHJKMNPQRSTVWXYZ0/small", // unknown id
		"/media/photos/" + strings.ToLower(ph.Id) + "/small",
		"/media/photos/..%2F..%2Fgrower.db/small",
		"/media/photos/%2e%2e/small",
		"/media/photos/" + ph.Id + "%2F..%2F..%2F..%2Fgrower.db/small",
		"/media/photos/" + ph.Id + "/small/extra",
		"/media/elsewhere", // not the SPA
		"/media/",
	} {
		t.Run(path, func(t *testing.T) {
			resp := e.do(request{path: path, cookie: c})
			if resp.StatusCode != http.StatusNotFound {
				t.Fatalf("status = %d, want 404", resp.StatusCode)
			}
			if b, _ := io.ReadAll(resp.Body); strings.Contains(string(b), "doctype") || strings.Contains(string(b), "SQLite") {
				t.Errorf("leaked body: %.60q", b)
			}
		})
	}
	// Dot segments are cleaned by the router, which redirects rather than serving.
	if resp := e.do(request{path: base + "/../../../grower.db", cookie: c}); resp.StatusCode == http.StatusOK {
		t.Errorf("dot-segment path served: %d", resp.StatusCode)
	}
}

func TestUploadCSRF(t *testing.T) {
	e := newEnv(t, envOptions{})
	c := e.loginCookie()
	p := e.newPlant(c, "Monty")
	body, multipartType := multipartBody(t, testPNG(t, 2, 2, 0), nil)
	upload := "/api/v1/plants/" + p.Id + "/photos"
	tests := []struct {
		name, path, contentType string
		headers                 map[string]string
		status                  int
	}{
		{"multipart upload", upload, multipartType, nil, http.StatusCreated},
		{"multipart upload, foreign origin", upload, multipartType, map[string]string{"Origin": "https://evil.example"}, http.StatusForbidden},
		{"json upload", upload, "application/json", nil, http.StatusUnsupportedMediaType},
		{"urlencoded upload", upload, "application/x-www-form-urlencoded", nil, http.StatusUnsupportedMediaType},
		{"multipart elsewhere", "/api/v1/plants", multipartType, nil, http.StatusUnsupportedMediaType},
		{"multipart on a photo update", "/api/v1/photos/x", multipartType, nil, http.StatusUnsupportedMediaType},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			headers := map[string]string{"Content-Type": tt.contentType}
			for k, v := range tt.headers {
				headers[k] = v
			}
			method := http.MethodPost
			if strings.HasPrefix(tt.path, "/api/v1/photos/") {
				method = http.MethodPut
			}
			resp := e.do(request{method: method, path: tt.path, body: body, cookie: c, headers: headers})
			if tt.status == http.StatusCreated {
				if resp.StatusCode != tt.status {
					t.Fatalf("status = %d", resp.StatusCode)
				}
				return
			}
			expectProblem(t, resp, tt.status)
		})
	}
}
