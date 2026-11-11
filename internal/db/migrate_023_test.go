package db

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io/fs"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/migrations"
)

// Migration 023 is additive. A database at 022 keeps every attempts, review_log,
// and confusion_misses row, and those tables' checksums do not move.
// INIT-038/SPEC-002.

func m23At022(t *testing.T) *sql.DB {
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
		if version >= 23 {
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
	want := make([]int, 0, 22)
	for i := 1; i <= 22; i++ {
		want = append(want, i)
	}
	if got := m6Applied(t, conn); !reflect.DeepEqual(got, want) {
		t.Fatalf("seed versions = %v, want 1..22", got)
	}
	mustSQL(t, conn, `INSERT INTO users (email, google_sub, role, created_at) VALUES (?, ?, 'learner', ?)`,
		"void-migrate@example.com", "sub-void-migrate", "2026-10-01T00:00:00Z")
	mustSQL(t, conn, `INSERT INTO lexemes (text_nfc, gloss_en) VALUES ('מַיִם', 'water')`)
	mustSQL(t, conn, `INSERT INTO attempts (
		user_id, lexeme_id, skill, activity, source, prompt, response, correct, dont_know, hint_used, latency_ms, created_at
	) VALUES (1, 1, 'read', 'read_pick', 'lesson', 'water', 'water', 1, 0, 0, 400, '2026-10-02T12:00:00Z')`)
	mustSQL(t, conn, `INSERT INTO attempts (
		user_id, lexeme_id, skill, activity, source, prompt, response, correct, dont_know, hint_used, latency_ms, created_at
	) VALUES (1, 1, 'recall', 'produce', 'review', 'water', 'מים', 0, 0, 0, 900, '2026-10-03T12:00:00Z')`)
	mustSQL(t, conn, `INSERT INTO cards (
		user_id, lexeme_id, skill, state, due, stability, difficulty, reps, lapses, suspended, created_at
	) VALUES (1, 1, 'read', 'review', '2026-10-04T12:00:00Z', 4, 5, 2, 0, 0, '2026-10-02T12:00:00Z')`)
	mustSQL(t, conn, `INSERT INTO review_log (card_id, rating, reviewed_at, elapsed_days, scheduled_days, peeked, implied)
		VALUES (1, 3, '2026-10-02T12:00:00Z', 0, 1, 0, 0)`)
	mustSQL(t, conn, `INSERT INTO review_log (card_id, rating, reviewed_at, elapsed_days, scheduled_days, peeked, chosen, implied)
		VALUES (1, 1, '2026-10-04T12:00:00Z', 2, 1, 0, 'מים', 0)`)
	mustSQL(t, conn, `INSERT INTO confusion_misses (learner_id, kind, key, lexeme_id, at)
		VALUES (1, 'letter_pair', 'ב/כ', 1, '2026-10-04T12:00:00Z')`)
	mustSQL(t, conn, `INSERT INTO confusion_misses (learner_id, kind, key, at)
		VALUES (1, 'agreement', 'הוא/היא', '2026-10-05T12:00:00Z')`)
	return conn
}

func m23Fingerprint(t *testing.T, conn *sql.DB) (counts map[string]int, sums map[string]string) {
	t.Helper()
	queries := []struct{ name, q string }{
		{"attempts", `SELECT id, user_id, lexeme_id, skill, activity, source, unit_id, prompt, response, correct, dont_know, hint_used, latency_ms, created_at FROM attempts ORDER BY id`},
		{"review_log", `SELECT id, card_id, rating, reviewed_at, elapsed_days, scheduled_days, peeked, chosen, implied FROM review_log ORDER BY id`},
		{"confusion_misses", `SELECT id, learner_id, kind, key, at, lexeme_id FROM confusion_misses ORDER BY id`},
	}
	counts = map[string]int{}
	sums = map[string]string{}
	for _, q := range queries {
		dump := m6Dump(t, conn, q.q)
		counts[q.name] = len(dump)
		sum := sha256.Sum256([]byte(fmt.Sprintf("%#v", dump)))
		sums[q.name] = hex.EncodeToString(sum[:])
		if n := m6One(t, conn, `SELECT COUNT(*) FROM `+q.name); n != len(dump) {
			t.Fatalf("%s count %d, dump %d", q.name, n, len(dump))
		}
	}
	return counts, sums
}

func TestMigrate023KeepsEvidenceRows(t *testing.T) {
	conn := m23At022(t)
	beforeN, beforeSum := m23Fingerprint(t, conn)
	if beforeN["attempts"] != 2 || beforeN["review_log"] != 2 || beforeN["confusion_misses"] != 2 {
		t.Fatalf("seed counts = %+v", beforeN)
	}
	appliedBefore := m6Dump(t, conn, `SELECT version, applied_at FROM schema_migrations WHERE version = 1`)

	if err := Migrate(context.Background(), conn, fixedClock{time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)}); err != nil {
		t.Fatalf("migrate 023: %v", err)
	}
	want := make([]int, 0, 23)
	for i := 1; i <= 25; i++ {
		want = append(want, i)
	}
	if got := m6Applied(t, conn); !reflect.DeepEqual(got, want) {
		t.Fatalf("versions after = %v, want 1..25", got)
	}
	afterN, afterSum := m23Fingerprint(t, conn)
	if !reflect.DeepEqual(afterN, beforeN) || !reflect.DeepEqual(afterSum, beforeSum) {
		t.Fatalf("evidence changed\n counts %v -> %v\n sums %v -> %v", beforeN, afterN, beforeSum, afterSum)
	}
	if got := m6Dump(t, conn, `SELECT version, applied_at FROM schema_migrations WHERE version = 1`); !reflect.DeepEqual(got, appliedBefore) {
		t.Fatalf("version 1 applied_at changed: %v", got)
	}
	if n := m6One(t, conn, `SELECT COUNT(*) FROM answer_voids`); n != 0 {
		t.Fatalf("answer_voids has %d rows after migrate", n)
	}
	if fk := m6Dump(t, conn, `PRAGMA foreign_key_check`); len(fk) != 0 {
		t.Fatalf("foreign_key_check: %v", fk)
	}

	if err := Migrate(context.Background(), conn, fixedClock{time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)}); err != nil {
		t.Fatal(err)
	}
	againN, againSum := m23Fingerprint(t, conn)
	if !reflect.DeepEqual(againN, beforeN) || !reflect.DeepEqual(againSum, beforeSum) {
		t.Fatal("a second migrate changed evidence rows")
	}
}

