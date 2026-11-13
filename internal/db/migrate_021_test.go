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
	m21Version = 21
	m21Seeded  = "2020-01-01T00:00:00Z"
)

// m21At020 builds a database at version 020 with a user, a lexeme, a card, a
// review_log row, and a confusion miss, then stops before 021.
func m21At020(t *testing.T) *sql.DB {
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
		if version >= m21Version {
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
	want := make([]int, 0, 20)
	for i := 1; i <= 20; i++ {
		want = append(want, i)
	}
	if got := m6Applied(t, conn); !reflect.DeepEqual(got, want) {
		t.Fatalf("seed database versions = %v, want 1..20", got)
	}
	mustSQL(t, conn, `INSERT INTO users (email, google_sub, role, created_at) VALUES (?, ?, 'learner', ?)`,
		"a@example.com", "sub-a", m21Seeded)
	mustSQL(t, conn, `INSERT INTO users (email, google_sub, role, created_at) VALUES (?, ?, 'learner', ?)`,
		"b@example.com", "sub-b", m21Seeded)
	mustSQL(t, conn, `INSERT INTO lexemes (text_nfc) VALUES (?)`, "שָׁלוֹם")
	mustSQL(t, conn, `INSERT INTO cards (
		user_id, lexeme_id, skill, state, due, stability, difficulty, reps, lapses, suspended
	) VALUES (1, 1, 'read', 'review', ?, 1.0, 5.0, 1, 0, 0)`, m21Seeded)
	mustSQL(t, conn, `INSERT INTO review_log (card_id, rating, reviewed_at, elapsed_days, scheduled_days)
		VALUES (1, 3, ?, 0, 1)`, m21Seeded)
	mustSQL(t, conn, `INSERT INTO confusion_misses (learner_id, kind, key, at)
		VALUES (1, 'letter_pair', 'ב/כ', ?)`, m21Seeded)
	return conn
}

func m21Migrate(conn *sql.DB) error {
	return Migrate(context.Background(), conn, fixedClock{time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)})
}

func m21SchemaSnapshot(t *testing.T, conn *sql.DB) map[string]any {
	t.Helper()
	out := map[string]any{}
	out["drill_cols"] = m6Dump(t, conn, `SELECT name, type, "notnull", dflt_value, pk FROM pragma_table_info('drill_answers') ORDER BY cid`)
	out["clear_cols"] = m6Dump(t, conn, `SELECT name, type, "notnull", dflt_value, pk FROM pragma_table_info('confusion_clears') ORDER BY cid`)
	out["log_cols"] = m6Dump(t, conn, `SELECT name, type, "notnull", dflt_value, pk FROM pragma_table_info('review_log') ORDER BY cid`)
	out["drill_idx"] = m6Dump(t, conn, `SELECT name FROM pragma_index_info('drill_answers_learner_kind_key_at') ORDER BY seqno`)
	out["clear_idx"] = m6Dump(t, conn, `SELECT name FROM pragma_index_info('confusion_clears_learner_kind_key_at') ORDER BY seqno`)
	out["drill_sql"] = m6SQLOf(t, conn, "drill_answers")
	out["clear_sql"] = m6SQLOf(t, conn, "confusion_clears")
	out["log_sql"] = m6SQLOf(t, conn, "review_log")
	out["drill_checks"] = strings.Contains(out["drill_sql"].(string), "letter_pair")
	out["clear_checks"] = strings.Contains(out["clear_sql"].(string), "agreement")
	out["log_implied"] = strings.Contains(out["log_sql"].(string), "implied") ||
		len(m6Dump(t, conn, `SELECT name FROM pragma_table_info('review_log') WHERE name = 'implied'`)) == 1
	return out
}

