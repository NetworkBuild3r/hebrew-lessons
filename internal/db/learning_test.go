package db

import (
	"context"
	"database/sql"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/migrations"
)

func TestMigration002AppliesOnTopOf001(t *testing.T) {
	ctx := context.Background()
	conn := openRaw(t)
	body, err := fs.ReadFile(migrations.FS, "001_init.sql")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "root TEXT") {
		t.Fatal("001 should already define lexemes.root")
	}
	for _, stmt := range statements(string(body)) {
		if _, err := conn.ExecContext(ctx, stmt); err != nil {
			t.Fatalf("001: %v", err)
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
	if _, err := conn.ExecContext(ctx, `INSERT INTO schema_migrations (version, applied_at) VALUES (1, ?)`, old); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.ExecContext(ctx, `INSERT INTO users (email, google_sub, role, created_at) VALUES ('a@example.com', 'sub-a', 'learner', ?)`, old); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.ExecContext(ctx, `INSERT INTO lexemes (text_nfc) VALUES ('שָׁלוֹם')`); err != nil {
		t.Fatal(err)
	}
	var userID, lexemeID int64
	if err := conn.QueryRowContext(ctx, `SELECT id FROM users`).Scan(&userID); err != nil {
		t.Fatal(err)
	}
	if err := conn.QueryRowContext(ctx, `SELECT id FROM lexemes`).Scan(&lexemeID); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.ExecContext(ctx, `
		INSERT INTO cards (user_id, lexeme_id, dir, state, due, stability, difficulty, reps, lapses, suspended)
		VALUES (?, ?, 'he_en', 'new', ?, 0, 0, 0, 0, 0)`, userID, lexemeID, old); err != nil {
		t.Fatal(err)
	}
	var cardID int64
	if err := conn.QueryRowContext(ctx, `SELECT id FROM cards`).Scan(&cardID); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.ExecContext(ctx, `
		INSERT INTO review_log (card_id, rating, reviewed_at, elapsed_days, scheduled_days)
		VALUES (?, 3, ?, 0, 0)`, cardID, old); err != nil {
		t.Fatal(err)
	}

	added, err := fs.ReadFile(migrations.FS, "002_learning.sql")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.ToLower(string(added)), "add column root") {
		t.Fatal("002 adds root, but 001 already has it")
	}

	when := time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)
	if err := Migrate(ctx, conn, fixedClock{when}); err != nil {
		t.Fatal(err)
	}

	var applied1, applied2 string
	if err := conn.QueryRowContext(ctx, `SELECT applied_at FROM schema_migrations WHERE version = 1`).Scan(&applied1); err != nil {
		t.Fatal(err)
	}
	if applied1 != old {
		t.Fatalf("version 1 applied_at = %s, want the pre-002 time", applied1)
	}
	if err := conn.QueryRowContext(ctx, `SELECT applied_at FROM schema_migrations WHERE version = 2`).Scan(&applied2); err != nil {
		t.Fatal(err)
	}
	if applied2 != FormatTime(when) {
		t.Fatalf("version 2 applied_at = %s", applied2)
	}

	lexCols := columnNames(t, conn, "lexemes")
	for _, name := range []string{"gender", "plural", "irregular_plural", "translit", "root"} {
		if !lexCols[name] {
			t.Errorf("lexemes missing %s", name)
		}
	}
	if got := columnCount(t, conn, "lexemes", "root"); got != 1 {
		t.Fatalf("root columns = %d, want 1", got)
	}
	if !columnNames(t, conn, "users")["address_gender"] {
		t.Fatal("users missing address_gender")
	}
	if !columnNames(t, conn, "review_log")["peeked"] {
		t.Fatal("review_log missing peeked")
	}
	missCols := columnNames(t, conn, "confusion_misses")
	for _, name := range []string{"id", "learner_id", "kind", "key", "at"} {
		if !missCols[name] {
			t.Errorf("confusion_misses missing %s", name)
		}
	}
	if missCols["count"] || missCols["counter"] {
		t.Fatal("confusion_misses stores a counter; the 7-day total is a query")
	}
	gotIndex := indexColumns(t, conn, "confusion_misses_learner_key_at")
	wantIndex := []string{"learner_id", "key", "at"}
	if strings.Join(gotIndex, ",") != strings.Join(wantIndex, ",") {
		t.Fatalf("index columns = %v, want %v", gotIndex, wantIndex)
	}

	var gender, plural, translit sql.NullString
	var irregular int
	if err := conn.QueryRowContext(ctx, `SELECT gender, plural, irregular_plural, translit FROM lexemes`).Scan(&gender, &plural, &irregular, &translit); err != nil {
		t.Fatal(err)
	}
	if gender.Valid || plural.Valid || translit.Valid || irregular != 0 {
		t.Fatalf("existing lexeme defaults: gender=%v plural=%v translit=%v irregular=%d", gender, plural, translit, irregular)
	}
	var address sql.NullString
	if err := conn.QueryRowContext(ctx, `SELECT address_gender FROM users`).Scan(&address); err != nil {
		t.Fatal(err)
	}
	if address.Valid {
		t.Fatalf("address_gender = %q, want NULL", address.String)
	}
	var peeked int
	if err := conn.QueryRowContext(ctx, `SELECT peeked FROM review_log`).Scan(&peeked); err != nil {
		t.Fatal(err)
	}
	if peeked != 0 {
		t.Fatalf("peeked = %d, want 0", peeked)
	}

	mustSQL(t, conn, `UPDATE lexemes SET gender = 'both', plural = 'שָׁלוֹמוֹת', translit = 'shalom', irregular_plural = 1`)
	mustSQL(t, conn, `UPDATE users SET address_gender = 'f'`)
	mustSQL(t, conn, `UPDATE review_log SET peeked = 1`)
	mustSQL(t, conn, `INSERT INTO confusion_misses (learner_id, kind, key, at) VALUES (?, 'letter_pair', 'ב/כ', ?)`, userID, old)
	mustSQL(t, conn, `INSERT INTO confusion_misses (learner_id, kind, key, at) VALUES (?, 'letter_pair', 'ב/כ', ?)`, userID, old)
	mustSQL(t, conn, `INSERT INTO confusion_misses (learner_id, kind, key, at) VALUES (?, 'agreement', 'הוא/היא', ?)`, userID, old)
	var misses int
	if err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM confusion_misses WHERE key = 'ב/כ'`).Scan(&misses); err != nil {
		t.Fatal(err)
	}
	if misses != 2 {
		t.Fatalf("rows for ב/כ = %d, want 2 distinct misses", misses)
	}

	rejectSQL(t, conn, "lexeme gender", `UPDATE lexemes SET gender = 'n'`)
	rejectSQL(t, conn, "address both", `UPDATE users SET address_gender = 'both'`)
	rejectSQL(t, conn, "irregular plural", `UPDATE lexemes SET irregular_plural = 2`)
	rejectSQL(t, conn, "peeked", `UPDATE review_log SET peeked = 2`)
	rejectSQL(t, conn, "confusion kind", `INSERT INTO confusion_misses (learner_id, kind, key, at) VALUES (?, 'root', 'ב/כ', ?)`, userID, old)
	rejectSQL(t, conn, "confusion user", `INSERT INTO confusion_misses (learner_id, kind, key, at) VALUES (999, 'letter_pair', 'ב/כ', ?)`, old)

	if _, err := conn.ExecContext(ctx, `DELETE FROM users WHERE id = ?`, userID); err != nil {
		t.Fatal(err)
	}
	if err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM confusion_misses`).Scan(&misses); err != nil {
		t.Fatal(err)
	}
	if misses != 0 {
		t.Fatalf("misses left after deleting the learner = %d", misses)
	}

	if err := Migrate(ctx, conn, fixedClock{when.Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM schema_migrations`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 4 {
		t.Fatalf("schema_migrations rows = %d after a second migrate, want 4", n)
	}
	if err := conn.QueryRowContext(ctx, `SELECT applied_at FROM schema_migrations WHERE version = 2`).Scan(&applied2); err != nil {
		t.Fatal(err)
	}
	if applied2 != FormatTime(when) {
		t.Fatalf("second migrate rewrote version 2 applied_at to %s", applied2)
	}
}

func openRaw(t *testing.T) *sql.DB {
	t.Helper()
	conn, err := sql.Open("sqlite", sqliteDSN(filepath.Join(t.TempDir(), "hebrew.sqlite")))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	conn.SetMaxOpenConns(1)
	if err := conn.PingContext(context.Background()); err != nil {
		t.Fatal(err)
	}
	return conn
}

func columnNames(t *testing.T, conn *sql.DB, table string) map[string]bool {
	t.Helper()
	rows, err := conn.QueryContext(context.Background(), `SELECT name FROM pragma_table_info(?)`, table)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	got := map[string]bool{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatal(err)
		}
		got[name] = true
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return got
}

func columnCount(t *testing.T, conn *sql.DB, table, column string) int {
	t.Helper()
	var n int
	if err := conn.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM pragma_table_info(?) WHERE name = ?`, table, column).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func indexColumns(t *testing.T, conn *sql.DB, index string) []string {
	t.Helper()
	rows, err := conn.QueryContext(context.Background(), `SELECT name FROM pragma_index_info(?) ORDER BY seqno`, index)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var got []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatal(err)
		}
		got = append(got, name)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return got
}

func mustSQL(t *testing.T, conn *sql.DB, query string, args ...any) {
	t.Helper()
	if _, err := conn.ExecContext(context.Background(), query, args...); err != nil {
		t.Fatalf("exec %s: %v", query, err)
	}
}

func rejectSQL(t *testing.T, conn *sql.DB, name, query string, args ...any) {
	t.Helper()
	_, err := conn.ExecContext(context.Background(), query, args...)
	if err == nil {
		t.Fatalf("%s: expected a constraint error", name)
	}
	if !strings.Contains(strings.ToLower(err.Error()), "constraint") {
		t.Fatalf("%s: error = %v, want a constraint failure", name, err)
	}
}
