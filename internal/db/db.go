// Package db opens hebrew.sqlite and applies the embedded migrations.
//
// The pool is one connection. SQLite has one writer, and a single connection
// keeps the foreign_keys and busy_timeout pragmas on every statement.
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

	"github.com/NetworkBuild3r/hebrew-lessons/internal/clock"

	_ "modernc.org/sqlite"
)

const busyTimeoutMS = 5000

// ErrNotFound is returned when a user or session row is not in the database.
var ErrNotFound = errors.New("db: not found")

// Store is the single writer connection for the process.
type Store struct {
	conn  *sql.DB
	clock clock.Clock
}

// Open creates the database file if needed, checks the SQLite pragmas, and
// applies any migrations that are not yet recorded. clk may be nil; the
// system clock is used then. Timestamps written by Migrate come from clk.
func Open(ctx context.Context, path string, clk clock.Clock) (*Store, error) {
	conn, err := openConn(ctx, path)
	if err != nil {
		return nil, err
	}
	if err := Migrate(ctx, conn, clk); err != nil {
		conn.Close()
		return nil, err
	}
	if clk == nil {
		clk = clock.System()
	}
	return &Store{conn: conn, clock: clk}, nil
}

// Clock returns the clock the store was opened with.
func (s *Store) Clock() clock.Clock { return s.clock }

// Ping reports whether the database answers.
func (s *Store) Ping(ctx context.Context) error {
	if s == nil || s.conn == nil {
		return errors.New("db: store is closed")
	}
	return s.conn.PingContext(ctx)
}

// DB returns the process connection.
// internal/mcpstore runs the authoring statements on it so this package
// does not import internal/mcp. Callers share the one-connection pool.
func (s *Store) DB() (*sql.DB, error) {
	return s.connOrErr()
}

// Close releases the connection. A later Ping fails, which is how /healthz
// reports the database as down.
func (s *Store) Close() error {
	if s == nil || s.conn == nil {
		return nil
	}
	err := s.conn.Close()
	s.conn = nil
	return err
}

func openConn(ctx context.Context, path string) (*sql.DB, error) {
	if path == "" {
		return nil, errors.New("db: path is empty")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return nil, fmt.Errorf("db: create directory for %s: %w", abs, err)
	}

	conn, err := sql.Open("sqlite", sqliteDSN(abs))
	if err != nil {
		return nil, fmt.Errorf("db: open %s: %w", abs, err)
	}
	// One writer connection. WAL would allow extra readers later; this
	// process keeps a single connection so locks and pragmas stay on it.
	conn.SetMaxOpenConns(1)
	conn.SetMaxIdleConns(1)

	if err := conn.PingContext(ctx); err != nil {
		conn.Close()
		return nil, fmt.Errorf("db: open %s: %w", abs, err)
	}
	if err := verifyPragmas(ctx, conn); err != nil {
		conn.Close()
		return nil, fmt.Errorf("db: open %s: %w", abs, err)
	}
	return conn, nil
}

func sqliteDSN(abs string) string {
	u := url.URL{
		Scheme: "file",
		Path:   filepath.ToSlash(abs),
	}
	q := url.Values{}
	q.Set("mode", "rwc")
	q.Set("_busy_timeout", strconv.Itoa(busyTimeoutMS))
	q.Set("_foreign_keys", "on")
	q.Set("_journal_mode", "WAL")
	// BEGIN IMMEDIATE. A deferred transaction can pass a read and then fail
	// with SQLITE_BUSY when it upgrades to a write. Lesson starts rely on that
	// lock so two connections cannot both miss the open run.
	q.Set("_txlock", "immediate")
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

// exec is used by tests in this package. Production queries will be methods
// on Store as the features land.
func (s *Store) exec(ctx context.Context, query string, args ...any) (sql.Result, error) {
	if s == nil || s.conn == nil {
		return nil, errors.New("db: store is closed")
	}
	return s.conn.ExecContext(ctx, query, args...)
}

// ExecContext runs one statement on the store connection.
// internal/db/testharness uses it to insert fixture rows. The server does not call it.
func (s *Store) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return s.exec(ctx, query, args...)
}

func (s *Store) queryRow(ctx context.Context, query string, args ...any) *sql.Row {
	return s.conn.QueryRowContext(ctx, query, args...)
}

// maxOpenConns is checked by tests. sql.DB.Stats is the public source.
func (s *Store) maxOpenConns() int {
	if s == nil || s.conn == nil {
		return 0
	}
	return s.conn.Stats().MaxOpenConnections
}
