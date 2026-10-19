package db

import (
	"context"
	"testing"
)

func TestEarlierUnitLexemes(t *testing.T) {
	s := openStore(t, nil)
	ctx := context.Background()
	const learner = int64(7)

	alef := insertFullLexeme(t, s, "א", "alef", "noun", "alef")
	bet := insertFullLexeme(t, s, "ב", "bet", "verb", "bet")
	gimel := insertFullLexeme(t, s, "ג", "gimel", "noun", "gimel")
	dalet := insertFullLexeme(t, s, "ד", "draft", "noun", "draft")
	he := insertFullLexeme(t, s, "ה", "other", "noun", "other")
	vav := insertFullLexeme(t, s, "ו", "current", "noun", "vav")
	zayin := insertFullLexeme(t, s, "ז", "later", "noun", "zayin")

	sharedEarly := insertPositionedUnit(t, s, 0, "published", "shared")
	learnerEarly := insertPositionedUnit(t, s, 1, "published", "learner:7")
	draft := insertPositionedUnit(t, s, 0, "draft", "shared")
	other := insertPositionedUnit(t, s, 0, "published", "learner:8")
	current := insertPositionedUnit(t, s, 2, "published", "shared")
	later := insertPositionedUnit(t, s, 3, "published", "shared")

	linkLexeme(t, s, sharedEarly, 0, alef)
	linkLexeme(t, s, sharedEarly, 1, alef)
	linkLexeme(t, s, learnerEarly, 0, bet)
	linkLexeme(t, s, learnerEarly, 0, gimel)
	linkLexeme(t, s, draft, 0, dalet)
	linkLexeme(t, s, other, 0, he)
	linkLexeme(t, s, current, 0, vav)
	linkLexeme(t, s, later, 0, zayin)

	got, err := s.EarlierUnitLexemes(ctx, learner, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 || got[0].ID != alef || got[1].ID != bet || got[2].ID != gimel {
		t.Fatalf("earlier = %+v", got)
	}
	if got[0].Text != "א" || got[0].Gloss != "alef" || got[0].POS != "noun" || got[0].Translit != "alef" || got[0].Gender != "m" {
		t.Fatalf("alef view = %+v", got[0])
	}
	if got[1].POS != "verb" || got[2].POS != "noun" {
		t.Fatalf("pos = %s %s", got[1].POS, got[2].POS)
	}
	none, err := s.EarlierUnitLexemes(ctx, learner, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(none) != 0 {
		t.Fatalf("position 0 still saw %+v", none)
	}
}

func insertFullLexeme(t *testing.T, s *Store, text, gloss, pos, translit string) int64 {
	t.Helper()
	res := mustExec(t, s, `INSERT INTO lexemes (text_nfc, gloss_en, pos, translit, gender) VALUES (?, ?, ?, ?, 'm')`, text, gloss, pos, translit)
	id, err := res.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func insertPositionedUnit(t *testing.T, s *Store, position int, status, scope string) int64 {
	t.Helper()
	res := mustExec(t, s, `INSERT INTO units (position, title, situation, concept, status, rev, doc, scope)
		VALUES (?, 'T', 's', 'c', ?, 1, '{"title":"T"}', ?)`, position, status, scope)
	id, err := res.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func linkLexeme(t *testing.T, s *Store, unitID int64, lessonIx int, lexemeID int64) {
	t.Helper()
	mustExec(t, s, `INSERT INTO lesson_items (unit_id, lesson_ix, lexeme_id, role, skill) VALUES (?, ?, ?, 'new', 'recall')`, unitID, lessonIx, lexemeID)
}
