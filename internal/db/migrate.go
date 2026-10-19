package db

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/clock"
	"github.com/NetworkBuild3r/hebrew-lessons/migrations"
)

// Migrate applies each embedded SQL file whose version is not in
// schema_migrations. Running it again is a no-op. applied_at is
// FormatTime(clk.Now()). A nil clock uses the system clock.
func Migrate(ctx context.Context, conn *sql.DB, clk clock.Clock) error {
	if conn == nil {
		return fmt.Errorf("db: migrate: nil connection")
	}
	if clk == nil {
		clk = clock.System()
	}
	if _, err := conn.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY, -- Version parsed from the SQL file name.
			applied_at TEXT NOT NULL -- UTC time from the process clock, written with FormatTime.
		)`); err != nil {
		return fmt.Errorf("db: create schema_migrations: %w", err)
	}

	entries, err := fs.ReadDir(migrations.FS, ".")
	if err != nil {
		return fmt.Errorf("db: read migrations: %w", err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		names = append(names, entry.Name())
	}
	sort.Strings(names)

	seen := map[int]string{}
	for _, name := range names {
		version, err := versionOf(name)
		if err != nil {
			return err
		}
		if prev, ok := seen[version]; ok {
			return fmt.Errorf("db: migration version %d is used by %s and %s", version, prev, name)
		}
		seen[version] = name

		var n int
		if err := conn.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM schema_migrations WHERE version = ?`, version,
		).Scan(&n); err != nil {
			return fmt.Errorf("db: read schema_migrations: %w", err)
		}
		if n > 0 {
			continue
		}
		body, err := fs.ReadFile(migrations.FS, name)
		if err != nil {
			return fmt.Errorf("db: read %s: %w", name, err)
		}
		if err := apply(ctx, conn, name, version, string(body), clk.Now()); err != nil {
			return err
		}
	}
	return nil
}

func apply(ctx context.Context, conn *sql.DB, name string, version int, script string, at time.Time) error {
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("db: begin %s: %w", name, err)
	}
	defer tx.Rollback()

	for _, stmt := range statements(script) {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("db: %s: %w", name, err)
		}
	}
	applied := FormatTime(at)
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO schema_migrations (version, applied_at) VALUES (?, ?)`,
		version, applied,
	); err != nil {
		return fmt.Errorf("db: record %s: %w", name, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("db: commit %s: %w", name, err)
	}
	return nil
}

func versionOf(name string) (int, error) {
	i := 0
	for i < len(name) && name[i] >= '0' && name[i] <= '9' {
		i++
	}
	if i == 0 {
		return 0, fmt.Errorf("db: migration %s: name must start with a version", name)
	}
	n, err := strconv.Atoi(name[:i])
	if err != nil {
		return 0, fmt.Errorf("db: migration %s: %w", name, err)
	}
	if n <= 0 {
		return 0, fmt.Errorf("db: migration %s: version must be positive", name)
	}
	return n, nil
}

// statements splits a SQL script on semicolons that are not inside quotes
// or line comments. Comment-only tails are dropped.
func statements(script string) []string {
	var out []string
	var b strings.Builder
	inSingle := false
	inDouble := false
	inLine := false
	for i := 0; i < len(script); i++ {
		c := script[i]
		if inLine {
			b.WriteByte(c)
			if c == '\n' {
				inLine = false
			}
			continue
		}
		if inSingle {
			b.WriteByte(c)
			if c == '\'' {
				if i+1 < len(script) && script[i+1] == '\'' {
					b.WriteByte('\'')
					i++
					continue
				}
				inSingle = false
			}
			continue
		}
		if inDouble {
			b.WriteByte(c)
			if c == '"' {
				inDouble = false
			}
			continue
		}
		if c == '-' && i+1 < len(script) && script[i+1] == '-' {
			inLine = true
			b.WriteByte(c)
			continue
		}
		if c == '\'' {
			inSingle = true
			b.WriteByte(c)
			continue
		}
		if c == '"' {
			inDouble = true
			b.WriteByte(c)
			continue
		}
		if c == ';' {
			if s := strings.TrimSpace(b.String()); meaningfulSQL(s) {
				out = append(out, s)
			}
			b.Reset()
			continue
		}
		b.WriteByte(c)
	}
	if s := strings.TrimSpace(b.String()); meaningfulSQL(s) {
		out = append(out, s)
	}
	return out
}

func meaningfulSQL(s string) bool {
	inLine := false
	for i := 0; i < len(s); i++ {
		if inLine {
			if s[i] == '\n' {
				inLine = false
			}
			continue
		}
		if s[i] == '-' && i+1 < len(s) && s[i+1] == '-' {
			inLine = true
			i++
			continue
		}
		switch s[i] {
		case ' ', '\n', '\t', '\r':
		default:
			return true
		}
	}
	return false
}
