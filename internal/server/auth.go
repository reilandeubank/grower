package server

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/reilandeubank/grower/internal/api"
	"github.com/reilandeubank/grower/internal/auth"
)

// sessions resolves the session cookie into an auth.Principal on the request context.
// It never rejects a request itself; requireAuth decides per route.
func sessions(store *auth.Sessions, cookie auth.CookieConfig, now auth.Clock, log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := cookie.Read(r)
			if token == "" {
				next.ServeHTTP(w, r)
				return
			}
			p, refreshed, err := store.Lookup(r.Context(), token)
			switch {
			case errors.Is(err, auth.ErrNoSession):
				cookie.Clear(w) // stale cookie: stop the browser sending it
			case err != nil:
				log.ErrorContext(r.Context(), "session lookup failed", "err", err)
				api.WriteProblem(w, http.StatusInternalServerError, "")
				return
			default:
				if refreshed {
					cookie.Set(w, token, p.ExpiresAt, now())
				}
				r = r.WithContext(auth.WithPrincipal(r.Context(), p))
			}
			next.ServeHTTP(w, r)
		})
	}
}

// isPublicOperation reports whether a route, given as the ServeMux pattern the
// generated router registers, is reachable without a session. Everything else needs one.
func isPublicOperation(pattern string) bool {
	switch pattern {
	case "POST /api/v1/auth/login", "POST /api/v1/auth/logout", "GET /api/v1/setup", "POST /api/v1/setup":
		return true
	}
	return false
}

// isTokenOperation reports whether a route also accepts an API token. Tokens are
// opt-in per route, so a leaked token reads the summary and nothing else.
func isTokenOperation(pattern string) bool {
	return pattern == "GET /api/v1/integrations/home-assistant"
}

// requireAuth runs per generated operation (not for unknown paths, which 404).
// On token routes, a bearer token takes precedence over the session cookie.
func requireAuth(tokens *auth.APITokens, log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if header := r.Header.Get("Authorization"); header != "" && isTokenOperation(r.Pattern) {
				p, err := tokenPrincipal(r, tokens, header)
				switch {
				case errors.Is(err, auth.ErrNoToken):
					w.Header().Set("WWW-Authenticate", `Bearer error="invalid_token"`)
					api.WriteProblem(w, http.StatusUnauthorized, "The API token is not valid. It may have been revoked; create a new one in Settings.")
					return
				case err != nil:
					log.ErrorContext(r.Context(), "api token lookup failed", "err", err)
					api.WriteProblem(w, http.StatusInternalServerError, "")
					return
				}
				r = r.WithContext(auth.WithPrincipal(r.Context(), p))
			}
			if _, ok := auth.PrincipalFrom(r.Context()); !ok && !isPublicOperation(r.Pattern) {
				if isTokenOperation(r.Pattern) {
					w.Header().Set("WWW-Authenticate", "Bearer")
				}
				api.WriteProblem(w, http.StatusUnauthorized, "Sign in to continue.")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// tokenPrincipal resolves an "Authorization: Bearer <token>" header.
func tokenPrincipal(r *http.Request, tokens *auth.APITokens, header string) (auth.Principal, error) {
	scheme, secret, ok := strings.Cut(header, " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") {
		return auth.Principal{}, auth.ErrNoToken
	}
	return tokens.Lookup(r.Context(), strings.TrimSpace(secret))
}
