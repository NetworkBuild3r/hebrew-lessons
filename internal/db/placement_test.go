package db

import (
	"context"
	"database/sql"
	"io/fs"
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

	for _, table := range []string{"placement_runs", "placement_answers", "learner_lexemes"} {
		if !tableExists(t, conn, table) {
			t.Errorf("missing table %s", table)
		}
	}
	runCols := columnNames(t, conn, "placement_runs")
	for _, name := range []string{"id", "user_id", "started_at", "finished_at", "false_alarm_rate", "reliable", "level", "cap", "result", "seed"} {
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
	if _, err := conn.ExecContext(ctx, `INSERT INTO placement_answers (run_id, item_id, rating, shown_at) VALUES (?, 'boker', 0, ?)`, runID, old); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.ExecContext(ctx, `
		INSERT INTO learner_lexemes (user_id, lexeme_id, status, seeded, source, created_at)
		VALUES (?, ?, 'known', 1, 'placement', ?)`, userID, lexemeID, old); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.ExecContext(ctx, `
		UPDATE placement_runs
		SET finished_at = ?, false_alarm_rate = 0, reliable = 1, level = -1, result = '{"bands":[]}'
		WHERE id = ?`, old, runID); err != nil {
		t.Fatal(err)
	}

	if _, err := conn.ExecContext(ctx, `INSERT INTO lexemes (text_nfc) VALUES ('סֵפֶר')`); err != nil {
		t.Fatal(err)
	}
	var otherID int64
	if err := conn.QueryRowContext(ctx, `SELECT id FROM lexemes WHERE text_nfc = 'סֵפֶר'`).Scan(&otherID); err != nil {
		t.Fatal(err)
	}
	rejectSQL(t, conn, "learner status", `
		INSERT INTO learner_lexemes (user_id, lexeme_id, status, seeded, source, created_at)
		VALUES (?, ?, 'knownish', 0, 'placement', ?)`, userID, otherID, old)
	rejectSQL(t, conn, "rating", `INSERT INTO placement_answers (run_id, item_id, rating, shown_at) VALUES (?, 'other', 5, ?)`, runID, old)
	rejectSQL(t, conn, "level", `UPDATE placement_runs SET level = 5 WHERE id = ?`, runID)
	rejectSQL(t, conn, "reliable", `UPDATE placement_runs SET reliable = 2 WHERE id = ?`, runID)
	rejectSQL(t, conn, "cap", `UPDATE placement_runs SET cap = 'later' WHERE id = ?`, runID)

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
	if err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM learner_lexemes`).Scan(&left); err != nil {
		t.Fatal(err)
	}
	if left != 0 {
		t.Fatalf("learner lexemes left = %d", left)
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

func tableExists(t *testing.T, conn *sql.DB, name string) bool {
	t.Helper()
	var n int
	if err := conn.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?`, name).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n == 1
}
