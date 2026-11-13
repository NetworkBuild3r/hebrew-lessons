package db

import (
	"context"
	"database/sql"
	"io/fs"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/migrations"
)

// Migration 024 is additive. A database at 023 keeps every attempts, units,
// and taught_words row, and the new columns read as null for them.
// INIT-039/SPEC-002.

func m24Apply(t *testing.T, conn *sql.DB, below, through int) {
	t.Helper()
	ctx := context.Background()
	mustSQL(t, conn, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version INTEGER PRIMARY KEY,
		applied_at TEXT NOT NULL
	)`)
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
		if version < below || version > through {
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
}

func m24Seed(t *testing.T, conn *sql.DB) (userID, lexemeID int64) {
	t.Helper()
	mustSQL(t, conn, `INSERT INTO users (email, google_sub, role, created_at) VALUES (?, ?, 'learner', ?)`,
		"sheets@example.com", "sub-sheets", "2026-10-01T00:00:00Z")
	mustSQL(t, conn, `INSERT INTO lexemes (text_nfc, gloss_en) VALUES ('מַיִם', 'water')`)
	if err := conn.QueryRow(`SELECT id FROM users WHERE email = 'sheets@example.com'`).Scan(&userID); err != nil {
		t.Fatal(err)
	}
	if err := conn.QueryRow(`SELECT id FROM lexemes WHERE gloss_en = 'water'`).Scan(&lexemeID); err != nil {
		t.Fatal(err)
	}
	return userID, lexemeID
}

func TestMigrate024KeepsRowsAndAddsNullColumns(t *testing.T) {
	conn := openRaw(t)
	m24Apply(t, conn, 1, 23)
	userID, lexemeID := m24Seed(t, conn)
	mustSQL(t, conn, `INSERT INTO taught_words (learner_id, lexeme_id, taught_on, cards_created, created_at)
		VALUES (?, ?, '2026-11-03', 'read', '2026-11-03T10:00:00Z')`, userID, lexemeID)
	mustSQL(t, conn, `INSERT INTO attempts (user_id, lexeme_id, skill, activity, source, prompt, response, correct, latency_ms, created_at)
		VALUES (?, ?, 'read', 'listen_pick', 'review', 'p', 'r', 1, 900, '2026-11-04T10:00:00Z')`, userID, lexemeID)
	m24Apply(t, conn, 24, 24)

	var runID, sheetID sql.NullInt64
	if err := conn.QueryRow(`SELECT run_id FROM attempts`).Scan(&runID); err != nil || runID.Valid {
		t.Fatalf("attempts.run_id = %v, %v; want null", runID, err)
	}
	if err := conn.QueryRow(`SELECT sheet_id FROM taught_words`).Scan(&sheetID); err != nil || sheetID.Valid {
		t.Fatalf("taught_words.sheet_id = %v, %v; want null", sheetID, err)
	}
	var n int
	if err := conn.QueryRow(`SELECT COUNT(*) FROM attempts`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("attempts rows = %d, %v; want 1", n, err)
	}
	if got := m6Applied(t, conn); got[len(got)-1] != 24 {
		t.Fatalf("applied versions = %v, want last 24", got)
	}
}

func TestMigrate024LessonTypeColumn(t *testing.T) {
	conn := openRaw(t)
	m24Apply(t, conn, 1, 24)
	mustSQL(t, conn, `INSERT INTO units (position, title, situation, concept, status, rev, doc, lesson_type)
		VALUES (1, 't', 's', 'c', 'draft', 1, '{}', 'word')`)
	mustSQL(t, conn, `INSERT INTO units (position, title, situation, concept, status, rev, doc)
		VALUES (2, 't2', 's', 'c', 'draft', 1, '{}')`)
	if _, err := conn.Exec(`INSERT INTO units (position, title, situation, concept, status, rev, doc, lesson_type)
		VALUES (3, 't3', 's', 'c', 'draft', 1, '{}', 'listening')`); err == nil {
		t.Fatal("a lesson type outside word and easy was accepted")
	}
	var typ sql.NullString
	if err := conn.QueryRow(`SELECT lesson_type FROM units WHERE position = 2`).Scan(&typ); err != nil || typ.Valid {
		t.Fatalf("hand-written unit lesson_type = %v, %v; want null", typ, err)
	}
}

func TestMigrate024ClassSheetConstraints(t *testing.T) {
	conn := openRaw(t)
	m24Apply(t, conn, 1, 24)
	userID, _ := m24Seed(t, conn)

	mustSQL(t, conn, `INSERT INTO class_sheets (learner_id, sheet_ref, class_on, created_at) VALUES (?, 'ref-1', '2026-11-03', '2026-11-04T00:00:00Z')`, userID)
	var sheetID int64
	if err := conn.QueryRow(`SELECT id FROM class_sheets`).Scan(&sheetID); err != nil {
		t.Fatal(err)
	}
	var status string
	if err := conn.QueryRow(`SELECT status FROM class_sheets WHERE id = ?`, sheetID).Scan(&status); err != nil || status != "held" {
		t.Fatalf("new sheet status = %q, %v; want held", status, err)
	}

	bad := []struct{ name, sql string }{
		{"repeat ref", `INSERT INTO class_sheets (learner_id, sheet_ref, class_on, created_at) VALUES (?, 'ref-1', '2026-11-05', 'x')`},
		{"not a date", `INSERT INTO class_sheets (learner_id, sheet_ref, class_on, created_at) VALUES (?, 'ref-2', '2026-13-45', 'x')`},
		{"empty ref", `INSERT INTO class_sheets (learner_id, sheet_ref, class_on, created_at) VALUES (?, '', '2026-11-05', 'x')`},
		{"unknown status", `INSERT INTO class_sheets (learner_id, sheet_ref, class_on, status, created_at) VALUES (?, 'ref-3', '2026-11-05', 'maybe', 'x')`},
	}
	for _, tc := range bad {
		if _, err := conn.Exec(tc.sql, userID); err == nil {
			t.Errorf("%s: accepted", tc.name)
		}
	}

	// A line has exactly one author.
	mustSQL(t, conn, `INSERT INTO class_lines (sheet_id, position, tutor_text, gloss_en) VALUES (?, 0, 'שלום', 'peace')`, sheetID)
	mustSQL(t, conn, `INSERT INTO class_lines (sheet_id, position, learner_text) VALUES (?, 1, 'מים')`, sheetID)
	badLines := []struct{ name, sql string }{
		{"both authors", `INSERT INTO class_lines (sheet_id, position, tutor_text, learner_text) VALUES (?, 2, 'a', 'b')`},
		{"no author", `INSERT INTO class_lines (sheet_id, position) VALUES (?, 3)`},
		{"empty text", `INSERT INTO class_lines (sheet_id, position, tutor_text) VALUES (?, 4, '')`},
		{"repeat position", `INSERT INTO class_lines (sheet_id, position, tutor_text) VALUES (?, 0, 'x')`},
		{"held without reason", `INSERT INTO class_lines (sheet_id, position, tutor_text, state) VALUES (?, 5, 'x', 'held')`},
	}
	for _, tc := range badLines {
		if _, err := conn.Exec(tc.sql, sheetID); err == nil {
			t.Errorf("%s: accepted", tc.name)
		}
	}
	mustSQL(t, conn, `INSERT INTO class_lines (sheet_id, position, tutor_text, state, held_reason) VALUES (?, 6, 'x', 'held', 'no gloss')`, sheetID)

	// Deleting a sheet deletes its lines and clears the taught-word link, not the taught word.
	_, lexemeID := m24Seed2(t, conn, userID)
	mustSQL(t, conn, `INSERT INTO taught_words (learner_id, lexeme_id, taught_on, created_at, sheet_id) VALUES (?, ?, '2026-11-03', 'x', ?)`, userID, lexemeID, sheetID)
	mustSQL(t, conn, `DELETE FROM class_sheets WHERE id = ?`, sheetID)
	var lines, taught int
	if err := conn.QueryRow(`SELECT COUNT(*) FROM class_lines`).Scan(&lines); err != nil || lines != 0 {
		t.Fatalf("lines after sheet delete = %d, %v; want 0", lines, err)
	}
	var link sql.NullInt64
	if err := conn.QueryRow(`SELECT COUNT(*), MAX(sheet_id) FROM taught_words`).Scan(&taught, &link); err != nil || taught != 1 || link.Valid {
		t.Fatalf("taught words after sheet delete = %d, link %v, %v; want 1 and null", taught, link, err)
	}
}

func m24Seed2(t *testing.T, conn *sql.DB, userID int64) (int64, int64) {
	t.Helper()
	var lexemeID int64
	if err := conn.QueryRow(`SELECT id FROM lexemes LIMIT 1`).Scan(&lexemeID); err != nil {
		t.Fatal(err)
	}
	return userID, lexemeID
}

func TestMigrate024ItemFlagConstraints(t *testing.T) {
	conn := openRaw(t)
	m24Apply(t, conn, 1, 24)
	userID, lexemeID := m24Seed(t, conn)

	mustSQL(t, conn, `INSERT INTO item_flags (learner_id, lexeme_id, reason, created_at) VALUES (?, ?, 'also_right', '2026-11-11T00:00:00Z')`, userID, lexemeID)
	mustSQL(t, conn, `INSERT INTO item_flags (learner_id, lexeme_id, reason, note, void_batch_id, created_at) VALUES (?, ?, 'wrong_question', 'two answers fit', 'batch-1', '2026-11-11T00:01:00Z')`, userID, lexemeID)
	if _, err := conn.Exec(`INSERT INTO item_flags (learner_id, lexeme_id, reason, created_at) VALUES (?, ?, 'lol', 'x')`, userID, lexemeID); err == nil {
		t.Fatal("an unknown flag reason was accepted")
	}
	if _, err := conn.Exec(`INSERT INTO item_flags (learner_id, lexeme_id, reason, note, created_at) VALUES (?, ?, 'bad_item', ?, 'x')`,
		userID, lexemeID, strings.Repeat("a", 501)); err == nil {
		t.Fatal("a note over 500 characters was accepted")
	}
	// A flagged word cannot be deleted out from under the flag.
	if _, err := conn.Exec(`DELETE FROM lexemes WHERE id = ?`, lexemeID); err == nil {
		t.Fatal("a flagged lexeme was deleted")
	}
	// Undo is a set undone_at, not a delete: the active-flag index sees only the open one.
	mustSQL(t, conn, `UPDATE item_flags SET undone_at = '2026-11-11T00:05:00Z' WHERE reason = 'also_right'`)
	var active int
	if err := conn.QueryRow(`SELECT COUNT(*) FROM item_flags WHERE learner_id = ? AND undone_at IS NULL`, userID).Scan(&active); err != nil || active != 1 {
		t.Fatalf("active flags = %d, %v; want 1", active, err)
	}
}

func TestMigrate024IsAdditiveOnly(t *testing.T) {
	body, err := fs.ReadFile(migrations.FS, "024_class_sheets_and_flags.sql")
	if err != nil {
		t.Fatal(err)
	}
	for i, line := range strings.Split(string(body), "\n") {
		up := strings.ToUpper(strings.TrimSpace(line))
		if strings.HasPrefix(up, "--") {
			continue
		}
		for _, word := range []string{"DROP ", "DELETE FROM", "UPDATE ", "RENAME "} {
			if strings.HasPrefix(up, word) {
				t.Fatalf("line %d is not additive: %q", i+1, line)
			}
		}
	}
}

// m24Aside drops what migration 024 adds or alters, so an earlier migration's
// "nothing else changed" check still holds. attempts and units gain a column;
// TestMigrate024KeepsRowsAndAddsNullColumns covers that their rows are kept.
// INIT-039/SPEC-002.
func m24Aside(st m7State) m7State {
	for _, name := range []string{"class_sheets", "class_lines", "item_flags", "attempts", "units", "taught_words"} {
		delete(st.tables, name)
		delete(st.rowCnts, name)
		st.schema = m8SchemaWithout(st.schema, name)
	}
	return m25Aside(st)
}

// m25Aside drops the story slot, which migration 025 adds.
func m25Aside(st m7State) m7State {
	delete(st.tables, "story_slots")
	delete(st.rowCnts, "story_slots")
	st.schema = m8SchemaWithout(st.schema, "story_slots")
	return m26Aside(st)
}

// m26Aside drops the course list, which migration 026 adds.
func m26Aside(st m7State) m7State {
	delete(st.tables, "course_words")
	delete(st.rowCnts, "course_words")
	st.schema = m8SchemaWithout(st.schema, "course_words")
	return st
}