// ac-1: a database at 020 migrates to 021 and keeps every row. Old review_log
// rows read implied = 0. The new tables are empty.
func TestMigrate021KeepsRowsAndDefaults(t *testing.T) {
	conn := m21At020(t)
	usersBefore := m6Dump(t, conn, `SELECT id, email, google_sub, role, created_at FROM users ORDER BY id`)
	cardsBefore := m6Dump(t, conn, `SELECT id, user_id, lexeme_id, skill, state FROM cards ORDER BY id`)
	logBefore := m6Dump(t, conn, `SELECT id, card_id, rating, reviewed_at, elapsed_days, scheduled_days FROM review_log ORDER BY id`)
	missBefore := m6Dump(t, conn, `SELECT id, learner_id, kind, key, at FROM confusion_misses ORDER BY id`)

	if err := m21Migrate(conn); err != nil {
		t.Fatalf("migrate 021: %v", err)
	}
	want := make([]int, 0, 21)
	for i := 1; i <= 27; i++ {
		want = append(want, i)
	}
	if got := m6Applied(t, conn); !reflect.DeepEqual(got, want) {
		t.Fatalf("versions after = %v, want 1..27", got)
	}

	if got := m6Dump(t, conn, `SELECT id, email, google_sub, role, created_at FROM users ORDER BY id`); !reflect.DeepEqual(got, usersBefore) {
		t.Fatalf("users changed\n got: %v\nwant: %v", got, usersBefore)
	}
	if got := m6Dump(t, conn, `SELECT id, user_id, lexeme_id, skill, state FROM cards ORDER BY id`); !reflect.DeepEqual(got, cardsBefore) {
		t.Fatalf("cards changed\n got: %v\nwant: %v", got, cardsBefore)
	}
	if got := m6Dump(t, conn, `SELECT id, card_id, rating, reviewed_at, elapsed_days, scheduled_days FROM review_log ORDER BY id`); !reflect.DeepEqual(got, logBefore) {
		t.Fatalf("review_log core columns changed\n got: %v\nwant: %v", got, logBefore)
	}
	if got := m6Dump(t, conn, `SELECT id, learner_id, kind, key, at FROM confusion_misses ORDER BY id`); !reflect.DeepEqual(got, missBefore) {
		t.Fatalf("confusion_misses changed\n got: %v\nwant: %v", got, missBefore)
	}

	implied := m6Dump(t, conn, `SELECT implied FROM review_log ORDER BY id`)
	if !reflect.DeepEqual(implied, [][]string{{"int64:0"}}) {
		t.Fatalf("implied = %v, want 0 for the old row", implied)
	}
	if n := m6One(t, conn, `SELECT COUNT(*) FROM drill_answers`); n != 0 {
		t.Fatalf("drill_answers has %d rows after migrate, want 0", n)
	}
	if n := m6One(t, conn, `SELECT COUNT(*) FROM confusion_clears`); n != 0 {
		t.Fatalf("confusion_clears has %d rows after migrate, want 0", n)
	}
	if fk := m6Dump(t, conn, `PRAGMA foreign_key_check`); len(fk) != 0 {
		t.Fatalf("foreign_key_check: %v", fk)
	}
}

// ac-2: bad kind, empty or 65-char key, correct = 2, negative latency, and implied = 2 fail.
func TestMigrate021Constraints(t *testing.T) {
	conn := m21At020(t)
	if err := m21Migrate(conn); err != nil {
		t.Fatal(err)
	}
	const at = "2026-11-01T12:00:00Z"
	mustSQL(t, conn, `INSERT INTO drill_answers (learner_id, kind, key, correct, latency_ms, answered_at)
		VALUES (1, 'letter_pair', 'ב/כ', 1, 100, ?)`, at)
	mustSQL(t, conn, `INSERT INTO confusion_clears (learner_id, kind, key, cleared_at)
		VALUES (1, 'letter_pair', 'ב/כ', ?)`, at)
	mustSQL(t, conn, `INSERT INTO review_log (card_id, rating, reviewed_at, elapsed_days, scheduled_days, implied)
		VALUES (1, 3, ?, 0, 1, 0)`, at)

	rejectSQL(t, conn, "bad drill kind",
		`INSERT INTO drill_answers (learner_id, kind, key, correct, latency_ms, answered_at)
			VALUES (1, 'other', 'ב/כ', 1, 0, ?)`, at)
	rejectSQL(t, conn, "empty drill key",
		`INSERT INTO drill_answers (learner_id, kind, key, correct, latency_ms, answered_at)
			VALUES (1, 'letter_pair', '', 1, 0, ?)`, at)
	rejectSQL(t, conn, "65-char drill key",
		`INSERT INTO drill_answers (learner_id, kind, key, correct, latency_ms, answered_at)
			VALUES (1, 'letter_pair', ?, 1, 0, ?)`, strings.Repeat("a", 65), at)
	rejectSQL(t, conn, "correct = 2",
		`INSERT INTO drill_answers (learner_id, kind, key, correct, latency_ms, answered_at)
			VALUES (1, 'letter_pair', 'ב/כ', 2, 0, ?)`, at)
	rejectSQL(t, conn, "negative latency",
		`INSERT INTO drill_answers (learner_id, kind, key, correct, latency_ms, answered_at)
			VALUES (1, 'letter_pair', 'ב/כ', 1, -1, ?)`, at)
	rejectSQL(t, conn, "bad clear kind",
		`INSERT INTO confusion_clears (learner_id, kind, key, cleared_at)
			VALUES (1, 'other', 'ב/כ', ?)`, at)
	rejectSQL(t, conn, "empty clear key",
		`INSERT INTO confusion_clears (learner_id, kind, key, cleared_at)
			VALUES (1, 'agreement', '', ?)`, at)
	rejectSQL(t, conn, "65-char clear key",
		`INSERT INTO confusion_clears (learner_id, kind, key, cleared_at)
			VALUES (1, 'agreement', ?, ?)`, strings.Repeat("b", 65), at)
	rejectSQL(t, conn, "implied = 2",
		`INSERT INTO review_log (card_id, rating, reviewed_at, elapsed_days, scheduled_days, implied)
			VALUES (1, 3, ?, 0, 1, 2)`, at)

	mustSQL(t, conn, `INSERT INTO drill_answers (learner_id, kind, key, correct, latency_ms, answered_at)
		VALUES (1, 'agreement', ?, 0, 0, ?)`, strings.Repeat("c", 64), at)
	mustSQL(t, conn, `INSERT INTO confusion_clears (learner_id, kind, key, cleared_at)
		VALUES (1, 'agreement', ?, ?)`, strings.Repeat("d", 64), at)
	mustSQL(t, conn, `INSERT INTO review_log (card_id, rating, reviewed_at, elapsed_days, scheduled_days, implied)
		VALUES (1, 3, ?, 0, 1, 1)`, at)
}

