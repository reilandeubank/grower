package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/reilandeubank/grower/internal/buildinfo"
	"github.com/reilandeubank/grower/internal/config"
	"github.com/reilandeubank/grower/internal/db"
)

func loadConfig() (config.Config, error) {
	return config.Load(os.Getenv)
}

func newLogger(cfg config.Config, w io.Writer) *slog.Logger {
	opts := &slog.HandlerOptions{Level: cfg.LogLevel}
	if cfg.LogFormat == "text" {
		return slog.New(slog.NewTextHandler(w, opts))
	}
	return slog.New(slog.NewJSONHandler(w, opts))
}

// openMigrated opens the database and brings its schema up to date. Every command
// that touches data migrates first, so the CLI works on a fresh data dir.
func openMigrated(ctx context.Context, cfg config.Config) (*db.DB, error) {
	d, err := db.Open(cfg.DataDir)
	if err != nil {
		return nil, err
	}
	if err := d.Migrate(ctx); err != nil {
		_ = d.Close()
		return nil, err
	}
	return d, nil
}

func cmdVersion(stdout io.Writer) error {
	_, err := fmt.Fprintln(stdout, "grower", buildinfo.Version)
	return err
}
