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
	for i := 1; i <= 25; i++ {
		want = append(want, i)
	}
	if got := m6Applied(t, conn); !reflect.DeepEqual(got, want) {
		t.Fatalf("versions after = %v, want 1..25", got)
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
			"snap_cols":    m6Dump(t, conn, `SELECT name, type, "notnull", dflt_value, pk FROM pragma_table_info('daily_snapshots') ORDER BY cid`),
			"tot_cols":     m6Dump(t, conn, `SELECT name, type, "notnull", dflt_value, pk FROM pragma_table_info('daily_totals') ORDER BY cid`),
			"tot_sql":      m6SQLOf(t, conn, "daily_totals"),
			"card_created": m6Dump(t, conn, `SELECT name, type, "notnull", dflt_value, pk FROM pragma_table_info('cards') WHERE name = 'created_at'`),
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

func TestMigrate022BackfillsCardCreatedAt(t *testing.T) {
	conn := m22At021(t)
	mustSQL(t, conn, `INSERT INTO lexemes (text_nfc, gloss_en) VALUES ('מַיִם', 'water')`)
	mustSQL(t, conn, `INSERT INTO lexemes (text_nfc, gloss_en) VALUES ('אֶרֶץ', 'land')`)
	// Card with a review: created_at backfills from review_log.
	mustSQL(t, conn, `INSERT INTO cards (user_id, lexeme_id, skill, state, due, stability, difficulty, reps, lapses, suspended)
		VALUES (1, 1, 'read', 'review', ?, 1, 5, 1, 0, 0)`, "2026-11-01T12:00:00Z")
	mustSQL(t, conn, `INSERT INTO review_log (card_id, rating, reviewed_at, elapsed_days, scheduled_days, implied)
		VALUES (1, 3, ?, 1, 2, 0)`, "2026-11-01T10:00:00Z")
	mustSQL(t, conn, `INSERT INTO review_log (card_id, rating, reviewed_at, elapsed_days, scheduled_days, implied)
		VALUES (1, 3, ?, 1, 3, 0)`, "2026-11-02T10:00:00Z")
	// An exposure and an attempt days before the first review must not backdate the card.
	mustSQL(t, conn, `INSERT INTO exposures (user_id, lexeme_id, context, created_at) VALUES (1, 1, 'lesson_intro', ?)`, "2026-10-20T10:00:00Z")
	mustSQL(t, conn, `INSERT INTO attempts (user_id, lexeme_id, skill, activity, source, prompt, response, correct, latency_ms, created_at) VALUES (1, 1, 'read', 'read_pick', 'lesson', 'water', 'water', 1, 900, ?)`, "2026-10-21T10:00:00Z")
	// Card with no review: created_at is the migration's own time, so it is left out of
	// every earlier day and kept in every later one.
	mustSQL(t, conn, `INSERT INTO cards (user_id, lexeme_id, skill, state, due, stability, difficulty, reps, lapses, suspended)
		VALUES (1, 2, 'read', 'new', ?, 0, 0, 0, 0, 0)`, "2026-11-03T12:00:00Z")

	if err := m22Migrate(conn); err != nil {
		t.Fatal(err)
	}
	got := m6Dump(t, conn, `SELECT id, created_at FROM cards WHERE id = 1`)
	if want := [][]string{{"int64:1", "string:2026-11-01T10:00:00Z"}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("reviewed card created_at = %v, want %v (first review, not the earlier exposure)", got, want)
	}
	var unreviewed sql.NullString
	if err := conn.QueryRow(`SELECT created_at FROM cards WHERE id = 2`).Scan(&unreviewed); err != nil {
		t.Fatal(err)
	}
	if !unreviewed.Valid {
		t.Fatal("unreviewed card created_at is NULL; every as-of read, including later days, would leave it out")
	}
	at, err := ParseTime(unreviewed.String)
	if err != nil {
		t.Fatalf("unreviewed card created_at %q is not in the FormatTime layout: %v", unreviewed.String, err)
	}
	if FormatTime(at) != unreviewed.String {
		t.Fatalf("unreviewed card created_at %q does not round-trip FormatTime (%q)", unreviewed.String, FormatTime(at))
	}
	if !at.After(time.Date(2026, 11, 2, 10, 0, 0, 0, time.UTC)) {
		t.Fatalf("unreviewed card created_at %s is not the migration time", at)
	}
	cols := m6Dump(t, conn, `SELECT name FROM pragma_table_info('cards') WHERE name = 'created_at'`)
	if len(cols) != 1 {
		t.Fatalf("cards.created_at missing: %v", cols)
	}
}
