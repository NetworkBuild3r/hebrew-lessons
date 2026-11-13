package db

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
)

func TestMigrate026KeepsLexemesAndCards(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hebrew.sqlite")
	conn, err := sql.Open("sqlite", sqliteDSN(path))
	if err != nil {
		t.Fatal(err)
	}
	conn.SetMaxOpenConns(1)
	m24Apply(t, conn, 1, 25)
	mustSQL(t, conn, `INSERT INTO users (email, google_sub, role, created_at) VALUES ('course@example.com', 'sub-course', 'learner', '2026-11-01T00:00:00Z')`)
	mustSQL(t, conn, `INSERT INTO lexemes (text_nfc, gloss_en) VALUES ('מַיִם', 'water')`)
	mustSQL(t, conn, `INSERT INTO cards (user_id, lexeme_id, skill, state, due, stability, difficulty, reps, lapses, suspended, created_at)
		VALUES (1, 1, 'read', 'review', '2026-11-01T12:00:00Z', 1, 5, 1, 0, 0, '2026-11-01T12:00:00Z')`)
	if err := conn.Close(); err != nil {
		t.Fatal(err)
	}

	store, err := Open(context.Background(), path, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	live, err := store.DB()
	if err != nil {
		t.Fatal(err)
	}
	var gloss string
	if err := live.QueryRow(`SELECT gloss_en FROM lexemes WHERE text_nfc = 'מַיִם'`).Scan(&gloss); err != nil {
		t.Fatal(err)
	}
	if gloss != "water" {
		t.Fatalf("gloss = %q", gloss)
	}
	var cards int
	if err := live.QueryRow(`SELECT COUNT(*) FROM cards`).Scan(&cards); err != nil {
		t.Fatal(err)
	}
	if cards != 1 {
		t.Fatalf("cards = %d, want 1", cards)
	}
	if _, err := live.Exec(`SELECT lexeme_id FROM course_words`); err != nil {
		t.Fatal(err)
	}
}
