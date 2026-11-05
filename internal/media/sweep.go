package media

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"time"
)

// SweepGrace is how old an unreferenced photo directory must be before Sweep removes it.
// Uploads and imports write the files before the photo's row commits, so a younger one
// may be in flight.
const SweepGrace = time.Hour

// sweepEvery is how often the Sweeper runs after its first sweep at startup.
const sweepEvery = 24 * time.Hour

// Swept is what a sweep removed (or, in a dry run, would remove).
type Swept struct {
	Photos int
	Bytes  int64
}

// Sweep removes photo directories whose id is not in known and that were last modified
// before cutoff: files left behind when a delete's removal failed or the process stopped
// after the row was gone. With dryRun it only reports them. Anything that isn't a photo
// directory is left alone. With no known photos at all it removes nothing: an empty photos
// table beside stored photos more likely means a database not restored yet than that every
// photo is orphaned.
func (s *Store) Sweep(known map[string]bool, cutoff time.Time, dryRun bool) (Swept, error) {
	var out Swept
	if len(known) == 0 {
		return out, nil
	}
	dir := filepath.Join(s.dir, "photos")
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return out, nil
	}
	if err != nil {
		return out, fmt.Errorf("list photos: %w", err)
	}
	var errs []error
	for _, e := range entries {
		id := e.Name()
		if !e.IsDir() || !validID(id) || known[id] {
			continue
		}
		fi, err := e.Info()
		if errors.Is(err, fs.ErrNotExist) {
			continue // removed meanwhile
		}
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if !fi.ModTime().Before(cutoff) {
			continue
		}
		size, err := dirSize(filepath.Join(dir, id))
		if err == nil && !dryRun {
			err = s.Remove(id)
		}
		if err != nil {
			errs = append(errs, err)
			continue
		}
		out.Photos++
		out.Bytes += size
	}
	return out, errors.Join(errs...)
}

func dirSize(dir string) (int64, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, err
	}
	var total int64
	for _, e := range entries {
		if fi, err := e.Info(); err == nil && !e.IsDir() {
			total += fi.Size()
		}
	}
	return total, nil
}

// Sweeper runs Sweep at startup, when a crash's leftovers appear, and then daily.
type Sweeper struct {
	Store *Store
	// PhotoIDs lists every photo row's id.
	PhotoIDs func(context.Context) ([]string, error)
	Now      func() time.Time
	Log      *slog.Logger
}

// Run sweeps once now and then daily until ctx is cancelled. Failures are logged and
// retried on the next run.
func (w *Sweeper) Run(ctx context.Context) {
	t := time.NewTicker(sweepEvery)
	defer t.Stop()
	for {
		swept, err := w.Sweep(ctx, false)
		if swept.Photos > 0 {
			w.Log.Info("removed orphaned photo files", "photos", swept.Photos, "bytes", swept.Bytes)
		}
		if err != nil && ctx.Err() == nil {
			w.Log.Error("sweep orphaned photos", "err", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

// Sweep runs one sweep against the current photo rows.
func (w *Sweeper) Sweep(ctx context.Context, dryRun bool) (Swept, error) {
	ids, err := w.PhotoIDs(ctx)
	if err != nil {
		return Swept{}, fmt.Errorf("list photo ids: %w", err)
	}
	known := make(map[string]bool, len(ids))
	for _, id := range ids {
		known[id] = true
	}
	return w.Store.Sweep(known, w.Now().Add(-SweepGrace), dryRun)
}
