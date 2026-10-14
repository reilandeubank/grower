package db

import (
	"context"
	"embed"
	"fmt"
	"io/fs"

	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

// MigrationStatus is one migration and whether it has been applied.
type MigrationStatus struct {
	Version int64
	Name    string
	Applied bool
}

func (d *DB) provider() (*goose.Provider, error) {
	sub, err := fs.Sub(migrationFiles, "migrations")
	if err != nil {
		return nil, fmt.Errorf("migrations fs: %w", err)
	}
	p, err := goose.NewProvider(goose.DialectSQLite3, d.Write, sub)
	if err != nil {
		return nil, fmt.Errorf("goose provider: %w", err)
	}
	return p, nil
}

// Migrate applies all pending migrations. It refuses to touch a database whose schema
// version is newer than this binary knows, which means a downgrade without a restore.
func (d *DB) Migrate(ctx context.Context) error {
	p, err := d.provider()
	if err != nil {
		return err
	}
	current, target, err := p.GetVersions(ctx)
	if err != nil {
		return fmt.Errorf("read schema version: %w", err)
	}
	if current > target {
		return fmt.Errorf("database schema version %d is newer than this binary supports (%d); run a newer grower or restore a backup", current, target)
	}
	if _, err := p.Up(ctx); err != nil {
		return fmt.Errorf("apply migrations: %w", err)
	}
	return nil
}

// MigrationStatuses lists every known migration and whether it is applied.
func (d *DB) MigrationStatuses(ctx context.Context) ([]MigrationStatus, error) {
	p, err := d.provider()
	if err != nil {
		return nil, err
	}
	statuses, err := p.Status(ctx)
	if err != nil {
		return nil, fmt.Errorf("migration status: %w", err)
	}
	out := make([]MigrationStatus, 0, len(statuses))
	for _, s := range statuses {
		out = append(out, MigrationStatus{
			Version: s.Source.Version,
			Name:    s.Source.Path,
			Applied: s.State == goose.StateApplied,
		})
	}
	return out, nil
}