// ac-3: deleting a user deletes their drill_answers and confusion_clears.
func TestMigrate021ForeignKeys(t *testing.T) {
	conn := m21At020(t)
	if err := m21Migrate(conn); err != nil {
		t.Fatal(err)
	}
	const at = "2026-11-01T12:00:00Z"
	mustSQL(t, conn, `INSERT INTO drill_answers (learner_id, kind, key, correct, latency_ms, answered_at)
		VALUES (1, 'letter_pair', 'ב/כ', 1, 50, ?)`, at)
	mustSQL(t, conn, `INSERT INTO drill_answers (learner_id, kind, key, correct, latency_ms, answered_at)
		VALUES (2, 'letter_pair', 'ד/ר', 0, 80, ?)`, at)
	mustSQL(t, conn, `INSERT INTO confusion_clears (learner_id, kind, key, cleared_at)
		VALUES (1, 'letter_pair', 'ב/כ', ?)`, at)
	mustSQL(t, conn, `INSERT INTO confusion_clears (learner_id, kind, key, cleared_at)
		VALUES (2, 'letter_pair', 'ד/ר', ?)`, at)

	mustSQL(t, conn, `DELETE FROM users WHERE id = 1`)
	if n := m6One(t, conn, `SELECT COUNT(*) FROM drill_answers WHERE learner_id = 1`); n != 0 {
		t.Fatalf("drill_answers for deleted user = %d, want 0", n)
	}
	if n := m6One(t, conn, `SELECT COUNT(*) FROM confusion_clears WHERE learner_id = 1`); n != 0 {
		t.Fatalf("confusion_clears for deleted user = %d, want 0", n)
	}
	if n := m6One(t, conn, `SELECT COUNT(*) FROM drill_answers`); n != 1 {
		t.Fatalf("other learner drill_answers = %d, want 1", n)
	}
	if n := m6One(t, conn, `SELECT COUNT(*) FROM confusion_clears`); n != 1 {
		t.Fatalf("other learner confusion_clears = %d, want 1", n)
	}
}

// ac-4: a fresh database at 021 has the same schema as a migrated one for the three changed tables.
func TestMigrate021FreshMatchesMigrated(t *testing.T) {
	migrated := m21At020(t)
	if err := m21Migrate(migrated); err != nil {
		t.Fatal(err)
	}
	fresh := openRaw(t)
	if err := m21Migrate(fresh); err != nil {
		t.Fatal(err)
	}
	got := m21SchemaSnapshot(t, migrated)
	want := m21SchemaSnapshot(t, fresh)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("migrated schema != fresh schema\n migrated: %#v\n fresh: %#v", got, want)
	}
}
