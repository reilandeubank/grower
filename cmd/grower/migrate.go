package main

import (
	"context"
	"flag"
	"fmt"
	"io"
)

func cmdMigrate(args []string, stdout io.Writer) error {
	flags := flag.NewFlagSet("migrate", flag.ContinueOnError)
	if err := flags.Parse(args); err != nil {
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

	statuses, err := d.MigrationStatuses(ctx)
	if err != nil {
		return err
	}
	for _, s := range statuses {
		state := "pending"
		if s.Applied {
			state = "applied"
		}
		fmt.Fprintf(stdout, "%04d  %-8s %s\n", s.Version, state, s.Name)
	}
	return nil
}
