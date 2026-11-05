package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"time"

	"github.com/reilandeubank/grower/internal/media"
)

// cmdMedia runs `grower media sweep`: the server's daily orphaned-photo sweep, on demand.
func cmdMedia(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 || args[0] != "sweep" {
		return fmt.Errorf("%w: media needs the subcommand sweep", errUsage)
	}
	flags := flag.NewFlagSet("media sweep", flag.ContinueOnError)
	flags.SetOutput(stderr)
	dryRun := flags.Bool("dry-run", false, "list what would be removed without removing it")
	if err := flags.Parse(args[1:]); err != nil {
		return fmt.Errorf("%w: %w", errUsage, err)
	}
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	ctx := context.Background()
	d, err := openMigrated(ctx, cfg)
	if err != nil {
		return err
	}
	defer d.Close()

	sweeper := media.Sweeper{Store: media.New(cfg.DataDir), PhotoIDs: d.Reader().ListPhotoIDs, Now: time.Now}
	swept, err := sweeper.Sweep(ctx, *dryRun)
	verb := "Removed"
	if *dryRun {
		verb = "Would remove"
	}
	noun := "photos"
	if swept.Photos == 1 {
		noun = "photo"
	}
	fmt.Fprintf(stdout, "%s %d orphaned %s (%.1f MB).\n", verb, swept.Photos, noun, float64(swept.Bytes)/(1<<20))
	return err
}
