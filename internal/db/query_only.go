package db

// OpenQueryOnly opens an existing hebrew.sqlite for operator audits
// (INIT-032/SPEC-009). It never creates the file, never migrates, and sets
// PRAGMA query_only = ON so a write attempt fails.

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/clock"
)

// OpenQueryOnly opens path for read-only operator use. The file must already
// exist. Migrations are not applied. PRAGMA query_only is on for the life of
// the connection: INSERT/UPDATE/DELETE fail.
func OpenQueryOnly(ctx context.Context, path string, clk clock.Clock) (*Store, error) {
	if path == "" {
		return nil, fmt.Errorf("db: path is empty")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return nil, fmt.Errorf("db: %w", err)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("db: %s is not a regular file", abs)
	}

	u := url.URL{Scheme: "file", Path: filepath.ToSlash(abs)}
	q := url.Values{}
	// mode=rw (not rwc): refuse to create. query_only then blocks writes.
	q.Set("mode", "rw")
	q.Set("_busy_timeout", strconv.Itoa(busyTimeoutMS))
	q.Set("_foreign_keys", "on")
	u.RawQuery = q.Encode()

	conn, err := sql.Open("sqlite", u.String())
	if err != nil {
		return nil, fmt.Errorf("db: open query-only %s: %w", abs, err)
	}
	conn.SetMaxOpenConns(1)
	conn.SetMaxIdleConns(1)
	if err := conn.PingContext(ctx); err != nil {
		conn.Close()
		return nil, fmt.Errorf("db: open query-only %s: %w", abs, err)
	}
	if _, err := conn.ExecContext(ctx, `PRAGMA query_only = ON`); err != nil {
		conn.Close()
		return nil, fmt.Errorf("db: PRAGMA query_only: %w", err)
	}
	var only int
	if err := conn.QueryRowContext(ctx, `PRAGMA query_only`).Scan(&only); err != nil {
		conn.Close()
		return nil, fmt.Errorf("db: read query_only: %w", err)
	}
	if only != 1 {
		conn.Close()
		return nil, fmt.Errorf("db: query_only=%d, want 1", only)
	}
	if clk == nil {
		clk = clock.System()
	}
	return &Store{conn: conn, clock: clk}, nil
}

// CountCardsBySkill returns how many FSRS cards this learner has in one skill.
func (s *Store) CountCardsBySkill(ctx context.Context, userID int64, skill string) (int, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return 0, err
	}
	var n int
	if err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM cards WHERE user_id = ? AND skill = ?`, userID, skill).Scan(&n); err != nil {
		return 0, fmt.Errorf("db: count cards by skill: %w", err)
	}
	return n, nil
}
