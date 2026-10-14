// Package api implements the generated strict server interface (internal/apigen).
// Handlers stay thin: parse, call auth/db, map the result to a response.
package api

import (
	"context"
	"log/slog"
	"net"
	"net/http"

	"github.com/reilandeubank/grower/internal/apigen"
	"github.com/reilandeubank/grower/internal/auth"
	"github.com/reilandeubank/grower/internal/db"
	"github.com/reilandeubank/grower/internal/media"
)

// Handler serves /api/v1.
type Handler struct {
	auth   *auth.Service
	db     *db.DB
	cookie auth.CookieConfig
	media  *media.Store
	log    *slog.Logger
}

var _ apigen.StrictServerInterface = (*Handler)(nil)

// New returns the API handler.
func New(authSvc *auth.Service, d *db.DB, store *media.Store, cookie auth.CookieConfig, log *slog.Logger) *Handler {
	return &Handler{auth: authSvc, db: d, media: store, cookie: cookie, log: log}
}

// ServerInterface adapts the strict handlers to net/http, with problem+json errors.
func (h *Handler) ServerInterface() apigen.ServerInterface {
	return apigen.NewStrictHandlerWithOptions(h, []apigen.StrictMiddlewareFunc{withRequest},
		apigen.StrictHTTPServerOptions{
			RequestErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) {
				WriteProblem(w, http.StatusBadRequest, "The request body is not valid JSON for this endpoint.")
			},
			ResponseErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) {
				// The single place handler errors are logged.
				h.log.ErrorContext(r.Context(), "api handler failed", "path", r.URL.Path, "err", err)
				WriteProblem(w, http.StatusInternalServerError, "")
			},
		})
}

type requestKey struct{}

// withRequest exposes the *http.Request to strict handlers, which otherwise only see
// the decoded body; login needs the client IP and User-Agent.
func withRequest(next apigen.StrictHandlerFunc, _ string) apigen.StrictHandlerFunc {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request, req any) (any, error) {
		return next(context.WithValue(ctx, requestKey{}, r), w, r, req)
	}
}

func requestFrom(ctx context.Context) *http.Request {
	r, _ := ctx.Value(requestKey{}).(*http.Request)
	if r == nil {
		// Only reachable if the middleware is not installed: a programming error.
		panic("api: request missing from context")
	}
	return r
}

// clientIP is the host part of RemoteAddr, which the server's proxy middleware has
// already rewritten to the real client address.
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
