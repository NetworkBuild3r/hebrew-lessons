// Package db opens hebrew.sqlite read-only and checks the migration ledger.
// There is no write path and no migration runner. ADR-0016: Go does not share
// the writer with Node until the storage class is confirmed.
package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

const (
	busyTimeoutMS = 5000
	openTimeout   = 15 * time.Second
)

// OpenReadOnly opens path with SQLite mode=ro. A missing file is an error and
// is not created. The connection uses the same PRAGMAs as server/db/index.js
// open(): journal_mode=WAL, foreign_keys=ON, busy_timeout=5000. journal_mode
// is persistent, so a file that is not already WAL is refused rather than
// converted.
func OpenReadOnly(path string) (*sql.DB, error) {
	if path == "" {
		return nil, errors.New("db: path is empty")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return nil, fmt.Errorf("db: open read-only %s: %w", abs, err)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("db: open read-only %s: is a directory", abs)
	}

	conn, err := sql.Open("sqlite", readOnlyDSN(abs))
	if err != nil {
		return nil, fmt.Errorf("db: open read-only %s: %w", abs, err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), openTimeout)
	defer cancel()
	if err := conn.PingContext(ctx); err != nil {
		conn.Close()
		return nil, fmt.Errorf("db: open read-only %s: %w", abs, err)
	}
	if err := verifyPragmas(ctx, conn); err != nil {
		conn.Close()
		return nil, fmt.Errorf("db: open read-only %s: %w", abs, err)
	}
	return conn, nil
}

func readOnlyDSN(abs string) string {
	u := url.URL{
		Scheme: "file",
		Path:   filepath.ToSlash(abs),
	}
	q := url.Values{}
	q.Set("mode", "ro")
	q.Set("_busy_timeout", strconv.Itoa(busyTimeoutMS))
	q.Set("_foreign_keys", "on")
	q.Set("_journal_mode", "WAL")
	u.RawQuery = q.Encode()
	return u.String()
}

func verifyPragmas(ctx context.Context, conn *sql.DB) error {
	var foreignKeys int
	if err := conn.QueryRowContext(ctx, "PRAGMA foreign_keys").Scan(&foreignKeys); err != nil {
		return fmt.Errorf("read foreign_keys: %w", err)
	}
	if foreignKeys != 1 {
		return fmt.Errorf("foreign_keys=%d, want 1", foreignKeys)
	}
	var busy int
	if err := conn.QueryRowContext(ctx, "PRAGMA busy_timeout").Scan(&busy); err != nil {
		return fmt.Errorf("read busy_timeout: %w", err)
	}
	if busy != busyTimeoutMS {
		return fmt.Errorf("busy_timeout=%d, want %d", busy, busyTimeoutMS)
	}
	var journal string
	if err := conn.QueryRowContext(ctx, "PRAGMA journal_mode").Scan(&journal); err != nil {
		return fmt.Errorf("read journal_mode: %w", err)
	}
	if !strings.EqualFold(journal, "wal") {
		return fmt.Errorf("journal_mode=%s, want wal", journal)
	}
	return nil
}
