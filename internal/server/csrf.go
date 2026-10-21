package server

import (
	"mime"
	"net/http"
	"strings"

	"github.com/reilandeubank/grower/internal/api"
)

// csrf guards state-changing API requests. SameSite=Lax already stops most cross-site
// POSTs; this also blocks them from sibling subdomains and old browsers:
//   - Origin, when sent, must equal the configured public origin;
//   - without Origin, Sec-Fetch-Site must say same-origin;
//   - bodies must be JSON, which a plain HTML form cannot send. Photo uploads and
//     identification are multipart, which a form can send, so they rely on the two checks above.
func csrf(origin string, isUpload func(*http.Request) bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case http.MethodGet, http.MethodHead, http.MethodOptions:
				next.ServeHTTP(w, r)
				return
			}
			if got := r.Header.Get("Origin"); got != "" {
				if !strings.EqualFold(got, origin) {
					api.WriteProblem(w, http.StatusForbidden, "Cross-origin request blocked.")
					return
				}
			} else if r.Header.Get("Sec-Fetch-Site") != "same-origin" {
				api.WriteProblem(w, http.StatusForbidden, "Cross-origin request blocked.")
				return
			}
			if r.ContentLength != 0 {
				want, detail := "application/json", "Request bodies must be application/json."
				if isUpload(r) {
					want, detail = "multipart/form-data", "Photo uploads must be multipart/form-data."
				}
				if mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type")); err != nil || mt != want {
					api.WriteProblem(w, http.StatusUnsupportedMediaType, detail)
					return
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}
