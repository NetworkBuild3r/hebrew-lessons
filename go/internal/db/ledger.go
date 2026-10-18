package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

// Migration is one schema_migrations row. Names are the Node filenames.
type Migration struct {
	Version int    `json:"version"`
	Name    string `json:"name"`
}

// ExpectedMigrations is versions 1–12 with the names in server/db/migrations.
// Go runs nothing when this ledger is already present.
var ExpectedMigrations = []Migration{
	{1, "001-initial.js"},
	{2, "002-plan-store.js"},
	{3, "003-media-and-ai-planning.js"},
	{4, "004-fsrs-review-state.js"},
	{5, "005-lexicon-library-mastery.js"},
	{6, "006-plan-revisions.js"},
	{7, "007-plan-revision-lesson-ops.js"},
	{8, "008-dictionary-glosses.js"},
	{9, "009-dictionary-entries.js"},
	{10, "010-lexicon-future-cells.js"},
	{11, "011-google-email.js"},
	{12, "012-assignee-username.js"},
}

// Ledger is the result of reading schema_migrations.
type Ledger struct {
	OK       bool        `json:"ok"`
	Versions []Migration `json:"versions,omitempty"`
	Error    string      `json:"error,omitempty"`
}

// CheckLedger reads schema_migrations and requires exactly ExpectedMigrations.
// A mismatch returns a Ledger with OK false and a non-nil error. The query is
// read-only.
func CheckLedger(ctx context.Context, conn *sql.DB) (Ledger, error) {
	if conn == nil {
		err := errors.New("db: nil connection")
		return Ledger{OK: false, Error: err.Error()}, err
	}
	if ctx == nil {
		err := errors.New("db: nil context")
		return Ledger{OK: false, Error: err.Error()}, err
	}

	var tables int
	err := conn.QueryRowContext(ctx,
		`SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name = 'schema_migrations'`,
	).Scan(&tables)
	if err != nil {
		return failLedger(fmt.Errorf("db: read sqlite_master: %w", err))
	}
	if tables == 0 {
		return failLedger(fmt.Errorf("schema_migrations table is missing; want %s", wantSummary()))
	}

	rows, err := conn.QueryContext(ctx, `SELECT version, name FROM schema_migrations ORDER BY version`)
	if err != nil {
		return failLedger(fmt.Errorf("db: read schema_migrations: %w", err))
	}
	defer rows.Close()

	var got []Migration
	seen := map[int]string{}
	for rows.Next() {
		var row Migration
		if err := rows.Scan(&row.Version, &row.Name); err != nil {
			return failLedger(fmt.Errorf("db: read schema_migrations: %w", err))
		}
		if _, ok := seen[row.Version]; ok {
			return failLedger(fmt.Errorf("schema_migrations has duplicate version %d", row.Version))
		}
		seen[row.Version] = row.Name
		got = append(got, row)
	}
	if err := rows.Err(); err != nil {
		return failLedger(fmt.Errorf("db: read schema_migrations: %w", err))
	}
	if len(got) == 0 {
		return failLedger(fmt.Errorf("schema_migrations has no rows; want %s", wantSummary()))
	}

	want := map[int]string{}
	for _, row := range ExpectedMigrations {
		want[row.Version] = row.Name
	}
	var problems []string
	for _, row := range ExpectedMigrations {
		name, ok := seen[row.Version]
		if !ok {
			problems = append(problems, fmt.Sprintf("missing version %d (%s)", row.Version, row.Name))
			continue
		}
		if name != row.Name {
			problems = append(problems, fmt.Sprintf("version %d name is %q, want %q", row.Version, name, row.Name))
		}
	}
	for _, row := range got {
		if _, ok := want[row.Version]; ok {
			continue
		}
		problems = append(problems, fmt.Sprintf("unexpected version %d (%s)", row.Version, row.Name))
	}
	if len(problems) > 0 {
		err := fmt.Errorf("schema_migrations mismatch: %s", strings.Join(problems, "; "))
		return Ledger{OK: false, Versions: got, Error: err.Error()}, err
	}
	return Ledger{OK: true, Versions: got}, nil
}

func failLedger(err error) (Ledger, error) {
	return Ledger{OK: false, Error: err.Error()}, err
}

func wantSummary() string {
	first := ExpectedMigrations[0]
	last := ExpectedMigrations[len(ExpectedMigrations)-1]
	return fmt.Sprintf("versions %d-%d (%s through %s)", first.Version, last.Version, first.Name, last.Name)
}
