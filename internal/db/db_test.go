package db

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/clock"
)

var ts = FormatTime(time.Date(2026, 10, 18, 0, 0, 0, 0, time.UTC))

type fixedClock struct{ t time.Time }

func (f fixedClock) Now() time.Time { return f.t }

func openStore(t *testing.T, clk clock.Clock) *Store {
	t.Helper()
	return openAt(t, t.TempDir(), clk)
}

func openAt(t *testing.T, dir string, clk clock.Clock) *Store {
	t.Helper()
	s, err := Open(context.Background(), filepath.Join(dir, "hebrew.sqlite"), clk)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestMigrateEmptyAndIdempotent(t *testing.T) {
	when := time.Date(2026, 10, 18, 16, 0, 0, 0, time.UTC)
	dir := t.TempDir()
	s := openAt(t, dir, fixedClock{when})
	if got := s.Clock().Now(); !got.Equal(when) {
		t.Fatalf("clock: got %s", got)
	}

	ctx := context.Background()
	if _, err := s.exec(ctx,
		`INSERT INTO users (email, google_sub, role, created_at) VALUES ('a@example.com', 'sub-a', 'learner', ?)`,
		ts,
	); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	s = openAt(t, dir, fixedClock{when})
	if got := count(t, s, `SELECT COUNT(*) FROM schema_migrations`); got != 8 {
		t.Fatalf("schema_migrations rows = %d, want 8", got)
	}
	wantApplied := FormatTime(when)
	rows, err := s.conn.QueryContext(ctx, `SELECT version, applied_at FROM schema_migrations ORDER BY version`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var versions []int
	for rows.Next() {
		var version int
		var applied string
		if err := rows.Scan(&version, &applied); err != nil {
			t.Fatal(err)
		}
		if applied != wantApplied {
			t.Fatalf("version %d applied_at = %s", version, applied)
		}
		versions = append(versions, version)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if len(versions) != 8 || versions[0] != 1 || versions[1] != 2 || versions[2] != 3 || versions[3] != 4 || versions[4] != 5 || versions[5] != 6 || versions[6] != 7 || versions[7] != 8 {
		t.Fatalf("versions = %v, want [1 2 3 4 5 6 7 8]", versions)
	}
	if got := count(t, s, `SELECT COUNT(*) FROM users`); got != 1 {
		t.Fatalf("users = %d, want 1 after a second open", got)
	}
}

func TestTables(t *testing.T) {
	s := openStore(t, nil)
	got := map[string]bool{}
	rows, err := s.conn.QueryContext(context.Background(),
		`SELECT name FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%'`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
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
	want := []string{
		"attempts",
		"audio_assets",
		"cards",
		"coach_notes",
		"confusion_misses",
		"daily_snapshots",
		"exposures",
		"lesson_briefs",
		"lesson_items",
		"lesson_runs",
		"lexemes",
		"oauth_clients",
		"oauth_codes",
		"oauth_requests",
		"oauth_tokens",
		"placement_answers",
		"placement_runs",
		"publish_audit",
		"regression_events",
		"review_log",
		"schema_migrations",
		"sessions",
		"tanakh_meta",
		"tanakh_verses",
		"unit_reports",
		"unit_revisions",
		"units",
		"users",
		"weekly_reports",
	}
	for _, name := range want {
		if !got[name] {
			t.Errorf("missing table %s", name)
		}
		delete(got, name)
	}
	for name := range got {
		t.Errorf("unexpected table %s", name)
	}
}

func TestPragmas(t *testing.T) {
	s := openStore(t, nil)
	ctx := context.Background()
	var foreignKeys, busy int
	var journal string
	if err := s.queryRow(ctx, `PRAGMA foreign_keys`).Scan(&foreignKeys); err != nil {
		t.Fatal(err)
	}
	if err := s.queryRow(ctx, `PRAGMA busy_timeout`).Scan(&busy); err != nil {
		t.Fatal(err)
	}
	if err := s.queryRow(ctx, `PRAGMA journal_mode`).Scan(&journal); err != nil {
		t.Fatal(err)
	}
	if foreignKeys != 1 || busy != busyTimeoutMS || !strings.EqualFold(journal, "wal") {
		t.Fatalf("pragmas: foreign_keys=%d busy_timeout=%d journal_mode=%s", foreignKeys, busy, journal)
	}
	if got := s.maxOpenConns(); got != 1 {
		t.Fatalf("max open conns = %d, want 1", got)
	}
}

func TestCheckConstraints(t *testing.T) {
	s := openStore(t, nil)
	ctx := context.Background()
	userID := insertUser(t, s, "a@example.com", "sub-a", "learner")
	lexemeID := insertLexeme(t, s, "שָׁלוֹם")
	unitID := insertUnit(t, s, "draft")
	insertUnit(t, s, "published")
	mustExec(t, s, `INSERT INTO oauth_clients (client_id, client_name, redirect_uris, token_endpoint_auth_method, grant_types, response_types, created_at)
		VALUES ('client-ok', 'ok', '[]', 'none', '["authorization_code"]', '["code"]', ?)`, ts)

	mustExec(t, s, `INSERT INTO cards (user_id, lexeme_id, skill, state, due, stability, difficulty, reps, lapses, suspended)
		VALUES (?, ?, 'read', 'new', ?, 0, 0, 0, 0, 0)`, userID, lexemeID, ts)
	var cardID int64
	if err := s.queryRow(ctx, `SELECT id FROM cards WHERE user_id = ?`, userID).Scan(&cardID); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name string
		q    string
		args []any
	}{
		{"user role", `INSERT INTO users (email, google_sub, role, created_at) VALUES ('b@example.com', 'sub-b', 'teacher', ?)`, []any{ts}},
		{"unit status", `INSERT INTO units (position, title, situation, concept, status, rev, doc) VALUES (2, 't', 's', 'c', 'archived', 1, '{"title":"t"}')`, nil},
		{"card skill", `INSERT INTO cards (user_id, lexeme_id, skill, state, due, stability, difficulty, reps, lapses, suspended) VALUES (?, ?, 'he_he', 'new', ?, 0, 0, 0, 0, 0)`, []any{userID, lexemeID, ts}},
		{"card state", `INSERT INTO cards (user_id, lexeme_id, skill, state, due, stability, difficulty, reps, lapses, suspended) VALUES (?, ?, 'recall', 'sm2', ?, 0, 0, 0, 0, 0)`, []any{userID, lexemeID, ts}},
		{"review rating", `INSERT INTO review_log (card_id, rating, reviewed_at, elapsed_days, scheduled_days) VALUES (?, 5, ?, 0, 0)`, []any{cardID, ts}},
		{"lesson role", `INSERT INTO lesson_items (unit_id, lesson_ix, lexeme_id, role, skill) VALUES (?, 0, ?, 'old', 'recall')`, []any{unitID, lexemeID}},
		{"oauth auth method", `INSERT INTO oauth_clients (client_id, client_name, redirect_uris, token_endpoint_auth_method, grant_types, response_types, created_at) VALUES ('c', 'n', '[]', 'client_secret_jwt', '["authorization_code"]', '["code"]', ?)`, []any{ts}},
		{"pkce method", `INSERT INTO oauth_requests (id, client_id, code_challenge, code_challenge_method, expires_at) VALUES ('r', 'client-ok', 'challenge', 'plain', ?)`, []any{ts}},
		{"token kind", `INSERT INTO oauth_tokens (token_hash, kind, client_id, user_id, family_id, expires_at, created_at) VALUES ('h', 'id', 'client-ok', ?, 'fam', ?, ?)`, []any{userID, ts, ts}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := s.exec(ctx, tc.q, tc.args...)
			if err == nil {
				t.Fatal("expected constraint error")
			}
			if !strings.Contains(err.Error(), "constraint") {
				t.Fatalf("error = %v, want a constraint failure", err)
			}
		})
	}
}

func TestCardsUniquePerDirection(t *testing.T) {
	s := openStore(t, nil)
	userID := insertUser(t, s, "a@example.com", "sub-a", "learner")
	lexemeID := insertLexeme(t, s, "שָׁלוֹם")
	mustExec(t, s, `INSERT INTO cards (user_id, lexeme_id, skill, state, due, stability, difficulty, reps, lapses, suspended)
		VALUES (?, ?, 'read', 'new', ?, 0, 0, 0, 0, 0)`, userID, lexemeID, ts)
	_, err := s.exec(context.Background(), `INSERT INTO cards (user_id, lexeme_id, skill, state, due, stability, difficulty, reps, lapses, suspended)
		VALUES (?, ?, 'read', 'new', ?, 0, 0, 0, 0, 0)`, userID, lexemeID, ts)
	if err == nil || !strings.Contains(err.Error(), "UNIQUE") {
		t.Fatalf("duplicate card error = %v", err)
	}
	mustExec(t, s, `INSERT INTO cards (user_id, lexeme_id, skill, state, due, stability, difficulty, reps, lapses, suspended)
		VALUES (?, ?, 'recall', 'learning', ?, 1, 5, 1, 0, 0)`, userID, lexemeID, ts)
}

func TestOnDelete(t *testing.T) {
	s := openStore(t, nil)
	ctx := context.Background()
	userID := insertUser(t, s, "a@example.com", "sub-a", "admin")
	mustExec(t, s, `INSERT INTO sessions (id_hash, user_id, csrf, expires_at) VALUES ('hash', ?, 'csrf', ?)`, userID, ts)
	audioID := strings.Repeat("ab", 32)
	mustExec(t, s, `INSERT INTO audio_assets (id, text_nfc, voice, model, bytes, created_at) VALUES (?, 'שָׁלוֹם', 'voice', 'eleven_v3', 12, ?)`, audioID, ts)
	mustExec(t, s, `INSERT INTO lexemes (text_nfc, audio_id) VALUES ('שָׁלוֹם', ?)`, audioID)
	mustExec(t, s, `INSERT INTO oauth_clients (client_id, client_secret_hash, client_name, redirect_uris, token_endpoint_auth_method, grant_types, response_types, created_at)
		VALUES ('client', 'secret-hash', 'Cursor', '["http://127.0.0.1/cb"]', 'none', '["authorization_code"]', '["code"]', ?)`, ts)
	mustExec(t, s, `INSERT INTO oauth_tokens (token_hash, kind, client_id, user_id, scope, family_id, expires_at, created_at)
		VALUES ('token-hash', 'refresh', 'client', ?, 'mcp', 'fam-1', ?, ?)`, userID, ts, ts)

	if _, err := s.exec(ctx, `DELETE FROM users WHERE id = ?`, userID); err != nil {
		t.Fatal(err)
	}
	if got := count(t, s, `SELECT COUNT(*) FROM sessions`); got != 0 {
		t.Fatalf("sessions left = %d", got)
	}
	if got := count(t, s, `SELECT COUNT(*) FROM oauth_tokens`); got != 0 {
		t.Fatalf("tokens left = %d", got)
	}

	if _, err := s.exec(ctx, `DELETE FROM audio_assets WHERE id = ?`, audioID); err != nil {
		t.Fatal(err)
	}
	var audio sql.NullString
	if err := s.queryRow(ctx, `SELECT audio_id FROM lexemes`).Scan(&audio); err != nil {
		t.Fatal(err)
	}
	if audio.Valid {
		t.Fatalf("audio_id = %q, want NULL after the clip is deleted", audio.String)
	}

	_, err := s.exec(ctx, `INSERT INTO sessions (id_hash, user_id, csrf, expires_at) VALUES ('hash', 999, 'csrf', ?)`, ts)
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "foreign key") {
		t.Fatalf("session without a user error = %v", err)
	}
}

func TestSplitStatementsKeepsSemicolonsInCommentsAndStrings(t *testing.T) {
	got := statements("CREATE TABLE t (id INTEGER); -- a; b\nINSERT INTO t (id, note) VALUES (1, 'a;b');")
	if len(got) != 2 {
		t.Fatalf("got %d statements: %#v", len(got), got)
	}
	if !strings.Contains(got[1], "'a;b'") {
		t.Fatalf("string was split: %s", got[1])
	}
	if strings.Contains(got[0], "INSERT") {
		t.Fatalf("first statement ate the second: %s", got[0])
	}
}

func insertUser(t *testing.T, s *Store, email, sub, role string) int64 {
	t.Helper()
	res := mustExec(t, s, `INSERT INTO users (email, google_sub, role, created_at) VALUES (?, ?, ?, ?)`, email, sub, role, ts)
	id, err := res.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func insertLexeme(t *testing.T, s *Store, text string) int64 {
	t.Helper()
	res := mustExec(t, s, `INSERT INTO lexemes (text_nfc) VALUES (?)`, text)
	id, err := res.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func insertUnit(t *testing.T, s *Store, status string) int64 {
	t.Helper()
	res := mustExec(t, s, `INSERT INTO units (position, title, situation, concept, status, rev, doc)
		VALUES (1, 'Hello', 'street', 'greetings', ?, 1, '{"title":"Hello"}')`, status)
	id, err := res.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func mustExec(t *testing.T, s *Store, query string, args ...any) sql.Result {
	t.Helper()
	res, err := s.exec(context.Background(), query, args...)
	if err != nil {
		t.Fatalf("exec %s: %v", query, err)
	}
	return res
}

func count(t *testing.T, s *Store, query string, args ...any) int {
	t.Helper()
	var n int
	if err := s.queryRow(context.Background(), query, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}
