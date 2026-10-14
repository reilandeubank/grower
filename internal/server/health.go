package server

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/reilandeubank/grower/internal/buildinfo"
	"github.com/reilandeubank/grower/internal/db"
)

// healthz reports liveness for Docker's HEALTHCHECK and the reverse proxy. It pings
// the database so a broken volume shows up as unhealthy.
func healthz(d *db.DB, log *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		status, code := "ok", http.StatusOK
		if err := d.Ping(ctx); err != nil {
			log.WarnContext(r.Context(), "health check: database unreachable", "err", err)
			status, code = "unavailable", http.StatusServiceUnavailable
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(code)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": status, "version": buildinfo.Version})
	})
}