func TestMigrate023IsCreateOnly(t *testing.T) {
	body, err := fs.ReadFile(migrations.FS, "023_answer_voids.sql")
	if err != nil {
		t.Fatal(err)
	}
	stmts := statements(string(body))
	if len(stmts) == 0 {
		t.Fatal("023 has no statements")
	}
	for _, stmt := range stmts {
		verb := sqlVerb(stmt)
		if verb != "CREATE" {
			t.Fatalf("023 statement starts with %s, want CREATE:\n%s", verb, stmt)
		}
	}
}

func sqlVerb(stmt string) string {
	var b strings.Builder
	inLine := false
	for i := 0; i < len(stmt); i++ {
		if inLine {
			if stmt[i] == '\n' {
				inLine = false
			}
			continue
		}
		if stmt[i] == '-' && i+1 < len(stmt) && stmt[i+1] == '-' {
			inLine = true
			i++
			continue
		}
		b.WriteByte(stmt[i])
	}
	fields := strings.Fields(b.String())
	if len(fields) == 0 {
		return ""
	}
	return strings.ToUpper(fields[0])
}

func TestMigrate023Constraints(t *testing.T) {
	conn := m23At022(t)
	if err := Migrate(context.Background(), conn, fixedClock{time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)}); err != nil {
		t.Fatal(err)
	}
	mustSQL(t, conn, `INSERT INTO answer_voids (
		attempt_id, learner_id, reason, actor, batch_id, created_at
	) VALUES (1, 1, 'report', 'learner', 'b1', '2026-11-09T12:00:00.000000000Z')`)
	rejectSQL(t, conn, "reason", `INSERT INTO answer_voids (
		attempt_id, learner_id, reason, actor, batch_id, created_at
	) VALUES (2, 1, 'bored', 'learner', 'b2', '2026-11-09T12:00:00.000000000Z')`)
	rejectSQL(t, conn, "actor", `INSERT INTO answer_voids (
		attempt_id, learner_id, reason, actor, batch_id, created_at
	) VALUES (2, 1, 'report', '', 'b2', '2026-11-09T12:00:00.000000000Z')`)
	rejectSQL(t, conn, "missing attempt", `INSERT INTO answer_voids (
		attempt_id, learner_id, reason, actor, batch_id, created_at
	) VALUES (99, 1, 'report', 'learner', 'b2', '2026-11-09T12:00:00.000000000Z')`)
}
