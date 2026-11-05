package media

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

const (
	kept    = "01M6M50ERVFX1QWZ24F0138G9Q"
	orphan  = "01M6M514GNBS4VZ1ABXT95MGM9"
	young   = "01M6M515NFX4QT0QDRWJJ561T0"
	another = "01M6M516AAAAAAAAAAAAAAAAAA"
)

// photoDir writes a fake photo directory of 10 bytes, last modified at mod.
func photoDir(t *testing.T, s *Store, name string, mod time.Time) string {
	t.Helper()
	dir := filepath.Join(s.dir, "photos", name)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "small.jpg"), []byte("0123456789"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(dir, mod, mod); err != nil {
		t.Fatal(err)
	}
	return dir
}

func exists(t *testing.T, path string) bool {
	t.Helper()
	_, err := os.Stat(path)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	return err == nil
}

func TestSweeperRemovesOldOrphansOnly(t *testing.T) {
	now := time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC)
	old := now.Add(-2 * time.Hour)
	s := New(t.TempDir())
	keptDir := photoDir(t, s, kept, old)
	orphanDir := photoDir(t, s, orphan, old)
	youngDir := photoDir(t, s, young, now.Add(-SweepGrace+time.Minute)) // an upload whose row isn't committed yet
	notPhoto := photoDir(t, s, "notes", old)

	w := Sweeper{
		Store:    s,
		PhotoIDs: func(context.Context) ([]string, error) { return []string{kept}, nil },
		Now:      func() time.Time { return now },
	}
	dry, err := w.Sweep(context.Background(), true)
	if err != nil {
		t.Fatal(err)
	}
	if dry != (Swept{Photos: 1, Bytes: 10}) || !exists(t, orphanDir) {
		t.Fatalf("dry run: got %+v, orphan still there: %v; want 1 photo, 10 bytes, nothing removed", dry, exists(t, orphanDir))
	}

	swept, err := w.Sweep(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	if swept != (Swept{Photos: 1, Bytes: 10}) {
		t.Errorf("swept %+v, want 1 photo, 10 bytes", swept)
	}
	if exists(t, orphanDir) {
		t.Error("old orphan was kept")
	}
	for name, dir := range map[string]string{"referenced": keptDir, "young": youngDir, "non-photo": notPhoto} {
		if !exists(t, dir) {
			t.Errorf("%s directory was removed", name)
		}
	}
}

func TestSweepRemovesNothingWithoutKnownPhotos(t *testing.T) {
	s := New(t.TempDir())
	dir := photoDir(t, s, another, time.Now().Add(-48*time.Hour))
	swept, err := s.Sweep(map[string]bool{}, time.Now(), false)
	if err != nil {
		t.Fatal(err)
	}
	if swept != (Swept{}) || !exists(t, dir) {
		t.Errorf("swept %+v with no known photos; want nothing removed", swept)
	}
}

func TestSweepWithoutPhotosDir(t *testing.T) {
	s := New(t.TempDir())
	if _, err := s.Sweep(map[string]bool{kept: true}, time.Now(), false); err != nil {
		t.Fatal(err)
	}
}
