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

	translit := "ani"
	_, _, err := s.UpsertLexemes(ctx, []lexicon.Input{{TextNFC: "אֲנִי", Gloss: "I", Translit: &translit}})
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
	saved, _, err := s.UpsertLexemes(ctx, []lexicon.Input{{TextNFC: "אֲנִי", Gloss: "I", Translit: &translit}})
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

func TestUpsertLexemesRootAndBinyan(t *testing.T) {
	s := openStore(t, fixedClock{time.Date(2026, 10, 23, 0, 0, 0, 0, time.UTC)})
	ctx := context.Background()
	tr := "shama"
	root, binyan := "שׁ-מ-ע", "paal"
	root2, binyan2 := "כ־ת־ב", "Paal"
	first, _, err := s.UpsertLexemes(ctx, []lexicon.Input{
		{TextNFC: "שָׁמַע", Gloss: "he heard", Translit: &tr, Root: &root, Binyan: &binyan},
		{TextNFC: "כָּתַב", Gloss: "he wrote", Translit: &tr, Root: &root2, Binyan: &binyan2},
	})
	if err != nil || len(first) != 2 {
		t.Fatalf("create %+v %v", first, err)
	}
	stored := readStoredLexeme(t, s, first[0].ID)
	if stored.root != "שׁ־מ־ע" || stored.binyan != "paal" {
		t.Fatalf("normalized %+v", stored)
	}

	second, _, err := s.UpsertLexemes(ctx, []lexicon.Input{{TextNFC: "שָׁמַע", Gloss: "heard"}})
	if err != nil || second[0].Status != lexicon.StatusUpdated {
		t.Fatalf("omit %+v %v", second, err)
	}
	if got := readStoredLexeme(t, s, first[0].ID); got.root != "שׁ־מ־ע" || got.binyan != "paal" {
		t.Fatalf("omitted fields changed %+v", got)
	}

	empty := ""
	if _, _, err := s.UpsertLexemes(ctx, []lexicon.Input{{TextNFC: "שָׁמַע", Gloss: "heard", Root: &empty, Binyan: &empty}}); err != nil {
		t.Fatal(err)
	}
	if got := readStoredLexeme(t, s, first[0].ID); got.root != "" || got.binyan != "" {
		t.Fatalf("cleared %+v", got)
	}

	byRoot, err := s.LexemesByRoot(ctx, "כ־ת־ב")
	if err != nil || len(byRoot) != 1 || byRoot[0].Text != "כָּתַב" || byRoot[0].Binyan != "paal" {
		t.Fatalf("by root %+v %v", byRoot, err)
	}
	counts, err := s.RootCounts(ctx)
	if err != nil || counts["כ־ת־ב"] != 1 || counts["שׁ־מ־ע"] != 0 {
		t.Fatalf("counts %+v %v", counts, err)
	}

	bad := "abc"
	if _, _, err := s.UpsertLexemes(ctx, []lexicon.Input{{TextNFC: "שָׁמַע", Gloss: "heard", Root: &bad}}); err == nil {
		t.Fatal("a non-Hebrew root was accepted")
	}
	if got := readStoredLexeme(t, s, first[0].ID); got.gloss != "heard" {
		t.Fatalf("rejected batch wrote %+v", got)
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
		SELECT id, bare, gloss_en, pos, gender, plural, translit, root, binyan, verified
		FROM lexemes WHERE id = ?`, id,
	).Scan(&row.id, &row.bare, &row.gloss, &row.pos, &gender, &plural, &translit, &row.root, &row.binyan, &verified)
	if err != nil {
		t.Fatal(err)
	}
	row.gender = gender.String
	row.plural = plural.String
	row.translit = translit.String
	row.verified = verified != 0
	return row
}
