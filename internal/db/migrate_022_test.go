package db

import (
	"context"
	"database/sql"
	"io/fs"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/migrations"
)

const (
	m22Version = 22
	m22Seeded  = "2020-01-01T00:00:00Z"
)

// m22At021 builds a database at version 021 with a daily_snapshots row, then stops before 022.
func m22At021(t *testing.T) *sql.DB {
	t.Helper()
	ctx := context.Background()
	conn := openRaw(t)
	if _, err := conn.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			applied_at TEXT NOT NULL
		)`); err != nil {
		t.Fatal(err)
	}
	entries, err := fs.ReadDir(migrations.FS, ".")
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".sql") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	for _, name := range names {
		version, err := versionOf(name)
		if err != nil {
			t.Fatal(err)
		}
		if version >= m22Version {
			continue
		}
		body, err := fs.ReadFile(migrations.FS, name)
		if err != nil {
			t.Fatal(err)
		}
		if err := apply(ctx, conn, name, version, string(body), time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)); err != nil {
			t.Fatalf("apply %s: %v", name, err)
		}
	}
	want := make([]int, 0, 21)
	for i := 1; i <= 21; i++ {
		want = append(want, i)
	}
	if got := m6Applied(t, conn); !reflect.DeepEqual(got, want) {
		t.Fatalf("seed database versions = %v, want 1..21", got)
	}
	mustSQL(t, conn, `INSERT INTO users (email, google_sub, role, created_at) VALUES (?, ?, 'learner', ?)`,
		"a@example.com", "sub-a", m22Seeded)
	mustSQL(t, conn, `INSERT INTO daily_snapshots (user_id, day, skill, state, count, est_known, retention)
		VALUES (1, '2026-11-01', 'read', 'known', 4, 4.0, NULL)`)
	return conn
}

func m22Migrate(conn *sql.DB) error {
	return Migrate(context.Background(), conn, fixedClock{time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)})
}

func TestMigrate022KeepsRowsAndDefaults(t *testing.T) {
	conn := m22At021(t)
	before := m6Dump(t, conn, `SELECT user_id, day, skill, state, count, est_known FROM daily_snapshots ORDER BY user_id, day, skill, state`)
	if err := m22Migrate(conn); err != nil {
		t.Fatalf("migrate 022: %v", err)
	}
	want := make([]int, 0, 22)
	for i := 1; i <= 22; i++ {
		want = append(want, i)
	}
	if got := m6Applied(t, conn); !reflect.DeepEqual(got, want) {
		t.Fatalf("versions after = %v, want 1..22", got)
	}
	if got := m6Dump(t, conn, `SELECT user_id, day, skill, state, count, est_known FROM daily_snapshots ORDER BY user_id, day, skill, state`); !reflect.DeepEqual(got, before) {
		t.Fatalf("snapshot core columns changed\n got: %v\nwant: %v", got, before)
	}
	rv := m6Dump(t, conn, `SELECT rule_version FROM daily_snapshots`)
	if !reflect.DeepEqual(rv, [][]string{{"int64:1"}}) {
		t.Fatalf("rule_version = %v, want 1 for the old row", rv)
	}
	if n := m6One(t, conn, `SELECT COUNT(*) FROM daily_totals`); n != 0 {
		t.Fatalf("daily_totals has %d rows after migrate, want 0", n)
	}
	cols := m6Dump(t, conn, `SELECT name FROM pragma_table_info('daily_snapshots') WHERE name = 'rule_version'`)
	if len(cols) != 1 {
		t.Fatalf("daily_snapshots.rule_version missing: %v", cols)
	}
	if fk := m6Dump(t, conn, `PRAGMA foreign_key_check`); len(fk) != 0 {
		t.Fatalf("foreign_key_check: %v", fk)
	}
}

func TestMigrate022FreshMatchesMigrated(t *testing.T) {
	migrated := m22At021(t)
	if err := m22Migrate(migrated); err != nil {
		t.Fatal(err)
	}
	fresh := openRaw(t)
	if err := m22Migrate(fresh); err != nil {
		t.Fatal(err)
	}
	snap := func(conn *sql.DB) map[string]any {
		return map[string]any{
			"snap_cols": m6Dump(t, conn, `SELECT name, type, "notnull", dflt_value, pk FROM pragma_table_info('daily_snapshots') ORDER BY cid`),
			"tot_cols":  m6Dump(t, conn, `SELECT name, type, "notnull", dflt_value, pk FROM pragma_table_info('daily_totals') ORDER BY cid`),
			"tot_sql":   m6SQLOf(t, conn, "daily_totals"),
		}
	}
	got, want := snap(migrated), snap(fresh)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("migrated schema != fresh schema\n migrated: %#v\n fresh: %#v", got, want)
	}
}

func TestMigrate022Constraints(t *testing.T) {
	conn := m22At021(t)
	if err := m22Migrate(conn); err != nil {
		t.Fatal(err)
	}
	mustSQL(t, conn, `INSERT INTO daily_totals (user_id, day, known_any_skill, rule_version) VALUES (1, '2026-11-02', 0, 1)`)
	rejectSQL(t, conn, "negative known_any",
		`INSERT INTO daily_totals (user_id, day, known_any_skill, rule_version) VALUES (1, '2026-11-03', -1, 1)`)
	rejectSQL(t, conn, "rule_version 0",
		`INSERT INTO daily_totals (user_id, day, known_any_skill, rule_version) VALUES (1, '2026-11-03', 1, 0)`)
	rejectSQL(t, conn, "bad day",
		`INSERT INTO daily_totals (user_id, day, known_any_skill, rule_version) VALUES (1, '2026-13-01', 1, 1)`)
}
