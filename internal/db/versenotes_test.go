package db

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
)

func TestVerseNotesStayDraftUntilChecked(t *testing.T) {
	when := time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)
	s := openStore(t, fixedClock{when})
	ctx := context.Background()
	checker, err := s.InsertUser(ctx, "checker@example.com", "sub-check", "admin")
	if err != nil {
		t.Fatal(err)
	}
	seedTanakh(t, s, "isaiah", 6, 1, "\u05e7\u05b8\u05d3\u05d5\u05b9\u05e9\u05c1", "Holy.")
	body := "The vision is in the temple."
	if err := s.UpsertVerseNotes(ctx, "isaiah", 6, 1, []VerseNoteInput{
		{Layer: LayerRoots, Body: body, DraftedBy: checker},
	}); err != nil {
		t.Fatal(err)
	}
	notes, err := s.VerseNotes(ctx, "isaiah", 6, 1, false)
	if err != nil || len(notes) != 1 || notes[0].Status != "draft" || notes[0].CheckedBy != 0 {
		t.Fatalf("notes = %+v %v", notes, err)
	}
	if err := s.CheckVerseNote(ctx, "isaiah", 6, 1, LayerRoots, checker); err != nil {
		t.Fatal(err)
	}
	checked, err := s.VerseNotes(ctx, "isaiah", 6, 1, true)
	if err != nil || len(checked) != 1 || checked[0].CheckedBy != checker {
		t.Fatalf("checked = %+v %v", checked, err)
	}
	if err := s.UpsertVerseNotes(ctx, "isaiah", 6, 1, []VerseNoteInput{
		{Layer: LayerRoots, Body: "The vision is in the temple, again."},
	}); err != nil {
		t.Fatal(err)
	}
	again, err := s.VerseNotes(ctx, "isaiah", 6, 1, false)
	if err != nil || len(again) != 1 || again[0].Status != "draft" || again[0].CheckedBy != 0 {
		t.Fatalf("rewritten = %+v %v", again, err)
	}
	only, err := s.VerseNotes(ctx, "isaiah", 6, 1, true)
	if err != nil || len(only) != 0 {
		t.Fatalf("checkedOnly = %+v %v", only, err)
	}
}

func TestVerseNoteRefusesAMissingVerse(t *testing.T) {
	s := openStore(t, fixedClock{time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)})
	err := s.UpsertVerseNotes(context.Background(), "isaiah", 99, 1, []VerseNoteInput{
		{Layer: LayerBridge, Body: "A note."},
	})
	var missing MissingVerseError
	if !errors.As(err, &missing) {
		t.Fatalf("err = %v", err)
	}
	n, err := s.VerseNotes(context.Background(), "isaiah", 99, 1, false)
	if err != nil || len(n) != 0 {
		t.Fatalf("wrote %+v %v", n, err)
	}
}

func TestVerseGlossCheckedOnly(t *testing.T) {
	s := openStore(t, fixedClock{time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)})
	ctx := context.Background()
	word := units.Normalize("\u05e7\u05b8\u05d3\u05d5\u05b9\u05e9\u05c1")
	seedTanakh(t, s, "isaiah", 6, 3, word, "Holy.")
	root := units.Normalize("\u05e7\u05d3\u05e9")
	if err := s.UpsertVerseGlosses(ctx, "isaiah", 6, 3, []VerseGlossInput{
		{Position: 0, WordHe: word, GlossEn: "holy", Root: root},
		{Position: 1, WordHe: word, GlossEn: "draft gloss", Root: root},
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.CheckVerseGloss(ctx, "isaiah", 6, 3, 0); err != nil {
		t.Fatal(err)
	}
	all, err := s.VerseGlosses(ctx, "isaiah", 6, 3, false)
	if err != nil || len(all) != 2 || all[0].Status != "checked" || all[1].Status != "draft" {
		t.Fatalf("glosses = %+v %v", all, err)
	}
	shown, err := s.VerseGlosses(ctx, "isaiah", 6, 3, true)
	if err != nil || len(shown) != 1 || shown[0].GlossEn != "holy" || shown[0].Root != root {
		t.Fatalf("checked = %+v %v", shown, err)
	}
	if err := s.UpsertVerseGlosses(ctx, "isaiah", 6, 3, []VerseGlossInput{
		{Position: 0, WordHe: word, GlossEn: "set apart", Root: root},
	}); err != nil {
		t.Fatal(err)
	}
	rewritten, err := s.VerseGlosses(ctx, "isaiah", 6, 3, true)
	if err != nil || len(rewritten) != 0 {
		t.Fatalf("a changed gloss stayed checked: %+v %v", rewritten, err)
	}
}

func TestVerseNoteRefusesMarkup(t *testing.T) {
	s := openStore(t, fixedClock{time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)})
	ctx := context.Background()
	seedTanakh(t, s, "isaiah", 1, 1, "\u05d7\u05b2\u05d6\u05d5\u05b9\u05df", "The vision.")
	err := s.UpsertVerseNotes(ctx, "isaiah", 1, 1, []VerseNoteInput{
		{Layer: LayerManner, Body: "See <script>bad</script>."},
	})
	if err == nil || !strings.Contains(err.Error(), "verse note") {
		t.Fatalf("err = %v", err)
	}
	n, qerr := s.VerseNotes(ctx, "isaiah", 1, 1, false)
	if qerr != nil || len(n) != 0 {
		t.Fatalf("wrote %+v %v", n, qerr)
	}
}

func seedTanakh(t *testing.T, s *Store, book string, chapter, verse int, he, en string) {
	t.Helper()
	if _, err := s.ExecContext(context.Background(), `
		INSERT INTO tanakh_verses (book, chapter, verse, he, he_plain, en, source_version, license)
		VALUES (?, ?, ?, ?, ?, ?, 'test', 'Public Domain')`,
		book, chapter, verse, he, units.Plain(he), en); err != nil {
		t.Fatal(err)
	}
}
