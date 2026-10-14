package server

import (
	"io"
	"net/http"
	"strings"
	"testing"
	"testing/fstest"
)

func spaFS() fstest.MapFS {
	return fstest.MapFS{
		"index.html":           {Data: []byte("<!doctype html><title>grower</title>")},
		"sw.js":                {Data: []byte("// sw")},
		"manifest.webmanifest": {Data: []byte(`{"name":"grower"}`)},
		"assets/app-1a2b3c.js": {Data: []byte("console.log(1)")},
		"icons/icon-192.png":   {Data: []byte("png")},
		".gitkeep":             {Data: nil},
	}
}

func TestSPA(t *testing.T) {
	e := newEnv(t, envOptions{spa: spaFS()})
	tests := []struct {
		path, wantCache, wantType, wantBody string
		status                              int
	}{
		{path: "/", status: 200, wantCache: "no-cache", wantType: "text/html", wantBody: "<!doctype html>"},
		{path: "/index.html", status: 200, wantCache: "no-cache", wantBody: "<!doctype html>"},
		{path: "/today", status: 200, wantCache: "no-cache", wantBody: "<!doctype html>"},
		{path: "/plants/01HX/edit", status: 200, wantCache: "no-cache", wantBody: "<!doctype html>"},
		{path: "/assets/app-1a2b3c.js", status: 200, wantCache: "public, max-age=31536000, immutable", wantType: "text/javascript"},
		{path: "/sw.js", status: 200, wantCache: "no-cache"},
		{path: "/manifest.webmanifest", status: 200, wantCache: "no-cache", wantType: "application/manifest+json"},
		{path: "/icons/icon-192.png", status: 200, wantCache: "public, max-age=3600"},
		{path: "/assets/missing.js", status: 404},
		{path: "/assets/nested", status: 404},
		{path: "/missing.png", status: 404},
		{path: "/.gitkeep", status: 404},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			resp := e.do(request{path: tt.path})
			if resp.StatusCode != tt.status {
				t.Fatalf("status = %d, want %d", resp.StatusCode, tt.status)
			}
			if tt.wantCache != "" && resp.Header.Get("Cache-Control") != tt.wantCache {
				t.Errorf("Cache-Control = %q, want %q", resp.Header.Get("Cache-Control"), tt.wantCache)
			}
			if tt.wantType != "" && !strings.HasPrefix(resp.Header.Get("Content-Type"), tt.wantType) {
				t.Errorf("Content-Type = %q, want %q", resp.Header.Get("Content-Type"), tt.wantType)
			}
			body, _ := io.ReadAll(resp.Body)
			if tt.wantBody != "" && !strings.HasPrefix(string(body), tt.wantBody) {
				t.Errorf("body = %q", body)
			}
		})
	}

	if resp := e.do(request{method: http.MethodPost, path: "/today"}); resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("POST to SPA: status = %d", resp.StatusCode)
	}
}

func TestSPANotBuilt(t *testing.T) {
	e := newEnv(t, envOptions{spa: fstest.MapFS{".gitkeep": {}}})
	resp := e.do(request{path: "/"})
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusServiceUnavailable || !strings.Contains(string(body), "frontend was not built") {
		t.Errorf("status = %d, body = %q", resp.StatusCode, body)
	}
}
