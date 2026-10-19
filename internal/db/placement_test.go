package db

import (
	"context"
	"database/sql"
	"io/fs"
	"strings"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/migrations"
)

func TestMigration003AppliesAfter002(t *testing.T) {
	ctx := context.Background()
	conn := openRaw(t)
	for _, name := range []string{"001_init.sql", "002_learning.sql"} {
		body, err := fs.ReadFile(migrations.FS, name)
		if err != nil {
			t.Fatal(err)
		}
		for _, stmt := range statements(string(body)) {
			if _, err := conn.ExecContext(ctx, stmt); err != nil {
				t.Fatalf("%s: %v", name, err)
			}
		}
	}
	if _, err := conn.ExecContext(ctx, `
		CREATE TABLE schema_migrations (
			version INTEGER PRIMARY KEY,
			applied_at TEXT NOT NULL
		)`); err != nil {
		t.Fatal(err)
	}
	const old = "2020-01-01T00:00:00Z"
	for _, version := range []int{1, 2} {
		if _, err := conn.ExecContext(ctx, `INSERT INTO schema_migrations (version, applied_at) VALUES (?, ?)`, version, old); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := conn.ExecContext(ctx, `INSERT INTO users (email, google_sub, role, created_at) VALUES ('a@example.com', 'sub-a', 'learner', ?)`, old); err != nil {
		t.Fatal(err)
	}

	when := time.Date(2026, 4, 2, 8, 0, 0, 0, time.UTC)
	if err := Migrate(ctx, conn, fixedClock{when}); err != nil {
		t.Fatal(err)
	}

	var applied1, applied2, applied3 string
	if err := conn.QueryRowContext(ctx, `SELECT applied_at FROM schema_migrations WHERE version = 1`).Scan(&applied1); err != nil {
		t.Fatal(err)
	}
	if err := conn.QueryRowContext(ctx, `SELECT applied_at FROM schema_migrations WHERE version = 2`).Scan(&applied2); err != nil {
		t.Fatal(err)
	}
	if applied1 != old || applied2 != old {
		t.Fatalf("earlier migrations were rewritten: %s %s", applied1, applied2)
	}
	if err := conn.QueryRowContext(ctx, `SELECT applied_at FROM schema_migrations WHERE version = 3`).Scan(&applied3); err != nil {
		t.Fatal(err)
	}
	if applied3 != placementTime(when) {
		t.Fatalf("version 3 applied_at = %s", applied3)
	}

	for _, table := range []string{"placement_runs", "placement_answers"} {
		if !tableExists(t, conn, table) {
			t.Errorf("missing table %s", table)
		}
	}
	if tableExists(t, conn, "learner_lexemes") {
		t.Error("learner_lexemes was created")
	}
	if !columnNames(t, conn, "review_log")["chosen"] {
		t.Error("review_log missing chosen")
	}
	if !columnNames(t, conn, "confusion_misses")["lexeme_id"] {
		t.Error("confusion_misses missing lexeme_id")
	}
	if !columnNames(t, conn, "units")["scope"] {
		t.Error("units missing scope")
	}
	if !columnNames(t, conn, "placement_answers")["lexeme_id"] {
		t.Error("placement_answers missing lexeme_id")
	}
	gotIndex := indexColumns(t, conn, "units_scope_position")
	if stringsJoin(gotIndex) != "scope,position" {
		t.Fatalf("units_scope_position = %v", gotIndex)
	}
	runCols := columnNames(t, conn, "placement_runs")
	for _, name := range []string{"id", "user_id", "started_at", "finished_at", "level", "stop_reason", "result", "seed"} {
		if !runCols[name] {
			t.Errorf("placement_runs missing %s", name)
		}
	}

	var userID int64
	if err := conn.QueryRowContext(ctx, `SELECT id FROM users`).Scan(&userID); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.ExecContext(ctx, `INSERT INTO lexemes (text_nfc, gloss_en, pos, verified) VALUES ('שָׁלוֹם', 'hello', 'noun', 0)`); err != nil {
		t.Fatal(err)
	}
	var lexemeID int64
	if err := conn.QueryRowContext(ctx, `SELECT id FROM lexemes`).Scan(&lexemeID); err != nil {
		t.Fatal(err)
	}
	res, err := conn.ExecContext(ctx, `INSERT INTO placement_runs (user_id, started_at, seed) VALUES (?, ?, 7)`, userID, old)
	if err != nil {
		t.Fatal(err)
	}
	runID, err := res.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conn.ExecContext(ctx, `INSERT INTO placement_answers (run_id, item_id, rating, shown_at) VALUES (?, 'boker', 1, ?)`, runID, old); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.ExecContext(ctx, `
		UPDATE placement_answers SET lexeme_id = ? WHERE item_id = 'boker'`, lexemeID); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.ExecContext(ctx, `
		INSERT INTO cards (user_id, lexeme_id, dir, state, due, stability, difficulty, reps, lapses)
		VALUES (?, ?, 'he_en', 'new', ?, 0, 0, 0, 0)`, userID, lexemeID, old); err != nil {
		t.Fatal(err)
	}
	var cardID int64
	if err := conn.QueryRowContext(ctx, `SELECT id FROM cards`).Scan(&cardID); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.ExecContext(ctx, `
		INSERT INTO review_log (card_id, rating, reviewed_at, elapsed_days, scheduled_days, chosen)
		VALUES (?, 3, ?, 0, 0, 'שָׁלוֹם')`, cardID, old); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.ExecContext(ctx, `
		INSERT INTO units (position, title, situation, concept, status, rev, doc)
		VALUES (0, 't', 's', 'c', 'draft', 1, '{"title":"t"}')`); err != nil {
		t.Fatal(err)
	}
	var scope string
	if err := conn.QueryRowContext(ctx, `SELECT scope FROM units`).Scan(&scope); err != nil {
		t.Fatal(err)
	}
	if scope != "shared" {
		t.Fatalf("scope = %q, want shared", scope)
	}
	if _, err := conn.ExecContext(ctx, `
		UPDATE placement_runs
		SET finished_at = ?, level = -1, stop_reason = 'exhausted', result = '{"bands":[]}'
		WHERE id = ?`, old, runID); err != nil {
		t.Fatal(err)
	}

	rejectSQL(t, conn, "unit scope", `UPDATE units SET scope = 'private'`)
	rejectSQL(t, conn, "unit scope learner", `UPDATE units SET scope = 'learner:'`)
	if _, err := conn.ExecContext(ctx, `UPDATE units SET scope = 'learner:4'`); err != nil {
		t.Fatal(err)
	}
	rejectSQL(t, conn, "confusion lexeme", `
		INSERT INTO confusion_misses (learner_id, kind, key, lexeme_id, at)
		VALUES (?, 'letter_pair', 'ב/כ', 99999, ?)`, userID, old)
	if _, err := conn.ExecContext(ctx, `INSERT INTO placement_answers (run_id, item_id, rating, shown_at) VALUES (?, 'other', 0, ?)`, runID, old); err != nil {
		t.Fatal(err)
	}
	rejectSQL(t, conn, "rating high", `INSERT INTO placement_answers (run_id, item_id, rating, shown_at) VALUES (?, 'high', 5, ?)`, runID, old)
	rejectSQL(t, conn, "level", `UPDATE placement_runs SET level = 5 WHERE id = ?`, runID)
	rejectSQL(t, conn, "stop reason", `UPDATE placement_runs SET stop_reason = 'later' WHERE id = ?`, runID)
	rejectSQL(t, conn, "old cap", `UPDATE placement_runs SET stop_reason = 'cap' WHERE id = ?`, runID)
	if _, err := conn.ExecContext(ctx, `UPDATE placement_runs SET stop_reason = 'cap_cards' WHERE id = ?`, runID); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.ExecContext(ctx, `UPDATE placement_runs SET stop_reason = 'cap_time' WHERE id = ?`, runID); err != nil {
		t.Fatal(err)
	}

	if _, err := conn.ExecContext(ctx, `DELETE FROM users WHERE id = ?`, userID); err != nil {
		t.Fatal(err)
	}
	var left int
	if err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM placement_runs`).Scan(&left); err != nil {
		t.Fatal(err)
	}
	if left != 0 {
		t.Fatalf("runs left after deleting the learner = %d", left)
	}
	if err := Migrate(ctx, conn, fixedClock{when.Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
	if err := conn.QueryRowContext(ctx, `SELECT applied_at FROM schema_migrations WHERE version = 3`).Scan(&applied3); err != nil {
		t.Fatal(err)
	}
	if applied3 != placementTime(when) {
		t.Fatalf("second migrate rewrote version 3 applied_at to %s", applied3)
	}
}

func stringsJoin(parts []string) string {
	return strings.Join(parts, ",")
}

func tableExists(t *testing.T, conn *sql.DB, name string) bool {
	t.Helper()
	var n int
	if err := conn.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?`, name).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n == 1
}
