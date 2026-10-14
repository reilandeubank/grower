// Package db opens the SQLite database, applies embedded migrations and exposes the
// sqlc-generated queries (package dbgen).
package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	"github.com/reilandeubank/grower/internal/db/dbgen"

	_ "modernc.org/sqlite" // registers the pure-Go "sqlite" driver
)

// FileName is the database file inside GROWER_DATA_DIR.
const FileName = "grower.db"

// readPoolSize bounds concurrent readers; WAL lets them run alongside the writer.
const readPoolSize = 4

// DB holds two pools over the same file. SQLite allows one writer at a time, so all
// writes go through a single connection (no SQLITE_BUSY between our own goroutines);
// reads use a small query_only pool.
type DB struct {
	Write *sql.DB
	Read  *sql.DB
}

// Open opens (creating if needed) <dataDir>/grower.db. It does not run migrations.
func Open(dataDir string) (*DB, error) {
	if err := os.MkdirAll(dataDir, 0o750); err != nil {
		return nil, fmt.Errorf("create data dir %q: %w", dataDir, err)
	}
	path := filepath.Join(dataDir, FileName)

	write, err := openPool(path, false)
	if err != nil {
		return nil, err
	}
	write.SetMaxOpenConns(1)

	read, err := openPool(path, true)
	if err != nil {
		_ = write.Close()
		return nil, err
	}
	read.SetMaxOpenConns(readPoolSize)

	return &DB{Write: write, Read: read}, nil
}

func openPool(path string, readOnly bool) (*sql.DB, error) {
	q := url.Values{}
	q.Add("_pragma", "busy_timeout(5000)")
	q.Add("_pragma", "journal_mode(WAL)")
	q.Add("_pragma", "foreign_keys(1)")
	q.Add("_pragma", "synchronous(NORMAL)")
	if readOnly {
		q.Add("_pragma", "query_only(1)")
	} else {
		// Take the write lock at BEGIN so a transaction never fails half-way on upgrade.
		q.Set("_txlock", "immediate")
	}
	dsn := "file:" + path + "?" + q.Encode()

	pool, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite %q: %w", path, err)
	}
	if err := pool.Ping(); err != nil {
		_ = pool.Close()
		return nil, fmt.Errorf("connect sqlite %q: %w", path, err)
	}
	return pool, nil
}

// Close closes both pools.
func (d *DB) Close() error {
	return errors.Join(d.Read.Close(), d.Write.Close())
}

// Ping checks that the database answers on the read pool.
func (d *DB) Ping(ctx context.Context) error {
	return d.Read.PingContext(ctx)
}

// Reader returns queries bound to the read pool.
func (d *DB) Reader() *dbgen.Queries { return dbgen.New(d.Read) }

// Writer returns queries bound to the single writer connection.
func (d *DB) Writer() *dbgen.Queries { return dbgen.New(d.Write) }

// InTx runs fn inside a write transaction, committing if fn returns nil.
func (d *DB) InTx(ctx context.Context, fn func(q *dbgen.Queries) error) error {
	tx, err := d.Write.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	if err := fn(dbgen.New(tx)); err != nil {
		_ = tx.Rollback()
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}
