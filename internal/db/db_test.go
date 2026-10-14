package db_test

import (
	"context"
	"strings"
	"testing"

	"github.com/reilandeubank/grower/internal/db"
	"github.com/reilandeubank/grower/internal/db/dbtest"
)

func TestMigrateIsIdempotentAndReportsStatus(t *testing.T) {
	d := dbtest.Open(t)
	ctx := context.Background()
	if err := d.Migrate(ctx); err != nil {
		t.Fatalf("second migrate: %v", err)
	}
	statuses, err := d.MigrationStatuses(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(statuses) == 0 || !statuses[0].Applied || statuses[0].Version != 1 {
		t.Fatalf("unexpected statuses: %+v", statuses)
	}
}

func TestPragmas(t *testing.T) {
	d := dbtest.Open(t)
	for _, tc := range []struct{ pragma, want string }{
		{"journal_mode", "wal"},
		{"foreign_keys", "1"},
		{"busy_timeout", "5000"},
		{"synchronous", "1"}, // NORMAL
	} {
		var got string
		if err := d.Read.QueryRow("PRAGMA " + tc.pragma).Scan(&got); err != nil {
			t.Fatal(err)
		}
		if got != tc.want {
			t.Errorf("%s = %s, want %s", tc.pragma, got, tc.want)
		}
	}
	if _, err := d.Read.Exec("DELETE FROM users"); err == nil {
		t.Error("read pool accepted a write")
	}
}

func TestRefusesNewerSchema(t *testing.T) {
	dir := t.TempDir()
	d, err := db.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	ctx := context.Background()
	if err := d.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Write.Exec(`INSERT INTO goose_db_version (version_id, is_applied) VALUES (9999, 1)`); err != nil {
		t.Fatal(err)
	}
	err = d.Migrate(ctx)
	if err == nil || !strings.Contains(err.Error(), "newer than this binary") {
		t.Fatalf("want newer-schema error, got %v", err)
	}
}
