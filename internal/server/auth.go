package server

import (
	"errors"
	"log/slog"
	"net/http"

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
	case "POST /api/v1/auth/login", "POST /api/v1/auth/logout":
		return true
	}
	return false
}

// requireAuth runs per generated operation (not for unknown paths, which 404).
func requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := auth.PrincipalFrom(r.Context()); !ok && !isPublicOperation(r.Pattern) {
			api.WriteProblem(w, http.StatusUnauthorized, "Sign in to continue.")
			return
		}
		next.ServeHTTP(w, r)
	})
}
