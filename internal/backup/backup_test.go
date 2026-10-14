package backup

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/reilandeubank/grower/internal/db"
	"github.com/reilandeubank/grower/internal/db/dbgen"
	"github.com/reilandeubank/grower/internal/db/dbtest"
)

var chicago, _ = time.LoadLocation("America/Chicago")

func newRunner(t *testing.T, now *time.Time) *Runner {
	t.Helper()
	return &Runner{
		DB: dbtest.Open(t), DataDir: t.TempDir(), Loc: chicago,
		Now: func() time.Time { return *now }, Log: slog.New(slog.DiscardHandler),
	}
}

func (r *Runner) names(t *testing.T) []string {
	t.Helper()
	names, err := list(filepath.Join(r.DataDir, DirName))
	if err != nil {
		t.Fatal(err)
	}
	return names
}

func TestBackupIsAConsistentCopy(t *testing.T) {
	now := time.Date(2026, 5, 1, 9, 0, 0, 0, chicago)
	r := newRunner(t, &now)
	ctx := context.Background()
	stamp := db.FormatTime(now)
	for _, name := range []string{"Shelf", "Window"} {
		if err := r.DB.Writer().CreateLocation(ctx, dbgen.CreateLocationParams{ID: db.NewID(), Name: name, CreatedAt: stamp, UpdatedAt: stamp}); err != nil {
			t.Fatal(err)
		}
	}

	if err := r.Check(ctx); err != nil {
		t.Fatal(err)
	}
	if got := r.names(t); !slices.Equal(got, []string{"grower-2026-05-01.db"}) {
		t.Fatalf("backups = %v", got)
	}
	// Restore it as docs/backup.md says: the copy becomes grower.db.
	restored := t.TempDir()
	if err := os.Rename(filepath.Join(r.DataDir, DirName, "grower-2026-05-01.db"), filepath.Join(restored, db.FileName)); err != nil {
		t.Fatal(err)
	}
	copyDB, err := db.Open(restored)
	if err != nil {
		t.Fatal(err)
	}
	defer copyDB.Close()
	locs, err := copyDB.Reader().ListLocations(ctx)
	if err != nil || len(locs) != 2 || locs[0].Name != "Shelf" {
		t.Errorf("backup locations = %v, %v", locs, err)
	}
	var check string
	if err := copyDB.Read.QueryRowContext(ctx, "PRAGMA integrity_check").Scan(&check); err != nil || check != "ok" {
		t.Errorf("integrity_check = %q, %v", check, err)
	}
	if leftovers, _ := filepath.Glob(filepath.Join(r.DataDir, DirName, ".*")); len(leftovers) != 0 {
		t.Errorf("temp files left: %v", leftovers)
	}
}

func TestBackupSchedule(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 5, 1, 1, 0, 0, 0, chicago)
	r := newRunner(t, &now)
	check := func(step string, want ...string) {
		t.Helper()
		if err := r.Check(ctx); err != nil {
			t.Fatalf("%s: %v", step, err)
		}
		if got := r.names(t); !slices.Equal(got, want) {
			t.Errorf("%s: backups = %v, want %v", step, got, want)
		}
	}

	// With no backup at all, the first check makes one even before 03:00.
	check("first run at 01:00", "grower-2026-05-01.db")
	check("same day again", "grower-2026-05-01.db")

	now = time.Date(2026, 5, 2, 2, 59, 0, 0, chicago)
	check("next day before 03:00", "grower-2026-05-01.db")
	now = now.Add(time.Minute)
	check("next day at 03:00", "grower-2026-05-01.db", "grower-2026-05-02.db")

	// The day is the configured zone's: 04:00 UTC on May 3 is still May 2 in Chicago.
	now = time.Date(2026, 5, 3, 4, 0, 0, 0, time.UTC)
	check("May 2 in Chicago", "grower-2026-05-01.db", "grower-2026-05-02.db")
}

func TestBackupKeepsNewest14(t *testing.T) {
	now := time.Date(2026, 5, 31, 4, 0, 0, 0, chicago)
	r := newRunner(t, &now)
	dir := filepath.Join(r.DataDir, DirName)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	var want []string
	for day := 1; day <= 20; day++ {
		name := fmt.Sprintf("grower-2026-05-%02d.db", day)
		if err := os.WriteFile(filepath.Join(dir, name), []byte("old"), 0o600); err != nil {
			t.Fatal(err)
		}
		if day > 7 {
			want = append(want, name)
		}
	}
	other := filepath.Join(dir, "keep-me.txt")
	if err := os.WriteFile(other, nil, 0o600); err != nil {
		t.Fatal(err)
	}

	if err := r.Check(context.Background()); err != nil {
		t.Fatal(err)
	}
	want = append(want, "grower-2026-05-31.db")
	if got := r.names(t); !slices.Equal(got, want) {
		t.Errorf("backups = %v, want %v", got, want)
	}
	if _, err := os.Stat(other); err != nil {
		t.Errorf("unrelated file removed: %v", err)
	}
}

func TestRunStopsOnCancel(t *testing.T) {
	now := time.Date(2026, 5, 1, 9, 0, 0, 0, chicago)
	r := newRunner(t, &now)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		r.Run(ctx)
	}()
	deadline := time.Now().Add(5 * time.Second)
	for len(r.names(t)) == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not stop")
	}
	if got := r.names(t); len(got) != 1 {
		t.Errorf("backups = %v", got)
	}
}

func TestLatest(t *testing.T) {
	dir := t.TempDir()
	if _, ok, err := Latest(dir); ok || err != nil {
		t.Fatalf("no backups dir: ok=%v err=%v", ok, err)
	}
	if err := os.MkdirAll(filepath.Join(dir, DirName), 0o750); err != nil {
		t.Fatal(err)
	}
	want := time.Date(2026, 5, 2, 8, 0, 0, 0, time.UTC)
	for name, mtime := range map[string]time.Time{
		"grower-2026-05-01.db":       want.Add(-24 * time.Hour),
		"grower-2026-05-02.db":       want,
		".grower-2026-05-03.db.tmp":  want.Add(time.Hour),
		"grower-2026-05-03.db-wal":   want.Add(time.Hour),
		"grower-2026-5-3.db":         want.Add(time.Hour),
		"grower-latest-copy-x.db.gz": want.Add(time.Hour),
	} {
		path := filepath.Join(dir, DirName, name)
		if err := os.WriteFile(path, nil, 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(path, mtime, mtime); err != nil {
			t.Fatal(err)
		}
	}
	got, ok, err := Latest(dir)
	if !ok || err != nil || !got.Equal(want) {
		t.Errorf("Latest = %v, %v, %v; want %v", got, ok, err, want)
	}
}
