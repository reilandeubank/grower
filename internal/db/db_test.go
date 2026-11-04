package db_test

import (
	"context"
	"database/sql"
	"path/filepath"
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

// URI-special characters in the data dir must not move the database or drop its pragmas (#20),
// and relative data dirs keep working.
func TestOpensDataDirWithURICharacters(t *testing.T) {
	t.Chdir(t.TempDir())
	for _, dir := range []string{filepath.Join(t.TempDir(), "plants#1?x=y 50%20off"), "data", "a:b/c#d"} {
		t.Run(dir, func(t *testing.T) {
			d, err := db.Open(dir)
			if err != nil {
				t.Fatal(err)
			}
			defer d.Close()
			want, err := filepath.Abs(filepath.Join(dir, db.FileName))
			if err != nil {
				t.Fatal(err)
			}
			for name, pool := range map[string]*sql.DB{"write": d.Write, "read": d.Read} {
				var seq int
				var schema, file string
				if err := pool.QueryRow("PRAGMA database_list").Scan(&seq, &schema, &file); err != nil {
					t.Fatal(err)
				}
				if file != want {
					t.Errorf("%s pool opened %q, want %q", name, file, want)
				}
				for pragma, want := range map[string]string{"foreign_keys": "1", "journal_mode": "wal"} {
					var got string
					if err := pool.QueryRow("PRAGMA " + pragma).Scan(&got); err != nil {
						t.Fatal(err)
					}
					if got != want {
						t.Errorf("%s pool: %s = %s, want %s", name, pragma, got, want)
					}
				}
			}
		})
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
