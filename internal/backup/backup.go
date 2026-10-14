// Package backup writes the nightly consistent copy of the database to
// <data>/backups/grower-YYYY-MM-DD.db and keeps the newest few (docs/backup.md).
// Off-box sync copies those files; the live grower.db may be mid-write.
package backup

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/reilandeubank/grower/internal/db"
)

const (
	// DirName is the backups directory inside the data dir.
	DirName = "backups"
	// Keep is how many daily copies survive.
	Keep = 14
	// startHour is the local hour after which the day's copy is due: the quiet hours,
	// and after any late-night care logging.
	startHour = 3
	// checkEvery is how often the runner looks; the file name makes extra checks free.
	checkEvery = time.Hour
	dateLayout = "2006-01-02"
	prefix     = "grower-"
	suffix     = ".db"
)

// Runner makes the daily backup. Now and Loc decide which day it is.
type Runner struct {
	DB      *db.DB
	DataDir string
	Loc     *time.Location
	Now     func() time.Time
	Log     *slog.Logger
}

// Run checks once now and then hourly until ctx is cancelled. Failures are logged
// and retried on the next check.
func (r *Runner) Run(ctx context.Context) {
	t := time.NewTicker(checkEvery)
	defer t.Stop()
	for {
		if err := r.Check(ctx); err != nil && ctx.Err() == nil {
			r.Log.Error("nightly backup failed", "err", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

// Check makes today's backup if it is due: none exists for today and it is past
// 03:00 local time, or there is no backup at all yet.
func (r *Runner) Check(ctx context.Context) error {
	dir := filepath.Join(r.DataDir, DirName)
	names, err := list(dir)
	if err != nil {
		return err
	}
	now := r.Now().In(r.Loc)
	name := prefix + now.Format(dateLayout) + suffix
	if slices.Contains(names, name) || (now.Hour() < startHour && len(names) > 0) {
		return nil
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("create backups dir: %w", err)
	}

	start := time.Now()
	size, err := r.write(ctx, filepath.Join(dir, name))
	if err != nil {
		return err
	}
	r.Log.Info("nightly backup written", "file", name, "bytes", size, "duration", time.Since(start))
	return prune(dir, append(names, name))
}

// write copies the database with VACUUM INTO, a consistent snapshot even while the
// app writes, then renames it into place so a partial file never has the final name.
func (r *Runner) write(ctx context.Context, path string) (int64, error) {
	tmp := filepath.Join(filepath.Dir(path), "."+filepath.Base(path)+".tmp")
	_ = os.Remove(tmp) // left by a crash; VACUUM INTO refuses an existing file
	if _, err := r.DB.Write.ExecContext(ctx, "VACUUM INTO ?", tmp); err != nil {
		_ = os.Remove(tmp)
		return 0, fmt.Errorf("vacuum into %s: %w", tmp, err)
	}
	size, err := syncFile(tmp)
	if err == nil {
		err = os.Rename(tmp, path)
	}
	if err != nil {
		_ = os.Remove(tmp)
		return 0, fmt.Errorf("finish backup %s: %w", path, err)
	}
	return size, nil
}

// syncFile flushes a file to disk before it is published by rename.
func syncFile(path string) (int64, error) {
	f, err := os.Open(path) //nolint:gosec // G304: our own temp path
	if err != nil {
		return 0, err
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return 0, err
	}
	return fi.Size(), f.Sync()
}

// prune deletes all but the newest Keep backups. Names sort by date.
func prune(dir string, names []string) error {
	slices.Sort(names)
	var errs []error
	for _, n := range names[:max(0, len(names)-Keep)] {
		errs = append(errs, os.Remove(filepath.Join(dir, n)))
	}
	if err := errors.Join(errs...); err != nil {
		return fmt.Errorf("prune old backups: %w", err)
	}
	return nil
}

// list returns the backup file names in dir (none if it does not exist).
func list(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("list backups: %w", err)
	}
	var names []string
	for _, e := range entries {
		if n := e.Name(); e.Type().IsRegular() && isBackup(n) {
			names = append(names, n)
		}
	}
	return names, nil
}

func isBackup(name string) bool {
	date, hasPrefix := strings.CutPrefix(name, prefix)
	date, hasSuffix := strings.CutSuffix(date, suffix)
	_, err := time.Parse(dateLayout, date)
	return hasPrefix && hasSuffix && err == nil
}

// Latest is when the newest backup in <dataDir>/backups was written, or ok=false when
// there is none.
func Latest(dataDir string) (t time.Time, ok bool, err error) {
	dir := filepath.Join(dataDir, DirName)
	names, err := list(dir)
	if err != nil {
		return time.Time{}, false, err
	}
	for _, n := range names {
		fi, err := os.Stat(filepath.Join(dir, n))
		if err != nil {
			return time.Time{}, false, fmt.Errorf("stat backup: %w", err)
		}
		if fi.ModTime().After(t) {
			t, ok = fi.ModTime(), true
		}
	}
	return t.UTC(), ok, nil
}
