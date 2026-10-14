package server

import (
	"bytes"
	"io"
	"io/fs"
	"net/http"
	"path"
	"strings"
	"time"
)

// mustRevalidate lists files browsers must check on every load: they name the hashed
// assets, and a stale service worker would pin users to an old release.
func mustRevalidate(name string) bool {
	switch name {
	case "index.html", "sw.js", "registerSW.js", "manifest.webmanifest":
		return true
	}
	return false
}

const frontendMissing = "grower: the frontend was not built into this binary.\n" +
	"Run `make build` (which builds web/dist before compiling Go) and restart.\n"

// spaHandler serves the built frontend. Paths that look like client-side routes fall
// back to index.html; missing files with an extension or under /assets/ are real 404s
// so a broken asset reference fails loudly instead of returning HTML.
func spaHandler(dist fs.FS) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		name := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
		if name == "" {
			name = "index.html"
		}
		if !hiddenPath(name) && serveFile(w, r, dist, name) {
			return
		}
		if name != "index.html" && (strings.HasPrefix(name, "assets/") || path.Ext(name) != "") {
			http.NotFound(w, r)
			return
		}
		if !serveFile(w, r, dist, "index.html") {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.Header().Set("Cache-Control", "no-store")
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = io.WriteString(w, frontendMissing)
		}
	})
}

// hiddenPath hides dotfiles such as dist/.gitkeep.
func hiddenPath(name string) bool {
	for seg := range strings.SplitSeq(name, "/") {
		if strings.HasPrefix(seg, ".") {
			return true
		}
	}
	return false
}

// serveFile writes name from dist and reports whether it was a regular file.
func serveFile(w http.ResponseWriter, r *http.Request, dist fs.FS, name string) bool {
	f, err := dist.Open(name)
	if err != nil {
		return false
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return false
	}

	content, ok := f.(io.ReadSeeker)
	if !ok {
		b, err := io.ReadAll(f)
		if err != nil {
			return false
		}
		content = bytes.NewReader(b)
	}

	h := w.Header()
	switch {
	case strings.HasPrefix(name, "assets/"):
		// Vite puts a content hash in every asset filename.
		h.Set("Cache-Control", "public, max-age=31536000, immutable")
	case mustRevalidate(name):
		h.Set("Cache-Control", "no-cache")
	default:
		h.Set("Cache-Control", "public, max-age=3600")
	}
	if path.Ext(name) == ".webmanifest" {
		h.Set("Content-Type", "application/manifest+json")
	}
	// embed.FS has no modification times; zero disables Last-Modified handling.
	http.ServeContent(w, r, name, time.Time{}, content)
	return true
}
