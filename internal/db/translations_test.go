package db

import (
	"context"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
)

func TestEarlierUnitTranslations(t *testing.T) {
	when := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	s := openStore(t, fixedClock{when})
	ctx := context.Background()

	shared := translationDoc(0, "good morning", "", "hello", "   ")
	own := translationDoc(1, "", "my line", "", "thanks")
	other := translationDoc(0, "not mine", "", "", "")
	draft := translationDoc(0, "still a draft", "", "", "")
	later := translationDoc(4, "later unit", "", "", "")
	same := translationDoc(2, "this position", "", "", "")

	sharedID := putPublished(t, s, shared, "shared")
	ownID := putPublished(t, s, own, "learner:7")
	putPublished(t, s, other, "learner:9")
	if _, err := s.PutUnit(ctx, nil, draft, "shared", "admin"); err != nil {
		t.Fatal(err)
	}
	putPublished(t, s, later, "shared")
	putPublished(t, s, same, "shared")

	got, err := s.EarlierUnitTranslations(ctx, 7, 2)
	if err != nil {
		t.Fatal(err)
	}
	want := []struct {
		TextNFC     string
		Translation string
	}{
		{units.Normalize("שָׁלוֹם בֹּקֶר"), "good morning"},
		{units.Normalize("שָׁלוֹם"), "hello"},
		{units.Normalize("תּוֹדָה"), "my line"},
		{units.Normalize("בֹּקֶר"), "thanks"},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d lines, want %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("line %d = %+v, want %+v", i, got[i], want[i])
		}
	}

	lower, err := s.EarlierUnitTranslations(ctx, 7, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(lower) != 2 || lower[0].Translation != "good morning" || lower[1].Translation != "hello" {
		t.Fatalf("position 1 sees %+v", lower)
	}
	none, err := s.EarlierUnitTranslations(ctx, 7, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(none) != 0 {
		t.Fatalf("position 0 sees %+v", none)
	}
	stranger, err := s.EarlierUnitTranslations(ctx, 4, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(stranger) != 2 || stranger[0].Translation != "good morning" {
		t.Fatalf("another learner sees %+v", stranger)
	}

	if sharedID == 0 || ownID == 0 {
		t.Fatal("ids")
	}
}

func translationDoc(position int, activity, recall, lineA, lineB string) units.Unit {
	return units.Unit{
		Title: "Cafe", Situation: "Ordering", Concept: "Greeting", Position: position,
		Lessons: []units.Lesson{{
			Kind: units.KindLesson,
			Activities: []units.Activity{
				{Type: units.ActivityListenPick, Text: "שָׁלוֹם בֹּקֶר", Translation: activity},
				{Type: units.ActivityRecall, Text: "תּוֹדָה", Translation: recall},
				{Type: units.ActivityDialogue, Lines: []units.DialogueLine{
					{Speaker: units.SpeakerA, Text: "שָׁלוֹם", Translation: lineA},
					{Speaker: units.SpeakerB, Text: "בֹּקֶר", Translation: lineB},
				}},
			},
		}},
	}
}

func putPublished(t *testing.T, s *Store, doc units.Unit, scope string) int64 {
	t.Helper()
	saved, err := s.PutUnit(context.Background(), nil, doc, scope, "admin")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.ExecContext(context.Background(), `UPDATE units SET status = 'published' WHERE id = ?`, saved.UnitID); err != nil {
		t.Fatal(err)
	}
	return saved.UnitID
}
