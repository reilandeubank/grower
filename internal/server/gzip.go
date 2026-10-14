package server

import (
	"bytes"
	"compress/gzip"
	"net/http"
	"path"
	"strings"
	"sync"
)

// compressible lists the text types worth gzipping. Images and fonts are already compressed.
var compressible = map[string]bool{
	".html": true, ".js": true, ".css": true, ".svg": true, ".json": true, ".webmanifest": true, ".txt": true,
}

// gzipCache holds gzipped copies of embedded files. The embedded frontend never changes
// while the server runs, so each file is compressed once, on its first request.
type gzipCache struct{ files sync.Map } // name → []byte

// get returns the gzipped form of name, compressing raw() the first time.
func (c *gzipCache) get(name string, raw func() ([]byte, error)) ([]byte, error) {
	if v, ok := c.files.Load(name); ok {
		return v.([]byte), nil
	}
	b, err := raw()
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	zw, err := gzip.NewWriterLevel(&buf, gzip.BestCompression)
	if err != nil {
		return nil, err
	}
	if _, err := zw.Write(b); err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	v, _ := c.files.LoadOrStore(name, buf.Bytes())
	return v.([]byte), nil
}

// shouldGzip reports whether to send name gzipped, and marks the response as varying
// by Accept-Encoding so caches keep both forms apart.
func shouldGzip(w http.ResponseWriter, r *http.Request, name string) bool {
	if !compressible[path.Ext(name)] {
		return false
	}
	w.Header().Add("Vary", "Accept-Encoding")
	for part := range strings.SplitSeq(r.Header.Get("Accept-Encoding"), ",") {
		coding, params, _ := strings.Cut(strings.TrimSpace(part), ";")
		if strings.EqualFold(strings.TrimSpace(coding), "gzip") {
			q := strings.ReplaceAll(params, " ", "")
			return q != "q=0" && q != "q=0.0" && q != "q=0.00" && q != "q=0.000"
		}
	}
	return false
}
