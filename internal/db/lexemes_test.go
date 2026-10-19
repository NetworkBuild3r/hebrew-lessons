package db

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/lexicon"
)

func TestUpsertLexemesKeepsOmittedFields(t *testing.T) {
	s := openStore(t, fixedClock{time.Date(2026, 10, 18, 0, 0, 0, 0, time.UTC)})
	ctx := context.Background()
	pos, gender, plural, translit := "noun", "f", "מילים", "milim"
	yes := true
	text := "מִלָּה"
	first, _, err := s.UpsertLexemes(ctx, []lexicon.Input{{
		TextNFC: text, Gloss: "word", POS: &pos, Gender: &gender, Plural: &plural, Translit: &translit, Verified: &yes,
	}})
	if err != nil || len(first) != 1 || first[0].Status != lexicon.StatusCreated {
		t.Fatalf("create %+v %v", first, err)
	}

	second, _, err := s.UpsertLexemes(ctx, []lexicon.Input{{TextNFC: text, Gloss: "a word"}})
	if err != nil || second[0].Status != lexicon.StatusUpdated || second[0].ID != first[0].ID {
		t.Fatalf("omit update %+v %v", second, err)
	}
	got := readStoredLexeme(t, s, first[0].ID)
	if got.pos != "noun" || got.gender != "f" || got.plural != "מילים" || got.translit != "milim" || !got.verified || got.gloss != "a word" {
		t.Fatalf("kept %+v", got)
	}

	third, _, err := s.UpsertLexemes(ctx, []lexicon.Input{{TextNFC: text, Gloss: "a word"}})
	if err != nil || third[0].Status != lexicon.StatusUnchanged {
		t.Fatalf("omit verified %+v %v", third, err)
	}
	if again := readStoredLexeme(t, s, first[0].ID); again != got {
		t.Fatalf("verified changed %+v", again)
	}

	no := false
	empty := ""
	fourth, _, err := s.UpsertLexemes(ctx, []lexicon.Input{{
		TextNFC: text, Gloss: "a word", POS: &empty, Gender: &empty, Plural: &empty, Translit: &empty, Verified: &no,
	}})
	if err != nil || fourth[0].Status != lexicon.StatusUpdated {
		t.Fatalf("clear %+v %v", fourth, err)
	}
	cleared := readStoredLexeme(t, s, first[0].ID)
	if cleared.pos != "" || cleared.gender != "" || cleared.plural != "" || cleared.translit != "" || cleared.verified {
		t.Fatalf("explicit empty %+v", cleared)
	}
}

func TestUpsertLexemesRebuildFailureRollsBack(t *testing.T) {
	s := openStore(t, fixedClock{time.Date(2026, 10, 18, 0, 0, 0, 0, time.UTC)})
	ctx := context.Background()
	insertUnit(t, s, "draft")
	mustExec(t, s, `UPDATE units SET doc = '[]'`)

	_, _, err := s.UpsertLexemes(ctx, []lexicon.Input{{TextNFC: "אֲנִי", Gloss: "I"}})
	if err == nil {
		t.Fatal("want rebuild error")
	}
	var bad *lexicon.Invalid
	if errors.As(err, &bad) {
		t.Fatalf("validation %v", err)
	}
	if n := count(t, s, `SELECT COUNT(*) FROM lexemes`); n != 0 {
		t.Fatalf("lexemes %d", n)
	}

	mustExec(t, s, `UPDATE units SET doc = '{"title":"t"}'`)
	saved, _, err := s.UpsertLexemes(ctx, []lexicon.Input{{TextNFC: "אֲנִי", Gloss: "I"}})
	if err != nil || len(saved) != 1 || saved[0].Status != lexicon.StatusCreated {
		t.Fatalf("after rollback %+v %v", saved, err)
	}
	if n := count(t, s, `SELECT COUNT(*) FROM lexemes`); n != 1 {
		t.Fatalf("lexemes %d", n)
	}
}

func TestUpsertLexemesOverCapWritesNothing(t *testing.T) {
	s := openStore(t, fixedClock{time.Date(2026, 10, 18, 0, 0, 0, 0, time.UTC)})
	entries := make([]lexicon.Input, lexicon.MaxBatch+1)
	for i := range entries {
		entries[i] = lexicon.Input{TextNFC: "א", Gloss: "a"}
	}
	_, _, err := s.UpsertLexemes(context.Background(), entries)
	var bad *lexicon.Invalid
	if !errors.As(err, &bad) || bad == nil || bad.Error() == "" {
		t.Fatalf("err %v", err)
	}
	if n := count(t, s, `SELECT COUNT(*) FROM lexemes`); n != 0 {
		t.Fatalf("lexemes %d", n)
	}
}

func readStoredLexeme(t *testing.T, s *Store, id int64) storedLexeme {
	t.Helper()
	conn, err := s.DB()
	if err != nil {
		t.Fatal(err)
	}
	var row storedLexeme
	var gender, plural, translit sql.NullString
	var verified int
	err = conn.QueryRowContext(context.Background(), `
		SELECT id, bare, gloss_en, pos, gender, plural, translit, verified
		FROM lexemes WHERE id = ?`, id,
	).Scan(&row.id, &row.bare, &row.gloss, &row.pos, &gender, &plural, &translit, &verified)
	if err != nil {
		t.Fatal(err)
	}
	row.gender = gender.String
	row.plural = plural.String
	row.translit = translit.String
	row.verified = verified != 0
	return row
}
