package server

import (
	"compress/gzip"
	"io"
	"net/http"
	"testing"
)

func TestSPAGzip(t *testing.T) {
	e := newEnv(t, envOptions{spa: spaFS()})
	tests := []struct {
		path, acceptEncoding string
		wantGzip, wantVary   bool
		wantBody             string
	}{
		{path: "/assets/app-1a2b3c.js", acceptEncoding: "gzip, deflate, br", wantGzip: true, wantVary: true, wantBody: "console.log(1)"},
		{path: "/today", acceptEncoding: "br;q=1.0, gzip;q=0.8", wantGzip: true, wantVary: true, wantBody: "<!doctype html><title>grower</title>"},
		{path: "/assets/app-1a2b3c.js", acceptEncoding: "", wantVary: true, wantBody: "console.log(1)"},
		{path: "/assets/app-1a2b3c.js", acceptEncoding: "gzip;q=0", wantVary: true, wantBody: "console.log(1)"},
		{path: "/icons/icon-192.png", acceptEncoding: "gzip", wantBody: "png"},
	}
	for _, tt := range tests {
		t.Run(tt.path+" "+tt.acceptEncoding, func(t *testing.T) {
			res := e.do(request{path: tt.path, headers: map[string]string{"Accept-Encoding": tt.acceptEncoding}})
			if res.StatusCode != http.StatusOK {
				t.Fatalf("status = %d", res.StatusCode)
			}
			if got := res.Header.Get("Content-Encoding") == "gzip"; got != tt.wantGzip {
				t.Errorf("gzipped = %v, want %v", got, tt.wantGzip)
			}
			if got := res.Header.Get("Vary") == "Accept-Encoding"; got != tt.wantVary {
				t.Errorf("Vary = %q", res.Header.Get("Vary"))
			}
			body := io.Reader(res.Body)
			if tt.wantGzip {
				zr, err := gzip.NewReader(res.Body)
				if err != nil {
					t.Fatal(err)
				}
				body = zr
			}
			b, err := io.ReadAll(body)
			if err != nil {
				t.Fatal(err)
			}
			if string(b) != tt.wantBody {
				t.Errorf("body = %q, want %q", b, tt.wantBody)
			}
		})
	}
}
