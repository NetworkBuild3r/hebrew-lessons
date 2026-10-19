package db

import (
	"errors"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
)

func TestPublishUnitRevisionAndIdempotency(t *testing.T) {
	when := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	s := openStore(t, fixedClock{when})
	ctx := t.Context()
	const shalom = "שָׁלוֹם"
	const olam = "עוֹלָם"
	shalomID := insertLexeme(t, s, shalom)
	olamID := insertLexeme(t, s, olam)

	firstDoc := units.Unit{
		Title: "Hello", Situation: "street", Concept: "greeting", Position: 4,
		NewWords: []string{shalom},
		Lessons:  []units.Lesson{{Kind: "lesson", Activities: []units.Activity{{Type: "recall", Text: shalom}}}},
	}
	first, err := s.PutUnit(ctx, nil, firstDoc, "shared", "admin-1")
	if err != nil {
		t.Fatal(err)
	}
	missing := 9
	if _, err := s.PublishUnit(ctx, first.UnitID, &missing); !errors.Is(err, ErrRevisionNotFound) {
		t.Fatalf("missing revision %v", err)
	}
	assertUnitRow(t, s, first.UnitID, "Hello", "street", "greeting", "draft", "shared", 4, 1, firstDoc)
	assertUnitCounts(t, s, 1, 1)

	published, err := s.PublishUnit(ctx, first.UnitID, nil)
	if err != nil || published.Unchanged || published.Revision != 1 {
		t.Fatalf("publish %+v %v", published, err)
	}
	assertUnitRow(t, s, first.UnitID, "Hello", "street", "greeting", "published", "shared", 4, 1, firstDoc)
	assertLessonItems(t, s, first.UnitID, []lessonItemRow{
		{lesson: 0, lexemeID: shalomID, role: "new", skill: "recall"},
	})
	again, err := s.PublishUnit(ctx, first.UnitID, &published.Revision)
	if err != nil || !again.Unchanged || again.Revision != 1 {
		t.Fatalf("republish %+v %v", again, err)
	}
	assertUnitCounts(t, s, 1, 1)

	secondDoc := units.Unit{
		Title: "World", Situation: "street", Concept: "greeting", Position: 4,
		NewWords: []string{olam},
		Lessons:  []units.Lesson{{Kind: "lesson", Activities: []units.Activity{{Type: "produce", Text: olam}}}},
	}
	second, err := s.PutUnit(ctx, &first.UnitID, secondDoc, "learner:7", "admin-2")
	if err != nil || second.Revision != 2 {
		t.Fatalf("second %+v %v", second, err)
	}
	restored, err := s.PublishUnit(ctx, first.UnitID, &first.Revision)
	if err != nil || restored.Unchanged || restored.Revision != 1 {
		t.Fatalf("restore %+v %v", restored, err)
	}
	assertUnitRow(t, s, first.UnitID, "Hello", "street", "greeting", "published", "learner:7", 4, 1, firstDoc)
	assertLessonItems(t, s, first.UnitID, []lessonItemRow{
		{lesson: 0, lexemeID: shalomID, role: "new", skill: "recall"},
	})
	assertRevision(t, s, first.UnitID, 2, mustJSON(t, secondDoc), "admin-2", FormatTime(when))

	thirdDoc := secondDoc
	thirdDoc.Title = "Later"
	third, err := s.PutUnit(ctx, &first.UnitID, thirdDoc, "learner:7", "admin-3")
	if err != nil || third.Unchanged || third.Revision != 3 {
		t.Fatalf("after rewind %+v %v", third, err)
	}
	assertUnitRow(t, s, first.UnitID, "Later", "street", "greeting", "draft", "learner:7", 4, 3, thirdDoc)
	assertLessonItems(t, s, first.UnitID, []lessonItemRow{
		{lesson: 0, lexemeID: olamID, role: "new", skill: "produce"},
	})
	assertUnitCounts(t, s, 1, 3)
}
