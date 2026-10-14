package auth

import (
	"net/http"
	"time"
)

// CookieConfig describes the session cookie for this deployment.
type CookieConfig struct {
	Name   string
	Secure bool
}

// Read returns the session token from r, or "" when absent.
func (c CookieConfig) Read(r *http.Request) string {
	ck, err := r.Cookie(c.Name)
	if err != nil {
		return ""
	}
	return ck.Value
}

// Set writes the session cookie, valid until expires.
func (c CookieConfig) Set(w http.ResponseWriter, token string, expires, now time.Time) {
	http.SetCookie(w, c.cookie(token, int(expires.Sub(now).Round(time.Second).Seconds())))
}

// Clear tells the browser to drop the session cookie.
func (c CookieConfig) Clear(w http.ResponseWriter) {
	http.SetCookie(w, c.cookie("", -1))
}

func (c CookieConfig) cookie(value string, maxAge int) *http.Cookie {
	//nolint:gosec // G124: Secure follows GROWER_BASE_URL; it must be off for plain-http dev setups.
	return &http.Cookie{
		Name:     c.Name,
		Value:    value,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   c.Secure,
		// Lax (not Strict) so opening the app from a link or the installed PWA still
		// sends the cookie on top-level navigation.
		SameSite: http.SameSiteLaxMode,
	}
}
