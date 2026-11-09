package db

import (
	"context"
	"testing"
	"time"
)

func TestTaughtUnitLexemesNewWordsAndCards(t *testing.T) {
	s := openStore(t, nil)
	ctx := context.Background()
	learner, err := s.CreateUser(ctx, "taught-unit@example.com", "sub-taught-unit", "learner")
	if err != nil {
		t.Fatal(err)
	}

	newWord := insertFullLexeme(t, s, "א", "new", "noun", "alef")
	cardOnly := insertFullLexeme(t, s, "ב", "card", "noun", "bet")
	untouched := insertFullLexeme(t, s, "ג", "skip", "noun", "gimel")
	unitID := insertPositionedUnit(t, s, 1, "published", "shared")
	mustExec(t, s, `INSERT INTO lesson_items (unit_id, lesson_ix, lexeme_id, role, skill) VALUES (?, 0, ?, 'new', 'recall')`, unitID, newWord)
	mustExec(t, s, `INSERT INTO lesson_items (unit_id, lesson_ix, lexeme_id, role, skill) VALUES (?, 0, ?, 'recycle', 'recall')`, unitID, cardOnly)
	mustExec(t, s, `INSERT INTO lesson_items (unit_id, lesson_ix, lexeme_id, role, skill) VALUES (?, 0, ?, 'recycle', 'recall')`, unitID, untouched)
	mustExec(t, s, `INSERT INTO cards (user_id, lexeme_id, skill, state, due, stability, difficulty, reps, lapses)
		VALUES (?, ?, 'recall', 'review', ?, 1, 1, 1, 0)`, learner.ID, cardOnly, FormatTime(time.Now().UTC()))

	got, err := s.TaughtUnitLexemes(ctx, unitID, map[int64]struct{}{cardOnly: {}})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].ID != newWord || got[1].ID != cardOnly {
		t.Fatalf("taught unit = %+v", got)
	}
	empty, err := s.TaughtUnitLexemes(ctx, unitID, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(empty) != 1 || empty[0].ID != newWord {
		t.Fatalf("without cards = %+v", empty)
	}
}

func TestTaughtEarlierUnitLexemesFinishedOrCard(t *testing.T) {
	s := openStore(t, nil)
	ctx := context.Background()
	learner, err := s.CreateUser(ctx, "taught-earlier@example.com", "sub-taught-earlier", "learner")
	if err != nil {
		t.Fatal(err)
	}

	finishedWord := insertFullLexeme(t, s, "א", "done", "noun", "alef")
	cardWord := insertFullLexeme(t, s, "ב", "card", "verb", "bet")
	untouched := insertFullLexeme(t, s, "ג", "skip", "noun", "gimel")
	finished := insertPositionedUnit(t, s, 0, "published", "shared")
	unfinished := insertPositionedUnit(t, s, 1, "published", "shared")
	_ = insertPositionedUnit(t, s, 2, "published", "shared")
	linkLexeme(t, s, finished, 0, finishedWord)
	linkLexeme(t, s, unfinished, 0, cardWord)
	linkLexeme(t, s, unfinished, 0, untouched)
	mustExec(t, s, `UPDATE units SET doc = ? WHERE id = ?`,
		`{"title":"T","lessons":[{"kind":"lesson","activities":[{"type":"recall","text":"א"}]}]}`, finished)
	mustExec(t, s, `UPDATE units SET doc = ? WHERE id = ?`,
		`{"title":"T","lessons":[{"kind":"lesson","activities":[{"type":"recall","text":"ב"}]}]}`, unfinished)
	when := FormatTime(time.Now().UTC())
	mustExec(t, s, `INSERT INTO lesson_runs (user_id, unit_id, lesson_ix, started_at, finished_at, score)
		VALUES (?, ?, 0, ?, ?, 1)`, learner.ID, finished, when, when)

	finishedIDs := map[int64]struct{}{finished: {}}
	cards := map[int64]struct{}{cardWord: {}}
	got, err := s.TaughtEarlierUnitLexemes(ctx, learner.ID, 2, finishedIDs, cards)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].ID != finishedWord || got[1].ID != cardWord {
		t.Fatalf("taught earlier = %+v", got)
	}
	none, err := s.TaughtEarlierUnitLexemes(ctx, learner.ID, 2, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(none) != 0 {
		t.Fatalf("no taught earlier still saw %+v", none)
	}
	onlyFinished, err := s.TaughtEarlierUnitLexemes(ctx, learner.ID, 2, finishedIDs, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(onlyFinished) != 1 || onlyFinished[0].ID != finishedWord {
		t.Fatalf("finished only = %+v", onlyFinished)
	}
}

func TestFinishedUnitIDs(t *testing.T) {
	s := openStore(t, nil)
	ctx := context.Background()
	learner, err := s.CreateUser(ctx, "fin@example.com", "sub-fin", "learner")
	if err != nil {
		t.Fatal(err)
	}
	done := insertPositionedUnit(t, s, 0, "published", "shared")
	open := insertPositionedUnit(t, s, 1, "published", "shared")
	mustExec(t, s, `UPDATE units SET doc = ? WHERE id IN (?, ?)`,
		`{"title":"T","lessons":[{"kind":"lesson","activities":[{"type":"story","text":"א"}]}]}`, done, open)
	when := FormatTime(time.Now().UTC())
	mustExec(t, s, `INSERT INTO lesson_runs (user_id, unit_id, lesson_ix, started_at, finished_at, score)
		VALUES (?, ?, 0, ?, ?, 1)`, learner.ID, done, when, when)
	mustExec(t, s, `INSERT INTO lesson_runs (user_id, unit_id, lesson_ix, started_at, score)
		VALUES (?, ?, 0, ?, 0)`, learner.ID, open, when)

	got, err := s.FinishedUnitIDs(ctx, learner.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("finished = %+v", got)
	}
	if _, ok := got[done]; !ok {
		t.Fatalf("missing finished unit: %+v", got)
	}
	if _, ok := got[open]; ok {
		t.Fatalf("unfinished unit listed: %+v", got)
	}
}
