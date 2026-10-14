package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/reilandeubank/grower/internal/auth"
	"github.com/reilandeubank/grower/internal/backup"
	"github.com/reilandeubank/grower/internal/buildinfo"
	"github.com/reilandeubank/grower/internal/server"
	"github.com/reilandeubank/grower/web"
)

func cmdServe(args []string) error {
	flags := flag.NewFlagSet("serve", flag.ContinueOnError)
	if err := flags.Parse(args); err != nil {
		return fmt.Errorf("%w: %w", errUsage, err)
	}
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	if err := cfg.RequireBaseURL(); err != nil {
		return err
	}
	log := newLogger(cfg, os.Stderr)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	d, err := openMigrated(ctx, cfg)
	if err != nil {
		return err
	}
	defer d.Close()

	authSvc, err := auth.NewService(d, auth.Options{})
	if err != nil {
		return err
	}

	// The backup runner stops with ctx and must finish before the database closes.
	backups := &backup.Runner{DB: d, DataDir: cfg.DataDir, Loc: cfg.Location, Now: time.Now, Log: log}
	backupsDone := make(chan struct{})
	go func() {
		defer close(backupsDone)
		backups.Run(ctx)
	}()
	defer func() {
		stop()
		<-backupsDone
	}()

	log.Info("starting grower", "version", buildinfo.Version, "data_dir", cfg.DataDir,
		"base_url", cfg.BaseURL.String(), "trusted_proxies", len(cfg.TrustedProxies))
	h := server.New(server.Deps{Config: cfg, DB: d, Auth: authSvc, SPA: web.FS(), Log: log})
	return server.Run(ctx, cfg.Listen, h, log)
}
