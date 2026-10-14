package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/reilandeubank/grower/internal/auth"
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

	log.Info("starting grower", "version", buildinfo.Version, "data_dir", cfg.DataDir,
		"base_url", cfg.BaseURL.String(), "trusted_proxies", len(cfg.TrustedProxies))
	h := server.New(server.Deps{Config: cfg, DB: d, Auth: authSvc, SPA: web.FS(), Log: log})
	return server.Run(ctx, cfg.Listen, h, log)
}
