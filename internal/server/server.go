// Package server wires the HTTP router, middleware, SPA and health check.
package server

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/reilandeubank/grower/internal/api"
	"github.com/reilandeubank/grower/internal/apigen"
	"github.com/reilandeubank/grower/internal/auth"
	"github.com/reilandeubank/grower/internal/config"
	"github.com/reilandeubank/grower/internal/db"
	"github.com/reilandeubank/grower/internal/media"
	"github.com/reilandeubank/grower/internal/push"
)

// Deps are everything the HTTP layer needs.
type Deps struct {
	Config config.Config
	DB     *db.DB
	Auth   *auth.Service
	// Push sends notifications; the daily reminder shares it.
	Push *push.Sender
	// SPA is the built frontend (the contents of web/dist).
	SPA fs.FS
	Log *slog.Logger
}

// maxJSONBody bounds API request bodies; JSON payloads here are tiny.
const maxJSONBody = 1 << 20

// New builds the root handler. Deps.Config must have a BaseURL.
func New(d Deps) http.Handler {
	cookie := auth.CookieConfig{Name: d.Config.SessionCookieName(), Secure: d.Config.SecureCookies()}
	store := media.New(d.Config.DataDir)
	if err := store.CleanTemp(); err != nil {
		d.Log.Warn("clean leftover uploads", "err", err)
	}
	withSession := sessions(d.Auth.Sessions(), cookie, d.Auth.Now, d.Log)

	apiMux := http.NewServeMux()
	apigen.HandlerWithOptions(api.New(d.Auth, d.DB, store, d.Push, cookie, d.Config.DataDir, d.Log).ServerInterface(), apigen.StdHTTPServerOptions{
		BaseURL:     "/api/v1",
		BaseRouter:  apiMux,
		Middlewares: []apigen.MiddlewareFunc{requireAuth},
		ErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) {
			api.WriteProblem(w, http.StatusBadRequest, err.Error())
		},
	})
	// Anything under /api that the spec does not define is a JSON 404, never the SPA.
	apiMux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		api.WriteProblem(w, http.StatusNotFound, "No such API endpoint.")
	})

	// The photo upload is the one route with a multipart body and a larger size cap.
	isUpload := func(r *http.Request) bool {
		_, pattern := apiMux.Handler(r)
		return pattern == "POST /api/v1/plants/{id}/photos"
	}
	apiHandler := chain(apiMux,
		noStore,
		limitBody(isUpload, maxJSONBody, api.MaxUploadBytes),
		csrf(d.Config.Origin(), isUpload),
		withSession,
	)

	mux := http.NewServeMux()
	mux.Handle("/api/", apiHandler)
	mux.Handle("GET /healthz", healthz(d.DB, d.Log))
	mux.Handle("GET /media/photos/{id}/{size}", chain(mediaHandler(store, d.Log), withSession, requireAuth))
	mux.HandleFunc("/media/", func(w http.ResponseWriter, r *http.Request) { // never the SPA
		api.WriteProblem(w, http.StatusNotFound, photoNotFound)
	})
	mux.Handle("/", spaHandler(d.SPA))

	return chain(mux,
		realIP(d.Config.TrustedProxies),
		logRequests(d.Log),
		recoverPanics(d.Log),
		securityHeaders,
	)
}

// chain applies middleware so the first listed is the outermost.
func chain(h http.Handler, mw ...func(http.Handler) http.Handler) http.Handler {
	for i := len(mw) - 1; i >= 0; i-- {
		h = mw[i](h)
	}
	return h
}

// shutdownTimeout lets in-flight requests finish; Docker's default stop grace is 10 s.
const shutdownTimeout = 8 * time.Second

// Run serves h on addr until ctx is cancelled, then shuts down gracefully.
func Run(ctx context.Context, addr string, h http.Handler, log *slog.Logger) error {
	srv := &http.Server{
		Addr:              addr,
		Handler:           h,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       60 * time.Second, // generous for photo uploads on slow mobile links (M4)
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    64 << 10,
		ErrorLog:          slog.NewLogLogger(log.Handler(), slog.LevelWarn),
		BaseContext:       func(net.Listener) context.Context { return context.WithoutCancel(ctx) },
	}

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", addr, err)
	}
	log.Info("listening", "addr", ln.Addr().String())

	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.Serve(ln) }()

	select {
	case err := <-serveErr:
		return fmt.Errorf("serve: %w", err)
	case <-ctx.Done():
	}

	log.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("graceful shutdown: %w", err)
	}
	if err := <-serveErr; !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("serve: %w", err)
	}
	return nil
}
