package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"time"

	"github.com/reilandeubank/grower/internal/api"
	"github.com/reilandeubank/grower/internal/media"
)

// cmdImport loads an export zip into an empty grower. It is CLI only: big zips don't
// fit through a reverse proxy's upload limit.
func cmdImport(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("import", flag.ContinueOnError)
	flags.SetOutput(stderr)
	username := flags.String("username", "", "the user who gets the export's settings (needed when there are several)")
	if err := flags.Parse(args); err != nil {
		return fmt.Errorf("%w: %w", errUsage, err)
	}
	if flags.NArg() != 1 {
		return fmt.Errorf("%w: import takes one export zip", errUsage)
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
	summary, err := api.Import(ctx, d, media.New(cfg.DataDir), flags.Arg(0), *username, time.Now())
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(stdout, summary)
	return err
}
