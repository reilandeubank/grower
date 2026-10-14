// Package dbtest opens throwaway migrated databases for tests.
package dbtest

import (
	"context"
	"testing"

	"github.com/reilandeubank/grower/internal/db"
)

// Open returns a migrated database in a temp dir, closed when the test ends.
func Open(t testing.TB) *db.DB {
	t.Helper()
	d, err := db.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })
	if err := d.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return d
}
