package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/reilandeubank/grower/internal/auth"
	"github.com/reilandeubank/grower/internal/backup"
	"github.com/reilandeubank/grower/internal/buildinfo"
	"github.com/reilandeubank/grower/internal/config"
	"github.com/reilandeubank/grower/internal/push"
	"github.com/reilandeubank/grower/internal/server"
	"github.com/reilandeubank/grower/internal/species"
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

	keys, err := push.LoadKeys(cfg.DataDir)
	if err != nil {
		return err
	}
	pusher := &push.Sender{DB: d, Keys: keys, Subject: cfg.BaseURL.String(), Now: time.Now, Log: log}

	// The background runners stop with ctx and must finish before the database closes.
	backups := &backup.Runner{DB: d, DataDir: cfg.DataDir, Loc: cfg.Location, Now: time.Now, Log: log}
	digest := &push.Digest{DB: d, Sender: pusher, Now: time.Now, Log: log}
	var runners sync.WaitGroup
	for _, run := range []func(context.Context){backups.Run, digest.Run} {
		runners.Go(func() { run(ctx) })
	}
	defer func() {
		stop()
		runners.Wait()
	}()

	lookup := speciesLookup(cfg)
	var identify *species.PlantNet
	if cfg.PlantNetAPIKey != "" {
		identify = species.NewPlantNet(cfg.PlantNetAPIKey)
	}
	log.Info("starting grower", "version", buildinfo.Version, "data_dir", cfg.DataDir,
		"base_url", cfg.BaseURL.String(), "trusted_proxies", len(cfg.TrustedProxies), "species_providers", speciesSources(lookup), "plant_identification", identify != nil)
	h := server.New(server.Deps{Config: cfg, DB: d, Auth: authSvc, Push: pusher, Species: lookup, Identify: identify, SPA: web.FS(), Log: log})
	return server.Run(ctx, cfg.Listen, h, log)
}

// speciesLookup enables each species provider whose credentials are configured.
func speciesLookup(cfg config.Config) *species.Lookup {
	var providers []species.Provider
	if cfg.OpenPlantbookClientID != "" {
		providers = append(providers, species.NewOpenPlantbook(cfg.OpenPlantbookClientID, cfg.OpenPlantbookClientSecret))
	}
	if cfg.PerenualAPIKey != "" {
		providers = append(providers, species.NewPerenual(cfg.PerenualAPIKey))
	}
	return species.NewLookup(time.Now, providers...)
}

// speciesSources names the enabled providers for the startup log (never their keys).
func speciesSources(l *species.Lookup) []string {
	out := []string{}
	for _, p := range l.Providers() {
		out = append(out, p.Source())
	}
	return out
}
